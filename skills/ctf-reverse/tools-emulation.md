# CTF Reverse - Emulation and Side-Channel Tooling

Emulation frameworks (Qiling, Triton) and side-channel measurement tools (Intel Pin, LD_PRELOAD hooks) for CTF challenges where anti-debug, self-modifying code, or cross-architecture targets make plain GDB/Frida impractical.

For core dynamic analysis tools (Frida, angr, lldb, x64dbg), see [tools-dynamic.md](tools-dynamic.md).

## Table of Contents
- [Qiling Framework (Cross-Platform Emulation)](#qiling-framework-cross-platform-emulation)
  - [Qiling Installation](#qiling-installation)
  - [Basic Usage](#basic-usage)
  - [Anti-Debug Bypass via Emulation](#anti-debug-bypass-via-emulation)
  - [Input Fuzzing with Qiling](#input-fuzzing-with-qiling)
- [Triton (Dynamic Symbolic Execution)](#triton-dynamic-symbolic-execution)
- [Intel Pin Instruction-Counting Side Channel (Hackover CTF 2015)](#intel-pin-instruction-counting-side-channel-hackover-ctf-2015)
  - [Intel Pin Instruction Counting with Genetic Algorithm (hxp CTF 2017)](#intel-pin-instruction-counting-with-genetic-algorithm-hxp-ctf-2017)
- [Opcode-Only Trace Reconstruction (0CTF 2016)](#opcode-only-trace-reconstruction-0ctf-2016)
- [LD_PRELOAD time() Freeze for Deterministic Analysis (EKOPARTY 2017)](#ld_preload-time-freeze-for-deterministic-analysis-ekoparty-2017)
  - [LD_PRELOAD memcmp Side-Channel for Byte-by-Byte Bruteforce (Blaze CTF 2018)](#ld_preload-memcmp-side-channel-for-byte-by-byte-bruteforce-blaze-ctf-2018)

---

## Qiling Framework (Cross-Platform Emulation)

Qiling emulates binaries with OS-level support (syscalls, filesystem, registry). Built on Unicorn but adds the OS layer that Unicorn lacks.

### Qiling Installation

```bash
pip install qiling
# Download rootfs for target OS:
git clone https://github.com/qilingframework/rootfs
```

### Basic Usage

```python
from qiling import Qiling
from qiling.const import QL_VERBOSE

# Linux ELF emulation
ql = Qiling(["./binary", "arg1"], "rootfs/x8664_linux",
            verbose=QL_VERBOSE.DEFAULT)
ql.run()

# Windows PE emulation (no Windows needed!)
ql = Qiling(["rootfs/x86_windows/bin/binary.exe"], "rootfs/x86_windows")
ql.run()

# ARM/MIPS emulation (IoT firmware)
ql = Qiling(["rootfs/arm_linux/bin/binary"], "rootfs/arm_linux")
ql.run()
```

### Anti-Debug Bypass via Emulation

```python
from qiling import Qiling

ql = Qiling(["./binary"], "rootfs/x8664_linux")

# Hook ptrace syscall — return 0 (success)
def hook_ptrace(ql, ptrace_request, pid, addr, data):
    ql.log.info("ptrace bypassed")
    return 0

ql.os.set_syscall("ptrace", hook_ptrace)

# Hook specific address (e.g., anti-VM check)
def skip_check(ql):
    ql.arch.regs.rax = 0  # Force success
    ql.log.info(f"Skipped check at {ql.arch.regs.rip:#x}")

ql.hook_address(skip_check, 0x401234)

ql.run()
```

### Input Fuzzing with Qiling

```python
# Emulate binary with different inputs to find flag
import string
from qiling import Qiling

def test_input(candidate):
    ql = Qiling(["./binary"], "rootfs/x8664_linux",
                verbose=QL_VERBOSE.DISABLED, stdin=candidate.encode())
    ql.run()
    return ql.os.stdout.read()

for ch in string.printable:
    output = test_input("flag{" + ch)
    if b"Correct" in output:
        print(f"Found: {ch}")
```

**Advantages over GDB/Frida:**
- No debugger artifacts (bypasses all anti-debug by default)
- Cross-platform without hardware (ARM, MIPS, RISC-V on x86 host)
- Scriptable with Python (faster iteration than GDB)
- Snapshot/restore for brute-forcing

**Key insight:** Qiling emulates the entire OS layer (syscalls, filesystem, registry), not just the CPU. This means anti-debug checks like `ptrace(TRACEME)` naturally return success without patching, and you can analyze ARM/MIPS binaries on an x86 host without QEMU or real hardware.

**When to use:** Foreign architecture binaries, IoT firmware, heavy anti-debug, automated testing of many inputs.

---

## Triton (Dynamic Symbolic Execution)

See [tools-advanced.md](tools-advanced.md#triton-dynamic-symbolic-execution) for full Triton reference. Quick usage:

```python
from triton import *

ctx = TritonContext(ARCH.X86_64)

# Symbolize input buffer
for i in range(32):
    ctx.symbolizeMemory(MemoryAccess(0x600000 + i, CPUSIZE.BYTE), f"flag_{i}")

# Process instructions and collect constraints
# At comparison point, solve for flag
model = ctx.getModel(ctx.getPathConstraintsAst())
flag = ''.join(chr(v.getValue()) for _, v in sorted(model.items()))
```

**Key insight:** Triton excels at single-path DSE (Dynamic Symbolic Execution) where angr's path explosion is a problem. Feed it a concrete execution trace, symbolize specific inputs, and solve for constraints at comparison points. Faster than angr for linear code paths with known execution flow.

**Best for:** Single-path symbolic execution, deobfuscation, taint analysis. Faster than angr for linear code paths.

---

## Intel Pin Instruction-Counting Side Channel (Hackover CTF 2015)

**Pattern:** Brute-force input character-by-character against a binary using Intel Pin's `inscount0` tool. Each correct character causes deeper execution (more instructions) in the comparison logic.

```python
import string
from subprocess import Popen, PIPE

pin = './pin'
tool = './source/tools/ManualExamples/obj-ia32/inscount0.so'
binary = './target'

key = ''
while True:
    best_count, best_char = 0, ''
    for c in string.printable:
        cmd = [pin, '-injection', 'child', '-t', tool, '--', binary]
        p = Popen(cmd, stdout=PIPE, stdin=PIPE, stderr=PIPE)
        p.communicate((key + c + '\n').encode())
        with open('inscount.out') as f:
            count = int(f.read().split()[-1])
        if count > best_count:
            best_count, best_char = count, c
    key += best_char
    print(f"Found: {key}")
```

**Key insight:** Movfuscated binaries (compiled with `movfuscator`) expand every instruction into sequences of `mov` operations, making static analysis impractical. However, character-by-character comparison still creates measurable instruction count differences. Pin's `inscount0.so` counts total executed instructions — the correct character at each position causes ~1000+ more instructions (proceeding further in the comparison). Also works for obfuscated binaries with sequential input checks.

---

### Intel Pin Instruction Counting with Genetic Algorithm (hxp CTF 2017)

For self-modifying code that decrypts the next chunk only after each character check passes, standard character-by-character Pin counting fails because the search space is too large and characters may interact. Use a genetic algorithm instead to explore the input space more efficiently.

```python
import subprocess
import random
import string

PIN_PATH = '/tmp/pin-3.5/pin'
TOOL_PATH = 'source/tools/ManualExamples/obj-intel64/inscount0.so'

def fitness(candidate):
    """Run binary under Pin and return instruction count as fitness."""
    proc = subprocess.Popen(
        [PIN_PATH, '-t', TOOL_PATH, '--', './binary'],
        stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    stdout, stderr = proc.communicate(candidate.encode())
    # inscount0 writes count to stderr or inscount.out
    try:
        with open('inscount.out') as f:
            return int(f.read().split()[-1])
    except:
        return 0

def mutate(individual, rate=0.1):
    """Randomly mutate characters in the individual."""
    result = list(individual)
    for i in range(len(result)):
        if random.random() < rate:
            result[i] = random.choice(string.printable[:62])
    return result

# Genetic algorithm parameters
FLAG_LEN = 40
POP_SIZE = 100
SURVIVORS = 20

# Initialize random population
population = [random.choices(string.printable[:62], k=FLAG_LEN) for _ in range(POP_SIZE)]

for generation in range(10000):
    # Score each individual by instruction count
    scored = [(fitness(''.join(p)), p) for p in population]
    scored.sort(reverse=True)
    best_score, best_individual = scored[0]
    print(f"Gen {generation}: {best_score} {''.join(best_individual)}")

    # Keep top survivors, mutate to refill population
    survivors = [s[1] for s in scored[:SURVIVORS]]
    population = survivors + [mutate(random.choice(survivors)) for _ in range(POP_SIZE - SURVIVORS)]
```

**Modified Pin for Go binaries (table-lookup flag checking):**
When standard `inscount` fails because counter increments don't correlate with correctness (e.g., table-lookup comparison), modify Pin's icount tool to only count executions at the success-branch address. Brute-force character-by-character with this targeted counter:
```cpp
// Modified inscount0.cpp — count only executions of a specific address
static ADDRINT target_addr = 0x401234;  // success-branch address
static UINT64 target_count = 0;

VOID CountAtTarget(ADDRINT ip) {
    if (ip == target_addr) target_count++;
}

VOID Instruction(INS ins, VOID *v) {
    INS_InsertCall(ins, IPOINT_BEFORE, (AFUNPTR)CountAtTarget,
                   IARG_INST_PTR, IARG_END);
}
```

**Key insight:** When each correct character unlocks a new code section (self-modifying or multi-stage decryption), instruction count increases monotonically with correctness. A genetic algorithm explores the input space more efficiently than character-by-character brute-force because it can discover multiple correct characters simultaneously. Converges in approximately 30 minutes for 40-character flags. For table-lookup comparisons where total instruction count doesn't correlate, target a specific branch address instead.

**References:** hxp CTF 2017

---

## Opcode-Only Trace Reconstruction (0CTF 2016)

Given an execution trace with only opcodes (no register/memory values), reconstruct the program: sort/dedup trace by address, split into basic blocks, annotate functions. Sorting algorithms are particularly vulnerable -- branch decisions leak element ordering.

**Approach:**
1. Sort trace entries by address, deduplicate to recover code layout
2. Identify basic block boundaries (jumps, calls, returns)
3. Map branch taken/not-taken decisions from trace order
4. For sorting algorithms, partition comparisons reveal relative ordering of all input elements

**Key insight:** Execution traces without data values still leak information through branch decisions. Quicksort partition comparisons reveal which element is greater/lesser at each step, enabling full recovery of the sorted input from branch direction alone.

---

## LD_PRELOAD time() Freeze for Deterministic Analysis (EKOPARTY 2017)

Override `time()` via LD_PRELOAD to return a constant value, freezing any timestamp-seeded PRNG. Once the binary's cipher becomes deterministic, brute-force each output byte without understanding the VM or cipher internals.

```c
// freeze_time.c — compile: gcc -shared -fPIC -o freeze.so freeze_time.c
#include <time.h>

time_t time(time_t *t) {
    if (t) *t = 1234567890;
    return 1234567890;
}
```

```bash
# Build and use:
gcc -shared -fPIC -o freeze.so freeze_time.c
LD_PRELOAD=./freeze.so ./binary

# Byte-at-a-time oracle: run with frozen time, try each candidate byte,
# observe output — correct byte produces expected output character.
for byte in $(seq 0 255); do
    output=$(echo -n "$(printf '\x%02x' $byte)" | LD_PRELOAD=./freeze.so ./binary)
    # Check output against known/expected
done
```

If `srand()` or `rand()` is also involved, override `rand()` too:
```c
int rand(void) { return 42; }
```

**Key insight:** LD_PRELOAD function interception freezes non-determinism sources (time, rand). Once deterministic, even complex VMs become tractable byte-at-a-time oracles.

**References:** EKOPARTY CTF 2017

---

### LD_PRELOAD memcmp Side-Channel for Byte-by-Byte Bruteforce (Blaze CTF 2018)

**Pattern:** Replace `memcmp` with an LD_PRELOAD library that returns the number of matching bytes instead of the standard -1/0/1 result. This converts any memcmp-based validation into a byte-by-byte oracle. Automate with GDB Python scripting to bruteforce each character position.

```c
// memcmp_hook.c - compile: gcc -shared -fPIC -o hook.so memcmp_hook.c
int memcmp(const char *s1, const char *s2, int n) {
    int cnt = 0;
    for (int i = 0; i < n; ++i) {
        if (s1[i] == s2[i]) cnt++;
        else break;
    }
    return cnt;
}
```

```bash
# Use with GDB: LD_PRELOAD=./hook.so gdb ./binary
# Set breakpoint after memcmp, read return value to count matching bytes
# Iterate characters at each position to find the one that increases count
```

**Key insight:** Replacing memcmp via LD_PRELOAD to return match count converts any comparison-based validation into a byte-by-byte oracle. Combined with GDB scripting, this automates bruteforce of password/flag checks without reversing the validation algorithm.

**Detection:** Binary uses `memcmp` or `strcmp` for flag validation (visible in `ltrace` output or import table). The comparison function is called with user input and a computed/stored expected value.

**References:** Blaze CTF 2018
