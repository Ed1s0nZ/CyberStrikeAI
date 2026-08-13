# CTF Reverse - Hardware and Advanced Architecture Reversing

HD44780 LCD GPIO reconstruction, RISC-V advanced extensions and debugging, ARM64/AArch64 reversing and exploitation.

## Table of Contents
- [HD44780 LCD Controller GPIO Reconstruction (32C3 2015)](#hd44780-lcd-controller-gpio-reconstruction-32c3-2015)
- [RISC-V (Advanced)](#risc-v-advanced)
  - [Custom Extensions](#custom-extensions)
  - [Privileged Modes](#privileged-modes)
  - [RISC-V Debugging](#risc-v-debugging)
- [ARM64/AArch64 Reversing and Exploitation](#arm64aarch64-reversing-and-exploitation)
- [MIPS64 Cavium OCTEON Coprocessor 2 Crypto (SEC-T CTF 2017)](#mips64-cavium-octeon-coprocessor-2-crypto-sec-t-ctf-2017)
- [EFM32 ARM Microcontroller MMIO AES (SEC-T CTF 2017)](#efm32-arm-microcontroller-mmio-aes-sec-t-ctf-2017)
- [MBR/Bootloader Reversing with QEMU + GDB (Square CTF 2017)](#mbrbootloader-reversing-with-qemu--gdb-square-ctf-2017)
- [Game Boy ROM Z80 Analysis in bgb Debugger (Square CTF 2017)](#game-boy-rom-z80-analysis-in-bgb-debugger-square-ctf-2017)
- [KVM Guest Analysis via ioctl + KVM_EXIT_HLT Block Chaining (CSAW 2018)](#kvm-guest-analysis-via-ioctl--kvm_exit_hlt-block-chaining-csaw-2018)
- [Coreboot ROM XOR-Pair Bit-Flip Address Discovery (Hack.lu 2018)](#coreboot-rom-xor-pair-bit-flip-address-discovery-hacklu-2018)

---

## HD44780 LCD Controller GPIO Reconstruction (32C3 2015)

Recover text displayed on an HD44780 LCD from raw Raspberry Pi GPIO recordings:

1. **Identify signal lines:** Map GPIO pins to HD44780 signals (RS, CLK, D4-D7 for 4-bit mode)
2. **Clock edge detection:** Sample data lines on falling clock edges (1->0 transition)
3. **Nibble assembly:** Combine two 4-bit samples into one 8-bit command/data byte
4. **DRAM address mapping:** HD44780 uses non-contiguous addressing for multi-line displays:
   - Line 0: 0x00-0x27
   - Line 1: 0x40-0x67
   - Line 2: 0x14-0x3B
   - Line 3: 0x54-0x7B

```python
display = [' '] * 80  # 4 lines x 20 chars
cursor = 0

for timestamp, gpio_state in sorted(gpio_log):
    if falling_edge(gpio_state, CLK_PIN):
        nibble = extract_data_bits(gpio_state)
        byte = assemble_nibble(nibble)  # Two nibbles per byte
        if rs_high(gpio_state):  # RS=1: data write
            display[dram_to_position(cursor)] = chr(byte)
            cursor += 1
        else:  # RS=0: command (set cursor, clear, etc.)
            cursor = parse_command(byte)
```

**Key insight:** GPIO pin-to-signal mapping is rarely documented; identify CLK by finding the pin with most transitions, RS by correlation with data patterns (alternating command/data phases).

---

## RISC-V (Advanced)

Beyond basic disassembly (see [tools.md](tools.md#risc-v-binary-analysis-ehax-2026)):

### Custom Extensions

```text
Bitmanip extensions (Zbb, Zbc, Zbs):
  clz, ctz, cpop         -> count leading/trailing zeros, popcount
  orc.b, rev8            -> byte-level bit manipulation
  andn, orn, xnor        -> negated logic operations
  clmul, clmulh, clmulr  -> carry-less multiplication (crypto)
  bset, bclr, binv, bext -> single-bit operations

Crypto extensions (Zk*):
  aes32esi, aes32dsmi     -> AES round operations
  sha256sig0, sha512sum0  -> SHA hash acceleration
  sm3p0, sm4ed            -> Chinese crypto standards
```

### Privileged Modes

```text
Machine mode (M):  Highest privilege, firmware/bootloader
Supervisor mode (S): OS kernel
User mode (U):      Applications

CSR registers to watch:
  mstatus/sstatus    -> privilege level, interrupt enable
  mtvec/stvec       -> trap handler address
  mepc/sepc         -> exception return address
  mcause/scause     -> trap cause
  satp              -> page table root (virtual memory)
```

### RISC-V Debugging

```bash
# OpenOCD + GDB for hardware debugging
openocd -f interface/jlink.cfg -f target/riscv.cfg

# GDB for RISC-V
riscv64-unknown-elf-gdb binary
(gdb) target remote :3333

# QEMU with GDB server
qemu-riscv64 -g 1234 -L /usr/riscv64-linux-gnu/ ./binary
riscv64-linux-gnu-gdb -ex 'target remote :1234' ./binary
```

---

## ARM64/AArch64 Reversing and Exploitation

AArch64 (ARM 64-bit) appears in mobile apps, cloud servers (AWS Graviton), Apple Silicon, and CTF challenges. Key differences from x86-64 affect both reversing and exploitation.

**Setup and emulation:**

```bash
# Install cross-toolchain and emulator
apt install gcc-aarch64-linux-gnu gdb-multiarch qemu-user-static

# Run AArch64 binary on x86 host
qemu-aarch64-static -L /usr/aarch64-linux-gnu/ ./arm64_binary

# Debug with GDB
qemu-aarch64-static -g 12345 -L /usr/aarch64-linux-gnu/ ./arm64_binary &
gdb-multiarch -ex 'set arch aarch64' -ex 'target remote :1234' ./arm64_binary

# With library preloading (for challenges that ship libc)
qemu-aarch64-static -g 12345 -E LD_PRELOAD=./libc.so.6 -L ./lib ./arm64_binary
```

**AArch64 calling convention (key differences from x86-64):**

```text
Registers:
  x0-x7    -- function arguments AND return values (x0 = first arg / return)
  x8       -- indirect result location (struct returns)
  x9-x15   -- caller-saved temporaries
  x19-x28  -- callee-saved (preserved across calls)
  x29 (fp) -- frame pointer
  x30 (lr) -- link register (return address, NOT on stack by default)
  sp       -- stack pointer (must be 16-byte aligned)
  xzr      -- zero register (reads as 0, writes discarded)

Key exploitation differences:
  - Return address in LR (x30), not on stack -- pushed only if function calls others
  - No RIP-relative addressing like x86 -- uses ADRP+ADD pairs for PC-relative loads
  - Fixed 4-byte instruction width -- no variable-length gadget tricks
  - NOP = 0xD503201F (not 0x90)
  - BLR x8 / BR x30 -- indirect calls/jumps use register operands
```

**Common AArch64 patterns in Ghidra/IDA:**

```text
# PC-relative address loading (equivalent to x86 LEA):
ADRP  x0, #0x411000      ; Load page address (4KB aligned)
ADD   x0, x0, #0x8       ; Add page offset -> x0 = 0x411008

# Function prologue:
STP   x29, x30, [sp, #-0x30]!  ; Push fp + lr, decrement sp
MOV   x29, sp                   ; Set frame pointer

# Function epilogue:
LDP   x29, x30, [sp], #0x30    ; Pop fp + lr, increment sp
RET                              ; Branch to x30 (lr)

# Switch/jump table:
ADR   x1, jump_table
LDRB  w2, [x1, x0]       ; Load offset byte
ADD   x1, x1, w2, SXTB   ; Sign-extend and add
BR    x1                   ; Indirect branch
```

**ROP on AArch64:**

```python
from pwn import *

# AArch64 gadgets differ from x86:
# - "pop {x0}; ret" equivalent: LDP x0, x1, [sp], #0x10; RET
# - Prologue gadgets: LDP x29, x30, [sp, #0x20]; ... RET
# - system() call: x0 = pointer to "/bin/sh", BLR to system

context.arch = 'aarch64'
elf = ELF('./arm64_binary')

# Common gadget pattern in AArch64 libc:
# LDP X19, X20, [SP,#var_s10]
# LDP X29, X30, [SP+var_s0],#0x20
# RET
# Controls x19, x20, x29, x30 and advances sp by 0x20
```

**Key insight:** AArch64's fixed instruction width and register-based return address (`lr`/`x30`) make ROP gadgets more constrained than x86. Look for `LDP` (load pair) gadgets that pop multiple registers from the stack. The `STP`/`LDP` instruction pairs that save/restore callee-saved registers in function prologues/epilogues are the primary gadget source.

**When to recognize:** `file` shows "ELF 64-bit LSB ... ARM aarch64". Ghidra auto-detects but may need manual processor selection for raw binaries. Use `qemu-aarch64-static` for emulation on x86 hosts.

**Tools:** radare2 (`r2 -AA -a arm -b 64`), Ghidra (auto-detect), `aarch64-linux-gnu-objdump -d`, Unicorn Engine (`UC_ARCH_ARM64`)

**References:** Google CTF 2016 "Forced Puns", Insomni'hack 2018 "onecall"

---

## MIPS64 Cavium OCTEON Coprocessor 2 Crypto (SEC-T CTF 2017)

Cavium OCTEON network processors implement hardware AES and SHA256 via MIPS Coprocessor 2 (CP2) using `dmtc2` (move to CP2) and `dmfc2` (move from CP2) instructions. These look like ordinary register moves to a disassembler but drive the hardware crypto engine.

**Key CP2 register layout (OCTEON):**
```text
AES key registers:
  0x0104 – AES key quadword 0
  0x0105 – AES key quadword 1
  0x0106 – AES key quadword 2
  0x0107 – AES key quadword 3

SHA256 hash registers:
  0x400E–0x4012 – SHA256 intermediate hash words
  0x404F        – SHA256 control/result

dmtc2  rN, 0x0104   ; load 64 bits of AES key into CP2 register 0x104
dmtc2  rN, 0x0105   ; ...next quadword
```

**Approach:**
1. Disassemble in IDA/Ghidra — `dmtc2`/`dmfc2` with selector in 0x100-0x40FF range indicates OCTEON CP2
2. Cross-reference the Cavium OCTEON Hardware Reference Manual for register semantics
3. Trace the key loading sequence to recover the AES or HMAC key material

**Key insight:** Hardware crypto accelerators on MIPS appear as CP2 register writes (`dmtc2`/`dmfc2`). Identify the base register address and cross-reference vendor documentation.

**References:** SEC-T CTF 2017

---

## EFM32 ARM Microcontroller MMIO AES (SEC-T CTF 2017)

Silicon Labs EFM32 Cortex-M binary — a flat binary loaded at 0x1000 in Thumb mode.

**IDA setup:**
```text
Processor: ARM Little-endian (ARMv7-M)
Load address: 0x1000
Set T register = 1 (force Thumb mode decoding)
```

**AES accelerator MMIO layout (EFM32 AES peripheral at 0x400E0000):**
```text
0x400E0000 + 0x000  CTRL   – enable, decrypt mode
0x400E0000 + 0x004  CMD    – start/stop
0x400E0000 + 0x010  KEYLA  – key low word 0
0x400E0000 + 0x014  KEYLB  – key low word 1
0x400E0000 + 0x018  KEYLC  – key low word 2
0x400E0000 + 0x01C  KEYLD  – key low word 3
```

The binary loads two separate values, XORs them together, then writes the result as the AES key. Decrypt the embedded ciphertext block with the composed key in ECB mode.

```python
from Crypto.Cipher import AES

key_part_a = bytes.fromhex("...")  # extracted from IDA .data section
key_part_b = bytes.fromhex("...")  # second value
key = bytes(a ^ b for a, b in zip(key_part_a, key_part_b))

cipher = AES.new(key, AES.MODE_ECB)
plaintext = cipher.decrypt(ciphertext)
```

**Key insight:** Hardware AES accelerators on microcontrollers appear as MMIO register writes at a specific base address — cross-reference the vendor reference manual (EFM32 Reference Manual for Silicon Labs peripherals).

**References:** SEC-T CTF 2017

---

## MBR/Bootloader Reversing with QEMU + GDB (Square CTF 2017)

Boot a floppy/disk image in QEMU with the GDB stub enabled, then attach GDB for full source-level debugging of 16-bit real mode or 32-bit protected mode bootloader code.

```bash
# Boot with GDB stub on port 1234; -S pauses execution at start
qemu-system-x86_64 -fda disk.img -s -S

# In another terminal, attach GDB
gdb -ex "set architecture i8086" \
    -ex "target remote :1234" \
    -ex "break *0x7c00" \
    -ex "continue"

# Common MBR entry point is 0x7c00 (BIOS loads MBR here)
# Step through bootloader, inspect registers and memory:
(gdb) x/20i $pc
(gdb) info registers
(gdb) x/16xb 0x7c00
```

To bypass a password check: identify the conditional jump after the comparison and NOP it out in the image file, or patch the comparison to always succeed.

```bash
# Find the comparison offset in the image and patch it
python3 -c "
data = open('disk.img', 'rb').read()
# Replace JNZ (0x75) with JMP-short-always or NOP
data = data[:offset] + b'\x90\x90' + data[offset+2:]
open('disk_patched.img', 'wb').write(data)
"
```

**Key insight:** QEMU's `-s` flag exposes a GDB stub on port 1234 for full debugging of MBR/bootloader code — workflow identical to userland debugging.

**References:** Square CTF 2017

---

## Game Boy ROM Z80 Analysis in bgb Debugger (Square CTF 2017)

Game Boy ROMs use the Sharp SM83 (LR35902) CPU, a Z80/8080 hybrid. Load the ROM in the **bgb** emulator which provides GDB-like debugging: breakpoints, memory inspection, and register display.

**Key instructions for flag comparisons:**
```asm
LD   A, [HL]    ; load byte from memory pointed to by HL into A
AND  [HL]       ; A = A & *HL  — compares player byte against memory value
CP   N          ; compare A with immediate N (sets Z flag if equal)
```

When `and (hl)` or `cp (hl)` fires during input validation, the expected byte is visible at the `(hl)` address in the memory view.

**bgb workflow:**
1. Load ROM: File → Open ROM
2. Right-click disassembly → "Run to cursor" or set breakpoint (F2)
3. When comparison fires, inspect Registers panel (HL value) and Memory panel (`*HL`)
4. Note expected value, advance to next comparison position

**Key insight:** Game Boy ROMs are Z80/SM83 architecture. The bgb debugger provides GDB-like functionality; key comparisons use `(hl)`-indirect addressing so the expected value is directly visible in the memory view during the comparison.

**References:** Square CTF 2017

---

## KVM Guest Analysis via ioctl + KVM_EXIT_HLT Block Chaining (CSAW 2018)

**Pattern:** A userspace process hosts a KVM-based VM whose guest "program" is nothing but a sequence of code blocks that end in `HLT`. The host handler reads `KVM_EXIT_HLT`, inspects guest registers, and dispatches to the next block by looking up `rax` in a jump table at `0x2020A0`. Reverse the whole program by (1) running the binary under `strace -v` to capture `KVM_GET_REGS`/`KVM_SET_REGS` pairs, (2) dumping each code block from the KVM memory region, and (3) reconstructing the dispatch graph from the host's rax-indexed table.

```bash
# 1. Observe KVM ioctls + register snapshots
strace -v -e ioctl ./challenge 2>&1 | grep -E "KVM_RUN|KVM_(GET|SET)_REGS"

# 2. Dump guest code memory (offset + size from KVM_SET_USER_MEMORY_REGION ioctl)
gdb -batch -ex "attach $(pgrep challenge)" \
    -ex "dump binary memory guest.bin 0x400000 0x410000" \
    -ex "detach"

# 3. Disassemble each HLT-terminated block
objdump -D -b binary -m i386:x86-64 guest.bin | less
```

```python
# Rebuild the dispatch graph
import struct
with open("challenge", "rb") as f:
    data = f.read()
# Host table at 0x2020A0 maps rax → next block offset
table = struct.unpack_from("<128Q", data, 0x2020A0)
for rax, ptr in enumerate(table):
    if ptr:
        print(f"rax={rax:02x} → block {ptr:#x}")
```

**Key insight:** KVM-backed challenges hide control flow by moving it into the host process, not the guest. The guest code itself is just a pile of opaque blocks. `strace` on the KVM ioctls replaces a debugger: every `KVM_EXIT_HLT` is a "basic block boundary" and the host's response is the real transition function. Any time you see a CTF binary linked against `-lkvm` or opening `/dev/kvm`, drop your normal reverse-engineering pipeline and start from the ioctl trace.

**References:** CSAW CTF Qualification Round 2018 — kvm, writeup 11206

---

## Coreboot ROM XOR-Pair Bit-Flip Address Discovery (Hack.lu 2018)

**Pattern:** A firmware image loads its flag location by XOR-ing two constants from ROM at boot. The challenge service lets the attacker flip a single bit anywhere in the ROM and observe the new flag address. Compute the intended address `X = C1 ^ C2`, compare against the advertised address, and the bit that differs — always exactly one bit away in a well-designed challenge — tells you which ROM byte + bit pair controls the redirect.

```python
# Two constants in ROM
C1 = 0xEF56BF92
C2 = 0xEF5A3F92
intended = C1 ^ C2          # 0xC8000 per the source
actual   = 0xC0000          # where the flag really lives in memory

diff = intended ^ actual    # 0x08000 → bit 15
# Find the ROM offset that, when a single bit is flipped, produces `actual`.
# The flip must land in either C1 or C2 so the XOR result has bit 15 cleared.
```

**Key insight:** XOR of two ROM constants is trivially a linear operation: any single-bit flip in either operand XORs the corresponding bit into the result. Walk the Hamming distance between computed and observed addresses and you get a bounded list of candidate patch sites — usually one or two. Pairs of addresses in firmware are suspicious: they frequently compose via XOR, ADD, or SUB and every arithmetic relation is a candidate for a targeted single-bit flip attack. Also applies to rowhammer: the sensitive bit is the *differential* between two constants, not the constants themselves.

**References:** Hack.lu CTF 2018 — 1-bit-missile, writeups 11862, 11865
