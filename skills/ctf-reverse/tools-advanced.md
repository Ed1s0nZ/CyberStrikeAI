# CTF Reverse - Advanced Tools & Deobfuscation

Advanced tooling for commercial packers/protectors, binary diffing, deobfuscation frameworks, emulation, and symbolic execution beyond angr.

For advanced GDB scripting, Ghidra automation, patching frameworks, and GDB-driven CTF techniques, see [tools-advanced-2.md](tools-advanced-2.md).

## Table of Contents
- [VMProtect Analysis](#vmprotect-analysis)
  - [Recognition](#recognition)
  - [Approach](#approach)
  - [Tools](#tools)
  - [CTF Strategy](#ctf-strategy)
- [Themida / WinLicense Analysis](#themida--winlicense-analysis)
  - [Themida Recognition](#themida-recognition)
  - [Approach for CTF](#approach-for-ctf)
- [Binary Diffing](#binary-diffing)
  - [BinDiff](#bindiff)
  - [Diaphora](#diaphora)
- [Deobfuscation Frameworks](#deobfuscation-frameworks)
  - [D-810 (IDA)](#d-810-ida)
  - [GOOMBA (Ghidra)](#goomba-ghidra)
  - [Miasm](#miasm)
- [Qiling Framework (Emulation)](#qiling-framework-emulation)
- [Triton (Dynamic Symbolic Execution)](#triton-dynamic-symbolic-execution)
- [Manticore (Symbolic Execution)](#manticore-symbolic-execution)
- [Rizin / Cutter](#rizin--cutter)
- [RetDec (Retargetable Decompiler)](#retdec-retargetable-decompiler)
- [Custom VM Bytecode Lifting to LLVM IR (Google CTF 2017)](#custom-vm-bytecode-lifting-to-llvm-ir-google-ctf-2017)

---

## VMProtect Analysis

VMProtect virtualizes x86/x64 code into custom bytecode interpreted by a generated VM. One of the most challenging protectors in CTF.

### Recognition

```bash
# VMProtect signatures
strings binary | grep -i "vmp\|vmprotect"
# PE sections: .vmp0, .vmp1 (VMProtect adds its own sections)
readelf -S binary | grep ".vmp"
# Large binary with entropy > 7.5 in certain sections
```

**Key indicators:**
- `push` / `pop` heavy prologues (VM entry pushes all registers to stack)
- Large switch-case dispatcher (the VM handler loop)
- Anti-debug checks embedded in VM handlers
- Mutation engine: same opcode has different handlers per build

### Approach

```text
1. Identify VM entry points — look for pushad/pushaq-like sequences
2. Find the handler table — large indirect jump (jmp [reg + offset])
3. Trace handler execution — each handler ends with jump to next
4. Identify handlers:
   - vAdd, vSub, vMul, vXor, vNot (arithmetic)
   - vPush, vPop (stack operations)
   - vLoad, vStore (memory access)
   - vJmp, vJcc (control flow)
   - vRet (VM exit — restores real registers)
5. Build disassembler for VM bytecode
6. Simplify / deobfuscate the lifted IL
```

### Tools

- **VMPAttack** (IDA plugin): Automatically identifies VM handlers
- **NoVmp**: Devirtualization via VTIL (open-source)
- **VMProtect devirtualizer scripts**: Community IDA/Binary Ninja scripts
- **Approach for CTF:** Often easier to trace specific operations (crypto, comparisons) than fully devirtualize

### CTF Strategy

```python
# Trace VM execution dynamically to extract operations on flag
# Hook VM handler dispatch to log opcode + operands

import frida

script = """
var vm_dispatch = ptr('0x...');  // Address of handler table jump
Interceptor.attach(vm_dispatch, {
    onEnter(args) {
        // Log handler index and stack state
        var handler_idx = this.context.rax;  // or whichever register
        console.log('Handler:', handler_idx, 'RSP:', this.context.rsp);
    }
});
"""
```

**Key insight:** Full devirtualization is rarely needed for CTF. Focus on tracing what operations are performed on your input. Hook comparison/crypto functions called from within the VM.

---

## Themida / WinLicense Analysis

Similar to VMProtect but with additional anti-debug layers.

### Themida Recognition
- Sections: `.themida`, `.winlice`
- Extremely heavy anti-debug (kernel-level checks, driver installation)
- Code mutation + virtualization + packing combined

### Approach for CTF
1. **Dump unpacked code:** Let it run, dump process memory after unpacking
2. **Bypass anti-debug:** ScyllaHide in x64dbg with Themida-specific preset
3. **Fix imports:** Use Scylla plugin for IAT reconstruction
4. **Focus on dumped code:** Once unpacked, analyze as normal binary

```bash
# x64dbg workflow for Themida:
1. Load binary
2. Enable ScyllaHide → Profile: Themida
3. Run to OEP (Original Entry Point) — may need several attempts
4. Dump with Scylla: OEP → IAT Autosearch → Get Imports → Dump
5. Fix dump: Scylla → Fix Dump
6. Analyze fixed dump in Ghidra/IDA
```

---

## Binary Diffing

Critical for patch analysis, 1-day exploit development, and CTF challenges that provide two versions of a binary.

### BinDiff

```bash
# Export from IDA/Ghidra first, then diff
# IDA: File → BinExport → Export as BinExport2
# Ghidra: Use BinExport plugin

# Command line diffing
bindiff primary.BinExport secondary.BinExport
# Opens in BinDiff GUI — shows matched/unmatched functions
```

**Key metrics:**
- Similarity score (0.0-1.0) per function pair
- Changed instructions highlighted
- Unmatched functions = new/removed code

### Diaphora

Free, open-source alternative to BinDiff, runs as IDA plugin.

```bash
# In IDA:
# File → Script file → diaphora.py
# Export first binary, then open second and diff

# Ghidra version: diaphora_ghidra.py
```

**Useful for CTF:** When challenge provides "patched" and "original" binaries, diff reveals the vulnerability or hidden functionality.

---

## Deobfuscation Frameworks

### D-810 (IDA)

Pattern-based deobfuscation plugin for IDA Pro. Excellent for OLLVM-obfuscated binaries.

```text
Capabilities:
- MBA simplification: (a ^ b) + 2*(a & b) → a + b
- Dead code elimination
- Opaque predicate removal
- Constant folding
- Control flow unflattening (partial)

Installation: Copy to IDA plugins directory
Usage: Edit → Plugins → D-810 → Select rules → Apply
```

### GOOMBA (Ghidra)

```text
GOOMBA (Ghidra-based Obfuscated Object Matching and Bytes Analysis):
- Integrates with Ghidra's P-Code
- Simplifies MBA expressions
- Pattern matching for known obfuscation

Installation: Copy .jar to Ghidra extensions
Usage: Code Browser → Analysis → GOOMBA
```

### Miasm

Powerful reverse engineering framework with symbolic execution and IR lifting.

```python
from miasm.analysis.binary import Container
from miasm.analysis.machine import Machine
from miasm.expression.expression import *

# Load binary and lift to Miasm IR
cont = Container.from_stream(open("binary", "rb"))
machine = Machine(cont.arch)
mdis = machine.dis_engine(cont.bin_stream, loc_db=cont.loc_db)

# Disassemble function
asmcfg = mdis.dis_multiblock(entry_addr)

# Lift to IR
lifter = machine.lifter_model_call(loc_db=cont.loc_db)
ircfg = lifter.new_ircfg_from_asmcfg(asmcfg)

# Symbolic execution
from miasm.ir.symbexec import SymbolicExecutionEngine
sb = SymbolicExecutionEngine(lifter)
# Execute symbolically, then simplify expressions
```

**Use case:** Deobfuscate expression trees, simplify complex arithmetic, trace data flow through obfuscated code.

---

## Qiling Framework (Emulation)

Cross-platform emulation framework built on Unicorn, with OS-level support (syscalls, filesystem, registry).

```python
from qiling import Qiling
from qiling.const import QL_VERBOSE

# Emulate Linux ELF
ql = Qiling(["./binary"], "rootfs/x8664_linux",
            verbose=QL_VERBOSE.DEBUG)

# Hook specific address
@ql.hook_address
def hook_check(ql, address, size):
    if address == 0x401234:
        ql.arch.regs.rax = 0  # Bypass check
        ql.log.info("Anti-debug bypassed")

# Hook syscall
@ql.hook_syscall(name="ptrace")
def hook_ptrace(ql, request, pid, addr, data):
    return 0  # Always succeed

# Hook API (Windows)
@ql.set_api("IsDebuggerPresent", target=ql.os.user_defined_api)
def hook_isdebug(ql, address, params):
    return 0

ql.run()
```

**Advantages over Unicorn:**
- OS emulation (file I/O, network, registry)
- Multi-platform (Linux, Windows, macOS, Android, UEFI)
- Built-in debugger interface
- Rootfs for library loading

**CTF use cases:**
- Emulate binaries for foreign architectures (ARM, MIPS, RISC-V)
- Bypass all anti-debug at once (no debugger artifacts)
- Fuzz embedded/IoT firmware without hardware
- Trace execution without code modification

---

## Triton (Dynamic Symbolic Execution)

Pin-based dynamic binary analysis framework with symbolic execution, taint analysis, and AST simplification.

```python
from triton import *

ctx = TritonContext(ARCH.X86_64)

# Load binary sections
with open("binary", "rb") as f:
    binary = f.read()
ctx.setConcreteMemoryAreaValue(0x400000, binary)

# Symbolize input
for i in range(32):
    ctx.symbolizeMemory(MemoryAccess(INPUT_ADDR + i, CPUSIZE.BYTE), f"input_{i}")

# Emulate instructions
pc = ENTRY_POINT
while pc:
    inst = Instruction(pc, ctx.getConcreteMemoryAreaValue(pc, 16))
    ctx.processing(inst)

    # At comparison point, extract path constraint
    if pc == CMP_ADDR:
        ast = ctx.getPathConstraintsAst()
        model = ctx.getModel(ast)
        for k, v in sorted(model.items()):
            print(f"input[{k}] = {chr(v.getValue())}", end="")
        break

    pc = ctx.getConcreteRegisterValue(ctx.registers.rip)
```

**Triton vs angr:**
| Feature | Triton | angr |
|---|---|---|
| Execution | Concrete + symbolic (DSE) | Fully symbolic |
| Speed | Faster (concrete-driven) | Slower (explores all paths) |
| Path explosion | Less prone (follows one path) | Major issue |
| API | C++ / Python | Python |
| Best for | Single-path deobfuscation, taint tracking | Multi-path exploration |

**Key use:** Triton excels at deobfuscation — run the program concretely, but track symbolic state, then simplify the collected constraints.

---

## Manticore (Symbolic Execution)

Trail of Bits' symbolic execution tool. Similar to angr but with native EVM (Ethereum) support.

```python
from manticore.native import Manticore

m = Manticore("./binary")

# Hook success/failure
@m.hook(0x401234)
def success(state):
    buf = state.solve_one_n_batched(state.input_symbols, 32)
    print("Flag:", bytes(buf))
    m.kill()

@m.hook(0x401256)
def fail(state):
    state.abandon()

m.run()
```

**Best for:** EVM/smart contract analysis, simpler Linux binaries. angr is generally more mature for complex RE tasks.

---

## Rizin / Cutter

Rizin is the maintained fork of radare2. Cutter is its Qt-based GUI.

```bash
# Rizin CLI (r2-compatible commands)
rizin -d ./binary
> aaa                    # Analyze all
> afl                    # List functions
> pdf @ main             # Print disassembly
> VV                     # Visual graph mode

# Cutter GUI
cutter binary           # Open in GUI with decompiler
```

**Cutter advantages:**
- Built-in Ghidra decompiler (via r2ghidra plugin)
- Graph view, hex editor, debug panel in one GUI
- Integrated Python/JavaScript scripting console
- Free and open source

---

## RetDec (Retargetable Decompiler)

LLVM-based decompiler supporting many architectures. Free and open-source.

```bash
# Install
pip install retdec-decompiler
# Or use web: https://retdec.com/decompilation/

# CLI
retdec-decompiler binary
# Outputs: binary.c (decompiled C), binary.dsm (disassembly)

# Specific function
retdec-decompiler --select-ranges 0x401000-0x401100 binary
```

**Strengths:** Multi-arch support (x86, ARM, MIPS, PowerPC, PIC32), free, produces compilable C. Good for architectures not well-supported by Ghidra.

---

## Custom VM Bytecode Lifting to LLVM IR (Google CTF 2017)

For complex custom VMs, transpile the VM bytecode to LLVM IR and use LLVM's optimization passes to simplify the code, then decompile the optimized IR.

```python
# Pipeline: VM bytecode → custom disassembler → LLVM IR → optimize → decompile
# 1. Write disassembler for the custom VM opcodes
# 2. Emit LLVM IR for each opcode:
#    INC reg  → %reg = add i32 %reg, 1
#    CDEC reg → conditional decrement
#    CALL fn  → call void @fn()
# 3. Use MCJIT or llc to optimize:
#    opt -O3 -S vm_lifted.ll -o vm_optimized.ll
# 4. Load optimized IR in IDA or decompile with RetDec
# Result: 1300 lines → 150 lines after inlining + constant folding
```

**Key insight:** LLVM's optimization passes (inlining, constant folding, dead code elimination) dramatically simplify lifted VM bytecode. A custom VM with 26 registers and 3 opcodes that produces 1300 lines of IL reduces to ~150 lines after `-O3`, revealing the underlying algorithm (e.g., Collatz sequence computation).
