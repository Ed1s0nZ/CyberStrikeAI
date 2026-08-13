# CTF Crypto - PRNG & Key Recovery

Foundational PRNG state/seed recovery techniques. For CTF-era advanced attacks (MT19937 constraint propagation, Rule 86 cellular automaton, Java LCG MITM, LFSR bit-fold, Z3 timing oracle, randcrack DSA, NTP-poisoned UUID), see [prng-attacks.md](prng-attacks.md).

## Table of Contents
- [Mersenne Twister (MT19937) State Recovery](#mersenne-twister-mt19937-state-recovery)
- [MT State Recovery from random.random() Floats via GF(2) Matrix (PHD CTF Quals 2012)](#mt-state-recovery-from-randomrandom-floats-via-gf2-matrix-phd-ctf-quals-2012)
- [Time-Based Seed Attacks](#time-based-seed-attacks)
- [C srand/rand Synchronization via Python ctypes](#c-srandrand-synchronization-via-python-ctypes)
- [Layered Encryption Recovery](#layered-encryption-recovery)
- [LCG Parameter Recovery Attack](#lcg-parameter-recovery-attack)
- [ChaCha20 Key Recovery](#chacha20-key-recovery)
- [GF(2) Matrix PRNG Seed Recovery (0xFun 2026)](#gf2-matrix-prng-seed-recovery-0xfun-2026)
- [Middle-Square PRNG Brute Force (UTCTF 2024)](#middle-square-prng-brute-force-utctf-2024)
- [Deterministic RNG from Flag Bytes + Hill Climbing (VuwCTF 2025)](#deterministic-rng-from-flag-bytes--hill-climbing-vuwctf-2025)
- [Byte-by-Byte Oracle with Random Mode Matching (VuwCTF 2025)](#byte-by-byte-oracle-with-random-mode-matching-vuwctf-2025)
- [RSA Key Reuse / Replay (UTCTF 2024)](#rsa-key-reuse--replay-utctf-2024)
- [Password Cracking Strategy](#password-cracking-strategy)
- [Logistic Map / Chaotic PRNG Seed Recovery (BYPASS CTF 2025)](#logistic-map--chaotic-prng-seed-recovery-bypass-ctf-2025)
- [V8 XorShift128+ State Recovery (Math.random Prediction)](#v8-xorshift128-state-recovery-mathrandom-prediction)

See [prng-attacks.md](prng-attacks.md) for CTF-era advanced attacks (2017+).

---

## Mersenne Twister (MT19937) State Recovery

Python's `random` module uses Mersenne Twister. If you can observe outputs, you can recover the state and predict future values.

**Key properties:**
- 624 × 32-bit internal state
- Each output is tempered from state
- After 624 outputs, state is twisted (regenerated)

**Basic untemper (reverse single output):**
```python
def untemper(y):
    y ^= y >> 18
    y ^= (y << 15) & 0xefc60000
    for _ in range(7):
        y ^= (y << 7) & 0x9d2c5680
    y ^= y >> 11
    y ^= y >> 22
    return y

# Given 624 consecutive outputs, recover state
state = [untemper(output) for output in outputs]
```

**Python's randrange(maxsize) on 64-bit:**
- `maxsize = 2^63 - 1`, so `getrandbits(63)` is used
- Each 63-bit output uses 2 MT outputs: `(mt1 << 31) | (mt2 >> 1)`
- One bit lost per output → need symbolic solving

**Symbolic approach with z3:**
```python
from z3 import *

def symbolic_temper(y):
    y = y ^ (LShR(y, 11))
    y = y ^ ((y << 7) & 0x9d2c5680)
    y = y ^ ((y << 15) & 0xefc60000)
    y = y ^ (LShR(y, 18))
    return y

# Create symbolic MT state
mt = [BitVec(f'mt_{i}', 32) for i in range(624)]
solver = Solver()

# For each observed 63-bit output
for i, out63 in enumerate(outputs):
    if 2*i + 1 >= 624: break
    y1 = symbolic_temper(mt[2*i])
    y2 = symbolic_temper(mt[2*i + 1])
    combined = Concat(Extract(31, 0, y1), Extract(31, 1, y2))
    solver.add(combined == out63)

if solver.check() == sat:
    state = [solver.model()[mt[i]].as_long() for i in range(624)]
```

**Applications:**
- MIME boundary prediction (email libraries)
- Session token prediction
- CAPTCHA bypass (predictable codes)
- Game RNG exploitation

## MT State Recovery from random.random() Floats via GF(2) Matrix (PHD CTF Quals 2012)

**Pattern:** Server exposes `random.random()` float outputs (e.g., via an API endpoint). Standard MT untemper requires 624 × 32-bit integer outputs, but `random.random()` produces 53-bit floats — truncating each to 8 usable bits per observation. A precomputed GF(2) magic matrix maps observed byte values back to the 624-word MT state.

**Key insight:** `random.random()` returns `(a*2^27+b)/2^53` where `a` = 27 bits from one MT output and `b` = 26 bits from the next. Truncating `int(float * 256)` yields only 8 bits per float, so 3360+ observations are needed (vs. 624 for integer outputs). The `not_random` library precomputes the GF(2) relationship between observed bits and state bits.

```python
import random, gzip, hashlib

# Load precomputed GF(2) magic matrix (from github.com/fx5/not_random)
f = gzip.GzipFile("magic_data", "r")
magic = eval(f.read())
f.close()

def rebuild_from_floats(floats):
    """Convert float observations to byte values, then recover MT state."""
    vals = [int(f * 256) for f in floats]  # truncate to 8-bit
    return rebuild_random(vals)

def rebuild_random(vals):
    """Recover MT19937 state from 3360+ byte observations using GF(2) matrix."""
    def getbit(bit):
        assert bit >= 0
        return (vals[bit // 8] >> (7 - bit % 8)) & 1
    state = []
    for i in range(624):
        val = 0
        data = magic[i % 2]
        for bit in data:
            val <<= 1
            for b in bit:
                val ^= getbit(b + (i // 2) * 8 - 8)
        state.append(val)
    state.append(0)
    ran = random.Random()
    ran.setstate((3, tuple(state), None))
    # Advance past consumed outputs
    for i in range(len(vals) - 3201 + 394):
        ran.randint(0, 255)
    return ran

# Collect 3360+ random.random() floats from the target
floats = [...]  # observed values from server API

# Recover state and predict future outputs
my_random = rebuild_from_floats(floats[:3360])

# Verify predictions match remaining observations
for observed, predicted in zip(floats[3360:], [my_random.random() for _ in range(40)]):
    assert '%.16f' % observed == '%.16f' % predicted

# Forge password reset token (same hash the server computes)
token = hashlib.md5(('%.16f' % my_random.random()).encode()).hexdigest()
reset_url = f'http://target/reset/{user_id}-{token}/'
```

**Attack flow (password reset token prediction):**
1. Request 3360+ random float values from an API endpoint that exposes them (e.g., `/?count=3360`)
2. Simultaneously trigger a password reset (the reset token is `md5(random.random())`)
3. Recover the MT state from the observed floats
4. Predict the `random.random()` call used for the reset token
5. Construct the reset URL with the predicted token

**When to use:** Server uses Python's `random.random()` for security-sensitive tokens (session IDs, password resets, CSRF tokens) and also exposes random values through another endpoint. The `not_random` library handles the bit-level math — focus on collecting enough float observations and synchronizing timing with the target operation.

---

## Time-Based Seed Attacks

When encryption uses time-based PRNG seed:
```python
seed = f"{username}_{timestamp}_{random_bits}"
```

**Attack approach:**
1. **Username:** Extract from metadata, email headers, challenge context
2. **Timestamp:** Get from file metadata (ZIP, exiftool)
3. **Random bits:** Check for hardcoded seed in binary, or bruteforce if small range

**Timestamp extraction:**
```bash
# Set timezone to match target
TZ=Pacific/Galapagos exiftool file.enc
# Look for File Modification Date/Time
```

**Bruteforce milliseconds:**
```python
from datetime import datetime
import random

for ms in range(1000):
    ts = f"2021-02-09!07:23:54.{ms:03d}"
    seed = f"{username}_{ts}_{rdata}"
    rng = random.Random()
    rng.seed(seed)
    key = bytes(rng.getrandbits(8) for _ in range(32))
    if try_decrypt(ciphertext, key):
        print(f"Found seed: {seed}")
        break
```

## C srand/rand Synchronization via Python ctypes

**Pattern:** Binary seeds C's PRNG with `srand(time(NULL))` at startup and uses `rand()` for encryption keys, random challenges, or XOR masks. Python's `random` module uses Mersenne Twister (different algorithm), so calling `random.seed(t)` produces wrong outputs. Use `ctypes` to load the same libc and call C's `srand()`/`rand()` directly.

**Basic synchronization (L3akCTF 2024, MireaCTF):**
```python
from ctypes import CDLL
from time import time

# Load the SAME libc used by the target binary
libc = CDLL('./libc.so.6')  # or CDLL('libc.so.6') for system libc

# Seed at the same second as the binary starts
libc.srand(int(time()))

# Generate the same sequence as the binary's rand() calls
for i in range(16):
    value = libc.rand() & 0xff  # match binary's truncation (e.g., & 0xff for byte)
    print(value)
```

**Decrypting XOR-encrypted data (L3akCTF 2024 chonccfile):**
```python
from ctypes import CDLL
from time import time
from pwn import u32, p32

libc_imp = CDLL('./libc.so.6')
libc_imp.srand(int(time()))

# Binary XORs each 4-byte block with rand() output
encrypted_data = b'...'  # read from heap/memory
result = b''
for i in range(0, len(encrypted_data), 4):
    block = u32(encrypted_data[i:i+4])
    libc_imp.rand()       # skip delay-related rand() call if binary does extra calls
    key = libc_imp.rand()
    block ^= key
    result += p32(block)
```

**Timing considerations:**
- `time(NULL)` has 1-second granularity — start the exploit within the same second as the binary
- Remote targets may have startup delay — try offsets of `+1` or `+2` seconds
- Account for any `rand()` calls between `srand()` and the target usage (e.g., random delays)
- Not 100% reliable on first try — retry with adjacent seeds if needed

**Key insight:** Python's `random` and C's `rand()` are completely different PRNGs. When a C binary uses `srand(time(NULL))`, the only way to reproduce the sequence from Python is `ctypes.CDLL` calling the same libc's `srand`/`rand`. Load the challenge's provided `libc.so.6` for exact compatibility. This works for any C PRNG output prediction — XOR keys, random challenges, token generation, or encrypted heap data.

**Alternative — custom shared library (MireaCTF):**
```c
// random_lib.c — compile with: gcc -shared -o random_lib.so random_lib.c
#include <stdlib.h>
void setseed(int seed) { srand(seed); }
int generate() { return rand() & 0xff; }
```
```python
from ctypes import CDLL
lib = CDLL('./random_lib.so')
lib.setseed(int(time()) + 1)  # +1 for remote delay
numbers = [lib.generate() for _ in range(16)]
```

---

## Layered Encryption Recovery

When binary uses multiple encryption layers:
1. Identify encryption order (e.g., Serpent → TEA)
2. Find seed derivation (e.g., sum of flag chars)
3. Keys often derived from `srand()` sequence
4. Bruteforce seed range (sum of printable ASCII is limited)

## LCG Parameter Recovery Attack

Linear Congruential Generators are weak PRNGs. Given consecutive outputs, recover parameters:

**LCG formula:** `x_{n+1} = (a * x_n + c) mod m`

**Recovery from output sequence (SageMath):**
```python
# Given sequence: [s0, s1, s2, s3, ...]
# crypto-attacks library: github.com/jvdsn/crypto-attacks
from attacks.lcg import parameter_recovery

sequence = [
    72967016216206426977511399018380411256993151454761051136963936354667101207529,
    49670218548812619526153633222605091541916798863041459174610474909967699929824,
    # ... more outputs
]

m, a, c = parameter_recovery.attack(sequence)
print(f"Modulus m: {m}")
print(f"Multiplier a: {a}")
print(f"Increment c: {c}")
```

**Weak RSA from LCG primes:**
- If RSA primes are generated from LCG, recover LCG params first
- Use known plaintext XOR ciphertext to extract LCG outputs
- Regenerate same prime sequence to factor N

```python
# Recover XOR key (which is LCG output)
def recover_lcg_output(plaintext, ciphertext, timestamp):
    pt_bytes = plaintext.encode('utf-8').ljust(32, b'\0')
    ct_int = int.from_bytes(bytes.fromhex(ciphertext), 'big')
    return timestamp ^ int.from_bytes(pt_bytes, 'big') ^ ct_int

# After recovering LCG params, generate RSA primes
lcg = LCG(a, c, m, seed)
primes = []
while len(primes) < 8:
    candidate = lcg.next()
    if is_prime(candidate) and candidate.bit_length() == 256:
        primes.append(candidate)

n = prod(primes)
phi = prod(p - 1 for p in primes)
d = pow(65537, -1, phi)
```

## ChaCha20 Key Recovery

When ChaCha20 key is derived from recoverable data:

```python
from Crypto.Cipher import ChaCha20

# If key derived from predictable source (timestamp, PID, etc.)
for candidate_key in generate_candidates():
    cipher = ChaCha20.new(key=candidate_key, nonce=nonce)
    plaintext = cipher.decrypt(ciphertext)
    if is_valid(plaintext):  # Check for expected format
        print(f"Key found: {candidate_key.hex()}")
        break
```

**Ghidra emulator for key extraction:**
When key is computed by complex function, emulate it rather than reimplementing.

## GF(2) Matrix PRNG Seed Recovery (0xFun 2026)

**Pattern (BitStorm):** PRNG using only XOR, shifts, and rotations is linear over GF(2).

**Key insight:** Express entire PRNG as matrix multiplication: `output_bits = M * seed_bits (mod 2)`. With enough outputs, Gaussian elimination recovers the seed.

```python
import numpy as np

def build_prng_matrix(prng_func, seed_bits=2048, output_bits=2048):
    """Build GF(2) matrix by running PRNG on unit vectors."""
    M = np.zeros((output_bits, seed_bits), dtype=np.uint8)
    for i in range(seed_bits):
        # Set bit i of seed
        seed = 1 << (seed_bits - 1 - i)
        output = prng_func(seed)
        for j in range(output_bits):
            M[j, i] = (output >> (output_bits - 1 - j)) & 1
    return M

# Given output, solve: M * seed = output (mod 2)
# Use GF(2) Gaussian elimination (see modern-ciphers.md solve_gf2)
seed = solve_gf2(M, output_bits_array)
```

**Identification:** Any PRNG using only `^`, `<<`, `>>`, bitwise rotate. DON'T try iterative state recovery — go straight to the matrix.

---

## Middle-Square PRNG Brute Force (UTCTF 2024)

**Pattern (numbers go brrr):** Middle-square method with small seed space.

```python
# PRNG: seed = int(str(seed * seed).zfill(12)[3:9])  — 6-digit seed
# Seed source: int(time.time() * 1000) % (10**6) — only 1M possibilities
# AES key: 8 rounds of PRNG, each produces seed % 2^16 as 2-byte fragment

def middle_square_keygen(seed):
    key = b''
    for _ in range(8):
        seed = int(str(seed * seed).zfill(12)[3:9])
        key += (seed % (2**16)).to_bytes(2, 'big')
    return key

# Brute-force: encrypt known plaintext, compare
for seed in range(10**6):
    key = middle_square_keygen(seed)
    if try_decrypt(ciphertext, key):
        print(f"Seed: {seed}")
        break
```

**Even with time-limited interactions:** 1 known-plaintext pair suffices for offline brute force.

---

## Deterministic RNG from Flag Bytes + Hill Climbing (VuwCTF 2025)

**Pattern (Totally Random Art):** Flag bytes seed Python `random.Random()`. First N bytes of flag are known format, remaining bytes produce deterministic output.

**Attack:** When PRNG seed is known/derivable from flag format, hill-climb unknown characters:
```python
import random

def render(flag_bytes):
    rng = random.Random()
    rng.seed(flag_bytes)
    grid = [[0]*10 for _ in range(5)]
    for b in flag_bytes:
        steps, stroke = divmod(b, 16)
        x, y = 0, 0
        for _ in range(steps):
            dx, dy = rng.choice([(0,1),(0,-1),(1,0),(-1,0)])
            x = (x + dx) % 10
            y = (y + dy) % 5
        grid[y][x] = (grid[y][x] + stroke) % 16
    return grid

# Hill climb: try each byte value, keep the one that maximizes grid match
target = parse_target_art()
flag = list(b'VuwCTF{')
for pos in range(7, 17):
    best_score, best_char = -1, 0
    for c in range(32, 127):
        candidate = bytes(flag + [c])
        score = sum(1 for y in range(5) for x in range(10)
                    if render(candidate)[y][x] == target[y][x])
        if score > best_score:
            best_score, best_char = score, c
    flag.append(best_char)
```

---

## Byte-by-Byte Oracle with Random Mode Matching (VuwCTF 2025)

**Pattern (Unorthodox IV):** Server encrypts with one of N random modes/IVs per encryption. Can submit own plaintexts.

**Attack strategy:**
1. Connect, get encrypted flag
2. Probe with known prefix to check if connection can "reach" the flag's mode (same mode = same ciphertext prefix). ~50 probes, if no match, reconnect.
3. Once reachable, test candidate characters. Mode match AND next byte match = correct char. Mode match but byte mismatch = eliminate candidate permanently.
4. Elimination persists across reconnections.

**Key insight:** Probe for mode reachability first to avoid wasting attempts. Elimination-based search is more efficient than confirmation-based when modes are randomized.

---

## RSA Key Reuse / Replay (UTCTF 2024)

**Pattern (simple signature):** RSA keys reused across rounds with alternating inputs.

**Attack:** Submit previously captured encrypted output back to the server. If keys are static across interactions, replay attacks are trivial. Always check if crypto keys change between rounds.

---

## Logistic Map / Chaotic PRNG Seed Recovery (BYPASS CTF 2025)

**Pattern (Chaotic Trust):** Stream cipher using the logistic map `x_{n+1} = r * x * (1 - x)` as PRNG. Keystream generated by packing iterated float values.

**Key insight:** Logistic map with `r ≈ 4.0` is chaotic but deterministic — recovering the seed (initial x value) enables full keystream reconstruction. Seed is usually a decimal between 0 and 1, such as 0.123456789.

```python
import struct

def logistic_map(x, r=3.99):
    return r * x * (1 - x)

def decrypt_logistic(cipher_hex, seed):
    cipher = bytes.fromhex(cipher_hex)
    x = seed
    stream = b""

    while len(stream) < len(cipher):
        x = logistic_map(x)
        # Pack float as bytes for keystream (check endianness)
        stream += struct.pack("<f", x)[-2:]  # or full 4 bytes

    stream = stream[:len(cipher)]
    return bytes(a ^ b for a, b in zip(cipher, stream))

# Brute-force seed precision
for precision in range(6, 12):
    for base in [123456, 234567, 314159, 271828]:
        seed = base / (10 ** precision)
        result = decrypt_logistic(cipher_hex, seed)
        if b"FLAG" in result or b"CTF" in result:
            print(f"Seed: {seed}, Flag: {result}")
```

**Variations:**
- **r parameter:** Usually `r = 3.99` or `r = 4.0` (full chaos regime)
- **Packing:** `struct.pack("<f", x)` (4 bytes), `struct.pack("<d", x)` (8 bytes), or `[-2:]` for 2-byte fragments
- **Seed range:** Often a recognizable decimal like `0.123456789` or derived from challenge hints

**Identification:** Challenge mentions "chaos", "logistic", "butterfly effect", or provides `r` parameter. Look for source code containing `x = r * x * (1 - x)` iteration.

---

## V8 XorShift128+ State Recovery (Math.random Prediction)

**Pattern:** Web challenge uses `Math.floor(CONST * Math.random())` to generate tokens, codes, or game values. V8's `Math.random()` uses XorShift128+ (xs128p) PRNG. Given consecutive floored outputs, recover the internal state (state0, state1) with Z3, then predict future values.

**V8 internals:**
1. xs128p produces 64-bit state; V8 uses `state0 >> 12 | 0x3FF0000000000000` to create a double in [1.0, 2.0), then subtracts 1.0
2. `Math.random()` reads from a **64-value LIFO cache**. When the cache is empty, `RefillCache()` generates 64 new values. Values are consumed in reverse order from the cache
3. Only `state0` is used for the output (not `state1`)

**xs128p algorithm:**
```python
def xs128p(state0, state1):
    s1 = state0 & 0xFFFFFFFFFFFFFFFF
    s0 = state1 & 0xFFFFFFFFFFFFFFFF
    s1 ^= (s1 << 23) & 0xFFFFFFFFFFFFFFFF
    s1 ^= (s1 >> 17) & 0xFFFFFFFFFFFFFFFF
    s1 ^= s0 & 0xFFFFFFFFFFFFFFFF
    s1 ^= (s0 >> 26) & 0xFFFFFFFFFFFFFFFF
    state0 = state1 & 0xFFFFFFFFFFFFFFFF
    state1 = s1 & 0xFFFFFFFFFFFFFFFF
    return state0, state1, state0  # output is new state0
```

**Z3 solver for `Math.floor(CONST * Math.random())`:**
```python
from z3 import *
from decimal import Decimal
import struct

def to_double(value):
    double_bits = (value >> 12) | 0x3FF0000000000000
    return struct.unpack('d', struct.pack('<Q', double_bits))[0] - 1

def from_double(dbl):
    return struct.unpack('<Q', struct.pack('d', dbl + 1))[0] & 0x7FFFFFFFFFFFFFFF

def sym_xs128p(s0, s1):
    s1_ = s0
    s0_ = s1
    s1_ ^= (s1_ << 23)
    s1_ ^= LShR(s1_, 17)
    s1_ ^= s0_
    s1_ ^= LShR(s0_, 26)
    return s1, s1_  # new state0, state1

def solve_v8_random(observed_values, multiple):
    """Recover xs128p state from consecutive Math.floor(multiple * Math.random()) outputs.
    observed_values must be in REVERSE order (oldest first after tac)."""
    ostate0, ostate1 = BitVecs('ostate0 ostate1', 64)
    sym_s0, sym_s1 = ostate0, ostate1
    slvr = SolverFor("QF_BV")

    for val in observed_values:
        sym_s0, sym_s1 = sym_xs128p(sym_s0, sym_s1)
        calc = LShR(sym_s0, 12)  # V8's ToDouble mantissa bits
        # Constrain: floor(multiple * to_double(state0)) == val
        lower = from_double(Decimal(val) / Decimal(multiple))
        upper = from_double(Decimal(val + 1) / Decimal(multiple))
        lower_m = lower & 0x000FFFFFFFFFFFFF
        upper_m = upper & 0x000FFFFFFFFFFFFF
        upper_e = (upper >> 52) & 0x7FF
        slvr.add(And(lower_m <= calc, Or(upper_m >= calc, upper_e == 1024)))

    if slvr.check() == sat:
        m = slvr.model()
        return m[ostate0].as_long(), m[ostate1].as_long()
    return None, None

# Predict next values after state recovery
def predict_next(state0, state1, multiple, count):
    results = []
    for _ in range(count):
        state0, state1, output = xs128p(state0, state1)
        import math
        results.append(math.floor(multiple * to_double(output)))
    return results
```

**Usage (tool: d0nutptr/v8_rand_buster):**
```bash
# Collect observed values, reverse them (LIFO cache order), pipe to solver
cat observed_codes.txt | tac | python3 xs128p.py --multiple 100000

# Generate predictions from recovered state
python3 xs128p.py --multiple 100000 --gen <state0>,<state1>,<count>
```

**Key insight:** The LIFO cache means observed values are in reverse generation order — reverse them with `tac` before solving. The Z3 `QF_BV` (quantifier-free bitvector) theory efficiently handles the bitwise operations. Typically 5-10 consecutive outputs suffice for a unique solution.

**Common pitfalls:**
- Forgetting to reverse the observation order (cache is LIFO)
- Multiple browser tabs or web workers may have separate PRNG states
- Cache boundary (every 64 calls) can introduce discontinuities if observations span a refill

**Inverse xorshift128+ (backward prediction):** After recovering state, step the PRNG backward to predict values generated *before* the observed sequence. Essential when the target value was generated earlier than observations (e.g., predicting another user's 2FA code). (Midnight Flag 2026)

```python
def undo_rshift_xor(val, shift):
    """Invert val ^= (val >> shift)"""
    result = val
    for _ in range(3):  # 3 iterations sufficient for 64-bit
        result = val ^ (result >> shift)
    return result & 0xFFFFFFFFFFFFFFFF

def undo_lshift_xor(val, shift):
    """Invert val ^= (val << shift)"""
    result = val
    for _ in range(3):
        result = val ^ ((result << shift) & 0xFFFFFFFFFFFFFFFF)
    return result & 0xFFFFFFFFFFFFFFFF

def reverse_step(s0, s1):
    """Run xs128p one step backward: (s0, s1) → (old_s0, old_s1)"""
    old_s1 = s0
    known = (s1 ^ s0 ^ ((s0 >> 26) & 0xFFFFFFFFFFFFFFFF)) & 0xFFFFFFFFFFFFFFFF
    x = undo_rshift_xor(known, 17)
    old_s0 = undo_lshift_xor(x, 23)
    return old_s0, old_s1

# Usage: step backward N times from recovered state
for _ in range(N):
    state0, state1 = reverse_step(state0, state1)
    predicted = math.floor(CONST * to_double(state0))
```

**When to use:** Web challenge where JavaScript generates predictable-looking random values (tokens, verification codes, game rolls) using `Math.random()`. Look for patterns like `Math.floor(N * Math.random())` or `Math.random().toString(36).substr(2)` in client-side or server-side Node.js code.

---

## Password Cracking Strategy

**Attack order for unknown passwords:**
1. Common wordlists: `rockyou.txt`, `10k-common.txt`
2. Theme-based wordlist (usernames, challenge keywords)
3. Rules attack: wordlist + `best66.rule`, `dive.rule`
4. Hybrid: `word + ?d?d?d?d` (word + 4 digits)
5. Brute force: start at 4 chars, increase

**SHA256 with hex salt (VuwCTF 2025, Delicious Cooking):** Format `hash$hex_salt`. Salt must be hex-decoded before `SHA256(password + salt_bytes)`. Password often derivable from security questions (e.g., "fav movie + PIN" = "ratatouille0000"-"ratatouille9999").

**CTF password patterns:**
```text
base_password + year     → actnowonclimatechange2026
username + digits        → nemo123, admin2026
theme + numbers          → flag2026, ctf2025
leet speak               → p@ssw0rd, s3cr3t
```

**Hashcat modes reference:**
```bash
# Common modes
-m 0      # MD5
-m 1000   # NTLM
-m 5600   # NTLMv2
-m 13600  # WinZip AES
-m 13000  # RAR5
-m 11600  # 7-Zip

# Attack modes
-a 0      # Dictionary
-a 3      # Brute force mask
-a 6      # Hybrid (word + mask)
-a 7      # Hybrid (mask + word)
```

**When password relates to another in challenge:**
- Try variations: `password + year`, `password + 123`
- Try reversed: `drowssap`
- Try with common suffixes: `!`, `@`, `#`, `1`, `123`
- If SMB/FTP password known, ZIP password often related

---

See [prng-attacks.md](prng-attacks.md) for CTF-era advanced attacks (2017+).
