# Advanced Reverse Engineering Tools (Part 2)

Advanced GDB scripting, Ghidra automation, patching frameworks, and CTF-specific GDB-driven techniques. Continuation of [tools-advanced.md](tools-advanced.md).

## Table of Contents
- [Advanced GDB Techniques](#advanced-gdb-techniques)
  - [Python Scripting](#python-scripting)
  - [Brute-Force with GDB Script](#brute-force-with-gdb-script)
  - [Conditional Breakpoints](#conditional-breakpoints)
  - [Watchpoints](#watchpoints)
  - [Reverse Debugging (rr)](#reverse-debugging-rr)
  - [GDB Dashboard / GEF / pwndbg](#gdb-dashboard--gef--pwndbg)
- [Advanced Ghidra Scripting](#advanced-ghidra-scripting)
- [Patching Strategies](#patching-strategies)
  - [Binary Ninja Patching (Python API)](#binary-ninja-patching-python-api)
  - [LIEF (Library for Instrumenting Executable Formats)](#lief-library-for-instrumenting-executable-formats)
- [GDB Constraint Extraction with ILP/LP Solver (BackdoorCTF 2017)](#gdb-constraint-extraction-with-ilplp-solver-backdoorctf-2017)
- [GDB Position-Encoded Input with Zero Flag Monitoring (EKOPARTY 2017)](#gdb-position-encoded-input-with-zero-flag-monitoring-ekoparty-2017)
- [LD_PRELOAD to Dump Execute-Only Binary (BackdoorCTF 2017)](#ld_preload-to-dump-execute-only-binary-backdoorctf-2017)
- [PEDA current_inst Bit-by-Bit Flag Scraper (CONFidence CTF 2019 Teaser)](#peda-current_inst-bit-by-bit-flag-scraper-confidence-ctf-2019-teaser)

---

## Advanced GDB Techniques

### Python Scripting

```python
# ~/.gdbinit or source from GDB
import gdb

class TraceCompare(gdb.Breakpoint):
    """Log all comparison operations."""
    def __init__(self, addr):
        super().__init__(f"*{addr}", gdb.BP_BREAKPOINT)

    def stop(self):
        frame = gdb.selected_frame()
        rdi = int(frame.read_register("rdi"))
        rsi = int(frame.read_register("rsi"))
        rdx = int(frame.read_register("rdx"))
        # Read compared buffers
        inferior = gdb.selected_inferior()
        buf1 = inferior.read_memory(rdi, rdx).tobytes()
        buf2 = inferior.read_memory(rsi, rdx).tobytes()
        print(f"memcmp({buf1!r}, {buf2!r}, {rdx})")
        return False  # Don't stop, just log

# Usage in GDB:
# (gdb) source trace_cmp.py
# (gdb) python TraceCompare(0x401234)
```

### Brute-Force with GDB Script

```python
# Byte-by-byte brute force via GDB Python API
import gdb, string

def bruteforce_flag(check_addr, success_addr, fail_addr, flag_len):
    flag = []
    for pos in range(flag_len):
        for ch in string.printable:
            candidate = ''.join(flag) + ch + 'A' * (flag_len - pos - 1)
            gdb.execute('start', to_string=True)
            gdb.execute(f'b *{check_addr}', to_string=True)
            # Write candidate to stdin pipe
            # ... (setup input)
            gdb.execute('continue', to_string=True)
            rip = int(gdb.parse_and_eval('$rip'))
            if rip == success_addr:
                flag.append(ch)
                break
        gdb.execute('delete breakpoints', to_string=True)
    return ''.join(flag)
```

### Conditional Breakpoints

```bash
# Break only when register has specific value
(gdb) b *0x401234 if $rax == 0x41
(gdb) b *0x401234 if *(char*)$rdi == 'f'

# Break on Nth hit
(gdb) b *0x401234
(gdb) ignore 1 99    # Skip first 99 hits, break on 100th

# Log without stopping
(gdb) b *0x401234
(gdb) commands
> silent
> printf "rax=%lx rdi=%lx\n", $rax, $rdi
> continue
> end
```

### Watchpoints

```bash
# Hardware watchpoint — break when memory changes
(gdb) watch *(int*)0x601050        # Break on write to address
(gdb) rwatch *(int*)0x601050       # Break on read
(gdb) awatch *(int*)0x601050       # Break on read or write

# Watch a variable by name (needs debug symbols)
(gdb) watch flag_buffer[0]

# Conditional watchpoint
(gdb) watch *(int*)0x601050 if *(int*)0x601050 == 0x42
```

### Reverse Debugging (rr)

```bash
# Record execution
rr record ./binary
# Replay with reverse execution support
rr replay

# In rr replay (GDB commands plus):
(gdb) reverse-continue     # Run backward to previous breakpoint
(gdb) reverse-stepi        # Step backward one instruction
(gdb) reverse-next         # Reverse next
(gdb) when                 # Show current event number

# Set checkpoint and return to it
(gdb) checkpoint
(gdb) restart 1           # Return to checkpoint 1
```

**Key use:** When you step past the critical moment, reverse back instead of restarting. Invaluable for anti-debug that corrupts state.

### GDB Dashboard / GEF / pwndbg

```bash
# pwndbg (most popular for CTF)
# https://github.com/pwndbg/pwndbg
git clone https://github.com/pwndbg/pwndbg && cd pwndbg && ./setup.sh

# Key pwndbg commands:
pwndbg> context           # Show registers, stack, code, backtrace
pwndbg> vmmap             # Memory map (like /proc/self/maps)
pwndbg> search -s "flag{" # Search memory for string
pwndbg> telescope $rsp 20 # Smart stack dump
pwndbg> cyclic 200        # Generate De Bruijn pattern
pwndbg> hexdump $rdi 64   # Pretty hex dump
pwndbg> got               # Show GOT entries
pwndbg> plt               # Show PLT entries

# GEF (alternative)
# https://github.com/hugsy/gef
bash -c "$(curl -fsSL https://gef.blah.cat/sh)"

# Key GEF commands:
gef> xinfo $rdi           # Detailed info about address
gef> checksec             # Binary security features
gef> heap chunks          # Heap chunk listing
gef> pattern create 100   # De Bruijn pattern
```

---

## Advanced Ghidra Scripting

```python
# Ghidra Python (Jython) — run via Script Manager or headless

# Batch rename functions matching a pattern
from ghidra.program.model.symbol import SourceType
fm = currentProgram.getFunctionManager()
for func in fm.getFunctions(True):
    if func.getName().startswith("FUN_"):
        # Check if function contains specific instruction pattern
        body = func.getBody()
        inst_iter = currentProgram.getListing().getInstructions(body, True)
        for inst in inst_iter:
            if inst.getMnemonicString() == "CPUID":
                func.setName("anti_vm_check_" + hex(func.getEntryPoint().getOffset()),
                            SourceType.USER_DEFINED)
                break

# Extract all XOR constants from a function
def extract_xor_constants(func):
    """Find all XOR operations and their immediate operands."""
    constants = []
    body = func.getBody()
    inst_iter = currentProgram.getListing().getInstructions(body, True)
    for inst in inst_iter:
        if inst.getMnemonicString() == "XOR":
            for i in range(inst.getNumOperands()):
                op = inst.getOpObjects(i)
                if op and hasattr(op[0], 'getValue'):
                    constants.append(int(op[0].getValue()))
    return constants

# Bulk decompile and search for pattern
from ghidra.app.decompiler import DecompInterface
decomp = DecompInterface()
decomp.openProgram(currentProgram)

for func in fm.getFunctions(True):
    result = decomp.decompileFunction(func, 30, monitor)
    if result.depiledFunction():
        code = result.getDecompiledFunction().getC()
        if "strcmp" in code or "memcmp" in code:
            print(f"Comparison in {func.getName()} at {func.getEntryPoint()}")
```

---

## Patching Strategies

### Binary Ninja Patching (Python API)

```python
import binaryninja as bn

bv = bn.open_view("binary")

# NOP out instruction
bv.write(0x401234, b"\x90" * 5)  # 5-byte NOP

# Patch conditional jump (JNZ → JZ)
bv.write(0x401234, b"\x74")  # 0x75 (JNZ) → 0x74 (JZ)

# Insert always-true (mov eax, 1; ret)
bv.write(0x401234, b"\xb8\x01\x00\x00\x00\xc3")

bv.save("patched")
```

### LIEF (Library for Instrumenting Executable Formats)

```python
import lief

# Parse and modify ELF/PE/Mach-O
binary = lief.parse("binary")

# Add a new section
section = lief.ELF.Section(".patch")
section.content = list(b"\xcc" * 0x100)
section.type = lief.ELF.SECTION_TYPES.PROGBITS
section.flags = lief.ELF.SECTION_FLAGS.EXECINSTR | lief.ELF.SECTION_FLAGS.ALLOC
binary.add(section)

# Modify entry point
binary.header.entrypoint = 0x401000

# Hook imported function
binary.patch_pltgot("strcmp", 0x401000)

binary.write("patched")
```

**LIEF advantages:** Cross-format (ELF, PE, Mach-O), Python API, can add sections/segments, modify headers, patch imports.

---

## GDB Constraint Extraction with ILP/LP Solver (BackdoorCTF 2017)

When a binary enforces linear arithmetic relationships between input bytes, extract constraints automatically via GDB and solve with an ILP solver.

**Technique:** Send position-encoded input (`input[i] = i`) so that when a comparison fires, you know exactly which positions are involved and what their sum/difference must equal. Collect all constraints from logged comparisons, then feed to PuLP or Gurobi.

```python
from pulp import *

n = 32  # flag length
prob = LpProblem("crackme", LpMinimize)
x = [LpVariable(f'x{i}', 32, 126, cat='Integer') for i in range(n)]
prob += 0  # dummy objective

# Constraints extracted via GDB automation (input[i]=i, monitor comparisons):
prob += x[3] + x[7] == 0xAB
prob += x[1] - x[5] == 0x0C
# ... add all extracted constraints ...

# Constrain to printable ASCII
for xi in x:
    prob += xi >= 32
    prob += xi <= 126

prob.solve(PULP_CBC_CMD(msg=0))
flag = ''.join(chr(int(value(xi))) for xi in x)
print("Flag:", flag)
```

**GDB automation to extract constraints:**
```python
# In GDB Python: set input[i]=i, run, log every CMP instruction result
import gdb

class CmpLogger(gdb.Breakpoint):
    def stop(self):
        frame = gdb.selected_frame()
        # Read compared values, map back to input indices via position encoding
        return False
```

**Key insight:** When a binary enforces linear arithmetic relationships between input bytes, ILP solvers directly find the satisfying assignment once constraints are extracted via GDB automation.

**References:** BackdoorCTF 2017

---

## GDB Position-Encoded Input with Zero Flag Monitoring (EKOPARTY 2017)

Send input where `input[i] = i` (position-encoded). Single-step through the binary monitoring the CPU zero flag (ZF). When ZF is set at a comparison involving a specific position's value, the comparison matched — log the expected value for that position.

```python
import gdb

# Script: single-step binary with position-encoded input, watch ZF
class ZFMonitor(gdb.Breakpoint):
    def stop(self):
        zf = (int(gdb.parse_and_eval('$eflags')) >> 6) & 1
        if zf:
            rip = int(gdb.parse_and_eval('$rip'))
            # Disassemble at rip to find the compared immediate
            disasm = gdb.execute(f'x/1i {rip-5}', to_string=True)
            print(f"ZF set at {rip:#x}: {disasm.strip()}")
        return False

# Run once with input b'\x00\x01\x02\x03...\x1f'
# ZF fires when comparison matches the position's own value -> that IS the key byte
```

Maps each input byte to its required value in one pass without manual reversing.

**Key insight:** Position-encoded input (`input[i]=i`) combined with zero flag monitoring reveals the full key/password in one pass — the zero flag fires when the expected value for position i equals i itself.

**References:** EKOPARTY CTF 2017

---

## LD_PRELOAD to Dump Execute-Only Binary (BackdoorCTF 2017)

A binary has execute-only permissions (mode `--x`, no read bit). The file cannot be read directly or with standard tools, but the kernel still maps it into memory on execution.

LD_PRELOAD a shared library with a constructor that runs inside the process and reads its own memory via `/proc/self/mem`:

```c
// dump_xo.c — compile: gcc -shared -fPIC -o dump_xo.so dump_xo.c
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

__attribute__((constructor)) void dump() {
    FILE *maps = fopen("/proc/self/maps", "r");
    char line[256];
    unsigned long base = 0, end = 0;

    // Find the execute-only binary's mapping (r-xp or --xp)
    while (fgets(line, sizeof(line), maps)) {
        if (strstr(line, "binary_name")) {
            sscanf(line, "%lx-%lx", &base, &end);
            break;
        }
    }
    fclose(maps);

    FILE *mem = fopen("/proc/self/mem", "rb");
    fseek(mem, base, SEEK_SET);
    size_t size = end - base;
    void *buf = malloc(size);
    fread(buf, 1, size, mem);
    fclose(mem);

    FILE *out = fopen("/tmp/dumped_binary", "wb");
    fwrite(buf, 1, size, out);
    fclose(out);
}
// Usage: LD_PRELOAD=./dump_xo.so ./binary_xo
```

**Key insight:** Execute-only prevents file reading but not execution. LD_PRELOAD constructors run inside the process where `/proc/self/mem` provides access to mapped memory regardless of file permissions.

**References:** BackdoorCTF 2017

---

## PEDA current_inst Bit-by-Bit Flag Scraper (CONFidence CTF 2019 Teaser)

**Pattern (Elementary):** A large obfuscated validator dispatches one `call functionN` per flag bit. Each check reads `flag[offset] >> bit & 1`, calls an opaque `functionN(that_bit)`, and compares the return with a constant. Rather than decompile hundreds of wrapper functions, drive GDB+PEDA to step through the dispatcher and read `edi` (argument) / `eax` (return) directly:

```python
# peda.current_inst(rip) returns (addr, mnemonic_str); use it as a cheap
# disassembler to track the shift/add offsets passed to each checker.
def get_current_inst():
    return peda.current_inst(peda.getreg("rip"))[1]

peda.execute('file ./elementary')
peda.set_breakpoint(0x555555554000 + 0xCEB88)
peda.execute("run < _input")         # _input = 'A' * 103 (length only)

flag = ['0'] * 832                    # 104 bytes * 8 bits
while peda.getreg("rip") < 0x555555554000 + 0xD827F:
    offset = bit = 0
    while 'and' != get_current_inst()[:3]:
        cur = get_current_inst()
        if cur == 'mov    rax,QWORD PTR [rbp-0x18]':
            offset = 0; bit = 0
        elif cur.startswith('sar'):
            bit = int(cur.split(',')[1], 16)     # shift count -> bit index
        elif cur.startswith('add'):
            offset = int(cur.split(',')[1], 16)  # byte index in flag
        peda.execute('si')
    while 'call' not in get_current_inst(): peda.execute('si')
    tmp = peda.getreg('edi')
    peda.execute('ni')
    ret = peda.getreg('eax')
    # Oracle: if return == arg the checker voted "bit=0", else "bit=1"
    flag[offset * 8 + bit] = '0' if (ret != 0 and tmp == ret) else '1'
    peda.execute('set $eax=0')        # neutralise so loop continues
```

**Key insight:** Any validator of the form `f_i(bit_i) == const_i` is a black-box oracle — you do not need to understand `f_i`. PEDA's `current_inst()` + `si`/`ni` give a 30-line Python scraper that harvests all bits in one run; parsing the preceding `sar imm` / `add imm` instructions recovers `(byte_offset, bit_index)` without disassembling the validator's arithmetic.

**References:** CONFidence CTF 2019 Teaser — Elementary, writeup 13927
