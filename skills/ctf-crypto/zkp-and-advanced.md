# CTF Crypto - ZKP, Solvers & Advanced Techniques

## Table of Contents
- [ZKP Attacks](#zkp-attacks)
- [Graph 3-Coloring](#graph-3-coloring)
- [Z3 SMT Solver Guide](#z3-smt-solver-guide)
- [Garbled Circuits: Free XOR Delta Recovery (LACTF 2026)](#garbled-circuits-free-xor-delta-recovery-lactf-2026)
- [Bigram/Trigram Substitution -> Constraint Solving (LACTF 2026)](#bigramtrigram-substitution---constraint-solving-lactf-2026)
- [Shamir Secret Sharing with Deterministic Coefficients (LACTF 2026)](#shamir-secret-sharing-with-deterministic-coefficients-lactf-2026)
- [Race Condition in Crypto-Protected Endpoints (LACTF 2026)](#race-condition-in-crypto-protected-endpoints-lactf-2026)
- [Garbled Circuits: AES Key Recovery via Metadata Leakage (srdnlenCTF 2026)](#garbled-circuits-aes-key-recovery-via-metadata-leakage-srdnlenctf-2026)
- [Post-Quantum Signature Fault Injection: MAYO (srdnlenCTF 2026)](#post-quantum-signature-fault-injection-mayo-srdnlenctf-2026)
- [Lattice-Based Threshold Signature Attack: FROST (srdnlenCTF 2026)](#lattice-based-threshold-signature-attack-frost-srdnlenctf-2026)
- [Groth16 Broken Trusted Setup — delta == gamma (DiceCTF 2026)](#groth16-broken-trusted-setup--delta--gamma-dicectf-2026)
- [Groth16 Proof Replay — Unconstrained Nullifier (DiceCTF 2026)](#groth16-proof-replay--unconstrained-nullifier-dicectf-2026)
- [DV-SNARG Forgery via Verifier Oracle (DiceCTF 2026)](#dv-snarg-forgery-via-verifier-oracle-dicectf-2026)
- [KZG Pairing Oracle for Permutation Recovery (UNbreakable 2026)](#kzg-pairing-oracle-for-permutation-recovery-unbreakable-2026)
- [Shamir Secret Sharing with Reused Polynomial Coefficients (PoliCTF 2017)](#shamir-secret-sharing-with-reused-polynomial-coefficients-polictf-2017)

---

## ZKP Attacks

- Look for information leakage in proofs
- If proving IMPOSSIBLE problem (e.g., 3-coloring K4), you must cheat
- Find hash collisions to commit to one value but reveal another
- PRNG state recovery: salts generated from seeded PRNG can be predicted
- Small domain brute force: if you know `commit(i) = sha256(salt(i), color(i))` and have salt, brute all colors

---

## Graph 3-Coloring

```python
import networkx as nx
nx.coloring.greedy_color(G, strategy='saturation_largest_first')
```

---

## Z3 SMT Solver Guide

Z3 solves constraint satisfaction - useful when crypto reduces to finding values satisfying conditions.

**Basic usage:**
```python
from z3 import *

# Boolean variables (for bit-level problems)
bits = [Bool(f'b{i}') for i in range(64)]

# Integer/bitvector variables
x = BitVec('x', 32)  # 32-bit bitvector
y = Int('y')         # arbitrary precision int

solver = Solver()
solver.add(x ^ 0xdeadbeef == 0x12345678)
solver.add(y > 100, y < 200)

if solver.check() == sat:
    model = solver.model()
    print(model.eval(x))
```

**BPF/SECCOMP filter solving:**

When challenges use BPF bytecode for flag validation (e.g., custom syscall handlers):

```python
from z3 import *

# Model flag as array of 4-byte chunks (how BPF sees it)
flag = [BitVec(f'f{i}', 32) for i in range(14)]
s = Solver()

# Constraint: printable ASCII
for f in flag:
    for byte in range(4):
        b = (f >> (byte * 8)) & 0xff
        s.add(b >= 0x20, b < 0x7f)

# Extract constraints from BPF dump (seccomp-tools dump ./binary)
mem = [BitVec(f'm{i}', 32) for i in range(16)]

# Example BPF constraint reconstruction
s.add(mem[0] == flag[0])
s.add(mem[1] == mem[0] ^ flag[1])
s.add(mem[4] == mem[0] + mem[1] + mem[2] + mem[3])
s.add(mem[8] == 4127179254)  # From BPF if statement

if s.check() == sat:
    m = s.model()
    flag_bytes = b''
    for f in flag:
        val = m[f].as_long()
        flag_bytes += val.to_bytes(4, 'little')
    print(flag_bytes.decode())
```

**Converting bits to flag:**
```python
from Crypto.Util.number import long_to_bytes

if solver.check() == sat:
    model = solver.model()
    flag_bits = ''.join('1' if model.eval(b) else '0' for b in bits)
    print(long_to_bytes(int(flag_bits, 2)))
```

**When to use Z3:**
- Type system constraints (OCaml GADTs, Haskell types)
- Custom hash/cipher with algebraic structure
- Equation systems over finite fields
- Boolean satisfiability encoded in challenge
- Constraint propagation puzzles

---

## Garbled Circuits: Free XOR Delta Recovery (LACTF 2026)

**Pattern (sisyphus):** Yao's garbled circuit with free XOR optimization. Circuit designed so normal evaluation only reaches one wire label, but the other is needed.

**Free XOR property:** Wire labels satisfy `W_0 XOR W_1 = delta` for global secret delta.

**Attack:** XOR three of four encrypted truth table entries to cancel AES terms:
```python
# Encrypted rows: E_i = AES(key_a_i XOR key_b_i, G_out_f(a,b))
# XOR of three rows where AES inputs differ by delta causes cancellation
# Reveals delta directly, then compute: W_1 = W_0 XOR delta
```

**General lesson:** In garbled circuits, if you can obtain any two labels for the same wire, you recover delta and can compute all labels.

---

## Bigram/Trigram Substitution -> Constraint Solving (LACTF 2026)

**Pattern (lazy-bigrams):** Bigram substitution cipher where plaintext has known structure (NATO phonetic alphabet).

**OR-Tools CP-SAT approach:**
1. Model substitution as injective mapping (IntVar per bigram)
2. Add crib constraints from known flag prefix
3. Add **regular language constraint** (automaton) for valid NATO word sequences
4. Solver finds unique solution

**Pattern (not-so-lazy-trigrams):** "Trigram substitution" that decomposes into three independent monoalphabetic ciphers on positions mod 3.

**Decomposition insight:** If cipher uses `shuffle[pos % n][char]`, each residue class `pos = k (mod n)` is an independent monoalphabetic substitution. Solve each separately with frequency analysis or known-plaintext.

---

## Shamir Secret Sharing with Deterministic Coefficients (LACTF 2026)

**Pattern (spreading-secrets):** Coefficients `a_1...a_9` are deterministic functions of secret s (via RNG seeded with s). One share (x_0, y_0) is revealed.

**Vulnerability:** Given one share, the equation `y_0 = s + g(s)*x_0 + g^2(s)*x_0^2 + ... + g^9(s)*x_0^9` is **univariate** in s.

**Root-finding via Frobenius:**
```python
# In GF(p), find roots of h(s) via gcd with x^p - x
# h(s) = s + g(s)*x_0 + ... + g^9(s)*x_0^9 - y_0
# Compute x^p mod h(x) via binary exponentiation with polynomial reduction
# gcd(x^p - x, h(x)) = product of (x - root_i) for all roots
R.<x> = PolynomialRing(GF(p))
h = construct_polynomial(x0, y0)
xp = pow(x, p, h)  # Fast modular exponentiation
g = gcd(xp - x, h)  # Extract linear factors
roots = [-g[0]/g[1]] if g.degree() == 1 else g.roots()
```

**General lesson:** If ALL Shamir coefficients are derived from the secret, a single share creates a univariate equation. This completely breaks the (k,n) threshold scheme.

---

## Race Condition in Crypto-Protected Endpoints (LACTF 2026)

**Pattern (misdirection):** Endpoint has TOCTOU vulnerability: `if counter < 4` check happens before increment, allowing concurrent requests to all pass the check.

**Exploitation:**
1. **Cache-bust signatures:** Modify each request slightly (e.g., prepend zeros to nonce) so server can't use cached verification results
2. **Synchronize requests:** Use multiprocessing with barrier to send ~80 simultaneous requests
3. All pass `counter < 4` check before any increments -> counter jumps past limit

```python
from multiprocessing import Process, Barrier
barrier = Barrier(80)

def make_request(barrier, modified_sig):
    barrier.wait()  # Synchronize all processes
    requests.post(url, json={"sig": modified_sig})

# Launch 80 processes with unique signature modifications
processes = [Process(target=make_request, args=(barrier, modify_sig(i))) for i in range(80)]
```

**Key insight:** TOCTOU in `check-then-act` patterns. Look for read-modify-write without atomicity/locking.

---

## Garbled Circuits: AES Key Recovery via Metadata Leakage (srdnlenCTF 2026)

**Pattern (FHAES):** Service evaluates AES via garbled circuits with a fixed per-connection key. Exploit garbling metadata rather than AES cryptanalysis.

**Attack:**
1. Construct a custom circuit with one attacker-controlled AND gate that leaks the global Free-XOR offset delta
2. Use delta to locally evaluate the key-schedule section (first 1360 AND gates) as the evaluator
3. For each of the first 16 key-schedule S-box calls, brute-force the input byte by re-garbling the S-box chunk and comparing observed AND tables
4. Reconstruct key words from S-box outputs and recover the full 128-bit key through algebraic manipulation of the AES-128 schedule recurrence

```python
def garble_and(A, B, D, and_idx):
    """Reproduce garbling with proper parity handling."""
    r = B & 1
    alpha = A & 1
    beta = B & 1
    # Computes gate0, gate1, z output via hash-based approach
    return gate0, gate1, z

def evaluator_and(A, B, gate0, gate1, and_idx):
    """Evaluate AND gate using hash-based approach."""
    hashA = h_wire(A, and_idx)
    hashB = h_wire(B, and_idx)
    L = hashA if (A & 1) == 0 else (hashA ^ gate0)
    R = hashB if (B & 1) == 0 else (hashB ^ gate1)
    return L ^ R ^ (A * (B & 1))
```

**Key insight:** Garbled circuits that use free XOR optimization with fixed keys across sessions leak key material through the AND gate truth tables. Each S-box has a small enough input space (256 values) to brute-force when you know delta. This extends the LACTF technique from "recovering delta" to "recovering the entire AES key."

---

## Post-Quantum Signature Fault Injection: MAYO (srdnlenCTF 2026)

**Pattern (Faulty Mayo):** One-byte fault injection window in `mayo_sign_signature` before final `s = v + O*x` construction. Controlled bit flips across 64 signature queries recover the secret matrix O row by row.

**Attack:**
1. Reverse binary to map fault offsets to `mayo_sign_signature` instructions
2. For each of 64 rows of secret matrix O, use faulted signatures to extract linear equations over GF(16)
3. Solve 17-variable linear systems over GF(16) for each row using Gaussian elimination
4. Rebuild equivalent signer using recovered O and public seed from compressed public key
5. Forge valid signature for challenge message

**GF(16) Gaussian elimination:**
```python
# Precompute multiplication and inverse tables for GF(16)
# GF(16) = GF(2)[x] / (x^4 + x + 1), elements 0-15
INV = [0] * 16  # multiplicative inverses
MUL = [[0]*16 for _ in range(16)]  # multiplication table

def solve_linear_gf16(equations, nvars=17):
    """Gaussian elimination over GF(16)."""
    A = [x[:] + [y] for x, y in equations]
    m, row = len(A), 0
    for col in range(nvars):
        piv = next((r for r in range(row, m) if A[r][col] != 0), None)
        if piv is None: continue
        A[row], A[piv] = A[piv], A[row]
        invp = INV[A[row][col]]
        A[row] = [MUL[invp][v] for v in A[row]]
        for r in range(m):
            if r != row and A[r][col] != 0:
                f = A[r][col]
                A[r] = [A[r][c] ^ MUL[f][A[row][c]] for c in range(nvars + 1)]
        row += 1
    return [A[i][nvars] for i in range(nvars)]
```

**Key insight:** Post-quantum signature schemes like MAYO can be broken with fault injection if you can cause controlled bit flips during signing. Each fault creates a linear equation over GF(16), and 17+ equations per row suffice to recover the secret. This is analogous to DFA on classical schemes but over extension fields.

---

## Lattice-Based Threshold Signature Attack: FROST (srdnlenCTF 2026)

**Pattern (Threshold):** Preprocessing queue capacity allows collecting many signatures. Fixed challenge construction enables solving 1D noisy linear equations per coefficient.

**Attack:**
1. Exploit queue-depth cap (≤8 active) rather than total-usage cap by alternating menu options
2. Force fixed challenge `c` by choosing commitment `w₀` each query to zero aggregate commitment before high-bit extraction
3. With fixed `c`, each coefficient becomes: `z = λ·u + noise (mod q)`
4. Select multiple signer subsets to obtain different Lagrange coefficient scales (small/mid/huge) for each target signer
5. Solve via interval intersection and maximum-likelihood selection
6. Recover 7 signer shares; combine with own share; reconstruct master secret via Lagrange interpolation

**Interval intersection algorithm:**
```python
from math import ceil, floor

def intersect_intervals(intervals, lam, z, q, B):
    """Refine candidate intervals using one (λ, z) observation with noise bound B."""
    out = []
    for lo, hi in intervals:
        if lam > 0:
            kmin = ceil((lam * lo - z - B) / q)
            kmax = floor((lam * hi - z + B) / q)
            for k in range(kmin, kmax + 1):
                a = (z + q * k - B) / lam
                b = (z + q * k + B) / lam
                lo2, hi2 = max(lo, a), min(hi, b)
                if lo2 <= hi2:
                    out.append((lo2, hi2))
    # Merge overlapping intervals
    out.sort()
    merged = [out[0]] if out else []
    for lo, hi in out[1:]:
        if lo <= merged[-1][1]:
            merged[-1] = (merged[-1][0], max(merged[-1][1], hi))
        else:
            merged.append((lo, hi))
    return merged
```

**Key insight:** Threshold signature schemes can leak individual shares when the challenge value is controlled. By querying with different signer subsets, you get different Lagrange coefficient scales for the same unknown share, allowing iterative interval refinement. With enough observations, the interval converges to a unique value.

---

## Groth16 Broken Trusted Setup — delta == gamma (DiceCTF 2026)

**Pattern (Housing Crisis):** Groth16 verifier has `vk_delta_2 == vk_gamma_2`, which breaks soundness entirely. Proofs are trivially forgeable.

**Forgery:**
```python
from py_ecc.bn128 import G1, G2, multiply, add, neg, pairing
from py_ecc.bn128 import curve_order as q

# When delta == gamma, the pairing equation simplifies:
# e(A, B) = e(alpha, beta) * e(vk_x + C, gamma)
# Set A = vk_alpha1, B = vk_beta2, then:
# e(alpha, beta) * e(vk_x + C, gamma) = e(alpha, beta)
# → e(vk_x + C, gamma) = 1 → C = -vk_x (point negation)

forged_A = vk_alpha1   # alpha point from verification key
forged_B = vk_beta2    # beta point from verification key
forged_C = neg(vk_x)   # negate the public input accumulator

# This proof verifies for ANY public inputs
```

**Detection:** Compare `vk_delta_2` and `vk_gamma_2` in the verifier contract. If equal, the entire Groth16 scheme collapses — any statement can be "proven."

**When to check:** Always inspect Groth16 verification key constants before attempting complex attacks. A broken trusted setup makes everything else unnecessary.

---

## Groth16 Proof Replay — Unconstrained Nullifier (DiceCTF 2026)

**Pattern (Housing Crisis):** DAO governance never tracks used `proposalNullifierHash` values, and the circuit leaves the nullifier unconstrained. A valid proof from the setup transaction can be replayed infinitely.

**Attack:**
1. Find the DAO contract's deployment/setup transaction
2. Extract constructor arguments containing valid Groth16 proof
3. Replay the same proof for every proposal — it always verifies
4. Use proposals to control DAO actions (betting, market creation, resolution)

**Key insight:** ZK circuits that leave inputs unconstrained and systems that don't track nullifiers are vulnerable to replay. Always check: does the verifier contract track proof nullifiers? Does the circuit actually constrain all declared public inputs?

---

## DV-SNARG Forgery via Verifier Oracle (DiceCTF 2026)

**Pattern (Dot):** DV-SNARG (Designated Verifier Succinct Non-interactive ARGument) for an adder circuit. Must produce 20 valid proofs for **wrong** answers.

**Key insight:** DV-SNARGs explicitly lose soundness when the prover has oracle access to the verifier (ePrint 2024/1138). The verifier's secret randomness can be extracted through query patterns.

**DPP (Dot Product Proof) structure:**
```text
q[i] = v[i] + b*(tensor[i] - constraint[i])
where b = fixed constant (e.g., 162817)
      v[i] = random in [-256, 256]
      constraint weights r = random in [-2^40, 2^40]
```

**Forgery via CRS entry cancellation:**
For a wrong answer, only the output constraint (wire N) is violated. Find two CRS entries whose constraint contributions cancel:

1. Wire N is touched by gate G AND the output constraint
2. `pair(input1, input2)` of gate G is touched ONLY by gate G
3. Adding `CRS[wire_N]` and subtracting `CRS[pair]` to the wrong proof cancels `b*r_G` terms
4. The remaining deficit `b*r_output` also cancels
5. Adjust `delta = -v[N] + 2*b*v[input1]*v[input2]` via `delta*G` on h2

**Learning secret v values via oracle:**
```python
# At streak=0, submitting correct answer is "safe" — doesn't reset streak
# Use oracle to learn |v[i]| from unconstrained diagonal pairs:

for guess in range(257):  # v[i] in [-256, 256], |v[i]| in [0, 256]
    # Set pair(i,i) coefficient to guess^2
    # If guess == |v[i]|, specific oracle response differs
    response = oracle_query(guess)
    if response == "hit":
        abs_v_i = guess
        break

# Learn signs from off-diagonal unconstrained pairs (1 query each)
# Learn product sign: v[a]*v[b] sign from pair(a,b)
```

**Performance:** ~364 oracle queries for Phase 1 (~97s), ~300s for 20 forged proofs ≈ 400s total.

**Key insight:** When attacking DV-SNARGs with oracle access, the strategy is: (1) learn a small number of secret values from the verifier's randomness, (2) use algebraic cancellation between CRS entries to forge proofs. Unconstrained pair indices expose pure tensor products of the secret vector.

---

## KZG Pairing Oracle for Permutation Recovery (UNbreakable 2026)

**Pattern (toxicwaste):** KZG commitment scheme publishes shuffled points `{alpha^i * G1}` for i=0..n. The shuffle hides which point corresponds to which exponent. Recover the exponent ordering using bilinear pairings as an oracle, then extract the toxic waste `alpha`.

**Distortion map technique:** On supersingular pairing-friendly curves, a distortion map `psi((x,y)) = (zeta*x, y)` (where `zeta^3 = 1`) enables additive exponent comparisons:

```python
from sage.all import *

# For points P_i = alpha^a_i * G1 and P_j = alpha^a_j * G1:
# e(P_i, psi(P_j)) = e(G1, psi(G1))^(alpha^(a_i + a_j))
# If e(P_i, psi(P_j)) == e(P_k, psi(G1)), then a_i + a_j == a_k

# Step 1: Identify G1 (alpha^0) — the only point where e(P, psi(P)) == e(G1, psi(G1))
g1 = None
base_pairing = None
for P in shuffled_points:
    val = P.weil_pairing(psi(P), order)
    if base_pairing is None:
        base_pairing = val
        g1 = P
    elif val == base_pairing:
        g1 = P
        break

# Step 2: Walk the chain — find alpha*G1 via e(P_?, psi(G1)) comparisons
# Then alpha^2*G1 via e(alpha*G1, psi(alpha*G1)) == e(alpha^2*G1, psi(G1))
# Continue until full ordering recovered

# Step 3: With ordered points, solve A(x) = 0 over GF(q) to get alpha
# Step 4: Forge KZG opening proofs using recovered alpha
```

**Key insight:** Bilinear pairings reveal additive relationships between exponents without solving discrete log. The pairing `e(P_i, psi(P_j))` depends on `alpha^(a_i + a_j)`, so comparing against known pairing values identifies which shuffled point has which exponent. This turns a cryptographic shuffle into a solvable ordering problem.

---

## Shamir Secret Sharing with Reused Polynomial Coefficients (PoliCTF 2017)

**Pattern:** When a Shamir SSS implementation reuses the same random polynomial coefficients for every character of the secret, share subtraction cancels the higher-order terms.

```python
# Standard Shamir: y_i = f_i + a1*x + a2*x^2 + ... (different a_j per character)
# Broken: y_i = f_i + a1*x + a2*x^2 + ... (SAME a_j for all characters)
# Since higher-order terms are identical:
# y_1[i] - y_1[0] = f[i] - f[0]  (for share x=1)
# If f[0] is known (e.g., 'f' from flag prefix):
flag = ''.join(chr(shares[i] - shares[0] + ord('f')) for i in range(len(shares)))
```

**Key insight:** In correct Shamir SSS, each secret byte uses independent random coefficients. When coefficients are reused, subtracting any two shares at the same evaluation point cancels all randomness, leaving only the difference between the corresponding secret bytes.

**References:** PoliCTF 2017
