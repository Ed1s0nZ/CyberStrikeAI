---
name: offensive-fuzzing
description: 'Practical offensive fuzzing methodology covering target identification,
  fuzzer selection (AFL++, libFuzzer, Honggfuzz, Boofuzz, syzkaller), harness writing,
  corpus curation, mutation strategies, coverage measurement, and crash triage. Use
  when setting up or running fuzz campaigns against any target: file parsers, network
  protocols, kernel drivers, EDR engines, embedded firmware, or language runtimes.'
---

# Offensive Fuzzing

## Fuzzer Types

| Type | Coverage | Speed | Tools |
|------|----------|-------|-------|
| BlackBox | Poor | Fast | Peach, Boofuzz |
| GreyBox | Good | Fast | AFL++, Honggfuzz, libFuzzer, WinAFL |
| Snapshot | Good | Fastest | Nyx, wtf, Snapchange |
| WhiteBox | Best | Slow | KLEE, QSYM, SymSan |
| Ensemble | Best | Fast | AFL++ + Honggfuzz + libFuzzer |

**GreyBox sub-variants:** Directed (AFLGo, UAFuzz), Grammar (AFLSmart, Tlspuffin), Concolic (QSYM, Driller), Kernel (syzkaller, kAFL, wtf).

## Core Workflow

```
Research target → Choose analyses → Build harness → Seed corpus → Instrument → Fuzz → Triage crashes → Report
```

### 1. Research Target

- Map all input surfaces (files, network, IPC, syscalls, IOCTL)
- Identify high-value areas: previously patched code, complex parsers, newly added code, input ingestion points
- For kernel modules: look beyond `copy_from_user` — DMA-BUF ops, page fault handlers, VM operation structs, allocation callbacks

### 2. Instrument and Build

```bash
# AFL++ (preferred for GreyBox)
CC=afl-clang-fast CXX=afl-clang-fast++ cmake -DCMAKE_BUILD_TYPE=Release .. && make -j

# libFuzzer + ASan/UBSan (C/C++)
cmake -DCMAKE_CXX_FLAGS="-fsanitize=fuzzer,address,undefined -O1 -g" ..

# CmpLog build for hard compares
AFL_LLVM_CMPLOG=1 CC=afl-clang-fast CXX=afl-clang-fast++ make clean all
```

**Windows (MSVC):** `Project Properties → C/C++ → Address Sanitizer: Yes (/fsanitize=address)`

### 3. Write Harness

**libFuzzer (C++):**
```cpp
#include <cstdint>
#include <cstddef>
extern "C" int LLVMFuzzerTestOneInput(const uint8_t* data, size_t size) {
    parse_or_process(data, size);
    return 0;
}
```

**Honggfuzz HF_ITER (persistent mode — preferred for large targets):**
```cpp
#include "honggfuzz.h"
int main(int argc, char** argv) {
    initialize_target(); // runs once
    for (;;) {
        size_t len; uint8_t *buf;
        HF_ITER(&buf, &len);
        FILE* s = fmemopen(buf, len, "r");
        target_function(s);
        fclose(s);
        reset_target_state();
    }
}
```

**AFL++ persistent mode (`__AFL_LOOP`):**
```cpp
while (__AFL_LOOP(10000)) {
    // re-read input and process
}
```

**macOS IPC (Mach message fuzzing):**
```c
void *lib_handle = dlopen("libexample.dylib", RTLD_LAZY);
pFunction = dlsym(lib_handle, "DesiredFunction");
```

### 4. Build Seed Corpus

- Pull from target's test suite, bug reports, and real-world samples
- Web-crawl (Common Crawl) for file formats; filter by MIME type
- Minimize: `afl-cmin -i raw_corpus -o seeds -- ./target @@`
- Trim inputs: `afl-tmin -i crash -o crash.min -- ./target @@`

### 5. Launch Fuzzing

**AFL++ parallel (primary + secondary with cmplog):**
```bash
afl-fuzz -M f1 -i seeds -o findings -x dict.txt -- ./target @@
afl-fuzz -S s1 -i seeds -o findings -c 0 -- ./target @@
```

**libFuzzer:**
```bash
./target_libfuzzer corpus/ -max_total_time=3600 -workers=4
```

**Binary-only (QEMU):**
```bash
afl-fuzz -Q -i seeds -o findings -- target.exe @@
```

**Snapshot (AFL++ Nyx):**
```bash
NYX_MODE=1 AFL_MAP_SIZE=1048576 afl-fuzz -i seeds -o findings -- ./target_nyx @@
```

**Ensemble (AFL++ + Honggfuzz sharing corpus):**
```bash
# Terminal 1
afl-fuzz -M fuzzer1 -i seeds -o sync_dir -- ./target @@
# Terminal 2
../honggfuzz/honggfuzz -i sync_dir/fuzzer1/queue -W sync_dir/hfuzz \
  --linux_perf_ipt_block -t 10 -- ./target ___FILE___
```

### 6. Monitor and Unstick

