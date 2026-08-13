# CTF Crypto - Advanced Mathematical Attacks

## Table of Contents
- [Elliptic Curve Isogenies](#elliptic-curve-isogenies)
- [Pohlig-Hellman Attack (Weak ECC)](#pohlig-hellman-attack-weak-ecc)
- [Baby-Step Giant-Step for General DLP](#baby-step-giant-step-for-general-dlp)
- [LLL Algorithm for Approximate GCD](#lll-algorithm-for-approximate-gcd)
- [Merkle-Hellman Knapsack Cryptosystem via LLL (ASIS 2014)](#merkle-hellman-knapsack-cryptosystem-via-lll-asis-2014)
- [Coppersmith's Method (Close Private Keys)](#coppersmiths-method-close-private-keys)
- [Coppersmith's Method (Structured Primes, LACTF 2026)](#coppersmiths-method-structured-primes-lactf-2026)
- [Clock Group (x^2+y^2=1 mod p) DLP (LACTF 2026)](#clock-group-x2y21-mod-p-dlp-lactf-2026)
- [Quaternion RSA](#quaternion-rsa)
- [Polynomial Arithmetic in GF(2)\[x\]](#polynomial-arithmetic-in-gf2x)
- [RSA Signing Bug](#rsa-signing-bug)
- [Non-Permutation S-box Collision Attack (Nullcon 2026)](#non-permutation-s-box-collision-attack-nullcon-2026)
- [Polynomial CRT in GF(2)\[x\] (Nullcon 2026)](#polynomial-crt-in-gf2x-nullcon-2026)
- [Manger's RSA Padding Oracle Attack (Nullcon 2026)](#mangers-rsa-padding-oracle-attack-nullcon-2026)
- [LWE Lattice Attack via CVP (EHAX 2026)](#lwe-lattice-attack-via-cvp-ehax-2026)
- [Affine Cipher over Non-Prime Modulus (Nullcon 2026)](#affine-cipher-over-non-prime-modulus-nullcon-2026)
- [Introspective CRC via GF(2) Linear Algebra (Google CTF 2017)](#introspective-crc-via-gf2-linear-algebra-google-ctf-2017)
- [Baby-Step Giant-Step for Sparse/Low Hamming Weight Exponents (SEC-T CTF 2017)](#baby-step-giant-step-for-sparselow-hamming-weight-exponents-sec-t-ctf-2017)
- [Hensel's Lemma: Polynomial Root Lifting mod p^k (CONFidence CTF 2019 Teaser)](#hensels-lemma-polynomial-root-lifting-mod-pk-confidence-ctf-2019-teaser)

---

## Elliptic Curve Isogenies

Isogeny-based crypto challenges are often **graph traversal problems in disguise**:

**Key concepts:**
- j-invariant uniquely identifies curve isomorphism class
- Curves connected by isogenies form a graph (often tree-like)
- Degree-2 isogenies: each node has ~3 neighbors (2 children + 1 parent)

**Modular polynomial approach:**
- Connected j-invariants j₁, j₂ satisfy Φ₂(j₁, j₂) = 0
- Find neighbors by computing roots of Φ₂(j, Y) in the finite field
- Much faster than computing actual isogenies

**Pathfinding in isogeny graphs:**
```python
# Height estimation via random walks to leaves
def estimate_height(j, neighbors_func, trials=100):
    min_depth = float('inf')
    for _ in range(trials):
        depth, curr = 0, j
        while True:
            nbrs = neighbors_func(curr)
            if len(nbrs) <= 1:  # leaf node
                break
            curr = random.choice(nbrs)
            depth += 1
        min_depth = min(min_depth, depth)
    return min_depth

# Find path between two nodes via LCA
def find_path(start, end):
    # Ascend from both nodes tracking heights
    # Find least common ancestor
    # Concatenate: path_up(start) + reversed(path_up(end))
```

**Complex multiplication (CM) curves:**
- Discriminant D = f² · D_K where D_K is fundamental discriminant
- Conductor f determines tree depth
- Look for special discriminants: -163, -67, -43, etc. (class number 1)

## Pohlig-Hellman Attack (Weak ECC)

For elliptic curves with smooth order (many small prime factors):

```python
from sage.all import *

# Factor curve order
E = EllipticCurve(GF(p), [a, b])
n = E.order()
factors = factor(n)

# Solve DLP in each small subgroup
partial_logs = []
for (prime, exp) in factors:
    # Compute subgroup generator
    cofactor = n // (prime ** exp)
    G_sub = cofactor * G
    P_sub = cofactor * P  # Target point

    # Solve small DLP
    d_sub = discrete_log(P_sub, G_sub, ord=prime**exp)
    partial_logs.append((d_sub, prime**exp))

# Combine with CRT
from sympy.ntheory.modular import crt
moduli = [m for (_, m) in partial_logs]
residues = [r for (r, _) in partial_logs]
private_key, _ = crt(moduli, residues)
```

## Baby-Step Giant-Step for General DLP

**Pattern:** Compute discrete logarithm `x` where `g^x = h (mod p)` in O(sqrt(n)) time and space, where n is the group order. Works for any cyclic group — multiplicative groups mod p, elliptic curves, or abstract groups. Combined with Pohlig-Hellman for smooth-order groups, solves DLP when `p-1` (or group order) has only small prime factors.

**Baby-step giant-step algorithm:**

```python
from math import isqrt

def bsgs(g, h, p, order=None):
    """Baby-step giant-step: find x such that g^x = h (mod p).

    Time/space: O(sqrt(order)). For subgroups, pass the subgroup order.
    """
    if order is None:
        order = p - 1
    m = isqrt(order) + 1

    # Baby step: build table of g^j for j in [0, m)
    table = {}
    power = 1
    for j in range(m):
        table[power] = j
        power = (power * g) % p

    # Giant step: compute g^(-m), then check h * (g^(-m))^i
    factor = pow(g, -m, p)  # g^(-m) mod p
    gamma = h
    for i in range(m):
        if gamma in table:
            return i * m + table[gamma]
        gamma = (gamma * factor) % p
    return None  # No solution found (order was wrong)

# Example: ElGamal with smooth p-1 (MMA CTF 2015 "Alicegame")
# p-1 = 2 * 3^4 * 5 * 13 * 397 * 34703 * ... (all small factors)
# Pohlig-Hellman: solve DLP in each prime-power subgroup, combine with CRT
```

**Full Pohlig-Hellman + BSGS pipeline:**

```python
from sympy.ntheory import factorint
from sympy.ntheory.modular import crt

def pohlig_hellman(g, h, p):
    """Solve g^x = h (mod p) when p-1 is smooth."""
    order = p - 1
    factors = factorint(order)  # {prime: exponent}

    residues = []
    moduli = []
    for prime, exp in factors.items():
        pe = prime ** exp
        # Project to subgroup of order prime^exp
        cofactor = order // pe
        gi = pow(g, cofactor, p)  # Generator of subgroup
        hi = pow(h, cofactor, p)  # Target in subgroup

        # Solve DLP in small subgroup via BSGS
        xi = bsgs(gi, hi, p, order=pe)
        if xi is None:
            return None
        residues.append(xi)
        moduli.append(pe)

    # Combine via CRT
    x, _ = crt(moduli, residues)
    assert pow(g, x, p) == h % p
    return x
```

**Key insight:** BSGS runs in O(sqrt(q)) for a subgroup of order q. Pohlig-Hellman decomposes the full DLP into subgroup DLPs. If `p-1 = q1^e1 * q2^e2 * ...` where all qi are small, the total cost is O(sum(sqrt(qi^ei))). A 1024-bit prime with smooth `p-1` (all factors under ~40 bits) is solvable in seconds. Sage's `discrete_log()` automatically applies Pohlig-Hellman + BSGS.

**When to recognize:**
- ElGamal, DSA, or Diffie-Hellman with a randomly generated prime — check if `p-1` is smooth: `factor(p-1)` in Sage
- ECC with smooth curve order — same approach, replace modular exponentiation with point multiplication
- Challenge generates new parameters on each connection — retry until you get a smooth prime
- Challenge description mentions "weak parameters" or uses suspiciously small primes

**Sage one-liner:** `discrete_log(Mod(h, p), Mod(g, p))` handles everything automatically.

**References:** MMA CTF 2015 "Alicegame", SEC-T CTF "Madlog", Crypto CTF 2021 "RoHaLd"

---

## LLL Algorithm for Approximate GCD

**Pattern (Grinch's Cryptological Defense):** Server gives hints `h_i = f * p_i + n_i` where f is the flag, p_i are small primes, n_i is small noise.

**Lattice construction:**
```python
from sage.all import *

# Collect 3 hints from server
# h_i = f * p_i + n_i (noise is small)
# Construct lattice where short vector reveals primes

M = matrix(ZZ, [
    [1, 0, 0, h1],
    [0, 1, 0, h2],
    [0, 0, 1, h3],
    [0, 0, 0, -1]  # Scaling factor
])

reduced = M.LLL()
# Short vector contains p1, p2, p3
# Recover f = (h1 - n1) / p1
```

## Merkle-Hellman Knapsack Cryptosystem via LLL (ASIS 2014)

The Merkle-Hellman knapsack is a broken asymmetric scheme. Given public key P = [p0, ..., pn-1] and ciphertext C (sum of selected public key elements), recover the binary plaintext vector:

```python
# Sage
nbit = len(pubKey)
A = Matrix(ZZ, nbit + 1, nbit + 1)

# Identity matrix in upper-left (tracks which elements are selected)
for i in range(nbit):
    A[i, i] = 1
    A[i, nbit] = pubKey[i]

# Target sum in bottom-right
A[nbit, nbit] = -int(encoded)

# LLL reduction finds short vector where last element is 0
res = A.LLL()

# Find row with last element == 0 and all others in {0, 1}
for row in res:
    if row[-1] == 0 and all(b in (0, 1) for b in row[:-1]):
        plaintext_bits = list(row[:-1])
        break
```

**Key insight:** The knapsack problem becomes easy when reformulated as a shortest vector problem. The LLL-reduced basis contains a row representing the binary plaintext when the last column is zero.

## Coppersmith's Method (Close Private Keys)

**Pattern (Duality of Key):** Two RSA key pairs with d1 ≈ d2 (small difference).

**Attack:**
```python
# From e1*d1 ≡ 1 mod φ and e2*d2 ≡ 1 mod φ:
# d2 - d1 ≡ (e1*e2)^(-1) * (e1 - e2) mod p

# Construct polynomial f(x) = (r - x) mod p where x = d2-d1
# Use Coppersmith small_roots() to find x

R.<x> = PolynomialRing(Zmod(N))
r = inverse_mod(e1*e2, N) * (e1 - e2) % N
f = r - x
roots = f.small_roots(X=2^128, beta=0.5)  # Adjust bounds
# x = d2 - d1, recover p from gcd(f(x), N)
```

## Coppersmith's Method (Structured Primes, LACTF 2026)

**Pattern (six-seven-again):** p = base + 10^k · x where base is fully known, x is small.

**Condition:** x < N^{1/e} for degree-e polynomial (≈ N^0.25 for linear).

**Attack:**
```python
# p = base + 10^k * x, so x ≡ -base * (10^k)^{-1} (mod p)
# Since p | N, construct polynomial with root x mod N
R.<x> = PolynomialRing(Zmod(N))
inv_10k = inverse_mod(10^k, N)
f = x + (base * inv_10k) % N  # Must be monic!
roots = f.small_roots(X=2^70, beta=0.5)
if roots:
    x_val = int(roots[0])
    p = base + 10^k * x_val
    q = N // p
```

**Key details:**
- Polynomial MUST be monic (leading coefficient 1)
- `beta=0.5` means we're looking for a factor ≥ N^0.5
- `X` parameter is upper bound on root size
- Works for any "partially known prime" pattern

## Clock Group (x^2+y^2=1 mod p) DLP (LACTF 2026)

**Pattern (the-clock):** Diffie-Hellman on the unit circle group.

**Group structure:**
```python
# Group law: (x1,y1) * (x2,y2) = (x1*y2 + y1*x2, y1*y2 - x1*x2)
# Identity: (0, 1)
# Inverse of (x, y): (-x, y)
# Group order: p + 1 (NOT p - 1!)

def clock_mul(P, Q, p):
    x1, y1 = P
    x2, y2 = Q
    return ((x1*y2 + y1*x2) % p, (y1*y2 - x1*x2) % p)

def clock_pow(P, n, p):
    result = (0, 1)  # identity
    base = P
    while n > 0:
        if n & 1:
            result = clock_mul(result, base, p)
        base = clock_mul(base, base, p)
        n >>= 1
    return result
```

**Recovering hidden prime p:**
```python
# Given points on the curve, p divides (x^2 + y^2 - 1)
from math import gcd
vals = [x**2 + y**2 - 1 for x, y in known_points]
p = reduce(gcd, vals)
# May need to remove small factors
```

**Pohlig-Hellman when p+1 is smooth:**
```python
order = p + 1
factors = factor(order)
# Standard Pohlig-Hellman in the clock group
# Solve d in each prime-power subgroup, CRT combine
```

**CRITICAL:** The order is p+1, isomorphic to norm-1 elements of GF(p²)*. This is different from multiplicative group (order p-1) and elliptic curves (order ≈ p).

## Quaternion RSA

**Pattern:** RSA encryption using Hamilton quaternion algebra over Z/nZ. The plaintext is embedded into quaternion components that are linear combinations of m, p, q, then the quaternion matrix is raised to power e mod n.

**Key structure:**
```python
# Quaternion q = a0 + a1*i + a2*j + a3*k
# Components are linear in m, p, q:
a0 = m
a1 = m + α1*p + β1*q  # e.g., m + 3p + 7q
a2 = m + α2*p + β2*q  # e.g., m + 11p + 13q
a3 = m + α3*p + β3*q  # e.g., m + 17p + 19q

# 4x4 matrix representation:
# Row 0: [a0, -a1, -a2, -a3]
# Row 1: [a1,  a0, -a3,  a2]
# Row 2: [a2,  a3,  a0, -a1]
# Row 3: [a3, -a2,  a1,  a0]

# Ciphertext = first row of matrix^e mod n
```

**Critical property:** For quaternion `q = s + v` (scalar + vector), `q^k = s_k + t_k*v` — the vector part stays **proportional** under exponentiation. This means the ratios of imaginary components are preserved:

`c1 : c2 : c3 = a1 : a2 : a3 (mod n)`

**Factoring n (the attack):**

```python
import math

# Extract quaternion components from ciphertext row [ct0, ct1, ct2, ct3]
# Row 0 = [c0, -c1, -c2, -c3], so negate last 3:
c0, c1, c2, c3 = ct[0], (-ct[1]) % n, (-ct[2]) % n, (-ct[3]) % n

# From ratio preservation: c1*a2 = c2*a1 (mod n), c1*a3 = c3*a1 (mod n)
# Substituting a_i = m + αi*p + βi*q and eliminating m between two equations:
# Result: A*p + B*q ≡ 0 (mod n=pq) => q|A, p|B

# For components a1=m+α1p+β1q, a2=m+α2p+β2q, a3=m+α3p+β3q:
# Eliminate m from (c1*a2=c2*a1) and (c1*a3=c3*a1):
A = (-(α1*c1 - α2*c2)*(c1-c3) + (α1*c1 - α3*c3)*(c1-c2)) % n
B = (-(β1*c1 - β2*c2)*(c1-c3) + (β1*c1 - β3*c3)*(c1-c2)) % n

# More concretely for coefficients [3,7], [11,13], [17,19]:
A = (-(11*c1-3*c2)*(c1-c3) + (17*c1-3*c3)*(c1-c2)) % n
B = (-(13*c1-7*c2)*(c1-c3) + (19*c1-7*c3)*(c1-c2)) % n

q_factor = math.gcd(A, n)  # gives q
p_factor = math.gcd(B, n)  # gives p
```

**Decryption after factoring:**

Over F_p, the quaternion algebra H_p ≅ M_2(F_p) (Wedderburn theorem), so the quaternion's multiplicative order divides p²-1. Decrypt using:

```python
# Group order for quaternions over F_p divides p²-1
d_p = pow(e, -1, p**2 - 1)
d_q = pow(e, -1, q**2 - 1)

# Decrypt mod p and mod q separately, then CRT
enc_mod_p = [[x % p for x in row] for row in enc_matrix]
enc_mod_q = [[x % q for x in row] for row in enc_matrix]
dec_p = matrix_pow(enc_mod_p, d_p, p)
dec_q = matrix_pow(enc_mod_q, d_q, q)

# CRT combine: dec_matrix[0][0] = m (the flag)
m = CRT(dec_p[0][0], dec_q[0][0], p, q)
flag = long_to_bytes(m)
```

**Why it works:** The "reduced dimension" is that 4D quaternion exponentiation reduces to a 2D recurrence (scalar + magnitude of vector), and the direction of the vector part is invariant. This leaks the ratio a1:a2:a3 directly from the ciphertext, enabling factorization.

**References:** SECCON CTF 2023 "RSA 4.0", 0xL4ugh CTF "Reduced Dimension"

---

## Polynomial Arithmetic in GF(2)[x]

**Key operations for CTF crypto:**
```python
def poly_add(a, b):
    """Addition in GF(2)[x] = XOR of coefficient integers."""
    return a ^ b

def poly_mul(a, b):
    """Carry-less multiplication in GF(2)[x]."""
    result = 0
    while b:
        if b & 1:
            result ^= a
        a <<= 1
        b >>= 1
    return result

def poly_divmod(a, b):
    """Division with remainder in GF(2)[x]."""
    if b == 0:
        raise ZeroDivisionError
    deg_a, deg_b = a.bit_length() - 1, b.bit_length() - 1
    q = 0
    while deg_a >= deg_b and a:
        shift = deg_a - deg_b
        q ^= (1 << shift)
        a ^= (b << shift)
        deg_a = a.bit_length() - 1
    return q, a  # quotient, remainder
```

**Applications:** CRT in GF(2)[x] for recovering secrets from polynomial remainders, Reed-Solomon-like error correction.

---

## RSA Signing Bug

**Vulnerability:** Using wrong exponent for signing
- Correct: `sign = m^d mod n` (private exponent)
- Bug: `sign = m^e mod n` (public exponent)

**Exploitation:**
```python
# If signature is m^e mod n, we can "encrypt" to verify
# and compute e-th root to forge signatures
from sympy import integer_nthroot

# For small e (e.g., 3), take e-th root if m^e < n
forged_sig, exact = integer_nthroot(message, e)
if exact:
    print(f"Forged signature: {forged_sig}")
```

---

## Non-Permutation S-box Collision Attack (Nullcon 2026)

**Detection:** Check if S-box is a permutation:
```python
sbox = [...]  # 256 entries
if len(set(sbox)) < 256:
    from collections import Counter
    counts = Counter(sbox)
    for val, cnt in counts.items():
        if cnt > 1:
            colliders = [i for i in range(256) if sbox[i] == val]
            delta = colliders[0] ^ colliders[1]
            print(f"S[{hex(colliders[0])}] = S[{hex(colliders[1])}] = {hex(val)}, delta = {hex(delta)}")
```

**Attack:** For each key byte position k (0-15):
1. Try all 256 values v: encrypt two plaintexts differing by `delta` at position k
2. When `ct1 == ct2`: S-box input at position k was in the collision set `{c0, c1}`
3. Deduce: `key[k] = v ^ round_const` OR `key[k] = v ^ round_const ^ delta`
4. 2-way ambiguity per byte -> 2^16 = 65,536 candidates, brute-force locally

**Total oracle queries:** 16 x 256 + 1 = 4,097 (reference ciphertext + probes).

**Key lessons:**
- SAT/SMT solvers time out on 15+ rounds of symbolic AES even with simplified S-box
- Integral/square attacks fail because non-permutation S-box breaks balance property
- Always check S-box for non-permutation FIRST before attempting complex cryptanalysis

---

## Polynomial CRT in GF(2)[x] (Nullcon 2026)

**Pattern:** Server gives `r = flag mod f` where `f` is a random polynomial over GF(2).

**Attack:** Chinese Remainder Theorem in polynomial ring GF(2)[x]:
1. Collect ~20 pairs `(r_i, f_i)` from server (each `f_i` is ~32-bit random polynomial)
2. Filter for coprime pairs using polynomial GCD
3. Apply CRT to combine: `flag = r_i (mod f_i)` for all i
4. With ~13-20 coprime 32-bit moduli (>= 400 bits combined), flag is unique

```python
def poly_crt(remainders, moduli):
    """CRT in GF(2)[x]: combine (r_i, f_i) pairs."""
    result, mod = remainders[0], moduli[0]
    for i in range(1, len(remainders)):
        g, s, t = poly_xgcd(mod, moduli[i])
        combined_mod = poly_mul(mod, moduli[i])
        result = poly_add(poly_mul(poly_mul(remainders[i], s), mod),
                         poly_mul(poly_mul(result, t), moduli[i]))
        result = poly_mod(result, combined_mod)
        mod = combined_mod
    return result, mod
```

---

## Manger's RSA Padding Oracle Attack (Nullcon 2026)

**Setup:**
- Key `k < 2^64` (small), RSA modulus `n` is large (1337+ bits)
- Oracle: "invalid padding" = `decrypt < threshold`, "error" = `decrypt >= threshold`
- No modular wrap-around because `k << n`

**Attack (simplified Manger's):**
```python
# Phase 1: Find f1 where k * f1 >= threshold
f1 = 1
while oracle(encrypt(f1)) == "below":  # multiply ciphertext by f1^e mod n
    f1 *= 2
# f1/2 < threshold/k <= f1, so k is in [threshold/f1, threshold/(f1/2)]

# Phase 2: Binary search for exact key
lo, hi = 0, threshold
while lo < hi:
    mid = (lo + hi) // 2
    f_test = ceil(threshold, mid + 1)  # f such that k*f >= threshold iff k > mid
    if oracle(encrypt(f_test)) == "above":
        hi = mid
    else:
        lo = mid + 1
key = lo  # ~64 queries for 64-bit key
```

**Total queries:** ~128 (64 for phase 1 + 64 for phase 2).

---

## LWE Lattice Attack via CVP (EHAX 2026)

**Pattern (Dream Labyrinth):** Multi-layer challenge ending with Learning With Errors (LWE) recovery. Secret vector `s` in {-1, 0, 1}^n, public matrix A, ciphertext `b = A*s + e (mod q)`.

**LWE solving with fpylll (CVP/Babai):**
```python
from fpylll import IntegerMatrix, LLL, CVP
import numpy as np

q = 3329  # Common LWE modulus (Kyber uses this)
n = 256   # Secret dimension
m = 512   # Number of samples

# A is m×n matrix, b is m-vector, all mod q
# Construct lattice basis for CVP approach
# Lattice: rows of [q*I_m | 0] on top, [A^T | I_n] below
# Target: b

def solve_lwe_cvp(A, b, q, n, m):
    # Build lattice basis (m+n) × (m+n)
    dim = m + n
    B = IntegerMatrix(dim, dim)

    # Top m rows: q*I_m (ensures solutions mod q)
    for i in range(m):
        B[i, i] = q

    # Bottom n rows: A columns + identity
    for j in range(n):
        for i in range(m):
            B[m + j, i] = int(A[i][j])
        B[m + j, m + j] = 1

    # LLL reduce the basis
    LLL.reduction(B)

    # Target vector: (b | 0...0)
    target = [int(b[i]) for i in range(m)] + [0] * n

    # CVP via Babai's nearest plane
    closest = CVP.babai(B, target)

    # Extract secret from last n components
    s_candidate = [closest[m + j] for j in range(n)]

    # Project to ternary {-1, 0, 1}
    s = []
    for val in s_candidate:
        val_mod = val % q
        if val_mod == 0:
            s.append(0)
        elif val_mod == 1:
            s.append(1)
        elif val_mod == q - 1:
            s.append(-1)
        else:
            # Try closest ternary value
            s.append(min([-1, 0, 1], key=lambda t: abs((val_mod - t) % q)))
    return s

s = solve_lwe_cvp(A, b, q, n, m)
```

**CRITICAL: Endianness gotcha.** Server may describe data as "big-endian" but actually use little-endian (or vice versa). If CVP produces garbage, try swapping byte order of the secret interpretation:
```python
# If server says big-endian but actually uses little-endian:
s_bytes_le = bytes([(v % 256) for v in s])  # little-endian
s_bytes_be = s_bytes_le[::-1]               # big-endian
# Try both interpretations for key derivation
```

**Key derivation after LWE recovery (common pattern):**
```python
import hashlib
from Cryptodome.Cipher import AES

s_bytes = bytes([(v % 256) for v in s])

# Recover session nonce: XOR wrapped_nonce with hash of secret
session_nonce = bytes(a ^ b for a, b in
    zip(wrapped_nonce, hashlib.sha256(s_bytes).digest()[:16]))

# Derive AES key from secret + nonce
aes_key = hashlib.sha256(s_bytes + session_nonce).digest()

# Decrypt AES-GCM
cipher = AES.new(aes_key, AES.MODE_GCM, nonce=aes_nonce)
plaintext = cipher.decrypt_and_verify(ciphertext, tag)
```

**Layer patterns in multi-stage crypto challenges:**
- **Layer 1 (Geometry):** Reconstruct point positions from noisy distance measurements. Use least-squares or trilateration with multiple models. Compute convex hull of recovered points.
- **Layer 2 (Subspace):** Find hidden low-dimensional subspace in high-dimensional data. Self-dot products of candidate vectors identify correct answers (smallest self-dot products = closest to subspace).
- **Layer 3 (LWE):** Recover secret vector from lattice problem. Use CVP with fpylll, project result to expected domain (ternary, binary, etc.).

**References:** EHAX CTF 2026 "Dream Labyrinth". Related: Kyber/CRYSTALS lattice cryptography.

---

## Affine Cipher over Non-Prime Modulus (Nullcon 2026)

**Pattern:** `c = A @ p + b (mod m)` where A is nxn matrix, m may not be prime (e.g., 65).

**Chosen-plaintext attack:**
1. Send n+1 crafted inputs to get n+1 ciphertext blocks
2. Difference attack: `c_i - c_0 = A @ (p_i - p_0) (mod m)`
3. Build difference matrices D (plaintext) and E (ciphertext)
4. Solve: `A = E @ D^{-1} (mod m)` using Gauss-Jordan with GCD invertibility checks
5. Recover: `b = c_0 - A @ p_0 (mod m)`

**CRT approach for composite modulus (preferred):**
```python
def crt2(r1, m1, r2, m2):
    """CRT: x = r1 (mod m1) and x = r2 (mod m2)"""
    m1_inv = pow(m1, m2 - 2, m2)  # Fermat's little theorem
    t = ((r2 - r1) * m1_inv) % m2
    return (r1 + m1 * t) % (m1 * m2)

def gauss_elim(A, b, mod):
    """Gaussian elimination over Z/modZ. A=matrix, b=vector, returns solution x."""
    n = len(b)
    M = [list(A[i]) + [b[i]] for i in range(n)]  # augmented matrix
    for col in range(n):
        pivot = next((r for r in range(col, n) if M[r][col] % mod), None)
        if pivot is None: continue
        M[col], M[pivot] = M[pivot], M[col]
        inv = pow(M[col][col], -1, mod)
        M[col] = [x * inv % mod for x in M[col]]
        for r in range(n):
            if r != col and M[r][col] % mod:
                f = M[r][col]
                M[r] = [(M[r][j] - f * M[col][j]) % mod for j in range(n + 1)]
    return [M[i][n] % mod for i in range(n)]

# For m=65=5x13: Gaussian elimination in GF(5) and GF(13) separately
A5, b5 = A % 5, rhs % 5
A13, b13 = A % 13, rhs % 13
x5 = gauss_elim(A5, b5, mod=5)
x13 = gauss_elim(A13, b13, mod=13)
x = [crt2(x5[i], 5, x13[i], 13) for i in range(len(x5))]
```

---

## Introspective CRC via GF(2) Linear Algebra (Google CTF 2017)

**Pattern:** Find an ASCII string whose CRC-N value equals the string itself (self-referential CRC). Model CRC as a linear function over GF(2) and solve the resulting system.

```python
# CRC is linear over GF(2): CRC(a XOR b) = CRC(a) XOR CRC(b)
# Goal: find x where CRC(x) = x (as ASCII hex)
# 1. Compute CRC of all-zeros baseline
# 2. For each bit position, compute the CRC difference (remainder)
# 3. Set up GF(2) linear system: CRC(x) XOR x = 0
# 4. Solve with Gaussian elimination over GF(2)
from sage.all import *
F = GF(2)
# Build matrix where each column represents flipping one bit
# Rows represent the CRC output bits XOR input bits
M = Matrix(F, n_bits, n_bits)
# ... fill with CRC remainders ...
solution = M.solve_right(target_vector)
```

**Key insight:** CRC is a linear function over GF(2). The self-referential constraint CRC(x)=x becomes a system of linear equations over GF(2), solvable by Gaussian elimination. The ASCII constraint requires choosing free variables to keep all bytes in the printable range.

**References:** Google CTF 2017

---

## Baby-Step Giant-Step for Sparse/Low Hamming Weight Exponents (SEC-T CTF 2017)

**Pattern:** DLP where the exponent is known to have low Hamming weight — e.g., at most k=11 bits set in a 128-bit exponent. Split the exponent into two halves `e = e1 * 2^64 + e2`. Precompute baby steps for all `e1` values with ⌊k/2⌋ = 5 bits set, then do giant steps for all `e2` values with ⌈k/2⌉ = 6 bits set.

**Complexity:** `C(128, 5) ≈ 10^8` baby steps + `C(128, 6) ≈ 10^9` giant steps — vastly less than `O(2^128)` brute force or `O(2^64)` standard BSGS.

```python
from itertools import combinations
from math import comb

# Parameters: g^x = a (mod p), x has at most k=11 bits set in 128 bits
# Split: x = x1 * 2^64 + x2, where x1 has 5 bits set, x2 has 6 bits set
half = 64
k_low, k_high = 5, 6

# Baby step: g^(x1 * 2^64) for all x1 with k_low bits set
baby = {}
for bit_positions in combinations(range(half), k_low):
    x1 = sum(1 << b for b in bit_positions)
    val = pow(g, x1 * (2**half), p)
    baby[val] = x1

# Giant step: check if a * g^(-x2) is in baby table
# a = g^x = g^(x1*2^64) * g^x2, so g^(x1*2^64) = a * g^(-x2)
g_inv = pow(g, -1, p)
for bit_positions in combinations(range(half), k_high):
    x2 = sum(1 << b for b in bit_positions)
    candidate = (a * pow(g_inv, x2, p)) % p
    if candidate in baby:
        x1 = baby[candidate]
        x = x1 * (2**half) + x2
        assert pow(g, x, p) == a
        print(f"Found exponent: {x}")
        break
```

**Verification:**
```python
# Check Hamming weight of recovered exponent
assert bin(x).count('1') <= 11
```

**Key insight:** Sparse-exponent DLP with only k bits set is attackable with meet-in-the-middle: each half uses `C(n, k/2)` entries, reducing complexity from `O(2^k)` to `O(C(n, k/2))`. For k=11 in 128 bits, this is ~10^8 vs 2^128. Always check if the challenge reveals or constrains the Hamming weight of the exponent.

**References:** SEC-T CTF 2017

---

## Hensel's Lemma: Polynomial Root Lifting mod p^k (CONFidence CTF 2019 Teaser)

**Pattern (Bro, do you even lift?):** Challenge gives a polynomial `P(x)` whose unique root mod `N = p^k` is the flag, where `p` is a small known prime and `k` is large (e.g. `p ~ 2^16`, `k = 100`). Brute force over `p^k` is hopeless, but Hensel's lemma lifts any simple root mod `p` to a unique root mod `p^k` via Newton iteration: given `P(r) ≡ 0 mod p^i`, the lift is `r' = r - P(r) * inverse(P'(r), p) mod p^(i+1)`. Factor out the intermediate reductions to `mod p^(i+1)` each step or the integers blow up exponentially.

```python
# sage
R.<x> = PolynomialRing(ZZ)
pol   = ...           # polynomial with huge coefficients
p     = 35671
k     = 100

# Step 1: roots mod p (small, enumerable)
roots_p = [r for r in range(p) if pol(r) % p == 0]

# Step 2: Newton lift to p, p^2, ..., p^k
def hensel_lift(pol, root, p, k):
    dpol = pol.derivative()
    r = root
    mod = p
    for i in range(1, k):
        mod_next = mod * p
        # r' = r - P(r) * inverse(P'(r), p) mod p^(i+1)
        inv = inverse_mod(int(dpol(r)) % p, p)
        r = (r - int(pol(r)) * inv) % mod_next
        mod = mod_next
    return r

flag_int = hensel_lift(pol, roots_p[0], p, k)
```

**Key insight:** Any polynomial equation `P(x) ≡ 0 mod p^k` with `p` small and `p` not dividing `P'(root)` collapses to enumerating roots mod `p` (cheap) plus `k - 1` Newton-style lifts. Each lift requires *reducing intermediate values mod `p^(i+1)` at every step* — Sage's naive `solve_right` or unreduced iteration grinds to a halt because `P(r)` grows as large integers. Works for any `N = p^k` or more generally `N = prod(p_i^{k_i})` by lifting each prime-power factor independently and recombining via CRT.
