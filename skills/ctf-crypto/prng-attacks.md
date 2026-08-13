# CTF Crypto - PRNG Attacks (CTF-Era Techniques)

Advanced CTF-specific PRNG attacks from 2017 onward. For foundational PRNG recovery (MT19937, LCG parameter recovery, ChaCha20, V8 XorShift128+, password cracking), see [prng.md](prng.md).

## Table of Contents
- [Mersenne Twister Seed Recovery from Subset Sum (Tokyo Westerns 2017)](#mersenne-twister-seed-recovery-from-subset-sum-tokyo-westerns-2017)
- [MT19937 State Recovery via Constraint Propagation (HITCON 2017)](#mt19937-state-recovery-via-constraint-propagation-hitcon-2017)
- [Rule 86 Cellular Automaton PRNG Reversal via Z3 (Insomni'hack 2018)](#rule-86-cellular-automaton-prng-reversal-via-z3-insomnihack-2018)
- [Java LCG Seed Meet-in-the-Middle via Partial Modulo (P.W.N. CTF 2018)](#java-lcg-seed-meet-in-the-middle-via-partial-modulo-pwn-ctf-2018)
- [LCG Backward Stepping via Multiplicative Inverse (P.W.N. CTF 2018)](#lcg-backward-stepping-via-multiplicative-inverse-pwn-ctf-2018)
- [LFSR Bit-Fold Recovery from ASCII Parity (X-MAS CTF 2018)](#lfsr-bit-fold-recovery-from-ascii-parity-x-mas-ctf-2018)
- [Z3 Solve-Time Timing Oracle on PRNG (X-MAS CTF 2018)](#z3-solve-time-timing-oracle-on-prng-x-mas-ctf-2018)
- [randcrack-Fed DSA k Prediction (CSAW CTF 2018)](#randcrack-fed-dsa-k-prediction-csaw-ctf-2018)
- [Time-Seeded PRNG Offset via Format-String Global Write (FireShell 2019)](#time-seeded-prng-offset-via-format-string-global-write-fireshell-2019)
- [NTP-Poisoned PRNG State Leak via UUID XOR (RuCTFe 2018)](#ntp-poisoned-prng-state-leak-via-uuid-xor-ructfe-2018)

---

## Mersenne Twister Seed Recovery from Subset Sum (Tokyo Westerns 2017)

**Pattern:** MT19937 seeded with a 32-bit value generates subset-sum problems (e.g., "which elements from this set sum to target?"). Solving small subset-sum problems leaks specific MT output values. Two recovered outputs at indices 0 and 227 are sufficient to invert the MT seeding process.

**MT twist function relationship:**
```text
mt[i] = mt[i-624] XOR twist(mt[i-624], mt[i-623])
```
At the wrap-around: `mt[624]` depends on `mt[0]` (new cycle) and `mt[397]` (old cycle). Recovering `mt[0]` and `mt[227]` (which is related to `mt[624-227] = mt[397]`) via subset-sum solutions reveals enough to invert the twist recurrence.

```python
import random

def crack_seed_from_two_outputs(mt0_val, mt227_val):
    """Try all 2^32 seeds until MT outputs match recovered values."""
    for seed in range(2**32):
        r = random.Random()
        r.seed(seed)
        # Generate enough to reach indices 0 and 227
        outputs = [r.getrandbits(32) for _ in range(228)]
        if outputs[0] == mt0_val and outputs[227] == mt227_val:
            return seed
    return None

# After recovering seed, all future (and past) outputs are predictable
r = random.Random()
r.seed(recovered_seed)
```

**Key insight:** MT19937 seeds recoverable from as few as two state values (indices 0 and 227) via the twist function's wrap-around relationship. Any challenge that exposes MT state values through solvable mathematical puzzles is vulnerable to full seed recovery.

**References:** Tokyo Westerns CTF 2017

---

## MT19937 State Recovery via Constraint Propagation (HITCON 2017)

**Pattern:** Server generates problems that leak 24-120 bits of PRNG output per round (e.g., partial bit-patterns, subset sums, modular reductions). Rather than collecting 624 full 32-bit outputs, model the MT state as an array of per-cell candidate sets and propagate constraints bidirectionally through the MT recurrence.

**MT recurrence dependencies:**
```text
state[i] = state[i-624] XOR twist(state[i-624], state[i-623])
```
This means `state[x]` depends on `state[x-624]`, `state[x-623]`, and `state[x-227]` (via the generate step). Partial knowledge at any index propagates in both directions.

**Constraint propagation approach:**
```python
# Model: each state word starts as a set of 2^32 candidates
# Partial observation: narrow candidates for observed indices
# Propagate: for each constrained cell, narrow related cells

def propagate_forward(state_candidates, idx):
    """MT: state[idx+624] = f(state[idx], state[idx+1])"""
    for s0 in state_candidates[idx]:
        for s1 in state_candidates[idx + 1]:
            new_val = mt_twist(s0, s1)
            state_candidates[idx + 624].add(new_val)

def propagate_backward(state_candidates, idx):
    """Invert MT twist to constrain earlier states from later ones."""
    for val in state_candidates[idx]:
        # Recover state[idx-624] given state[idx] and state[idx-623]
        for s1 in state_candidates[idx - 623]:
            s0 = mt_untwist(val, s1)
            state_candidates[idx - 624].add(s0)

# After ~20 partial observations across different positions:
# Most cells converge to single candidates → full state determined
```

**Key insight:** MT19937's recurrence dependencies allow bidirectional constraint propagation — partial knowledge at multiple positions narrows candidates until the full 624-word state is determined. The number of partial observations needed scales inversely with bits leaked per observation: ~20 observations of 24+ bits each typically suffice.

**References:** HITCON CTF 2017

---

## Rule 86 Cellular Automaton PRNG Reversal via Z3 (Insomni'hack 2018)

**Pattern:** Wolfram elementary cellular automaton Rule 86 used as PRNG. Reverse through 128 rounds using Z3 Bool arrays:

```python
from z3 import *

def RULE86(x, y, z):
    return Or(And(Not(x), Not(y), z), And(Not(x), y, Not(z)),
              And(x, Not(y), Not(z)), And(x, y, Not(z)))

s = Solver()
state = [Bool(f'b{i}') for i in range(256)]
# Forward-compute 128 rounds symbolically
for round in range(128):
    new_state = [RULE86(state[(i-1)%256], state[i], state[(i+1)%256]) for i in range(256)]
    state = new_state
# Constrain final state to known output
for i, bit in enumerate(known_output):
    s.add(state[i] == (bit == 1))
s.check()
model = s.model()
```

**Key insight:** Elementary cellular automata are NOT injective -- multiple preimages may exist. But Z3 handles the search efficiently by treating each cell as a boolean variable and each rule application as a CNF clause. For Rule 86 specifically, the DNF has 4 terms (bits 1,2,4,6 of rule number 86 = 01010110). Use `s.push()`/`s.pop()` to iteratively backtrack through rounds. This approach generalizes to any elementary CA rule used as a PRNG: encode the rule's truth table as a boolean formula, compose symbolically for N rounds, and constrain to the known output.

**References:** Insomni'hack CTF 2018

---

## Java LCG Seed Meet-in-the-Middle via Partial Modulo (P.W.N. CTF 2018)

**Pattern:** Java `Random` outputs are only visible mod 62 (e.g., characters in a password). Full 48-bit seed search is infeasible, but partial output still leaks the LSB (`output mod 2`) and the high bits can be recovered independently because Java's LCG takes `(seed*a + c) >> 16`. Split into `2^18` low-bit search and `2^30` high-bit search.

```python
# Phase 1: enumerate 2^18 low-18-bit candidates whose nextInt(62) parities match known chars
for low in range(1 << 18):
    if simulate(low)[: K] == known_prefix: candidates.append(low)

# Phase 2: extend each candidate to 48 bits, matching next outputs
for low in candidates:
    for high in range(1 << 30):
        seed = (high << 18) | low
        if simulate(seed) == full_known: return seed
```

**Key insight:** Java's LCG discards the lowest 16 bits, so the low 18 bits only affect the LSB of each `nextInt()`. Split the seed at that boundary to turn an infeasible `2^48` search into two `2^18 + 2^30` passes.

**References:** P.W.N. CTF 2018 — PW API, writeup 12065

---

## LCG Backward Stepping via Multiplicative Inverse (P.W.N. CTF 2018)

**Pattern:** After recovering a forward seed, step backward with the modular inverse of the multiplier: `prev = a^-1 * (state - c) mod m`. For Java: `a^-1 = -35320271006875` mod `2^48`.

```python
M = 1 << 48
a_inv = pow(25214903917, -1, M)  # Java multiplier inverse
prev_state = (a_inv * (state - 11)) % M
```

**Key insight:** Every power-of-two-modulus LCG is invertible; once you have any state, you can walk the chain both directions without re-simulating from seed.

**References:** P.W.N. CTF 2018 — PW API, writeup 12065

---

## LFSR Bit-Fold Recovery from ASCII Parity (X-MAS CTF 2018)

**Pattern:** Custom PRNG XORs shifts of a 32-bit state into an 8-bit output byte. Observed bytes are ASCII (`top bit = 0`), so each observed byte leaks one parity bit of the internal state. Combine enough parity constraints via Gaussian elimination over GF(2) to recover state without brute force.

```python
# Collect parity constraints: each observed ASCII byte gives top_bit == 0
# Each bit of each byte is a linear combination of state bits
# Stack rows, solve in GF(2)
import numpy as np
A = np.array(constraint_rows, dtype=np.uint8)
b = np.array(known_bits,      dtype=np.uint8)
state = gf2_solve(A, b)
```

**Key insight:** Output byte folding does not hide the state; each output bit is still linear in the state. Use the ASCII constraint to turn output bytes into free parity equations.

**References:** X-MAS CTF 2018 — Probably Really Nice Goodies from Santa, writeup 12686

---

## Z3 Solve-Time Timing Oracle on PRNG (X-MAS CTF 2018)

**Pattern:** Cannot directly check a PRNG guess, but `Solver.check()` takes measurably longer for correct inputs because the constraint graph becomes UNSAT-hard instead of trivially SAT. Brute-force each character by timing Z3 with a tight timeout.

```python
from z3 import Solver, sat
for c in string.printable:
    s = Solver()
    s.set('timeout', 500)
    s.add(prng_constraints(flag + c, ciphertext))
    t = time.time()
    if s.check() == sat and (time.time() - t) > 0.4:
        flag += c
        break
```

**Key insight:** Many oracles run the solver internally; wrong guesses return fast because the SAT instance is already obviously satisfiable. Set a tight timeout and promote slow-solve candidates.

**References:** X-MAS CTF 2018 — Probably Really Nice Goodies from Santa, writeup 12686

---

## randcrack-Fed DSA k Prediction (CSAW CTF 2018)

**Pattern:** DSA signing uses `random.randrange()` to pick `k`. If the server also exposes a "forgot password" endpoint that leaks Python `random` outputs, feed 624 × 32-bit samples to `randcrack`, predict the next `k`, and solve `x = (s*k - h) * r^-1 mod q` for the private key.

```python
from randcrack import RandCrack
rc = RandCrack()
for _ in range(624 // 2):
    v = getrand64()
    rc.submit(v & 0xffffffff); rc.submit(v >> 32)
k = rc.predict_randrange(2, q)
x = ((s*k - h) * pow(r, -1, q)) % q
```

**Key insight:** Python's `random` is shared across all calls. Any endpoint that leaks `random` output poisons every other RNG consumer including DSA signing.

**References:** CSAW CTF 2018 — Disastrous Security Apparatus, writeup 12495

---

## Time-Seeded PRNG Offset via Format-String Global Write (FireShell 2019)

**Pattern:** Server seeds `srand((time(0)/10) + bet)` where `bet` is a writable global. Use a format string primitive to set `bet`, then predict all subsequent `rand()` outputs offline with matching libc.

```python
# 1. Format-string: %Xc%Y$n  writes desired bet value at &bet (0x602020)
# 2. Locally: predict rand() with C stdlib
from ctypes import CDLL
libc = CDLL('libc.so.6')
libc.srand((int(time.time())//10) + bet_value)
predicted = [libc.rand() for _ in range(n)]
```

**Key insight:** Shifting a known seed by a writable constant turns a time-of-day RNG into a deterministic RNG because the attacker now controls the whole seed.

**References:** FireShell CTF 2019 — casino, writeup 12916

---

## NTP-Poisoned PRNG State Leak via UUID XOR (RuCTFe 2018)

**Pattern:** Server derives UUIDs as `uuid = time ^ hash_state`. Register a user with a custom NTP endpoint returning `0x00`; the returned UUID now equals `hash_state` directly. Future UUIDs are predictable: `target_uuid ^ hash_state = required_timestamp`.

```python
# 1. Point the server at attacker-controlled NTP that returns 0
# 2. Register; received UUID == internal state
# 3. For desired uuid, compute needed timestamp = uuid ^ state
# 4. Send the crafted timestamp and read the message
```

**Key insight:** Any randomness mixed via XOR with a user-controlled value is the same as giving the attacker the state directly. Check all time sources for user control.

**References:** RuCTFe 2018 — vch, writeup 12146
