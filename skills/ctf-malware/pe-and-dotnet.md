# PE, .NET, and Binary Malware Analysis

## Table of Contents
- [PE Analysis](#pe-analysis)
- [Sandbox Evasion Checks](#sandbox-evasion-checks)
- [Malware Configuration Extraction](#malware-configuration-extraction)
- [.NET DNS-based C2](#net-dns-based-c2)
- [.NET Malware Analysis (C2 Extraction)](#net-malware-analysis-c2-extraction)
- [PyInstaller + PyArmor Unpacking](#pyinstaller--pyarmor-unpacking)

---

## PE Analysis

```bash
peframe malware.exe      # Quick triage
pe-sieve                 # Runtime analysis
pestudio                 # Static analysis (Windows)
```

## Sandbox Evasion Checks

Look for:
- VM detection (VMware, VirtualBox artifacts)
- Debugger detection (IsDebuggerPresent)
- Timing checks (sleep acceleration)
- Environment checks (username, computername)
- File/registry checks for analysis tools

## Malware Configuration Extraction

**Common storage locations:**
- .data section (hardcoded)
- Resources (PE resources, .NET resources)
- Registry keys written at install
- Encrypted config file dropped to disk

**Extraction tools:**
```bash
# PE resources
wrestool -x -t 10 malware.exe -o config.bin

# .NET resources
monodis --mresources malware.exe

# Strings in .rdata/.data
objdump -s -j .rdata malware.exe
```

## .NET DNS-based C2

**Pattern:** Deobfuscated .NET malware with DNS C2

**Analysis with dnSpy:**
1. Find network functions (TcpClient, DnsClient, etc.)
2. Identify encoding/encryption wrappers
3. Look for command dispatch (switch on opcode)

**AsmResolver for programmatic analysis:**
```csharp
using AsmResolver.DotNet;
var module = ModuleDefinition.FromFile("malware.dll");
foreach (var type in module.GetAllTypes()) {
    foreach (var method in type.Methods) {
        // Analyze method body
    }
}
```

## .NET Malware Analysis (C2 Extraction)

**Tools:** ILSpy, dnSpy, dotPeek

**LimeRAT C2 extraction (Whisper Of The Pain):**
1. Open .NET binary in dnSpy
2. Find configuration class with Base64 encoded string
3. Identify decryption method (typically AES-256-ECB with derived key)
4. Key derivation: MD5 of hardcoded string -> first 15 + full 16 bytes + null = 32-byte key
5. Decrypt: Base64 decode -> AES-ECB decrypt -> reveals C2 IP/domain

```python
from Crypto.Cipher import AES
import hashlib, base64

key_source = '${8\',`d0}n,~@J;oZ"9a'
md5 = hashlib.md5(key_source.encode()).hexdigest()
# Key = first 15 bytes of MD5 + full 16 bytes + null (64 hex chars -> 32 bytes)
key = bytes.fromhex(md5[:30] + md5 + '00')[:32]

cipher = AES.new(key, AES.MODE_ECB)
plaintext = cipher.decrypt(base64.b64decode(encrypted_b64))
```

## PyInstaller + PyArmor Unpacking

```bash
# Step 1: Extract PyInstaller archive
python pyinstxtractor.py malware.exe
# Look for main .pyc file in extracted directory

# Step 2: If PyArmor-protected, use unpacker
# github.com/Svenskithesource/PyArmor-Unpacker
# Three methods available; choose based on PyArmor version

# Step 3: Clean up deobfuscated source
# Remove fake/dead-code functions (confusion code)
# Identify core encryption/exfiltration logic
```
