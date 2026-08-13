# Reverse Engineering Field Notes

Detailed quick notes that support [`SKILL.md`](SKILL.md). Read this file after triage, not before.

## Table of Contents

- [Binary Types](#binary-types)
  - [Python .pyc](#python-pyc)
  - [WASM](#wasm)
  - [Android APK](#android-apk)
  - [Flutter APK (Dart AOT)](#flutter-apk-dart-aot)
  - [.NET](#net)
  - [Packed (UPX)](#packed-upx)
  - [Tauri Packed Desktop Apps](#tauri-packed-desktop-apps)
- [Anti-Debugging Bypass](#anti-debugging-bypass)
- [Specialized Patterns](#specialized-patterns)
  - [S-Box / Keystream Patterns](#s-box--keystream-patterns)
  - [Custom VM Analysis](#custom-vm-analysis)
  - [Python Bytecode Reversing](#python-bytecode-reversing)
  - [Signal-Based Binary Exploration](#signal-based-binary-exploration)
  - [Malware Anti-Analysis Bypass via Patching](#malware-anti-analysis-bypass-via-patching)
  - [Expected Values Tables](#expected-values-tables)
  - [x86-64 Gotchas](#x86-64-gotchas)
  - [Iterative Solver Pattern](#iterative-solver-pattern)
  - [Unicorn Emulation (Complex State)](#unicorn-emulation-complex-state)
  - [Multi-Stage Shellcode Loaders](#multi-stage-shellcode-loaders)
  - [Timing Side-Channel Attack](#timing-side-channel-attack)
  - [Godot Game Asset Extraction](#godot-game-asset-extraction)
  - [Roblox Place File Analysis](#roblox-place-file-analysis)
  - [Unstripped Binary Information Leaks](#unstripped-binary-information-leaks)
  - [Custom Mangle Function Reversing](#custom-mangle-function-reversing)
  - [Rust serde_json Schema Recovery](#rust-serde_json-schema-recovery)
  - [Position-Based Transformation Reversing](#position-based-transformation-reversing)
  - [Hex-Encoded String Comparison](#hex-encoded-string-comparison)
- [CTF Case Notes](#ctf-case-notes)
  - [Embedded ZIP + XOR License Decryption](#embedded-zip--xor-license-decryption)
  - [Stack String Deobfuscation (.rodata XOR Blob)](#stack-string-deobfuscation-rodata-xor-blob)
  - [Prefix Hash Brute-Force](#prefix-hash-brute-force)
  - [Mathematical Convergence Bitmap](#mathematical-convergence-bitmap)
  - [RISC-V Binary Analysis](#risc-v-binary-analysis)
  - [Sprague-Grundy Game Theory Binary](#sprague-grundy-game-theory-binary)
  - [Kernel Module Maze Solving](#kernel-module-maze-solving)
  - [Multi-Threaded VM with Channels](#multi-threaded-vm-with-channels)
  - [CVP/LLL Lattice for Constrained Integer Validation](#cvplll-lattice-for-constrained-integer-validation)
  - [Decision Tree Function Obfuscation](#decision-tree-function-obfuscation)
  - [Android JNI RegisterNatives Obfuscation](#android-jni-registernatives-obfuscation)
  - [Multi-Layer Self-Decrypting Binary](#multi-layer-self-decrypting-binary)
  - [GLSL Shader VM with Self-Modifying Code](#glsl-shader-vm-with-self-modifying-code)
  - [GF(2^8) Gaussian Elimination for Flag Recovery](#gf28-gaussian-elimination-for-flag-recovery)
  - [Z3 for Single-Line Python Boolean Circuit](#z3-for-single-line-python-boolean-circuit)
  - [Sliding Window Popcount Differential Propagation](#sliding-window-popcount-differential-propagation)
  - [Ruby/Perl Polyglot Constraint Satisfaction](#rubyperl-polyglot-constraint-satisfaction)
  - [Verilog/Hardware RE](#veriloghardware-re)
  - [Custom binfmt Kernel Module with RC4 Flat Binaries](#custom-binfmt-kernel-module-with-rc4-flat-binaries)
  - [Hash-Resolved Imports / No-Import Ransomware](#hash-resolved-imports--no-import-ransomware)
  - [ELF Section Header Corruption for Anti-Analysis](#elf-section-header-corruption-for-anti-analysis)
  - [Brainfuck Character-by-Character Static Analysis](#brainfuck-character-by-character-static-analysis)
  - [Brainfuck Side-Channel via Read Count Oracle](#brainfuck-side-channel-via-read-count-oracle)
  - [Brainfuck Comparison Idiom Detection](#brainfuck-comparison-idiom-detection)
  - [Backdoored Shared Library Detection](#backdoored-shared-library-detection)
  - [Go Binary Reversing](#go-binary-reversing)
  - [Go Binary UUID Patching for C2 Enumeration](#go-binary-uuid-patching-for-c2-enumeration)
  - [D Language Binary Reversing](#d-language-binary-reversing)
  - [Rust Binary Reversing](#rust-binary-reversing)
  - [Frida Dynamic Instrumentation](#frida-dynamic-instrumentation)
  - [Frida Firebase Cloud Functions Bypass](#frida-firebase-cloud-functions-bypass)
  - [angr Symbolic Execution](#angr-symbolic-execution)
  - [Qiling Emulation](#qiling-emulation)
  - [VMProtect / Themida Analysis](#vmprotect--themida-analysis)
  - [Binary Diffing](#binary-diffing)
  - [Advanced GDB (pwndbg, rr)](#advanced-gdb-pwndbg-rr)
  - [macOS / iOS Reversing](#macos--ios-reversing)
  - [Embedded / IoT Firmware RE](#embedded--iot-firmware-re)
  - [Kernel Driver Reversing](#kernel-driver-reversing)
  - [Game Engine Reversing](#game-engine-reversing)
  - [Swift / Kotlin Binary Reversing](#swift--kotlin-binary-reversing)
  - [INT3 Patch + Coredump Brute-Force Oracle](#int3-patch--coredump-brute-force-oracle)
  - [Signal Handler Chain + LD_PRELOAD Oracle](#signal-handler-chain--ld_preload-oracle)
  - [Font Ligature Exploitation](#font-ligature-exploitation)
  - [Instruction Counter as Cryptographic State](#instruction-counter-as-cryptographic-state)
  - [Burrows-Wheeler Transform Inversion](#burrows-wheeler-transform-inversion)
  - [FRACTRAN Program Inversion](#fractran-program-inversion)
  - [Opcode-Only Trace Reconstruction](#opcode-only-trace-reconstruction)
  - [Thread Race Signed Integer Overflow](#thread-race-signed-integer-overflow)
  - [ESP32/Xtensa Firmware Reversing](#esp32xtensa-firmware-reversing)
  - [Custom VM Bytecode Lifting to LLVM IR](#custom-vm-bytecode-lifting-to-llvm-ir)
  - [SIGFPE Signal Handler Side-Channel](#sigfpe-signal-handler-side-channel)
  - [Batch Crackme Automation via objdump](#batch-crackme-automation-via-objdump)
  - [Android DEX Runtime Bytecode Patching](#android-dex-runtime-bytecode-patching)
  - [Fork + Pipe + Dead Branch Anti-Analysis](#fork--pipe--dead-branch-anti-analysis)

## Binary Types

### Python .pyc
Disassemble with `marshal.load()` + `dis.dis()`. Header: 8 bytes (2.x), 12 (3.0-3.6), 16 (3.7+). See [languages.md](languages.md#python-bytecode-reversing-disdis-output).

### WASM
```bash
wasm2c checker.wasm -o checker.c
gcc -O3 checker.c wasm-rt-impl.c -o checker

# WASM patching (game challenges):
wasm2wat main.wasm -o main.wat    # Binary → text
# Edit WAT: flip comparisons, change constants
wat2wasm main.wat -o patched.wasm # Text → binary
```

**WASM game patching (Tac Tic Toe, Pragyan 2026):** If proof generation is independent of move quality, patch minimax (flip `i64.lt_s` → `i64.gt_s`, change bestScore sign) to make AI play badly while proofs remain valid. Invoke `/ctf-misc` for full game patching patterns (games-and-vms).

### Android APK
`apktool d app.apk -o decoded/` for resources; `jadx app.apk` for Java decompilation. Check `decoded/res/values/strings.xml` for flags. See [tools.md](tools.md#android-apk).

### Flutter APK (Dart AOT)
If `lib/arm64-v8a/libapp.so` + `libflutter.so` present, use [Blutter](https://github.com/worawit/blutter): `python3 blutter.py path/to/app/lib/arm64-v8a out_dir`. Outputs reconstructed Dart symbols + Frida script. See [tools.md](tools.md#flutter-apk-blutter).

### .NET
- dnSpy - debugging + decompilation
- ILSpy - decompiler

### Packed (UPX)
```bash
upx -d packed -o unpacked
```
If unpacking fails, inspect UPX metadata first: verify UPX section names, header fields, and version markers are intact. If metadata looks tampered or uncertain, review UPX source on GitHub to identify likely modification points.

### Tauri Packed Desktop Apps
Tauri embeds Brotli-compressed frontend assets in the executable. Find `index.html` xrefs to locate asset index table, dump blobs, Brotli decompress. Reference: `tauri-codegen/src/embedded_assets.rs`.

## Anti-Debugging Bypass

Common checks:
- `IsDebuggerPresent()` / PEB.BeingDebugged / NtQueryInformationProcess (Windows)
- `ptrace(PTRACE_TRACEME)` / `/proc/self/status` TracerPid (Linux)
- TLS callbacks (run before main — check PE TLS Directory)
- Timing checks (`rdtsc`, `clock_gettime`, `GetTickCount`)
- Hardware breakpoint detection (DR0-DR3 via GetThreadContext)
- INT3 scanning / code self-hashing (CRC over .text section)
- Signal-based: SIGTRAP handler, SIGALRM timeout, SIGSEGV for real logic
- Frida/DBI detection: `/proc/self/maps` scan, port 27042, inline hook checks

Bypass: Set breakpoint at check, modify register to bypass conditional. pwntools patch: `elf.asm(elf.symbols.ptrace, 'ret')` to replace function with immediate return. See [patterns.md](patterns.md#pwntools-binary-patching-crypto-cat).

For comprehensive anti-analysis techniques and bypasses (30+ methods with code), see [anti-analysis.md](anti-analysis.md).

## Specialized Patterns

### S-Box / Keystream Patterns
**Xorshift32:** Shifts 13, 17, 5  
**Xorshift64:** Shifts 12, 25, 27  
**Magic constants:** `0x2545f4914f6cdd1d`, `0x9e3779b97f4a7c15`

### Custom VM Analysis
1. Identify structure: registers, memory, IP
2. Reverse `executeIns` for opcode meanings
3. Write disassembler mapping opcodes to mnemonics
4. Often easier to bruteforce than fully reverse
5. Look for the bytecode file loaded via command-line arg

See [patterns.md](patterns.md#custom-vm-reversing) for VM workflow, opcode tables, and state machine BFS.

**Sequential key-chain brute-force:** When a VM validates input in small blocks (e.g., 3 bytes = 2^24 candidates) with each block's output key feeding the next, brute-force each block sequentially with OpenMP parallelization. Compile solver with `gcc -O3 -march=native -fopenmp`. See [patterns-ctf-3.md](patterns-ctf-3.md#vm-sequential-key-chain-brute-force-midnight-flag-2026).

### Python Bytecode Reversing
XOR flag checkers with interleaved even/odd tables are common. See [languages.md](languages.md#python-bytecode-reversing-disdis-output) for bytecode analysis tips and reversing patterns.

### Signal-Based Binary Exploration
Binary uses UNIX signals as binary tree navigation; hook `sigaction` via `LD_PRELOAD`, DFS by sending signals. See [patterns.md](patterns.md#signal-based-binary-exploration).

### Malware Anti-Analysis Bypass via Patching
Flip `JNZ`/`JZ` (0x75/0x74), change sleep values, patch environment checks in Ghidra (`Ctrl+Shift+G`). See [patterns-runtime.md](patterns-runtime.md#malware-anti-analysis-bypass-via-patching).

### Expected Values Tables
Locate with `objdump -s -j .rodata binary | less` — look near comparison instructions, size matches flag length.

### x86-64 Gotchas
Sign extension and 32-bit truncation pitfalls. See [patterns.md](patterns.md#x86-64-gotchas) for details and code examples.

### Iterative Solver Pattern
Try each byte (0-255) per position, match against expected output. **Uniform transform shortcut:** if one input byte only changes one output byte, build 0..255 mapping then invert. See [patterns.md](patterns.md) for full implementation.

### Unicorn Emulation (Complex State)
`from unicorn import *` -- map segments, set up stack, hook to trace. **Mixed-mode pitfall:** 64-bit stub jumping to 32-bit via `retf` requires switching to UC_MODE_32 and copying GPRs + EFLAGS + XMM regs. See [tools.md](tools.md#unicorn-emulation).

### Multi-Stage Shellcode Loaders
Nested shellcode with XOR decode loops; break at `call rax`, bypass ptrace with `set $rax=0`, extract flag from `mov` instructions. See [patterns-runtime.md](patterns-runtime.md#multi-stage-shellcode-loaders).

### Timing Side-Channel Attack
Validation time varies per correct character; measure elapsed time per candidate to recover flag byte-by-byte. See [patterns-runtime.md](patterns-runtime.md#timing-side-channel-attack).

### Godot Game Asset Extraction
Use KeyDot to extract encryption key from executable, then gdsdecomp to extract .pck package. See [languages-platforms.md](languages-platforms.md#godot-game-asset-extraction).

### Roblox Place File Analysis
Query Asset Delivery API for version history; parse `.rbxlbin` chunks (INST/PROP/PRNT) to diff script sources across versions. See [languages-platforms.md](languages-platforms.md#roblox-place-file-analysis).

### Unstripped Binary Information Leaks
**Pattern:** Debug info and file paths leak author identity. Quick checks: `strings binary | grep "/home/"` (home dirs), `file binary` (stripped?), `readelf -S binary | grep debug` (debug sections).

### Custom Mangle Function Reversing
Binary mangles input 2 bytes at a time with running state; extract target from `.rodata`, write inverse function. See [patterns.md](patterns.md#custom-mangle-function-reversing).

### Rust serde_json Schema Recovery
Disassemble serde `Visitor` implementations to recover expected JSON schema; field names in order reveal flag. See [languages-platforms.md](languages-platforms.md#rust-serde_json-schema-recovery).

### Position-Based Transformation Reversing
Binary adds/subtracts position index; reverse by undoing per-index offset. See [patterns.md](patterns.md#position-based-transformation-reversing).

### Hex-Encoded String Comparison
Input converted to hex, compared against constant. Decode with `xxd -r -p`. See [patterns.md](patterns.md#hex-encoded-string-comparison).

## CTF Case Notes

### Embedded ZIP + XOR License Decryption
Binary with named symbols (`EMBEDDED_ZIP`, `ENCRYPTED_MESSAGE`) in `.rodata` → extract ZIP containing license, XOR encrypted message with license bytes to recover flag. No execution needed. See [patterns-ctf-2.md](patterns-ctf-2.md#embedded-zip--xor-license-decryption-metactf-2026).

### Stack String Deobfuscation (.rodata XOR Blob)
Binary mmaps `.rodata` blob, XOR-deobfuscates, uses it to validate input. Reimplement verification loop with pyelftools to extract blob. Look for `0x9E3779B9`, `0x85EBCA6B` constants and `rol32()`. See [patterns-ctf-2.md](patterns-ctf-2.md#stack-string-deobfuscation-from-rodata-xor-blob-nullcon-2026).

### Prefix Hash Brute-Force
Binary hashes every prefix independently. Recover one character at a time by matching prefix hashes. See [patterns-ctf-2.md](patterns-ctf-2.md#prefix-hash-brute-force-nullcon-2026).

### Mathematical Convergence Bitmap
**Pattern:** Binary classifies coordinate pairs by Newton's method convergence (e.g., z^3-1=0). Grid of pass/fail results renders ASCII art flag. Key: the binary is a classifier, not a checker — reverse the math and visualize. See [patterns-ctf.md](patterns-ctf.md#mathematical-convergence-bitmap-ehax-2026).

### RISC-V Binary Analysis
Statically linked, stripped RISC-V ELF. Use Capstone with `CS_MODE_RISCVC | CS_MODE_RISCV64` for mixed compressed instructions. Emulate with `qemu-riscv64`. Watch for fake flags and XOR decryption with incremental keys. See [tools.md](tools.md#risc-v-binary-analysis-ehax-2026).

### Sprague-Grundy Game Theory Binary
Game binary plays bounded Nim with PRNG for losing-position moves. Identify game framework (Grundy values = pile % (k+1), XOR determines position), track PRNG state evolution through user input feedback. See [patterns-ctf.md](patterns-ctf.md#sprague-grundy-game-theory-binary-dicectf-2026).

### Kernel Module Maze Solving
Rust kernel module implements maze via device ioctls. Enumerate commands dynamically, build DFS solver with decoy avoidance, deploy as minimal static binary (raw syscalls, no libc). See [patterns-ctf.md](patterns-ctf.md#kernel-module-maze-solving-dicectf-2026).

### Multi-Threaded VM with Channels
Custom VM with 16+ threads communicating via futex channels. Trace data flow across thread boundaries, extract constants from GDB, watch for inverted validity logic, solve via BFS state space search. See [patterns-ctf.md](patterns-ctf.md#multi-threaded-vm-with-channel-synchronization-dicectf-2026).

### CVP/LLL Lattice for Constrained Integer Validation
Binary validates flag via matrix multiplication with 64-bit coefficients; solutions must be printable ASCII. Use LLL reduction + CVP in SageMath to find nearest lattice point in the constrained range. Two-phase pattern: Phase 1 recovers AES key, Phase 2 decrypts custom VM bytecode with another linear system (mod 2^32). See [patterns-ctf-2.md](patterns-ctf-2.md#cvplll-lattice-for-constrained-integer-validation-htb-shadowlabyrinth).

### Decision Tree Function Obfuscation
~200+ auto-generated functions routing input through polynomial comparisons. Script extraction via Ghidra headless rather than reversing each function manually. Constraint propagation from known output format cascades through arithmetic constraints. See [patterns-ctf-2.md](patterns-ctf-2.md#decision-tree-function-obfuscation-htb-wondersms).

### Android JNI RegisterNatives Obfuscation
`RegisterNatives` in `JNI_OnLoad` hides which C++ function handles each Java native method (no standard `Java_com_pkg_Class_method` symbol). Find the real handler by tracing `JNI_OnLoad` → `RegisterNatives` → `fnPtr`. Use x86_64 `.so` from APK for best Ghidra decompilation. See [languages-platforms.md](languages-platforms.md#android-jni-registernatives-obfuscation-htb-wondersms).

### Multi-Layer Self-Decrypting Binary
N-layer binary where each layer decrypts the next using user-provided key bytes + SHA-NI. Use oracle (correct key → valid code with expected pattern). JIT execution with fork-per-candidate COW isolation for speed. See [patterns-ctf-2.md](patterns-ctf-2.md#multi-layer-self-decrypting-binary-dicectf-2026).

### GLSL Shader VM with Self-Modifying Code
**Pattern:** WebGL2 fragment shader implements Turing-complete VM on a 256x256 RGBA texture (program memory + VRAM). Self-modifying code (STORE opcode) patches drawing instructions. GPU parallelism causes write conflicts — emulate sequentially in Python to recover full output. See [patterns-ctf-3.md](patterns-ctf-3.md#glsl-shader-vm-with-self-modifying-code-apoorvctf-2026).

### GF(2^8) Gaussian Elimination for Flag Recovery
**Pattern:** Binary performs Gaussian elimination over GF(2^8) with the AES polynomial (0x11b). Matrix + augmentation vector in `.rodata`; solution vector is the flag. Look for constant `0x1b` in disassembly. Addition is XOR, multiplication uses polynomial reduction. See [patterns-ctf-2.md](patterns-ctf-2.md#gf28-gaussian-elimination-for-flag-recovery-apoorvctf-2026).

### Z3 for Single-Line Python Boolean Circuit
**Pattern:** Single-line Python (2000+ semicolons) with walrus operator chains validates flag as big-endian integer via boolean circuit. Obfuscated XOR `(a | b) & ~(a & b)`. Split on semicolons, translate to Z3 symbolically, solve in under a second. See [patterns-ctf-3.md](patterns-ctf-3.md#z3-for-single-line-python-boolean-circuit-bearcatctf-2026).

### Sliding Window Popcount Differential Propagation
**Pattern:** Binary validates input via expected popcount for each position of a 16-bit sliding window. Popcount differences create a recurrence: `bit[i+16] = bit[i] + (data[i+1] - data[i])`. Brute-force ~4000-8000 valid initial 16-bit windows; each determines the entire bit sequence. See [patterns-ctf-3.md](patterns-ctf-3.md#sliding-window-popcount-differential-propagation-bearcatctf-2026).

### Ruby/Perl Polyglot Constraint Satisfaction
**Pattern:** Single file valid in both Ruby and Perl, each imposing different constraints on a key. Exploits `=begin`/`=end` (Ruby block comment) vs `=begin`/`=cut` (Perl POD) to run different code per interpreter. Intersect constraints from both languages to recover the unique key. See [languages-platforms.md](languages-platforms.md#rubyperl-polyglot-constraint-satisfaction-bearcatctf-2026).

### Verilog/Hardware RE
**Pattern:** Verilog HDL source for state machines with hidden conditions gated on shift register history. Analyze `always @(posedge clk)` blocks and `case` statements to find correct input sequences. See [languages-platforms.md](languages-platforms.md#veriloghardware-reverse-engineering-srdnlenctf-2026).

### Custom binfmt Kernel Module with RC4 Flat Binaries
**Pattern:** Kernel module registers binfmt handler for encrypted flat binaries. Reverse the `.ko` to find RC4 key (in `movabs` immediates), decrypt the flat binary, import at the fixed virtual address from the module's `vm_mmap` call. See [patterns-ctf.md](patterns-ctf.md#custom-binfmt-kernel-module-with-rc4-flat-binaries-bsidessf-2026).

### Hash-Resolved Imports / No-Import Ransomware
**Pattern:** Binary with zero visible imports resolves APIs via symbol name hashing at runtime. Skip the hash reversing — hook OpenSSL functions via `LD_PRELOAD` in Docker to capture AES keys directly. See [patterns-ctf.md](patterns-ctf.md#hash-resolved-imports--no-import-ransomware-bsidessf-2026).

### ELF Section Header Corruption for Anti-Analysis
**Pattern:** Corrupted section headers crash analysis tools but program headers are intact so binary runs normally. Patch `e_shoff` to zero or use `readelf -l` (program headers only). Flag hidden after corrupted sections with magic marker + XOR. See [patterns-ctf.md](patterns-ctf.md#elf-section-header-corruption-for-anti-analysis-bsidessf-2026).

### Brainfuck Character-by-Character Static Analysis
**Pattern:** BF programs validating input have `,` (read char) followed by `+` operations whose count = expected ASCII value. Extract increment counts per input position to recover expected input without execution. See [languages.md](languages.md#brainfuck-character-by-character-static-analysis-bsidessf-2026).

### Brainfuck Side-Channel via Read Count Oracle
**Pattern:** BF input validators read more bytes when a character is correct. Count `,` operations per candidate — highest read count = correct byte. Character-by-character recovery. See [languages.md](languages.md#brainfuck-side-channel-via-read-count-oracle-bsidessf-2026).

### Brainfuck Comparison Idiom Detection
**Pattern:** Compiled BF uses fixed idioms for equality checks (`<[-<->] +<[>-<[-]]>[-<+>]`). Instrument interpreter to detect patterns and extract comparison operands (expected flag bytes). See [languages.md](languages.md#brainfuck-comparison-idiom-detection-bsidessf-2026).

### Backdoored Shared Library Detection
Binary works in GDB but fails when run normally (suid)? Check `ldd` for non-standard libc paths, then `strings | diff` the suspicious vs. system library to find injected code/passwords. See [patterns-ctf.md](patterns-ctf.md#backdoored-shared-library-detection-via-string-diffing-hacklu-ctf-2012).

### Go Binary Reversing
Large static binary with `go.buildid`? Use GoReSym to recover function names (works even on stripped binaries). Go strings are `{ptr, len}` pairs — not null-terminated. Look for `main.main`, `runtime.gopanic`, channel ops (`runtime.chansend1`/`chanrecv1`). Use Ghidra golang-loader plugin for best results. See [languages-compiled.md](languages-compiled.md#go-binary-reversing).

### Go Binary UUID Patching for C2 Enumeration
**Pattern:** Go C2 client with UUID from `-ldflags -X`. Binary-patch UUID bytes (same length), register with C2, enumerate clients/files via API. See [languages-compiled.md](languages-compiled.md#go-binary-uuid-patching-for-c2-client-enumeration-bsidessf-2026).

### D Language Binary Reversing
D language binaries have unique symbol mangling (not C++ style). Template-heavy, many function variants. Look for `_D` prefix in symbols. See [languages-compiled.md](languages-compiled.md#d-language-binary-reversing-csaw-ctf-2016).

### Rust Binary Reversing
Binary with `core::panicking` strings and `_ZN` mangled symbols? Use `rustfilt` for demangling. Panic messages contain source paths and line numbers — `strings binary | grep "panicked"` is the fastest approach. Option/Result enums use discriminant byte (0=None/Err, 1=Some/Ok). See [languages-compiled.md](languages-compiled.md#rust-binary-reversing).

### Frida Dynamic Instrumentation
Hook runtime functions without modifying binary. `frida -f ./binary -l hook.js` to spawn with instrumentation. Hook `strcmp`/`memcmp` to capture expected values, bypass anti-debug by replacing `ptrace` return value, scan memory for flag patterns, replace validation functions. See [tools-dynamic.md](tools-dynamic.md#frida-dynamic-instrumentation).

### Frida Firebase Cloud Functions Bypass
**Pattern:** Android app validates via Firebase Cloud Functions. Post-login Frida hook constructs valid payload (UID + value + timestamp) and calls Cloud Function directly, bypassing QR/payment validation. See [languages-platforms.md](languages-platforms.md#frida-firebase-cloud-functions-bypass-bsidessf-2026).

### angr Symbolic Execution
Automatic path exploration to find inputs satisfying constraints. Load binary with `angr.Project`, set find/avoid addresses, call `simgr.explore()`. Constrain input to printable ASCII and known prefix for faster solving. Hook expensive functions (crypto, I/O) to prevent path explosion. See [tools-dynamic.md](tools-dynamic.md#angr-symbolic-execution).

### Qiling Emulation
Cross-platform binary emulation with OS-level support (syscalls, filesystem). Emulate Linux/Windows/ARM/MIPS binaries on any host. No debugger artifacts — bypasses all anti-debug by default. Hook syscalls and addresses with Python API. See [tools-dynamic.md](tools-emulation.md#qiling-framework-cross-platform-emulation).

### VMProtect / Themida Analysis
VMProtect virtualizes code into custom bytecode. Identify VM entry (pushad-like), find handler table (large indirect jump), trace handlers dynamically. For CTF, focus on tracing operations on input rather than full devirtualization. Themida: dump at OEP with ScyllaHide + Scylla. See [tools-advanced.md](tools-advanced.md#vmprotect-analysis).

### Binary Diffing
BinDiff and Diaphora compare two binaries to highlight changes. Essential when challenge provides patched/original versions. Export from IDA/Ghidra, diff to find vulnerability or hidden functionality. See [tools-advanced.md](tools-advanced.md#binary-diffing).

### Advanced GDB (pwndbg, rr)
pwndbg: `context`, `vmmap`, `search -s "flag{"`, `telescope $rsp`. GEF alternative. Reverse debugging with `rr record`/`rr replay` — step backward through execution. Python scripting for brute-force and automated tracing. See [tools-advanced-2.md](tools-advanced-2.md#advanced-gdb-techniques).

### macOS / iOS Reversing
Mach-O binaries: `otool -l` for load commands, `class-dump` for Objective-C headers. Swift: `swift demangle` for symbols. iOS apps: decrypt FairPlay DRM with frida-ios-dump, bypass jailbreak detection with Frida hooks. Re-sign patched binaries with `codesign -f -s -`. See [platforms.md](platforms.md#macos--ios-reversing).

### Embedded / IoT Firmware RE
`binwalk -Me firmware.bin` for recursive extraction. Hardware: UART/JTAG/SPI flash for firmware dumps. Filesystems: SquashFS (`unsquashfs`), JFFS2, UBI. Emulate with QEMU: `qemu-arm -L /usr/arm-linux-gnueabihf/ ./binary`. See [platforms.md](platforms.md#embedded--iot-firmware-re).

### Kernel Driver Reversing
Linux `.ko`: find ioctl handler via `file_operations` struct, trace `copy_from_user`/`copy_to_user`. Debug with QEMU+GDB (`-s -S`). eBPF: `bpftool prog dump xlated`. Windows `.sys`: find `DriverEntry` → `IoCreateDevice` → IRP handlers. See [platforms.md](platforms.md#kernel-driver-reversing).

### Game Engine Reversing
Unreal: extract .pak with UnrealPakTool, reverse Blueprint bytecode with FModel. Unity Mono: decompile Assembly-CSharp.dll with dnSpy. Anti-cheat (EAC, BattlEye, VAC): identify system, bypass specific check. Lua games: `luadec`/`unluac` for bytecode. See [platforms.md](platforms.md#game-engine-reversing).

### Swift / Kotlin Binary Reversing
Swift: `swift demangle` symbols, protocol witness tables for dispatch, `__swift5_*` sections. Kotlin/JVM: coroutines compile to state machines in `invokeSuspend`, `jadx` with Kotlin mode for best decompilation. Kotlin/Native: LLVM backend, looks like C++ in disassembly. See [languages-compiled.md](languages-compiled.md#swift-binary-reversing).

### INT3 Patch + Coredump Brute-Force Oracle
Patch `0xCC` (INT3) after transform output, enable core dumps, brute-force each input character by extracting computed state from coredump via `strings`. Avoids full reverse of transformation. See [patterns.md](patterns-runtime.md#int3-patch--coredump-brute-force-oracle-pwn2win-2016).

### Signal Handler Chain + LD_PRELOAD Oracle
Binary uses signal handler chains for per-character password validation. Hook `signal()` via LD_PRELOAD -- the call to install the next handler confirms the current character is correct. See [patterns.md](patterns-runtime.md#signal-handler-chain--ld_preload-oracle-nuit-du-hack-2016).

### Font Ligature Exploitation
Custom OpenType font maps multi-character ligature sequences to single glyphs; reverse the GSUB table to decode hidden messages. See [patterns-ctf-3.md](patterns-ctf-3.md#opentype-font-ligature-exploitation-for-hidden-messages-hack-the-vote-2016).

### Instruction Counter as Cryptographic State
**Pattern:** Hand-written assembly uses a dedicated register (e.g., `r12`) as an instruction counter incremented after nearly every instruction. The counter feeds into XOR/ROL/multiply transformations on input bytes, making transformation path-dependent. Byte-by-byte brute force with Unicorn emulation recovers the flag. See [patterns-ctf-3.md](patterns-ctf-3.md#instruction-counter-as-cryptographic-state-metactf-flash-2026).

### Burrows-Wheeler Transform Inversion
Invert BWT without terminator character by trying all possible row indices. Standard `bwtool` or manual column-sorting reconstruction. See [patterns-ctf-3.md](patterns-ctf-3.md#burrows-wheeler-transform-inversion-without-terminator-asis-ctf-finals-2016).

### FRACTRAN Program Inversion
Esoteric language using iterated fraction multiplication. Invert by swapping numerator/denominator in fraction table, run output backward. I/O encoded as prime factorization exponents. See [languages.md](languages.md#fractran-program-inversion-boston-key-party-2016).

### Opcode-Only Trace Reconstruction
Execution traces with only opcodes (no data) still leak info through branch decisions. Sorting algorithm comparisons reveal element ordering. Reconstruct by deduplicating trace, splitting into basic blocks. See [tools-dynamic.md](tools-emulation.md#opcode-only-trace-reconstruction-0ctf-2016).

### Thread Race Signed Integer Overflow
Game binary with thread-unsafe skill lock. Race between skill selection and damage calculation; `cdqe` sign-extends 0xFFFFFFFF to -1 (signed), causing HP overflow on subtraction. See [patterns-ctf-3.md](patterns-ctf-3.md#thread-race-condition-with-signed-integer-overflow-codegate-2017).

### ESP32/Xtensa Firmware Reversing
No IDA support — use radare2 + ESP-IDF ROM linker script (`esp32.rom.ld`) for symbol resolution. Cross-reference with public ESP-IDF HTTP server examples to identify app logic. See [patterns-ctf-3.md](patterns-ctf-3.md#esp32xtensa-firmware-reversing-with-rom-symbol-map-insomnihack-2017).

### Custom VM Bytecode Lifting to LLVM IR
Transpile custom VM bytecode to LLVM IR, then use `opt -O3` to simplify (inlining, constant folding, dead code elimination). Reduces 1300 lines to ~150 lines, revealing the underlying algorithm. See [tools-advanced.md](tools-advanced.md#custom-vm-bytecode-lifting-to-llvm-ir-google-ctf-2017).

### SIGFPE Signal Handler Side-Channel
SIGFPE signal handlers create implicit control flow invisible to static analysis. Count SIGFPE signals via `strace -e signal=SIGFPE` per candidate character -- correct characters produce more signals. See [anti-analysis.md](anti-analysis-ctf.md#sigfpe-signal-handler-side-channel-via-strace-counting-plaidctf-2017).

### Batch Crackme Automation via objdump
Mass crackme challenges (100s of binaries) with identical structure: script `objdump` to extract CMP immediates and add/sub arithmetic sequences, then reverse-compute keys algebraically without execution. See [patterns-ctf-3.md](patterns-ctf-3.md#batch-crackme-automation-via-objdump-pattern-extraction-def-con-2017).

### Android DEX Runtime Bytecode Patching
Native JNI library patches Dalvik bytecode in memory via `/proc/self/maps` + `mprotect` + XOR. Static APK analysis alone is insufficient -- extract XOR key and offsets from the native `.so` to reconstruct the runtime DEX. See [languages-platforms.md](languages-platforms.md#android-dex-runtime-bytecode-patching-via-procselfmaps-google-ctf-2017).

### Fork + Pipe + Dead Branch Anti-Analysis
Fork/pipe IPC where parent writes data and exits, child reads and continues. Real validation hidden in a dead branch (always-false comparison). `strace` reveals the fork/pipe pattern; patch the comparison constant to reach hidden code. See [patterns-ctf-3.md](patterns-ctf-3.md#fork--pipe--dead-branch-anti-analysis-rctf-2017).
