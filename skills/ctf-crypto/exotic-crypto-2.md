# CTF Crypto - Exotic Algebraic Structures (Part 2)

Covers 2017+ era exotic crypto attacks (BB-84 QKD, ElGamal variants, Paillier oracles, differential privacy, homomorphic bit extraction, Jordan normal form, OSS forgery, Cayley-Purser, BIP39 brute, Asmuth-Bloom, Rabin polynomial primes, LCG period, Vandermonde recovery). For Part 1 foundational exotic structures, see [exotic-crypto.md](exotic-crypto.md).

## Table of Contents
- [BB-84 Quantum Key Distribution MITM Attack (PlaidCTF 2017)](#bb-84-quantum-key-distribution-mitm-attack-plaidctf-2017)
- [ElGamal Trivial DLP When B = p-1 (Hack.lu 2017)](#elgamal-trivial-dlp-when-b--p-1-hacklu-2017)
- [Paillier LSB Oracle via Homomorphic Doubling (CODE BLUE 2017)](#paillier-lsb-oracle-via-homomorphic-doubling-code-blue-2017)
- [Differential Privacy Laplace Noise Cancellation (Pwn2Win 2017)](#differential-privacy-laplace-noise-cancellation-pwn2win-2017)
- [Homomorphic Encryption Oracle Bit-Extraction (Tokyo Westerns 2017)](#homomorphic-encryption-oracle-bit-extraction-tokyo-westerns-2017)
- [ElGamal over Matrices via Jordan Normal Form (SharifCTF 8)](#elgamal-over-matrices-via-jordan-normal-form-sharifctf-8)
- [OSS (Ong-Schnorr-Shamir) Signature Forgery via Pollard's Method (SharifCTF 8)](#oss-ong-schnorr-shamir-signature-forgery-via-pollards-method-sharifctf-8)
- [Cayley-Purser Decryption Without Private Key (TJCTF 2018)](#cayley-purser-decryption-without-private-key-tjctf-2018)
- [BIP39 Partial-Mnemonic Brute Force via Checksum (SECCON 2018)](#bip39-partial-mnemonic-brute-force-via-checksum-seccon-2018)
- [Asmuth-Bloom Threshold Secret Sharing via CRT (X-MAS 2018)](#asmuth-bloom-threshold-secret-sharing-via-crt-x-mas-2018)
- [Rabin Cryptosystem with Polynomial Primes (X-MAS 2018)](#rabin-cryptosystem-with-polynomial-primes-x-mas-2018)
- [LCG Period Detection for Unlimited Output Prediction (X-MAS 2018)](#lcg-period-detection-for-unlimited-output-prediction-x-mas-2018)
- [Polynomial Coefficient Recovery via Vandermonde Linear System (X-MAS 2018)](#polynomial-coefficient-recovery-via-vandermonde-linear-system-x-mas-2018)
- [Rabin Decryption via Four-Roots CRT Combination (Pragyan CTF 2019)](#rabin-decryption-via-four-roots-crt-combination-pragyan-ctf-2019)

---

## BB-84 Quantum Key Distribution MITM Attack (PlaidCTF 2017)

**Pattern:** In simulated BB-84 QKD without authentication, perform a full man-in-the-middle by independently negotiating with both Alice and Bob.

```python
# Strategy: Always use basis Z, always send value 1 to Bob
# Alice side: measure in random bases, record results
# Bob side: always receives 1 in basis Z
# Bob's key = all 1s (known to attacker)
# Alice's key = attacker's measured qbit values

# Heuristic: throttle Bob's correct-guess count to match Alice's
# Both parties verify by comparing subset of bits — attacker controls both sides
for qbit in alice_qbits:
    my_basis = 'Z'  # always measure in Z basis
    my_value = measure(qbit, my_basis)
    send_to_bob(basis='Z', value=1)  # always send 1

# After basis reconciliation:
# key_with_alice = [measured values where bases matched]
# key_with_bob = [all 1s]
```

**Key insight:** BB-84 QKD is secure only with authenticated classical channels. Without authentication, an attacker can independently negotiate keys with both parties. Forcing a constant value to one party makes their key entirely predictable, while the other party's key is captured through measurement.

**References:** PlaidCTF 2017

---

## ElGamal Trivial DLP When B = p-1 (Hack.lu 2017)

**Pattern:** ElGamal public key `B = g^key mod p`. If `B + 1 == p`, then `B = p-1 = -1 mod p`. By Euler's criterion, `g^((p-1)/2) ≡ -1 (mod p)` for any primitive root `g`. Therefore `g^key ≡ g^((p-1)/2) (mod p)`, so `key = (p-1)/2` directly. No DLP algorithm needed.

```python
# Check for trivial case
if (B + 1) == p:
    key = (p - 1) // 2
    # Verify
    assert pow(g, key, p) == B
    # Decrypt ElGamal: shared_secret = pow(ephemeral, key, p)
```

**Key insight:** The generator raised to `(p-1)/2` always equals `-1 mod p` (Euler's criterion for quadratic residues). When the public key `B` equals `p-1`, the private key is trivially `(p-1)/2`. Always check `B == p-1` (and `B == 1` for key=0) before attempting general DLP.

**References:** Hack.lu CTF 2017

---

## Paillier LSB Oracle via Homomorphic Doubling (CODE BLUE 2017)

**Pattern:** Paillier encryption is additively homomorphic: multiplying a ciphertext by itself (`ct^2 mod n^2`) doubles the plaintext. Doubling repeatedly and observing when the LSB changes (due to modular reduction by n) reveals plaintext bits one at a time — a binary search identical to the RSA LSB oracle.

**Attack (bit-by-bit recovery from MSB to LSB):**
```python
def paillier_double(ct, n):
    """Homomorphically double the plaintext."""
    return pow(ct, 2, n * n)

def recover_plaintext(ct, oracle_lsb, n):
    """Oracle returns LSB of decrypted plaintext."""
    lower, upper = 0, n
    current_ct = ct
    for _ in range(n.bit_length()):
        current_ct = paillier_double(current_ct)
        lsb = oracle_lsb(current_ct)
        mid = (lower + upper) // 2
        if lsb == 1:
            lower = mid  # plaintext > n/2, wraparound occurred
        else:
            upper = mid
    return lower

# Alternative: homomorphic subtraction to isolate each bit
def paillier_encrypt_scalar(m, n, g=None):
    """Encrypt scalar m under Paillier (with r=1 for known randomness)."""
    g = g or (n + 1)
    return pow(g, m, n * n)  # simplified (r=1)

def subtract_plaintext(ct, val, n):
    """Compute E(pt - val) = ct * E(-val) mod n^2."""
    neg_enc = paillier_encrypt_scalar(n - val, n)
    return (ct * neg_enc) % (n * n)
```

**Key insight:** Paillier's additive homomorphism enables a binary search oracle: doubling the plaintext via `ct^2` and observing LSB changes reveals one bit per query. Equivalently, use homomorphic subtraction of known masks to isolate each bit. Total queries: log2(n) ≈ 2048 for 2048-bit modulus.

**References:** CODE BLUE CTF 2017

---

## Differential Privacy Laplace Noise Cancellation (Pwn2Win 2017)

**Pattern:** Server implements differential privacy by adding Laplace noise (mean 0, scale λ) to character ordinals before returning them. Since Laplace noise has zero mean, querying the same position many times and averaging the results cancels the noise via the Law of Large Numbers.

```python
import requests
import statistics

def recover_char(position, num_queries=1000):
    """Average 1000 noisy responses to cancel Laplace noise."""
    samples = []
    for _ in range(num_queries):
        noisy_val = query_server(position)
        samples.append(noisy_val)
    # Mean converges to true value as queries → ∞
    true_val = round(statistics.mean(samples))
    return chr(true_val)

flag = ''.join(recover_char(i) for i in range(flag_length))
```

**Key insight:** Laplace differential privacy with zero mean is breakable with sufficient queries — averaging N samples reduces noise variance by factor N (standard error ∝ 1/sqrt(N)). With λ=1 and 1000 queries, the mean is within ±0.1 of the true value. Round to nearest integer to recover the exact character ordinal. This applies to any additive zero-mean noise mechanism.

**References:** Pwn2Win CTF 2017

---

## Homomorphic Encryption Oracle Bit-Extraction (Tokyo Westerns 2017)

**Pattern:** An encryption oracle has homomorphic properties — you can add 1 to the plaintext by performing a known operation on the ciphertext. Extract bits from an unknown plaintext by observing how the ciphertext changes as the plaintext value crosses power-of-2 boundaries.

**Low-bit extraction (observe overflow):**
```python
# Increment plaintext by 1 repeatedly via homomorphic add-1
# Detect when bit N overflows: ciphertext "wraps" at value 2^N
ct = target_ciphertext
for bit_pos in range(num_bits):
    threshold = 2 ** bit_pos
    # Add 1 repeatedly until bit flips
    increments = 0
    prev_ct = ct
    while True:
        ct = homomorphic_add_one(ct)
        increments += 1
        if bit_has_flipped(ct, prev_ct, bit_pos):
            low_bits = (threshold - increments) % threshold
            break
```

**High-bit extraction (divide by 2 on even values):**
```python
# Subtract recovered low bits to make value even
even_ct = homomorphic_subtract(target_ct, low_bits)
# Repeatedly divide by 2 and observe the resulting high bits
for i in range(high_bit_count):
    even_ct = homomorphic_halve(even_ct)
    high_bits = (high_bits << 1) | observe_lsb(even_ct)
```

**Key insight:** Homomorphic oracles enable bit-extraction: detect overflow in specific bit positions when incrementing for low bits; use division-by-2 on even numbers for high bits. The total number of queries scales linearly with the bit count of the plaintext.

**References:** Tokyo Westerns CTF 2017

---

## ElGamal over Matrices via Jordan Normal Form (SharifCTF 8)

**Pattern:** Discrete log on matrices: convert generator G to Jordan normal form, then extract exponent from off-diagonal elements.

```sage
G = Matrix(GF(p), [[...]])  # generator matrix
H = Matrix(GF(p), [[...]])  # H = G^alpha
J, P = G.jordan_form(transformation=True)
H_prime = ~P * H * P  # H in Jordan basis
# For Jordan block with eigenvalue lambda:
# J^alpha has alpha * lambda^(alpha-1) on super-diagonal
# alpha = J[3][3] * H_prime[3][4] / H_prime[3][3]
alpha = int(J[3][3] * H_prime[3][4] / H_prime[4][4])
```

**Key insight:** Matrix DLP reduces to scalar DLP when the matrix is diagonalizable, or to polynomial extraction when Jordan blocks have repeated eigenvalues. The super-diagonal element of J^alpha is `alpha * lambda^(alpha-1)`, giving alpha directly via division when lambda is known. For diagonalizable matrices, the DLP decomposes into independent scalar DLPs per eigenvalue. Always compute the Jordan form first to determine which reduction applies.

**References:** SharifCTF 8 (2018)

---

## OSS (Ong-Schnorr-Shamir) Signature Forgery via Pollard's Method (SharifCTF 8)

**Pattern:** Given two valid OSS signatures, forge a signature for the product of their messages using Pollard's composition formula:

```python
# OSS signature: (x, y) valid for message m if x^2 + k*y^2 = m (mod n)
# Pollard's forgery for m1*m2:
def forge_product(x1, y1, x2, y2, k, n):
    X = (x1*x2 + k*y1*y2) % n
    Y = (x1*y2 - x2*y1) % n
    return X, Y
# (X, Y) is a valid signature for m1*m2 mod n
```

**Key insight:** The OSS signature scheme is based on the quadratic form `x^2 + ky^2 = m (mod n)`. Pollard showed that these forms compose multiplicatively -- given signatures for m1 and m2, you can forge a signature for m1*m2 without the private key. This is a fundamental algebraic break, not an implementation bug. To sign an arbitrary target message `m_target`, factor it as a product of signed messages, or use the homomorphic property with `m1 = known_signed_message` and construct `m2 = m_target * modinv(m1, n)` if a signature for m2 is obtainable.

**References:** SharifCTF 8 (2018)

---

## Cayley-Purser Decryption Without Private Key (TJCTF 2018)

**Pattern:** Cayley-Purser is a matrix-based public-key system using 2x2 matrices modulo a prime. Public key is `(alpha, beta, gamma)` where `gamma = alpha^r * beta * alpha^(-r)` and `epsilon = gamma^s`. The private key is `r`, but decryption only needs a matrix `H` that satisfies `H * gamma = gamma * H`.

**Exploit:** Any matrix `H` commuting with `gamma` decrypts correctly, and the Cayley-Hamilton theorem lets you build one entirely from public values — no need to recover `r`.

```python
from sage.all import matrix, identity_matrix
import operator

# Given public alpha, beta, gamma, epsilon, mu (ciphertext)
invalpha = alpha.inverse()
# Recover scaling entry h via elementwise division
h_elems = (invalpha * gamma - gamma * beta)
h_denom = (beta - invalpha)
h = matrix([[h_elems[i][j] / h_denom[i][j] for j in range(2)] for i in range(2)])

H = h[0][0] * identity_matrix(2) + gamma
plaintext = (H.inverse() * epsilon * H) * mu * (H.inverse() * epsilon * H)
```

**Key insight:** Any commuting matrix works as the decryption key. Cayley-Hamilton guarantees that `H = c1 * I + c2 * gamma` commutes with `gamma`, and the needed scalar `c1` can be read off by comparing entries of `alpha^(-1) * gamma` against `gamma * beta`. Always check whether "private key" operations can be replaced by a commutation-equivalent derived from public data.

**References:** TJCTF 2018 — writeup 10680

---

## BIP39 Partial-Mnemonic Brute Force via Checksum (SECCON 2018)

**Pattern:** Challenge provides 23 of 24 BIP39 mnemonic words (Japanese wordlist) and the target flag = `md5(entropy)`. Each word encodes 11 bits, so the missing word costs only `2^11 = 2048` guesses. Validate each candidate by running BIP39's built-in SHA-256 checksum over the reassembled entropy — only the correct guess passes.

```python
from mnemonic import Mnemonic
lg = Mnemonic("japanese")
known = ["...23 words..."]
for w in lg.wordlist:
    try:
        if lg.check(" ".join(known + [w])):
            entropy = lg.to_entropy(" ".join(known + [w]))
            print(md5(entropy).hexdigest())
    except Exception: pass
```

**Key insight:** BIP39 has a built-in 4-bit-per-word checksum, so partial mnemonics are self-verifying. Same trick applies to Electrum's seed format and any mnemonic scheme with internal parity.

**References:** SECCON 2018 — mnemonic, writeup 12053

---

## Asmuth-Bloom Threshold Secret Sharing via CRT (X-MAS 2018)

**Pattern:** Instead of Shamir's polynomial interpolation, Asmuth-Bloom splits a secret `S` into shares `(s_i, p_i)` where each `s_i = S mod p_i` and `p_i` are pairwise-coprime primes. Recover `S` by applying the Chinese Remainder Theorem to any threshold number of shares.

```python
from sympy.ntheory.modular import crt
# shares = [(s1,p1), (s2,p2), ..., (sk,pk)]
residues = [s for s, _ in shares]
moduli   = [p for _, p in shares]
S, M = crt(moduli, residues)
flag = long_to_bytes(int(S))
```

**Key insight:** Threshold sharing schemes can be CRT-based, not polynomial. Recognise Asmuth-Bloom by the `(residue, modulus)` share format; Shamir's scheme only publishes `(x, y)` coordinates without moduli.

**References:** X-MAS CTF 2018 — writeup 12660

---

## Rabin Cryptosystem with Polynomial Primes (X-MAS 2018)

**Pattern:** Rabin key generator derives `p, q` from a polynomial in a base value `r`, e.g. `p = r^2 + 3`, `q = r^2 + 7`. Modulus `N = p*q` is a polynomial in `r`; solve for `r` via `iroot(N - known_constant, 4)`, then recover both primes and decrypt the four sqrt candidates in the usual way.

```python
from gmpy2 import iroot
# N = (r^2+3)(r^2+7) = r^4 + 10 r^2 + 21
r, _ = iroot(N - 21, 4)
p, q = r*r + 3, r*r + 7
x_p = pow(ct, (p+1)//4, p)
x_q = pow(ct, (q+1)//4, q)
# CRT combine x_p, x_q; flag is the candidate with known padding
```

**Key insight:** Any cryptosystem whose primes come from a polynomial in a small variable collapses under integer-root extraction. Recognize these by plotting `N` against hypothetical `r` guesses, or by seeing suspicious constant differences.

**References:** X-MAS CTF 2018 — writeups 12657, 12724

---

## LCG Period Detection for Unlimited Output Prediction (X-MAS 2018)

**Pattern:** Server uses an LCG for RNG with a short period. Send repeated requests until you see a previously-observed output — you've found the period. From that point forward, every future value is known because the LCG cycles.

```python
seen = {}
for i in itertools.count():
    v = fetch_next()
    if v in seen:
        period = i - seen[v]
        break
    seen[v] = i
# Now predict: future[i] == history[(i - period_start) % period]
```

**Key insight:** All LCGs are periodic and the period is bounded by `m`. Any output buffer long enough to contain the period gives you free prediction for the rest of the game.

**References:** X-MAS CTF 2018 — writeups 12668, 12669

---

## Polynomial Coefficient Recovery via Vandermonde Linear System (X-MAS 2018)

**Pattern:** Oracle evaluates a hidden degree-`n` polynomial at `n+1` points. Build the Vandermonde matrix of evaluation inputs and solve the linear system for coefficients; recovered polynomial reveals the secret constants.

```python
from sage.all import matrix, vector, GF
pts = [(x_i, f(x_i)) for x_i in range(degree+1)]
A = matrix([[xi**k for k in range(degree+1)] for xi, _ in pts])
b = vector([yi for _, yi in pts])
coeffs = A.solve_right(b)
```

**Key insight:** Any secret polynomial, Shamir-style sharing, or "interpolate a curve" oracle falls to a Vandermonde solve with `degree + 1` points. Sage's `solve_right` handles huge degrees.

**References:** X-MAS CTF 2018 — writeup 12722

---

## Rabin Decryption via Four-Roots CRT Combination (Pragyan CTF 2019)

**Pattern (Help Rabin):** Rabin encrypts `c = m^2 mod n` with `n = p*q` and `p, q ≡ 3 mod 4`. Once `p, q` are recovered (here by Fermat-style square-root search because `q = nextPrime(p+1)` sits right next to `p`), compute `mp = c^((p+1)/4) mod p` and `mq = c^((q+1)/4) mod q`, then combine via extended GCD to yield four square roots `±r, ±s`. Only one of the four decodes to readable text — that's the plaintext.

```python
from Crypto.Util.number import inverse

def ext_gcd(a, b):
    c0, c1, a0, a1, b0, b1 = a, b, 1, 0, 0, 1
    while c1:
        q, r = divmod(c0, c1)
        c0, c1 = c1, r
        a0, a1 = a1, a0 - q * a1
        b0, b1 = b1, b0 - q * b1
    return a0, b0, c0

# p, q already recovered (e.g. via Fermat: p ~ sqrt(n))
pe, qe = (p + 1) // 4, (q + 1) // 4
mp, mq = pow(c, pe, p), pow(c, qe, q)
yp, yq, _ = ext_gcd(p, q)                      # yp*p + yq*q == 1

r1 = (yp * p * mq + yq * q * mp) % n
r2 = n - r1
s1 = (yp * p * mq - yq * q * mp) % n
s2 = n - s1

for cand in (r1, r2, s1, s2):
    try:
        pt = bytes.fromhex(hex(cand)[2:])
        if pt.isascii(): print(pt)            # pick the readable one
    except Exception: pass
```

**Key insight:** Rabin decryption inherently produces four candidates because `x^2 ≡ c mod n` has four roots mod `n = p*q`. When `p, q ≡ 3 mod 4`, per-prime roots are the closed-form exponentiation `c^((p+1)/4) mod p` — no Tonelli-Shelanks needed. Combine with Bezout coefficients `yp*p + yq*q = 1` to get the four CRT candidates `±(yp*p*mq ± yq*q*mp) mod n`, and select by plaintext sanity (ASCII, magic bytes, known prefix). The four-root ambiguity is why Rabin typically needs redundancy in the plaintext to be useful as a cryptosystem.