If progress stalls:
- Enable CmpLog: `-c 0` on AFL++ secondaries
- Add dictionary: `-x dict.txt` or `AFL_TOKEN_FILE`
- Switch to directed fuzzing (AFLGo) targeting specific BBs/functions
- Use concolic assistance (QSYM, Driller) on hard branches
- Snapshot the target to increase exec/s
- `AFL_MAP_SIZE=1048576`, `-L 0` for MOpt scheduler

### 7. Triage Crashes

```bash
# 1. Minimize
afl-tmin -i crash -o crash.min -- ./target @@
# 2. Symbolize
ASAN_OPTIONS=abort_on_error=1:symbolize=1 ./target crash.min 2>asan.log
# 3. Hash + bucket
./cov-tool --bbids ./target crash.min > cov.hash
./bucket.py --key "$(cat cov.hash)" --log asan.log --out triage/
```

**Sanitizer env quick reference:**
```
ASAN_OPTIONS=abort_on_error=1:symbolize=1:detect_stack_use_after_return=1
UBSAN_OPTIONS=print_stacktrace=1:halt_on_error=1
TSAN_OPTIONS=halt_on_error=1:history_size=7
MSAN_OPTIONS=poison_in_dtor=1:track_origins=2
```

## Oracle Selection

| Bug Class | Oracle |
|-----------|--------|
| Memory safety | ASan, HWASan (AArch64, lower overhead) |
| Uninitialized reads | MSan |
| Concurrency | TSan |
| Undefined behavior | UBSan |
| Type safety | TypeSan |
| Heap hardening | Scudo Hardened Allocator |
| Logic bugs | Differential / idempotency oracles |
| Kernel memory | KASAN, KMSAN, KCSAN |
| Kernel UB | KUBSan (`CONFIG_UBSAN_TRAP=y`) |
| CFI | KCFI (`-fsanitize=kcfi`, Clang 18) |
| Binary-only | QASAN (QEMU+ASan), DynamoRIO |

**Property oracle patterns:**
- Idempotency: `f(x) == f(f(x))`
- Differential: compare two impls, bucket on output mismatch
- Invariants: monotonic lengths, checksum equality, schema validation post-parse

## Specialized Targets

### Kernel (Linux) — syzkaller

```json
{
  "target": "linux/arm64",
  "http": ":56700",
  "workdir": "/path/to/workdir",
  "kernel_obj": "/path/to/kernel",
  "image": "/path/to/rootfs.ext3",
  "sshkey": "/path/to/id_rsa",
  "procs": 8,
  "enable_syscalls": ["openat$module_name", "ioctl$IOCTL_CMD", "mmap"],
  "type": "qemu",
  "vm": { "count": 4, "cpu": 2, "mem": 2048 }
}
```

- Limit `enable_syscalls` to deepen coverage on specific subsystems
- Use `syz-extract` to pull constants for custom modules
- Enable `CONFIG_KASAN=y`, `CONFIG_KCFI=y`, `CONFIG_DEBUG_INFO_BTF=y`
- Use `kcov` filters and `syz_cover_filter` to direct coverage
- Network fuzzing: inject via `TUN/TAP` + pseudo-syscalls (`syz_emit_ethernet`)
- Crash decode: `./scripts/decode_stacktrace.sh vmlinux ... < dmesg.log`

**syzkaller repro:**
```bash
syz-execprog -repeat=0 -procs=1 -cover=0 -debug target.repro
```

### EDR / Windows Scanning Engines

**WTF snapshot harness skeleton (mpengine.dll / mini-filter):**
```cpp
g_Backend->SetBreakpoint("nt!KeBugCheck2", [](Backend_t *Backend) {
    const uint64_t BCode = Backend->GetArg(0);
    Backend->Stop(Crash_t(fmt::format("crash-{:#x}", BCode)));
});
```

**FilterConnectionPort fuzzing:**
```cpp
HANDLE hPort;
FilterConnectCommunicationPort(L"\\PortName", 0, NULL, 0, NULL, &hPort);
FilterSendMessage(hPort, fuzzData, sizeof(fuzzData), NULL, 0, &bytesReturned);
```

**IOCTL fuzzing pattern:**
```cpp
HANDLE hDev = CreateFile(L"\\\\.\\DeviceName", GENERIC_READ|GENERIC_WRITE, ...);
DeviceIoControl(hDev, ioctlCode, inputBuf, inputLen, outBuf, outLen, &ret, NULL);
```

- Take snapshots after initialization, right before parse/dispatch loop
- Use IDA Lighthouse for coverage visualization
- Monitor: `DRIVER_VERIFIER_DETECTED_VIOLATION (0xc4)`, `IRQL_NOT_LESS_OR_EQUAL (0xa)`
- WinDbg: `.symfix; !analyze -v; k; !heap -p -a @rax`

**Cross-platform mpengine.dll on Linux (loadlibrary + HF_ITER + Intel PT):**
```cpp
// Bypass Lua VM to avoid stability issues
insert_function_redirect((void*)luaV_execute_address, my_lua_exec, HOOK_REPLACE_FUNCTION);
for (;;) {
    HF_ITER(&buf, &len);
    ScanDescriptor.UserPtr = fmemopen(buf, len, "r");
    __rsignal(&KernelHandle, RSIG_SCAN_STREAMBUFFER, &ScanParams, sizeof ScanParams);
}
```

