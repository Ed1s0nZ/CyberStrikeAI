# CTF Crypto - RSA Attacks (Part 2: Specialized Techniques)

## Table of Contents
- [RSA p=q Validation Bypass (BearCatCTF 2026)](#rsa-pq-validation-bypass-bearcatctf-2026)
- [RSA Cube Root CRT when gcd(e, phi) > 1 (BearCatCTF 2026)](#rsa-cube-root-crt-when-gcde-phi--1-bearcatctf-2026)
- [Factoring n from Multiple of phi(n) (BearCatCTF 2026)](#factoring-n-from-multiple-of-phin-bearcatctf-2026)
- [RSA Signature Forgery via Multiplicative Homomorphism (MMA CTF 2015)](#rsa-signature-forgery-via-multiplicative-homomorphism-mma-ctf-2015)
- [Weak RSA Key Generation via Base Representation (Sharif CTF 2016)](#weak-rsa-key-generation-via-base-representation-sharif-ctf-2016)
- [RSA with gcd(e, phi(n)) > 1 (CSAW 2015)](#rsa-with-gcde-phin--1-csaw-2015)
- [Batch GCD for Shared Prime Factoring (BSidesSF 2025)](#batch-gcd-for-shared-prime-factoring-bsidessf-2025)
- [RSA Partial Key Recovery from dp dq qinv (0CTF 2016)](#rsa-partial-key-recovery-from-dp-dq-qinv-0ctf-2016)
- [RSA-CRT Fault Attack / Bit-Flip Recovery (CSAW CTF 2016)](#rsa-crt-fault-attack--bit-flip-recovery-csaw-ctf-2016)
- [RSA Homomorphic Decryption Oracle Bypass (ECTF 2016)](#rsa-homomorphic-decryption-oracle-bypass-ectf-2016)
- [RSA with Small Prime Factors and CRT Decomposition (Hack The Vote 2016)](#rsa-with-small-prime-factors-and-crt-decomposition-hack-the-vote-2016)
- [RSA Timing Attack on Montgomery Reduction (DEF CON 2017)](#rsa-timing-attack-on-montgomery-reduction-def-con-2017)
- [Bleichenbacher Low-Exponent RSA Signature Forgery (Google CTF 2017)](#bleichenbacher-low-exponent-rsa-signature-forgery-google-ctf-2017)
- [Coppersmith Small Roots for Linearly Related Primes (Tokyo Westerns 2017)](#coppersmith-small-roots-for-linearly-related-primes-tokyo-westerns-2017)
- [ROCA Attack on RSA CVE-2017-15361 (EasyCTF IV)](#roca-attack-on-rsa-cve-2017-15361-easyctf-iv)
- [RSA Signature Bypass with e=1 and Crafted Modulus (BackdoorCTF 2018)](#rsa-signature-bypass-with-e1-and-crafted-modulus-backdoorctf-2018)
- [Dependent-Prime RSA: q = e^-1 mod p (TokyoWesterns CTF 4th 2018)](#dependent-prime-rsa-q--e-1-mod-p-tokyowesterns-ctf-4th-2018)
- [RSA Three-Key Pairwise GCD Triangle (Trend Micro 2018)](#rsa-three-key-pairwise-gcd-triangle-trend-micro-2018)
- [RSA n = p^2*q Schmidt-Samoa Variant (ASIS Finals 2018)](#rsa-n--p2q-schmidt-samoa-variant-asis-finals-2018)
- [Modulus Recovery via GCD of Encryption Residuals (X-MAS CTF 2018)](#modulus-recovery-via-gcd-of-encryption-residuals-x-mas-ctf-2018)
- [Textbook RSA Negation via encrypt(-1) (X-MAS CTF 2018)](#textbook-rsa-negation-via-encrypt-1-x-mas-ctf-2018)
- [Poly-Exponent RSA: GCD of p^p Combinations (ASIS Finals 2018)](#poly-exponent-rsa-gcd-of-pp-combinations-asis-finals-2018)
- [Biased LSB Oracle with Mode-of-Runs Recovery (CSAW CTF 2018)](#biased-lsb-oracle-with-mode-of-runs-recovery-csaw-ctf-2018)
- [Cube-Root Wraparound via AES-CTR Length Hint (hxp 2018)](#cube-root-wraparound-via-aes-ctr-length-hint-hxp-2018)
- [RSA p = next_prime(2^k + small) Shared-Prime Batch GCD (ASIS Finals 2018)](#rsa-p--next_prime2k--small-shared-prime-batch-gcd-asis-finals-2018)
- [PNG Encryption Bounded by 512-bit Key → Trailer Replacement (ASIS Finals 2018)](#png-encryption-bounded-by-512-bit-key--trailer-replacement-asis-finals-2018)
- [Modulus Recovery via Plaintext Malleability (X-MAS 2018)](#modulus-recovery-via-plaintext-malleability-x-mas-2018)
- [RSA CRT d_p NULL-Byte Overflow Primes Leak (P.W.N. CTF 2018)](#rsa-crt-d_p-null-byte-overflow-primes-leak-pwn-ctf-2018)
- [Textbook RSA Signature Blinding via Message Factoring (P.W.N. CTF 2018)](#textbook-rsa-signature-blinding-via-message-factoring-pwn-ctf-2018)
- [Last-Byte Modulus Overwrite via strlen-1 Null Truncation (OTW Advent 2018)](#last-byte-modulus-overwrite-via-strlen-1-null-truncation-otw-advent-2018)
- [CRC32 Collision Oracle + RSA Homomorphic Signature Forgery (BSidesSF 2019)](#crc32-collision-oracle--rsa-homomorphic-signature-forgery-bsidessf-2019)

See also: [rsa-attacks.md](rsa-attacks.md) for foundational RSA attacks (small e, Wiener, Fermat, Pollard, Hastad, common modulus, Manger oracle, Coppersmith).

---

## RSA p=q Validation Bypass (BearCatCTF 2026)

**Pattern (Pickme):** Server validates user-submitted RSA key (checks `n`, `e`, `d`, `p*q=n`, `e*d ≡ 1 mod phi`), encrypts the flag, then tries test decryption. If decryption fails, leaks ciphertext in error message.

**Exploit:** Set `p = q`. The server computes `phi = (p-1)*(q-1) = (p-1)^2` (incorrect — real totient of `p^2` is `p*(p-1)`). All validation checks pass, but decryption with the wrong `d` fails, leaking the ciphertext.

```python
from Crypto.Util.number import getPrime, inverse

p = getPrime(512)
q = p  # p = q!
n = p * q  # = p^2
e = 65537
wrong_phi = (p - 1) * (q - 1)  # = (p-1)^2
d = inverse(e, wrong_phi)  # passes server validation

# Server encrypts flag with our key, test decryption fails → leaks ciphertext c
# Decrypt with correct totient:
real_phi = p * (p - 1)
real_d = inverse(e, real_phi)
flag = pow(c, real_d, n)
```

**Key insight:** `phi(p^2) = p*(p-1)`, NOT `(p-1)^2`. When a server validates RSA parameters but uses `(p-1)*(q-1)` without checking `p != q`, setting `p=q` creates a working key that the server will miscompute the private exponent for, causing decryption failure and error-path data leakage.

---

## RSA Cube Root CRT when gcd(e, phi) > 1 (BearCatCTF 2026)

**Pattern (Kidd's Crypto):** RSA with `e=3`, modulus composed of many small primes all ≡ 1 (mod 3). Since each `p-1` is divisible by 3, `gcd(e, phi(n)) = 3^k` and the standard modular inverse `d = e^-1 mod phi` doesn't exist.

**Solution:** Compute cube roots per-prime via CRT:
```python
from sympy.ntheory.residues import nthroot_mod
from sympy.ntheory.modular import crt

primes = [p1, p2, ..., p13]  # All ≡ 1 mod 3

# For each prime, find all 3 cube roots of c mod p
roots_per_prime = []
for p in primes:
    roots = nthroot_mod(c % p, 3, p, all_roots=True)
    roots_per_prime.append(roots)

# Try all 3^13 = 1,594,323 combinations
from itertools import product
for combo in product(*roots_per_prime):
    result, mod = crt(primes, list(combo))
    try:
        text = long_to_bytes(result).decode('ascii')
        if text.isprintable():
            print(f"Flag: {text}")
            break
    except:
        continue
```

**Key insight:** When `gcd(e, phi(n)) > 1`, standard RSA decryption fails. Factor `n`, compute eth roots modulo each prime separately (each prime ≡ 1 mod e gives `e` roots), then enumerate all CRT combinations. Feasible when the number of primes is small (3^13 ≈ 1.6M combinations).

---

## Factoring n from Multiple of phi(n) (BearCatCTF 2026)

**Pattern (Twisted Pair):** Given RSA `n` and a leaked pair `(re, rd)` where `re * rd ≡ 1 (mod k*phi(n))`. The value `re*rd - 1` is a multiple of `phi(n)`, enabling probabilistic factoring.

```python
import random
from math import gcd

def factor_from_phi_multiple(n, phi_multiple):
    """Factor n given any multiple of phi(n) using Miller-Rabin variant."""
    # Write phi_multiple = 2^s * d where d is odd
    s, d = 0, phi_multiple
    while d % 2 == 0:
        s += 1
        d //= 2

    for _ in range(100):  # 100 attempts
        a = random.randrange(2, n - 1)
        x = pow(a, d, n)
        if x == 1 or x == n - 1:
            continue
        for _ in range(s - 1):
            prev = x
            x = pow(x, 2, n)
            if x == n - 1:
                break
            if x == 1:
                # prev is non-trivial square root of 1
                p = gcd(prev - 1, n)
                if 1 < p < n:
                    return p, n // p
        # Check final
        if x != n - 1:
            p = gcd(x - 1, n)
            if 1 < p < n:
                return p, n // p
    return None

phi_mult = re * rd - 1
p, q = factor_from_phi_multiple(n, phi_mult)
```

**Key insight:** Any multiple of `phi(n)` — not just `phi(n)` itself — enables factoring via the Miller-Rabin square root technique. If a server leaks `e*d` for any key pair, or if `re*rd - 1` is given, compute `gcd(a^(m/2) - 1, n)` for random `a` values. Succeeds with probability ≥ 1/2 per attempt.

---

## RSA Signature Forgery via Multiplicative Homomorphism (MMA CTF 2015)

**Pattern:** Signing oracle refuses to sign the target message `m` but will sign other values. Unpadded RSA is multiplicatively homomorphic: `S(a) * S(b) mod n == S(a * b) mod n`.

```python
# Factor target message and sign each factor separately
divisor = 2
assert target_msg % divisor == 0
sig_a = sign_oracle(target_msg // divisor)
sig_b = sign_oracle(divisor)
forged_sig = (sig_a * sig_b) % n
```

**Key insight:** Textbook RSA signatures are homomorphic: `m^d mod n` preserves multiplication. If the oracle blacklists `m` but signs its factors, multiply the partial signatures. To find a suitable factorization, try small divisors (2, 3, ...) until `m / divisor` also passes the blacklist check. This is why PKCS#1 padding is essential — padded messages cannot be factored into other valid padded messages.

---

## Weak RSA Key Generation via Base Representation (Sharif CTF 2016)

When RSA primes are generated as `p = kp * B + tp` where B = product_of_small_primes * 2^400 and kp is small (< 2^12):

1. **Compute n mod B^2:** Since n = p*q and both p,q have form k*B + t, expansion gives: `n = kp*kq*B^2 + (kp*tq + kq*tp)*B + tp*tq`
2. **Recover kp*kq:** Brute-force 2^24 possibilities for (kp, kq) where each < 2^12
3. **Solve quadratic:** From known kp*kq and the middle coefficient, recover tp and tq

```python
B = product_of_first_443_primes * (2**400)
B2 = B * B

# n = A*B^2 + C*B + D where A=kp*kq, D=tp*tq
A = n // B2
D = n % B

# Brute-force kp, kq such that kp*kq == A
for kp in range(1, 2**12):
    if A % kp == 0:
        kq = A // kp
        # Solve for tp, tq from remaining equations
```

**Key insight:** Structured prime generation creates a mixed-radix representation of n, allowing efficient factorization by reducing the search space from exponential to polynomial.

---

## RSA with gcd(e, phi(n)) > 1 (CSAW 2015)

When `gcd(e, phi(n)) = g > 1`, standard RSA decryption fails because `d = e^(-1) mod phi(n)` doesn't exist. Instead:

1. Compute `e' = e / g` (reduced exponent)
2. Compute `d' = e'^(-1) mod phi(n)` (now coprime)
3. Compute `m^g = pow(c, d', n)` (partial decryption)
4. Take g-th root: iterate candidate m values where `pow(m, g, n) == m^g`

```python
from sympy import factorint, mod_inverse
from gmpy2 import iroot

g = gcd(e, phi_n)
e_prime = e // g
d_prime = mod_inverse(e_prime, phi_n)
m_g = pow(c, d_prime, n)

# For small g, try integer root first
m, is_exact = iroot(m_g, g)
if is_exact:
    plaintext = int(m)
else:
    # Brute-force: m_g + k*n for small k
    for k in range(10000):
        m, exact = iroot(m_g + k * n, g)
        if exact:
            plaintext = int(m)
            break
```

**Key insight:** Reduce e by the GCD to make decryption possible, then recover the g-th root. Filter candidates by checking plaintext length or ASCII validity.

---

## Batch GCD for Shared Prime Factoring (BSidesSF 2025)

When multiple RSA moduli share a common prime factor (due to faulty hardware RNG, smartcard bugs, or weak seeding):

```python
from math import gcd
from functools import reduce

def batch_gcd(moduli):
    """Find shared factors among a list of RSA moduli"""
    # Product tree
    product = reduce(lambda a, b: a * b, moduli)

    factors = {}
    for n in moduli:
        g = gcd(n, product // n)
        if g != 1 and g != n:
            p = g
            q = n // p
            factors[n] = (p, q)
    return factors

# Usage: given list of public keys from smartcards
moduli = [key.n for key in public_keys]
shared = batch_gcd(moduli)
for n, (p, q) in shared.items():
    phi = (p - 1) * (q - 1)
    d = pow(e, -1, phi)  # Private exponent
```

For keys with patterned primes (hardware RNG faults producing primes with fixed bit patterns), combine with Coppersmith's method to recover remaining random bits. See [advanced-math.md](advanced-math.md) for Coppersmith.

**Key insight:** A single shared prime compromises both keys. Batch GCD runs in O(n log n) time via product/remainder trees, making it feasible for thousands of keys. Real-world incidents: Taiwanese citizen smartcards (2013), many IoT device certificates.

---

## RSA Partial Key Recovery from dp dq qinv (0CTF 2016)

**Pattern:** Given only the CRT (Chinese Remainder Theorem) exponents (dp, dq, qinv) from a partial PEM (Privacy Enhanced Mail) key leak (e.g., bottom portion of private key file), recover the full key. Since `dp = d mod (p-1)`, iterate k and check if `p = (dp * e - 1) / k + 1` is prime.

```python
import gmpy2
# dp, dq, qinv extracted from partial PEM; e is known (usually 65537)
for k in range(3, e):
    p_candidate = (dp * e - 1) // k + 1
    if gmpy2.is_prime(p_candidate):
        p = p_candidate
        break
# Similarly recover q from dq; verify qinv * q % p == 1
```

**Key insight:** Leaking just the CRT exponents from an RSA private key is sufficient to fully recover p and q. Recovery is O(e) -- essentially instant for e=65537.

---

## RSA-CRT Fault Attack / Bit-Flip Recovery (CSAW CTF 2016)

RSA signing service with intermittent single-bit errors in private exponent d during CRT computation. Collect valid and faulty signatures, then detect which bit flipped.

```python
from Crypto.Util.number import inverse

def recover_d_bits(n, e, valid_sig, faulty_sigs, msg):
    """Recover private key d bit-by-bit from CRT fault signatures"""
    d_bits = [0] * 1024
    m = pow(msg, 1, n)
    s_good = valid_sig
    for s_bad in faulty_sigs:
        # ratio reveals which bit was flipped
        ratio = (s_bad * inverse(s_good, n)) % n
        for k in range(1024):
            if ratio == pow(2, pow(2, k, n), n) or ratio == pow(inverse(2, n), pow(2, k, n), n):
                d_bits[k] = 1
                break
    return d_bits
```

**Key insight:** When RSA-CRT has a single-bit fault in d, the ratio `faulty_sig * valid_sig^(-1) mod n` equals `2^(2^k) mod n` for the flipped bit position k, enabling bit-by-bit private key recovery.

---

## RSA Homomorphic Decryption Oracle Bypass (ECTF 2016)

Service decrypts any ciphertext except the target flag ciphertext. Exploit RSA's multiplicative homomorphism: `Dec(a * b mod n) = Dec(a) * Dec(b) mod n`.

```python
from Crypto.Util.number import long_to_bytes, inverse

# Server refuses to decrypt enc_flag directly
# But RSA is homomorphic: Dec(A*B) = Dec(A) * Dec(B) mod n
enc_2 = pow(2, e, n)  # encrypt the number 2
enc_flag_times_2 = (enc_flag * enc_2) % n  # = Enc(flag * 2)

dec_flag_times_2 = oracle_decrypt(enc_flag_times_2)  # server allows this
dec_2 = oracle_decrypt(enc_2)                         # server allows this

# Recover flag: (flag * 2) * inverse(2) mod n = flag
flag = (dec_flag_times_2 * inverse(dec_2, n)) % n
print(long_to_bytes(flag))
```

**Key insight:** RSA without padding is multiplicatively homomorphic. Multiply the forbidden ciphertext by `Enc(r)` for any `r`, decrypt the product, then divide by `r` to recover the original plaintext.

---

## RSA with Small Prime Factors and CRT Decomposition (Hack The Vote 2016)

RSA modulus composed of many small prime factors (primes < 251, each appearing ~1500 times). Factor n, then use CRT to decompose decryption.

```python
from sympy import factorint
from sympy.ntheory.residues import primitive_root
from functools import reduce

n = ...  # modulus with small factors
e = 65537
c = ...  # ciphertext

factors = factorint(n)  # {p1: k1, p2: k2, ...} where pi are small primes

# Decrypt modulo each prime power, then combine with CRT
from sympy.ntheory.modular import crt as chinese_remainder_theorem

remainders = []
moduli = []
for p, k in factors.items():
    pk = p ** k
    phi_pk = (p - 1) * p ** (k - 1)  # Euler's totient for prime power
    d_pk = pow(e, -1, phi_pk)
    m_pk = pow(c, d_pk, pk)
    remainders.append(m_pk)
    moduli.append(pk)

# Combine using CRT
m = chinese_remainder_theorem(moduli, remainders)[0]
```

**Key insight:** When n has many small prime factors, compute `d mod phi(p^k)` for each prime power independently, decrypt mod each, then combine via CRT. Much faster than computing `d mod phi(n)` directly.

---

## RSA Timing Attack on Montgomery Reduction (DEF CON 2017)

**Pattern:** Recover RSA private key bits via Kocher's timing attack when the number of modular subtractions during Montgomery reduction is leaked.

```python
# Montgomery multiplication: extra subtraction when result >= modulus
# Leaked: count of extra subtractions per signature operation
# Attack: predict subtraction count for each private key bit guess

# For each bit position i (MSB to LSB):
#   Guess bit = 0: predict timing for square only
#   Guess bit = 1: predict timing for square + multiply
#   Compare predictions against observed timing data
#   Correct guess produces statistically significant correlation

# ~200K signatures needed for 768-bit key recovery
# Attacking squaring reduction is more effective than multiply
import numpy as np
for bit_pos in range(key_bits):
    for guess in [0, 1]:
        predicted = predict_reductions(known_bits + [guess], messages)
        correlation = np.corrcoef(predicted, observed)[0, 1]
    known_bits.append(0 if corr_0 > corr_1 else 1)
```

**Key insight:** Montgomery multiplication performs an extra conditional subtraction when the intermediate result exceeds the modulus. If this count leaks (via timing, power, or as in this CTF -- directly), each bit of the private exponent can be determined by comparing predicted vs. observed subtraction patterns across many operations.

**References:** DEF CON 2017

---

## Bleichenbacher Low-Exponent RSA Signature Forgery (Google CTF 2017)

**Pattern:** Forge PKCS#1 v1.5 RSA signatures when e=3 by constructing a value whose cube root produces valid padding without knowing the private key.

```python
# PKCS#1 v1.5 signature format:
# 00 01 FF FF ... FF 00 [DigestInfo] [Hash]
# With e=3, forge signature s where s^3 has correct prefix

# Construct target block (2048-bit key, SHA-256):
# 00 01 FF ... FF 00 [SHA-256 DigestInfo] [hash] [garbage]
import gmpy2
prefix = b'\x00\x01' + b'\xff' * padding_len + b'\x00' + digest_info + hash_value
# Convert to integer, append zeros for garbage bytes
target = int.from_bytes(prefix + b'\x00' * garbage_len, 'big')
# Cube root (rounds down, garbage absorbs the remainder)
forged_sig = gmpy2.iroot(target, 3)[0] + 1  # +1 to round up
# Verify: forged_sig^3 starts with correct PKCS#1 prefix
```

**Key insight:** PKCS#1 v1.5 signature verification checks that `sig^e mod n` starts with `00 01 FF...FF 00 DigestInfo Hash`. With e=3, an attacker computes the cube root of a carefully constructed value with the correct prefix and ignores trailing bytes. Implementations that don't verify the padding extends to the full block length (CVE-2006-4339) accept the forgery.

**References:** Google CTF 2017

---

## Coppersmith Small Roots for Linearly Related Primes (Tokyo Westerns 2017)

**Pattern:** RSA where `q = k*p + delta` for a known small constant `k` and unknown small `delta`. Since `p ≈ sqrt(N/k)`, approximate `q_approx = k * isqrt(N // k) + 2^512` as an upper bound. The univariate polynomial `F(x) = q_approx - x` has `delta` as a small root modulo `q` (which divides N). Coppersmith's method finds this root when `delta < N^(1/4)`.

```python
from sage.all import *

N, e, c = ...  # RSA parameters
k = 19  # known relationship: q = k*p + delta

# Approximate q from sqrt(N/k)
q_approx = k * isqrt(N // k) + 2**512

# Build univariate polynomial with q_approx as approximate root of N mod q
R.<x> = PolynomialRing(Zmod(N))
f = q_approx - x  # root is delta = q_approx - q

# Coppersmith: find small root x = delta < N^0.25
roots = f.small_roots(X=2**512, beta=0.5)
if roots:
    q = int(q_approx - roots[0])
    p = N // q
    assert p * q == N
    phi = (p - 1) * (q - 1)
    d = pow(e, -1, phi)
    flag = long_to_bytes(pow(c, d, N))
```

**Key insight:** When `q ≈ k*p`, approximately half the bits of `p` (and `q`) are recoverable from `sqrt(N/k)`. The remaining unknown `delta` is small enough for Coppersmith when `delta < N^(1/4)`. The upper bound `q_approx` must exceed `q`; add a safety margin of `2^(bitlen/2)` to ensure the root is captured.

**References:** Tokyo Westerns CTF 2017

---

## ROCA Attack on RSA CVE-2017-15361 (EasyCTF IV)

**Pattern:** Infineon RSA library generates keys with structured primes (detectable via fingerprint). Factor 512-bit keys in minutes:

```bash
# Detect ROCA vulnerability
pip install roca-detect
roca-detect rsa_key.pub
# Factor with neca tool
git clone https://gitlab.com/jix/neca.git
cd neca && cargo build --release
./target/release/neca <N_decimal>
# Or use: https://github.com/crocs-muni/roca (original research tool)
```

**Key insight:** CVE-2017-15361 affects Infineon TPMs, smart cards, and YubiKey 4. Primes have the form `p = k * M + (65537^a mod M)` where M is the product of first n primes. Detection is instant (check `65537^x mod M` divides `n mod M`). Factoring uses Coppersmith's method on the known structure. Keys up to 2048 bits are practically attackable. For CTF use: if `roca-detect` reports vulnerable, use `neca` for 512-bit keys or the `crocs-muni/roca` tool for larger keys.

**References:** EasyCTF IV (2018)

---

### RSA Signature Bypass with e=1 and Crafted Modulus (BackdoorCTF 2018)

**Pattern:** Server generates RSA signature and asks for `(n, e)` to verify it. With `e=1`, `pow(s, 1, n) = s mod n`. Set `n = signature - PKCS1_pad(message)` so verification passes. (BackdoorCTF 2018)

```python
e = 1
n = signature ** e - PKCS1_pad(h.hexdigest())
# Now pow(signature, 1, n) == PKCS1_pad(message)
```

**Key insight:** When the verifier accepts user-supplied public key parameters without constraints, setting `e=1` makes modular exponentiation trivial. Choose `n` such that `signature mod n` equals the expected padded hash.

---

## Dependent-Prime RSA: q = e^-1 mod p (TokyoWesterns CTF 4th 2018)

**Pattern:** Key generation picks prime `p`, then sets `q = e^(-1) mod p` and rerolls until `q` is prime. The dependency `e*q ≡ 1 (mod p)` means `e*q = k*p + 1` for some small positive integer `k`, so `n = p*q = p*(k*p + 1)/e` — a quadratic in `p` that factors after enumerating a few candidate `k` values.

**Exploit:**
```python
from sage.all import PolynomialRing, ZZ

def factor_dependent_n(n, e, max_k=100000):
    P = PolynomialRing(ZZ, 'p'); p = P.gen()
    for k in range(2, max_k, 2):
        # e*q = k*p + 1 and n = p*q  =>  e*n = p*(k*p + 1)
        poly = k * p * p + p - e * n
        roots = poly.roots()
        for root, _ in roots:
            if root > 1 and n % root == 0:
                return int(root), n // int(root)
    return None
```

**Key insight:** Any key generator that derives `q` from `p` via a public arithmetic relation collapses RSA security to a small search. Write the relation as a polynomial `f_k(p) = 0` parameterized by a small integer, and use `.roots()` in Sage to recover `p`. The search space for `k` is typically under 2^16 because `q < p` forces `k ≈ e`.

**References:** TokyoWesterns CTF 4th 2018 — writeup 10862

---

## RSA Three-Key Pairwise GCD Triangle (Trend Micro 2018)

**Pattern:** Three RSA moduli `N1 = p1*p2`, `N2 = p1*p3`, `N3 = p2*p3` share primes pairwise (every pair has exactly one common factor). A single pairwise `gcd` reveals the shared prime, giving a complete factorisation of all three moduli without running batch-GCD.

```python
from math import gcd

def factor_triangle(n1, n2, n3):
    p1 = gcd(n1, n2)        # shared between N1 and N2
    p2 = gcd(n1, n3)        # shared between N1 and N3
    p3 = gcd(n2, n3)        # shared between N2 and N3
    assert n1 == p1 * p2 and n2 == p1 * p3 and n3 == p2 * p3
    return p1, p2, p3
```

**Key insight:** Batch-GCD handles arbitrary sets of moduli that might share a prime, but the three-key triangle is a closed-form case: every prime appears in exactly two moduli, so three `gcd()` calls factor all three keys. Spot the pattern when a challenge hands you exactly three public keys with matching bit-lengths and no other hint — contrast with [batch GCD for shared prime factoring](#batch-gcd-for-shared-prime-factoring-bsidessf-2025), which is the general case over larger key sets.

**References:** Trend Micro CTF 2018 — Offensive-Analysis 300, writeup 11129

---

## RSA n = p^2*q Schmidt-Samoa Variant (ASIS Finals 2018)

**Pattern:** Modulus is generated as `n = p*p*q` (not the usual `p*q`). Naive `phi = (p-1)*(q-1)` is wrong — real totient is `phi = p*(p-1)*(q-1)`. If `gcd(e, phi) != 1`, reduce ciphertext to smaller field `q` and invert there.

```python
phi = p*(p-1)*(q-1)
# Reduce enc to mod-q by inverse_mod(q, phi)
qinv = inverse_mod(q, phi)
enc = pow(enc, qinv, n) % q
# Now m < q; invert e*p^2 over phi(q) = q-1
pinv = inverse_mod(p*p, q-1)
m = pow(enc, pinv, q)
```

**Key insight:** Spot `n = p^2*q` by trying `iroot(n, 3)` near its cube root and running Pollard/Lenstra on small fraction of `n`. Once factored, switch to the correct totient and field before inversion.

**References:** ASIS CTF Finals 2018 — John-Bull, writeup 12401

---

## Modulus Recovery via GCD of Encryption Residuals (X-MAS CTF 2018)

**Pattern:** Oracle encrypts arbitrary plaintexts with a hidden RSA key but does not disclose `n`. Compute `m^e - enc(m)` for two plaintexts; both differences are multiples of `n`, so their GCD equals `n` (or a small multiple).

```python
e = 65537
r1 = bytes_to_long(b'a')**e - encrypt(b'a')
r2 = bytes_to_long(b'b')**e - encrypt(b'b')
n = gcd(r1, r2)
```

**Key insight:** Works whenever the oracle is a black box but accepts chosen plaintexts. Strip small prime factors from the GCD to recover the true `n`.

**References:** X-MAS CTF 2018 — Santa's list, writeups 12701-12800

---

## Textbook RSA Negation via encrypt(-1) (X-MAS CTF 2018)

**Pattern:** Decrypt oracle refuses to decrypt the target ciphertext directly but accepts multiplications. Because `(-1)^e ≡ -1 (mod n)`, multiplying by `pow(-1, e, n)` flips the plaintext sign modulo `n`.

```python
ct_mutated = (ct_flag * pow(-1, e, n)) % n
plaintext = (-decrypt(ct_mutated)) % n
```

**Key insight:** Any odd `e` makes `encrypt(-1) = -1 mod n`, so the oracle returns `-m mod n` for the original ciphertext even if the original `m` is blacklisted.

**References:** X-MAS CTF 2018 — Santa's list 2.0, writeups 12701-12800

---

## Poly-Exponent RSA: GCD of p^p Combinations (ASIS Finals 2018)

**Pattern:** Challenge publishes several linear/polynomial combinations of `p` and `q`, e.g. `c1 = p^p mod q`, `c2 = (p+q)^(p+q) mod n`. Form two expressions both divisible by the target prime, then take their GCD to factor `n`.

```python
p = gcd(c2*c4 - c3, pow(c4, c4, c2*c4 - c3) - c3)
q = gcd(c1*c4 - c3, pow(c4, c4, c1*c4 - c3) - c3)
for x in small_primes:
    while p % x == 0: p //= x
```

**Key insight:** If two quantities share a secret factor but differ only by known scalars, `gcd(a, b)` recovers that factor. Strip cofactors with trial division on small primes.

**References:** ASIS CTF Finals 2018 — NMC, writeup 12427

---

## Biased LSB Oracle with Mode-of-Runs Recovery (CSAW CTF 2018)

**Pattern:** LSB oracle leaks the least significant bit of `m * 2^i mod n`, but the binary search converges inconsistently (oracle gives slightly wrong answers at random). Run the full recovery loop many times and take the per-byte mode across runs — correct bytes appear most often even when the full plaintext never converges.

```python
def recover():
    beg, end = 0, n - 1
    for bit in bits:
        mid = (beg + end) // 2
        if bit: beg = mid
        else:   end = mid
    return long_to_bytes(end)

byte_counts = [Counter() for _ in range(flag_len)]
for _ in range(N):
    flag = recover()
    for i, b in enumerate(flag):
        byte_counts[i][b] += 1
flag = bytes(c.most_common(1)[0][0] for c in byte_counts)
```

**Key insight:** Noisy side channels still leak the correct byte per position if each run independently biases toward the true value. Vote across runs rather than trying to perfect a single decryption.

**References:** CSAW CTF 2018 — Lost Mind, writeup 12490

---

## Cube-Root Wraparound via AES-CTR Length Hint (hxp 2018)

**Pattern:** Low-exponent RSA (`e = 3`) with padded plaintext where `m^3 > n`, so naive cube root fails. AES-CTR ciphertext reveals the plaintext length (CTR preserves length), so you know exactly how many `n` to add. Rescale with `inverse(2, n)^k` to cancel padding shifts, then try `iroot(c + k*n, 3)` for small `k`.

```python
inv = pow(inverse(2, n), 2040, n)
c = c * inv % n
for k in range(1000):
    m, ok = gmpy2.iroot(c + k*n, 3)
    if ok: flag = long_to_bytes(int(m)); break
```

**Key insight:** When padding shifts the plaintext past `n`, the correct root lies in `c + k*n` for some small `k`; AES-CTR leaks the exact plaintext length so you can size the search.

**References:** hxp CTF 2018 — daring, writeup 12588

---

## RSA p = next_prime(2^k + small) Shared-Prime Batch GCD (ASIS Finals 2018)

**Pattern:** Key generator constructs `p = next_prime(2^k + random_small_delta)` where `delta` is bounded by a few bits. Two independent keys generated with the same `k` converge on the same next prime whenever their `delta` values fall between the same two consecutive primes — common enough that any pair of collected moduli has `gcd(n1, n2) > 1`.

```python
from math import gcd
for n_a in collected:
    for n_b in collected:
        if n_a == n_b: continue
        p = gcd(n_a, n_b)
        if 1 < p < n_a:
            q = n_a // p
            d = pow(e + 2, -1, (p-1)*(q-1))  # NMC-style
            break
```

**Key insight:** Any RSA keygen that sources primes from a low-entropy neighborhood (`next_prime(constant + small)`, `2^k + random_byte`, `Mersenne ± delta`) collapses to pairwise GCD because the `next_prime` step collapses many deltas onto the same prime.

**References:** ASIS CTF Finals 2018 — Ariogen, writeup 12414

---

## PNG Encryption Bounded by 512-bit Key → Trailer Replacement (ASIS Finals 2018)

**Pattern:** Custom "polynomial bit sum" cipher encrypts PNG bytes as `C = sum(bit_i * (exp^i + (-1)^i))`. The key length (≤512 bits) means at most 64 bytes of plaintext are affected. Since PNG's trailing IDAT + IEND chunks are format-recoverable from any reference image, simply splice a valid trailer onto the decrypted prefix.

```python
# Only the first 64 bytes of output depend on the key; the rest matches plaintext bit-for-bit
first64 = decrypt_affected_prefix(ct)
rest    = original_png[64:]                # copy from any reference PNG with same image data
open('recovered.png', 'wb').write(first64 + rest)
```

**Key insight:** Whenever the key size caps the *number of affected bytes*, tail-of-file reconstruction beats full cryptanalysis. Works for any format (PNG, ZIP, MP3, PDF) where the tail has standard chunks or a parser tolerates garbage.

**References:** ASIS CTF Finals 2018 — Made by baby, writeup 12423

---

## Modulus Recovery via Plaintext Malleability (X-MAS 2018)

**Pattern:** Oracle decrypts ciphertext and returns the plaintext but hides the modulus `N`. Send two related ciphertexts `c` and `c * 2^e`. The decrypted plaintexts are `m` and `(2m) mod N`. If `2*m != (2m mod N)`, the difference `2m - (2m mod N) = N` recovers the modulus.

```python
m1 = decrypt(ct)                       # m
m2 = decrypt((ct * pow(2, e, m1)) % m1)  # 2m mod N
N = 2*m1 - m2 if 2*m1 != m2 else None
```

**Key insight:** RSA's multiplicative homomorphism leaks the modulus whenever you can send `c * k^e` and observe the wraparound. Use any small multiplier that nudges the plaintext past `N`.

**References:** X-MAS CTF 2018 — Santa's list, writeup 12658

---

## RSA CRT d_p NULL-Byte Overflow Primes Leak (P.W.N. CTF 2018)

**Pattern:** Server reads `d_p` into a buffer via `fgets()` that lacks a bounds check. Sending 33+ null bytes NULL-terminates `d_p_str` so the parsed integer is 0. The CRT path computes `m_2 = 0^0 = 1`, then signs `m - m_2 = m - 1`, which when squared in the gcd step yields `p = gcd(sig, N)`.

```python
io.send(b'\x00' * 40)                 # overflow d_p buffer
sig = io.recvline()
p = gcd(sig - 1, N)                   # factored!
```

**Key insight:** Any CRT-based RSA implementation that parses private key fields from network input is vulnerable to "make `d_p` equal to 0" tricks. The gcd of a forged signature and the modulus reveals a prime.

**References:** P.W.N. CTF 2018 — City RSA, writeup 12052

---

## Textbook RSA Signature Blinding via Message Factoring (P.W.N. CTF 2018)

**Pattern:** Unpadded RSA refuses to sign certain messages (blacklist), but multiplicative homomorphism lets you decompose the forbidden message `m` into coprime factors `m = x * y mod N`, request signatures for `x` and `y` separately, and combine: `sig(m) = sig(x) * sig(y) mod N`.

```python
for x in range(2, 10000):
    if (m * pow(x, -1, N)) % N == allowed_y:
        y = allowed_y
        sig = (sign(x) * sign(y)) % N
        break
```

**Key insight:** Any unpadded RSA signing oracle is vulnerable to factoring-based blinding. The attacker only needs two valid-looking messages whose product equals the target.

**References:** P.W.N. CTF 2018 — City RSA, writeup 12052

---

## Last-Byte Modulus Overwrite via strlen-1 Null Truncation (OTW Advent 2018)

**Pattern:** Server truncates `username[strlen(username)-1] = 0` to strip the trailing newline. Passing an empty username writes the null byte to the byte *before* the buffer — which turns out to be the last byte of `N`. The modified `N` is composite in a small way (differs from the real `N` by at most 255) and is factorable by Carmichael or `sage.factor`.

```python
io.sendline(b'')                       # empty username overwrites N[-1]
N_corrupt = recv_N()
for delta in range(256):
    if is_factorable(N_corrupt + delta - 255):
        N_real = N_corrupt + delta - 255; break
```

**Key insight:** "Strip the last character" idioms (`buf[strlen-1] = 0`) are classic off-by-one when `strlen == 0`. They write one byte backwards into whatever sits before the buffer — often a crypto constant.

**References:** OverTheWire Advent Bonanza 2018 — Day 14, writeup 12752

---

## CRC32 Collision Oracle + RSA Homomorphic Signature Forgery (BSidesSF 2019)

**Pattern (rsaos):** Shell exposes `RSA(foldhash(cmd))` as a "signature" where `foldhash` is a 10-byte digest built from a CRC-like, factorable fold. Privileged commands are blocked, but signatures for arbitrary strings are handed out. Pick a target privileged command whose fold value factors entirely into primes `< 2^32`. For each small factor `f_i`, use [crchack](https://github.com/resilar/crchack) to craft an innocuous command whose CRC32 equals `f_i` exactly, get its signature, and multiply them modulo `N`. RSA's multiplicativity means the product is the signature of the fold *product*, which is the target fold.

```python
from primefac import primefac
import subprocess, random

def find_cmd_crc(target_crc):
    while True:
        open('/tmp/cmd', 'w').write(f'echo {random.randint(0, 10000)}')
        cmd = subprocess.check_output(['./crchack/crchack', '/tmp/cmd', hex(target_crc)])
        if b'\n' not in cmd:
            return cmd

def find_cmd_fac(priv):
    while True:
        c = f'{priv} {random.randint(0, 10000)}'.encode()
        fs = list(primefac(foldhash(c)))
        if all(f < 2**32 for f in fs):
            return c, fs

priv_cmd, factors = find_cmd_fac('get-flag')
t = 1
for f in factors:                               # each factor fits in CRC32 range
    cmd = find_cmd_crc(f)
    _, sig = get_sig(cmd)                        # oracle signs arbitrary cmd
    t = (t * sig) % N                            # RSA: s1*s2 = sign(h1*h2)
send_priv(priv_cmd, sig=hex(t % N))              # forged signature for get-flag
```

**Key insight:** Textbook RSA signatures are multiplicative: `sign(a) * sign(b) = sign(a*b) mod N`. If the "hash" is actually a linear/factorable function (CRC32, fold-XOR), factor the target digest into pieces small enough to fit in CRC output space, then use a CRC collision finder (`crchack`) to realise each factor as an innocuous message the oracle will sign. Multiply the signatures mod `N` to forge the privileged signature. Works for any signature scheme over a hash that is both homomorphic-friendly *and* collidable to specific targets.