### Rust

```bash
# Full Rust fuzzing pipeline
cargo test                                         # 1. property tests
cargo +nightly miri test                           # 2. UB via interpreter
cargo +nightly careful test                        # 3. runtime bounds checks
cargo fuzz run fuzz_target_1 -- -max_total_time=3600  # 4. libFuzzer crashes
RUSTFLAGS="--cfg loom" cargo test --release        # 5. concurrency (if needed)
cargo fuzz coverage fuzz_target_1                  # 6. coverage report
```

Focus unsafe blocks on: `Vec::from_raw_parts`, unchecked indexing, `transmute` size mismatches, pointer arithmetic, FFI integer truncation.

### Embedded / Binary-Only

- **LibAFL**: Modular Rust framework; Unicorn engine, snapshot module, LBRFeedback (zero-instrumentation on Intel), SAND decoupled sanitization
- **Retrowrite / QASAN**: Binary rewriting for coverage + ASan without source
- **Nautilus**: Grammar-based fuzzing for structured formats

### Language Ecosystems

- **Go 1.18+**: `go test -fuzz=Fuzz -run=^$ ./...`
- **Python**: [Atheris](https://github.com/google/atheris) (CPython native extension fuzzing)
- **Rust**: `cargo-fuzz` or `honggfuzz-rs`
- **JS engines**: Fuzzilli with extended instrumentation (`__builtin_return_address(0)` for PC tracking)
- **Wasm runtimes**: `wasmtime-fuzz`, `wafl` for differential fuzzing across V8/Wasmer/Wasmtime
- **Smart contracts**: Echidna, Foundry-fuzz (Solidity); Move-Fuzz (Aptos/Sui)

## CI/CD Integration

```yaml
- name: Build with afl-clang-fast
  run: CC=afl-clang-fast make -j
- name: Fuzz (smoke, 15 min)
  run: timeout 15m afl-fuzz -i seeds -o findings -- ./target @@ || true
- name: Upload crashes
  if: always()
  uses: actions/upload-artifact@v4
  with:
    path: findings/**/crashes/*
```

Use **ClusterFuzzLite** for persistent continuous fuzzing; cache corpora between runs.

## Crash Analysis Quick Reference

**Linux:**
```bash
ulimit -c unlimited && sysctl -w kernel.core_pattern=core.%e.%p
gdb -q ./target core.* -ex 'bt' -ex 'info reg' -ex q
addr2line -e ./target 0xDEADBEEF
```

**Windows:**
```powershell
# Enable local dumps
New-Item 'HKLM:\SOFTWARE\Microsoft\Windows\Windows Error Reporting\LocalDumps' -Force
# PageHeap
gflags /p /enable target.exe /full
```

**Kernel KASAN/KMSAN:**
```bash
dmesg -T | egrep -i 'kasan|kmsan' -A 60
./scripts/decode_stacktrace.sh vmlinux /lib/modules/$(uname -r)/build < dmesg.log
```

**Reproducibility:** pin CPU governor, disable ASLR only where safe, fix RNG seeds, save input sequences in persistent mode, record binary hashes and sanitizer options with every crash.

## Tool Index

| Tool | Use Case |
|------|----------|
| [AFL++](https://github.com/AFLplusplus/AFLplusplus) | General GreyBox, CmpLog, MOpt, Nyx |
| [Honggfuzz](https://github.com/google/honggfuzz) | Intel PT, crash detection, HF_ITER |
| [libFuzzer](https://llvm.org/docs/LibFuzzer.html) | In-process, source available |
| [syzkaller](https://github.com/google/syzkaller) | Linux/Windows kernel syscall fuzzing |
| [wtf](https://github.com/0vercl0k/wtf) | Snapshot fuzzing, Windows targets |
| [Nyx](https://github.com/nyx-fuzz/Nyx) | AFL++ snapshot mode (Intel PT) |
| [Snapchange](https://github.com/awslabs/snapchange) | AWS snapshot fuzzing |
| [LibAFL](https://github.com/AFLplusplus/LibAFL) | Custom Rust fuzzing framework |
| [AFLGo](https://github.com/aflgo/aflgo) | Directed fuzzing to target BB/function |
| [kAFL](https://github.com/IntelLabs/kAFL) | Kernel + OS fuzzing |
| [Jackalope](https://github.com/googleprojectzero/Jackalope) | Binary coverage-guided (Windows/macOS) |
| [cargo-fuzz](https://github.com/rust-fuzz/cargo-fuzz) | Rust libFuzzer integration |
| [Atheris](https://github.com/google/atheris) | Python fuzzing |
| [Nautilus](https://github.com/nautilus-fuzz/nautilus) | Grammar-based fuzzing |
| [AFLTriage](https://github.com/quic/AFLTriage) | Automated crash triage |
| [afl-cov](https://github.com/mrash/afl-cov) | Coverage analysis for AFL++ |
| [ClusterFuzz](https://github.com/google/clusterfuzz) | Distributed fuzzing infrastructure |
