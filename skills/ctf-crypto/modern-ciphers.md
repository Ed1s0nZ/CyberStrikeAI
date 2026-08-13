# CTF Crypto - Modern Cipher Attacks

Block cipher attacks, MAC forgery, padding oracles, and authenticated encryption. For hash/signature attacks (hash extension, PBKDF2, MD5 collision, Rabin, ECB oracles), see [modern-ciphers-2.md](modern-ciphers-2.md). For stream cipher attacks (LFSR, RC4, XOR), see [stream-ciphers.md](stream-ciphers.md).

## Table of Contents
- [AES-CFB-8 Static IV State Forging](#aes-cfb-8-static-iv-state-forging)
- [ECB Pattern Leakage on Images](#ecb-pattern-leakage-on-images)
- [Padding Oracle Attack](#padding-oracle-attack)
- [CBC-MAC vs OFB-MAC Vulnerability](#cbc-mac-vs-ofb-mac-vulnerability)
- [Non-Permutation S-box Collision Attack](#non-permutation-s-box-collision-attack)
- [LCG Partial Output Recovery (0xFun 2026)](#lcg-partial-output-recovery-0xfun-2026)
- [Weak Hash Functions / GF(2) Gaussian Elimination](#weak-hash-functions--gf2-gaussian-elimination)
- [Affine Cipher over Composite Modulus (Nullcon 2026)](#affine-cipher-over-composite-modulus-nullcon-2026)
- [AES-GCM with Derived Keys (EHAX 2026)](#aes-gcm-with-derived-keys-ehax-2026)
- [AES-GCM Nonce Reuse / Forbidden Attack](#aes-gcm-nonce-reuse--forbidden-attack)
- [Ascon-like Reduced-Round Differential Cryptanalysis (srdnlenCTF 2026)](#ascon-like-reduced-round-differential-cryptanalysis-srdnlenctf-2026)
- [Custom Linear MAC Forgery (Nullcon 2026)](#custom-linear-mac-forgery-nullcon-2026)
- [CBC Padding Oracle Attack](#cbc-padding-oracle-attack)
- [Bleichenbacher / PKCS#1 v1.5 RSA Padding Oracle](#bleichenbacher--pkcs1-v15-rsa-padding-oracle)
- [Birthday Attack / Meet-in-the-Middle](#birthday-attack--meet-in-the-middle)
- [CRC32 Collision-Based Signature Forgery (iCTF 2013)](#crc32-collision-based-signature-forgery-ictf-2013)
- [AES Key Recovery via Byte-by-Byte Zeroing Oracle (CONFidence CTF 2017)](#aes-key-recovery-via-byte-by-byte-zeroing-oracle-confidence-ctf-2017)
- [AES-CTR Constant Counter / Repeating Keystream (SHA2017)](#aes-ctr-constant-counter--repeating-keystream-sha2017)
- [Custom SPN Column-Wise XOR Brute-Force (Hack Dat Kiwi 2017)](#custom-spn-column-wise-xor-brute-force-hack-dat-kiwi-2017)
- [AES-CTR Bitflip + CRC Linearity Signature Forgery (hxp CTF 2017)](#aes-ctr-bitflip--crc-linearity-signature-forgery-hxp-ctf-2017)
- [AES-CBC Ciphertext Forging via Error-Message Decryption Oracle (Nuit du Hack CTF 2018)](#aes-cbc-ciphertext-forging-via-error-message-decryption-oracle-nuit-du-hack-ctf-2018)
- [SHA-1 Chosen-Prefix Collision for PDF Signature Forgery (DEF CON Quals 2018)](#sha-1-chosen-prefix-collision-for-pdf-signature-forgery-def-con-quals-2018)
- [Hash Chain Preimage Authentication Bypass (picoCTF 2017)](#hash-chain-preimage-authentication-bypass-picoctf-2017)
- [AES-CBC Nonce Strip via Block Boundary Alignment (Trend Micro 2018)](#aes-cbc-nonce-strip-via-block-boundary-alignment-trend-micro-2018)

See also [modern-ciphers-2.md](modern-ciphers-2.md) for CRC32 forgery, Blum-Goldwasser, hash length extension, compression oracle, hash time reversal, OFB invertible RNG, weak key derivation, HMAC-CRC, DES weak keys, SRP bypass, modified AES S-Box, square attack, AES-ECB byte-at-a-time, AES-ECB cut-and-paste, AES-CBC IV bit-flip, Rabin LSB parity oracle, PBKDF2 pre-hash bypass, MD5 multi-collision, custom hash state reversal, and CRC32 brute-force.

---

## AES-CFB-8 Static IV State Forging

**Pattern (Cleverly Forging Breaks):** AES-CFB with 8-bit feedback and reused IV allows state reconstruction.

**Key insight:** After encrypting 16 known bytes, the AES internal shift register state is fully determined by those ciphertext bytes. Forge new ciphertexts by continuing encryption from known state.

---

## ECB Pattern Leakage on Images

**Pattern (Electronic Christmas Book):** AES-ECB on BMP/image data preserves visual patterns.

**Exploitation:** Identical plaintext blocks produce identical ciphertext blocks, revealing image structure even when encrypted. Rearrange or identify patterns visually.

---

## Padding Oracle Attack

**Pattern (The Seer):** Server reveals whether decrypted padding is valid.

**Byte-by-byte decryption:**
```python
def decrypt_byte(block, prev_block, position, oracle, known):
    """known = bytearray(16) tracking recovered intermediate bytes for this block."""
    for guess in range(256):
        modified = bytearray(prev_block)
        # Set known bytes to produce valid padding
        pad_value = 16 - position
        for j in range(position + 1, 16):
            modified[j] = known[j] ^ pad_value
        modified[position] = guess
        if oracle(bytes(modified) + block):
            return guess ^ pad_value
```

---

## CBC-MAC vs OFB-MAC Vulnerability

OFB mode creates a keystream that can be XORed for signature forgery.

**Attack:** If you have signature for known plaintext P1, forge for P2:
```text
new_sig = known_sig XOR block2_of_P1 XOR block2_of_P2
```

**Important:** Don't forget PKCS#7 padding in calculations! Small bruteforce space? Just try all combinations (e.g., 100 for 2 unknown digits).

**Key insight:** OFB-MAC generates a keystream independent of the plaintext, so knowing one (message, MAC) pair lets you forge MACs for arbitrary messages by XORing the known plaintext blocks out and XORing the new ones in. CBC-MAC does not have this weakness because each block's encryption depends on the previous ciphertext block.

---

## Non-Permutation S-box Collision Attack

**Pattern (Tetraes, Nullcon 2026):** Custom AES-like cipher with S-box collisions.

**Detection:** `len(set(sbox)) < 256` means collisions exist. Find collision pairs and their XOR delta.

**Attack:** For each key byte, try 256 plaintexts differing by delta. When `ct1 == ct2`, S-box input was in collision set. 2-way ambiguity per byte, 2^16 brute-force. Total: 4,097 oracle queries.

See [advanced-math.md](advanced-math.md) for full S-box collision analysis code.

---

## LCG Partial Output Recovery (0xFun 2026)

**Known parameters:** If LCG (Linear Congruential Generator) constants (M, A, C) are known and output is `state mod N`, iterate by N through modulus to find state:
```python
# output = state % N, state = (A * prev + C) % M
for candidate in range(output, M, N):
    # Check if candidate is consistent with next output
    next_state = (A * candidate + C) % M
    if next_state % N == next_output:
        print(f"State: {candidate}")
```

**Upper bits only (e.g., upper 32 of 64):** Brute-force lower 32 bits:
```python
for low in range(2**32):
    state = (observed_upper << 32) | low
    next_state = (A * state + C) % M
    if (next_state >> 32) == next_observed_upper:
        print(f"Full state: {state}")
```

**Key insight:** LCG output truncation (modulo or upper bits only) hides part of the state, but consecutive outputs constrain it. When output is `state mod N`, iterate candidates by N through the modulus. When only upper bits are visible, brute-force the hidden lower bits and validate against the next output.

---

## Weak Hash Functions / GF(2) Gaussian Elimination

Linear permutations (only XOR, rotations) are algebraically attackable. Build transformation matrix and solve over GF(2).

```python
import numpy as np

def solve_gf2(A, b):
    """Solve Ax = b over GF(2)."""
    m, n = A.shape
    Aug = np.hstack([A, b.reshape(-1, 1)]) % 2
    pivot_cols, row = [], 0
    for col in range(n):
        pivot = next((r for r in range(row, m) if Aug[r, col]), None)
        if pivot is None: continue
        Aug[[row, pivot]] = Aug[[pivot, row]]
        for r in range(m):
            if r != row and Aug[r, col]: Aug[r] = (Aug[r] + Aug[row]) % 2
        pivot_cols.append((row, col)); row += 1
    if any(Aug[r, -1] for r in range(row, m)): return None
    x = np.zeros(n, dtype=np.uint8)
    for r, c in reversed(pivot_cols):
        x[c] = Aug[r, -1] ^ sum(Aug[r, c2] * x[c2] for c2 in range(c+1, n)) % 2
    return x
```

**Key insight:** Hash functions built from only XOR and rotations (no S-boxes or modular addition) are linear over GF(2). Build the transformation as a binary matrix, then invert it with Gaussian elimination to recover the preimage directly. This breaks any "custom hash" that avoids non-linear operations.

---

## Affine Cipher over Composite Modulus (Nullcon 2026)

Affine encryption `c = A*x + b (mod M)` with composite M: split into prime factor fields, invert independently, CRT recombine. See [advanced-math.md](advanced-math.md#affine-cipher-over-non-prime-modulus-nullcon-2026) for full chosen-plaintext key recovery and implementation.

---

## AES-GCM with Derived Keys (EHAX 2026)

**Pattern:** Final decryption step after recovering a secret (e.g., from LWE, key exchange). Session nonce and AES key derived via SHA-256 hashing of the recovered secret.

```python
import hashlib
from Cryptodome.Cipher import AES

# Common key derivation chain:
# 1. Recover secret bytes (s_bytes) from crypto challenge
# 2. Unwrap session nonce: nonce = wrapped_nonce XOR SHA256(s_bytes)[:nonce_len]
# 3. Derive AES key: key = SHA256(s_bytes + session_nonce)
# 4. Decrypt AES-GCM

def decrypt_with_derived_key(s_bytes, wrapped_nonce, ciphertext, aes_nonce, tag, nonce_len=16):
    secret_hash = hashlib.sha256(s_bytes).digest()
    session_nonce = bytes(a ^ b for a, b in zip(wrapped_nonce, secret_hash[:nonce_len]))
    aes_key = hashlib.sha256(s_bytes + session_nonce).digest()
    cipher = AES.new(aes_key, AES.MODE_GCM, nonce=aes_nonce)
    return cipher.decrypt_and_verify(ciphertext, tag)
```

**Key insight:** When AES-GCM authentication fails (`ValueError: MAC check failed`), the derived key is wrong — usually means the upstream secret recovery was incorrect or endianness is swapped.

---

## AES-GCM Nonce Reuse / Forbidden Attack

AES-GCM (Galois/Counter Mode) combines AES-CTR encryption with a GHASH polynomial authentication tag. Reusing a nonce with the same key is catastrophic -- it enables both plaintext recovery AND authentication key recovery.

**CTR keystream reuse:** Same nonce = same keystream. XOR two ciphertexts to cancel the keystream: `C1 XOR C2 = P1 XOR P2`. With known plaintext in one message, recover the other.

**GHASH authentication key recovery:** The authentication tag is a polynomial evaluation over GF(2^128). Two messages with the same nonce produce two equations in the same authentication key H. XOR the tag polynomials and factor over GF(2^128) to recover H. With H, forge valid tags for arbitrary messages.

```python
from Crypto.Cipher import AES
from sage.all import GF, PolynomialRing

# Given: two (ciphertext, tag, nonce) pairs with same nonce
# Step 1: Recover plaintext via CTR keystream reuse
keystream = xor(known_plaintext, ciphertext1)
plaintext2 = xor(keystream, ciphertext2)

# Step 2: Recover GHASH auth key H
# Construct tag difference polynomial in GF(2^128)
F = GF(2**128, 'x', modulus=...)  # GCM polynomial
# T1 XOR T2 = P(H) where P is polynomial from ciphertext difference
# Factor P(H) = 0 to find H candidates
# Verify H against known tags

# Step 3: Forge tags for arbitrary messages
# GHASH(H, aad, ciphertext) computed with recovered H
```

**Tool:** [nonce-disrespect](https://github.com/nonce-disrespect/nonce-disrespect) automates GHASH key recovery and tag forgery from nonce-reused GCM ciphertexts.

**Short nonce brute-force:** When GCM uses a short nonce (1-4 bytes), brute-force all nonce values if the key is known. AES-GCM with 1-byte nonce = only 256 candidates.

**Key insight:** AES-GCM is a "one-time nonce" scheme -- a single nonce reuse breaks both confidentiality (CTR keystream reuse) AND authenticity (GHASH key recovery). Always check for repeated nonces in GCM challenge traffic.

---

## Ascon-like Reduced-Round Differential Cryptanalysis (srdnlenCTF 2026)

**Pattern (Lightweight):** 4-round Ascon-like permutation with reduced diffusion. Key-dependent biases in output-bit differentials allow key recovery via chosen input differences.

**Attack:**
1. Reproduce the permutation exactly (critical: post-S-box x4 assignment order matters)
2. Invert the linear layer of x0 using a precomputed 64×64 GF(2) inverse matrix
3. For each bit position i, query with `diff = (1<<i, 1<<i)` across multiple samples
4. Measure empirical biases at output bits `j1 = (i+1) mod 64` and `j2 = (i+14) mod 64`
5. Classify key bits `(k0[i], k1[i])` via centroid-based clustering with sign-pattern mask
6. Verify candidate key in-session; refine low-margin bits with additional samples

**GF(2) linear layer inversion:**
```python
def build_inverse(shifts=(19, 28)):
    """Construct GF(2) inverse matrix for Ascon-like linear layer: x ^= rot(x,19) ^ rot(x,28)."""
    # Build 64x64 matrix over GF(2)
    M = [[0]*64 for _ in range(64)]
    for out_bit in range(64):
        M[out_bit][out_bit] = 1
        for shift in shifts:
            M[out_bit][(out_bit + shift) % 64] ^= 1
    # Gaussian elimination to find inverse
    aug = [row + [1 if i == j else 0 for j in range(64)] for i, row in enumerate(M)]
    for col in range(64):
        pivot = next(r for r in range(col, 64) if aug[r][col])
        aug[col], aug[pivot] = aug[pivot], aug[col]
        for r in range(64):
            if r != col and aug[r][col]:
                aug[r] = [a ^ b for a, b in zip(aug[r], aug[col])]
    return [row[64:] for row in aug]
```

**Centroid clustering for key classification:**
```python
# For each bit position, measure bias at two output positions
# 4 possible (k0[i], k1[i]) pairs → 4 centroid patterns
# Uses sign-pattern mask CMASK=0x73 to account for bit-position-dependent behavior
# Classify by minimum Euclidean distance in 2D bias space
CMASK = 0x73
for i in range(64):
    bias_j1, bias_j2 = measure_biases(i, samples)
    mask_bit = (CMASK >> (i % 8)) & 1
    centroids = centroid_table[mask_bit]  # Precomputed per-position centroids
    k0_bit, k1_bit = min(range(4), key=lambda c: euclidean_dist(
        (bias_j1, bias_j2), centroids[c]))
```

**Key insight:** Reduced-round lightweight ciphers (Ascon, GIFT, etc.) have exploitable biases when the number of rounds is insufficient for full diffusion. The linear layer's inverse can be computed algebraically, and differential biases measured across chosen-plaintext queries reveal individual key bits. This is practical even with noisy measurements if you collect enough samples.

---

## Custom Linear MAC Forgery (Nullcon 2026)

**Pattern (Pasty):** Server signs paste IDs with a custom SHA-256-based construction. The signature is linear in three 8-byte secret blocks derived from the key.

**Structure:** For each 8-byte output block `i`:
- `selector = SHA256(id)[i*8] % 3` → chooses which secret block to use
- `out[i] = hash_block[i] XOR secret[selector] XOR chain[i-1]`

**Recovery:** Create ~10 pastes to collect `(id, sig)` pairs. Each pair reveals `secret[selector]` for 4 selectors. With ~4-5 pairs, all 3 secret blocks are recovered. Then forge for target ID.

**Key insight:** Linearity in custom crypto constructions (XOR-based signing) makes them trivially forgeable. Always check if the MAC has the property: knowing the secret components lets you compute valid signatures for arbitrary inputs.

---

## CBC Padding Oracle Attack

**Pattern:** Server reveals whether CBC-mode ciphertext has valid PKCS#7 padding (via error messages, timing, or status codes). Decrypt any ciphertext block-by-block without the key.

```python
from pwn import *

def padding_oracle(iv, ct):
    """Returns True if server accepts padding."""
    resp = requests.post(URL, data={'iv': iv.hex(), 'ct': ct.hex()})
    return 'padding' not in resp.text.lower()  # or check status code

def decrypt_block(prev_block, target_block):
    """Decrypt one 16-byte block using padding oracle."""
    intermediate = bytearray(16)
    plaintext = bytearray(16)

    for byte_pos in range(15, -1, -1):
        pad_val = 16 - byte_pos
        # Set already-known bytes to produce correct padding
        crafted = bytearray(16)
        for k in range(byte_pos + 1, 16):
            crafted[k] = intermediate[k] ^ pad_val

        for guess in range(256):
            crafted[byte_pos] = guess
            if padding_oracle(bytes(crafted), target_block):
                intermediate[byte_pos] = guess ^ pad_val
                plaintext[byte_pos] = intermediate[byte_pos] ^ prev_block[byte_pos]
                break

    return bytes(plaintext)
```

**Tools:**
```bash
# PadBuster — automated padding oracle exploitation
padbuster http://target/decrypt.php ENCRYPTED_B64 16 \
  -encoding 0 -error "Invalid padding"

# Python: pip install padding-oracle
from padding_oracle import PaddingOracle
oracle = PaddingOracle(block_size=16, oracle_fn=check_padding)
plaintext = oracle.decrypt(ciphertext, iv=iv)
```

**Key insight:** The oracle only needs to distinguish "valid padding" from "invalid padding." This can be a different HTTP status code, error message, response time, or even whether the application processes the request further. A single bit of information per query is sufficient. Decryption requires at most 256 x 16 = 4096 queries per block.

**Detection:** CBC mode encryption + any distinguishable behavior difference on padding errors. Common in cookie encryption, token systems, and encrypted API parameters.

---

## Bleichenbacher / PKCS#1 v1.5 RSA Padding Oracle

**Pattern:** RSA encryption with PKCS#1 v1.5 padding where the server reveals whether decrypted plaintext has valid `0x00 0x02` prefix. Adaptive chosen-ciphertext attack recovers the plaintext.

```python
import gmpy2

def bleichenbacher_oracle(c, n, e):
    """Returns True if RSA decryption has valid PKCS#1 v1.5 padding (0x00 0x02 prefix)."""
    resp = send_to_server(c)
    return resp.status_code != 400  # Server returns 400 on bad padding

def bleichenbacher_attack(c0, n, e, oracle, k):
    """
    c0: target ciphertext (integer)
    k: byte length of modulus (e.g., 256 for RSA-2048)
    """
    B = pow(2, 8 * (k - 2))

    # Step 1: Start with s1 = ceil(n / 3B)
    s = (n + 3 * B - 1) // (3 * B)

    # Step 2: Search for s where oracle(c0 * s^e mod n) is True
    while True:
        c_prime = (c0 * pow(s, e, n)) % n
        if oracle(c_prime, n, e):
            break
        s += 1

    # Step 3: Narrow interval [a, b] using s values
    # Repeat: find new s, narrow interval, until a == b
    # When interval collapses, plaintext = a * modinv(s, n) % n
    # (Full implementation requires interval tracking — use existing tools)
```

**Tools:**
```bash
# ROBOT attack scanner (modern Bleichenbacher variant)
python3 robot-detect.py -H target.com

# TLS-Attacker framework
java -jar TLS-Attacker.jar -connect target:443 -workflow_type BLEICHENBACHER
```

**Key insight:** The attack is adaptive — each oracle response narrows the range of possible plaintexts. Typically requires ~10,000 oracle queries for RSA-2048. The ROBOT attack (Return Of Bleichenbacher's Oracle Threat) showed this affects modern TLS implementations through subtle timing differences. Any server that distinguishes "bad padding" from "bad content" is vulnerable.

---

## Birthday Attack / Meet-in-the-Middle

**Pattern:** Find collisions in hash functions or MACs using the birthday paradox. With an n-bit hash, expect a collision after ~2^(n/2) random inputs.

```python
import hashlib, os

def birthday_collision(hash_fn, output_bits, prefix=b''):
    """Find two inputs with the same truncated hash."""
    target_bytes = output_bits // 8
    seen = {}

    while True:
        msg = prefix + os.urandom(16)
        h = hash_fn(msg).digest()[:target_bytes]
        if h in seen:
            return seen[h], msg  # Collision found!
        seen[h] = msg

# Example: find collision on first 4 bytes of SHA-256 (~65536 attempts)
msg1, msg2 = birthday_collision(hashlib.sha256, 32)
```

**Meet-in-the-Middle (2DES, double encryption):**
```python
def meet_in_the_middle(encrypt_fn, decrypt_fn, plaintext, ciphertext, keyspace):
    """Break double encryption E(k2, E(k1, pt)) = ct."""
    # Forward: encrypt plaintext with all possible k1
    forward = {}
    for k1 in keyspace:
        intermediate = encrypt_fn(k1, plaintext)
        forward[intermediate] = k1

    # Backward: decrypt ciphertext with all possible k2
    for k2 in keyspace:
        intermediate = decrypt_fn(k2, ciphertext)
        if intermediate in forward:
            return forward[intermediate], k2  # Found k1, k2!
```

**Key insight:** Birthday attack: n-bit hash needs ~2^(n/2) queries for 50% collision probability. 32-bit hash -> ~65K, 64-bit -> ~4 billion. Meet-in-the-middle reduces double encryption from O(2^(2k)) to O(2^k) time + O(2^k) space — this is why 2DES provides only 1 extra bit of security over DES.

---

## CRC32 Collision-Based Signature Forgery (iCTF 2013)

**Pattern:** CRC32 is linear — appending 4 carefully chosen bytes to any message produces a target CRC32 value, enabling signature forgery without knowing the secret key.

**Key insight:** `CRC32(msg || secret)` is not a secure MAC. Given any signed response `(msg, sig)`, compute 4 suffix bytes that force `CRC32(forged_msg || suffix || secret) == target_sig`. The linearity of CRC32 means the suffix computation is deterministic and instant.

```python
import struct, binascii

def crc32_forge(data, target_crc):
    """Append 4 bytes to data so CRC32(data + suffix) == target_crc"""
    current = binascii.crc32(data) & 0xFFFFFFFF
    # CRC32 polynomial table lookup to find suffix bytes
    # that transform current CRC into target_crc
    suffix = b''
    crc = target_crc ^ 0xFFFFFFFF
    for _ in range(4):
        byte = (crc & 0xFF)
        crc = (crc >> 8)
        suffix = bytes([byte]) + suffix
    return data + suffix  # Simplified — full implementation requires polynomial division
```

**When to use:** Any protocol using CRC32 as a message authentication code (MAC). CRC32 is a checksum, not a cryptographic hash — it provides no integrity guarantees against adversarial modification.

---

## AES Key Recovery via Byte-by-Byte Zeroing Oracle (CONFidence CTF 2017)

**Pattern:** When a service allows selective zeroing of key bytes (e.g., via integer overflow in key slot indexing), recover the full AES key by testing one byte at a time.

```python
# Service has key slots and a "regenerate" function with integer overflow
# offset = index * ENTRY_SIZE wraps around, allowing arbitrary byte zeroing

# Strategy: zero bytes progressively, brute-force each unknown byte
for byte_pos in range(16):
    # Zero all bytes EXCEPT byte_pos (by overflowing index calculation)
    zero_index = (target_offset * modinv(ENTRY_SIZE, 2**32)) % 2**32
    regenerate(zero_index)

    # Key is now: [0,0,...,key[byte_pos],...,0,0]
    # Brute-force the single non-zero byte (256 possibilities)
    known_ct = encrypt(known_pt)
    for guess in range(256):
        test_key = bytes([0]*byte_pos + [guess] + [0]*(15-byte_pos))
        if AES.new(test_key, AES.MODE_ECB).encrypt(known_pt) == known_ct:
            recovered_key[byte_pos] = guess
            break
```

**Key insight:** Integer overflow in `index * ENTRY_SIZE` calculations can target arbitrary memory offsets. By selectively zeroing all-but-one key bytes, the key becomes trivially brute-forceable one byte at a time (256 attempts per byte, 4096 total vs 2^128 for the full key).

**References:** CONFidence CTF 2017

---

## AES-CTR Constant Counter / Repeating Keystream (SHA2017)

**Pattern:** When an AES-CTR implementation uses `counter=lambda: secret` (a constant function), the counter never increments. AES-CTR with a fixed counter produces the same 16-byte block on every call — equivalent to Vigenère cipher at the byte level with a 16-byte repeating key.

```python
# Constant counter makes CTR equivalent to repeating-key XOR
key_byte = ciphertext_byte ^ known_plaintext_byte
# Apply recovered key bytes across all 16-byte-aligned blocks
for i, ct_byte in enumerate(ciphertext):
    plaintext_byte = ct_byte ^ keystream[i % 16]
```

**Exploit using file format headers:**
1. Identify the file format from context (e.g., `%PDF-1.` for PDF files)
2. XOR the known header bytes against the ciphertext to recover `keystream[0:len(header)]`
3. Iteratively extend: use recovered plaintext to guess the next structural keyword (`endobj`, `/Page`, `stream`, etc.), verify XOR produces consistent ASCII, and extend the keystream further
4. Tool: `otp_pwn` supports interactive block-aligned crib-dragging for this workflow

**Key insight:** Constant AES-CTR counter = repeating 16-byte Vigenère key. Known file format magic bytes bootstrap iterative key recovery via crib-dragging. Any known-plaintext at block-aligned positions reveals the full keystream byte at that position.

**References:** SHA2017

---

## Custom SPN Column-Wise XOR Brute-Force (Hack Dat Kiwi 2017)

**Pattern:** SPN (Substitution-Permutation Network) cipher with a seed-based sbox/pbox and a final XOR key layer. If the XOR key is applied column-wise (each key byte affects one column position independently), each key byte can be brute-forced separately using printable-text consistency as an oracle.

**Attack:**
1. Collect multiple ciphertext blocks (same key, different plaintexts)
2. For each column position `c` (0-15), try all 256 candidate key bytes `k`
3. Apply the inverse pbox and sbox to undo the SPN rounds, then XOR with candidate `k`
4. Keep only candidates where ALL blocks produce printable ASCII at position `c`
5. The intersection of valid candidates across blocks recovers each key byte

**Multi-round variant:** Peel one round at a time. After recovering the outermost XOR key, apply the inverse pbox/sbox for that round using the recovered bytes, then repeat for the next inner round.

**Seed-based permutation dependency:** When sbox and pbox are generated from a shared seed, recovering partial key bytes constrains the seed (and thus the remaining permutation entries). Use this to propagate partial solutions across columns with cross-column dependencies.

**Key insight:** Column-aligned XOR layers in SPN ciphers allow independent per-byte brute-force using printable-text consistency as an oracle. Cross-column key reuse from seed-based permutations propagates partial solutions.

**References:** Hack Dat Kiwi 2017

---

## AES-CTR Bitflip + CRC Linearity Signature Forgery (hxp CTF 2017)

**Pattern:** AES-CTR allows targeted plaintext modification via XOR. CRC is linear w.r.t. XOR: `CRC(A ^ B) = CRC(A) ^ CRC(B) ^ CRC(zeros)`. Flip `{admin: 0}` to `{admin: 1}` in ciphertext and fix the encrypted CRC:

```python
import binascii
# X = desired_plaintext XOR original_plaintext (flip bit)
X = b'\x00' * offset + b'\x01' + b'\x00' * remaining
crc_diff = binascii.crc32(X) ^ binascii.crc32(b'\x00' * len(X))
# New ciphertext = old_ciphertext XOR X (for data portion)
# New CRC ciphertext = old_CRC_ciphertext XOR pack(crc_diff)
```

**Key insight:** CRC is GF(2)-linear -- XOR-based modifications to plaintext produce predictable CRC changes without knowing the key. When a system uses AES-CTR for confidentiality + CRC for integrity (instead of a proper MAC like HMAC or GCM), you can flip arbitrary plaintext bits and fix the CRC simultaneously. This is a fundamental failure of using CRC as a MAC: CRC detects random errors but provides zero protection against adversarial modification under stream ciphers.

**References:** hxp CTF 2017

---

### AES-CBC Ciphertext Forging via Error-Message Decryption Oracle (Nuit du Hack CTF 2018)

**Pattern:** Server decrypts AES-CBC cookie and displays decrypted value in error messages. Send zero blocks, read decrypted intermediates from error, XOR with desired plaintext to forge ciphertext block-by-block. Use forged ciphertext to deliver blind SQLi payloads through encrypted cookies. (Nuit du Hack CTF 2018)

```python
# Forge ciphertext for arbitrary plaintext
for i in range(blocks):
    payload = b'\x00' * 16 * (blocks - 1) + last_forged_block
    response = send_payload(payload)
    decrypted = parse_error_message(response)  # server leaks decrypted bytes
    intermediate = decrypted[-16:]
    new_block = xor(target_plaintext_block, intermediate)
    forged_blocks.append(new_block)
```

**Key insight:** When the server reveals decrypted ciphertext in error messages, you can forge arbitrary plaintext without knowing the key. Send zero IV blocks to learn the intermediate state, then XOR with desired plaintext to produce the correct ciphertext. Build block-by-block from last to first.

---

## SHA-1 Chosen-Prefix Collision for PDF Signature Forgery (DEF CON Quals 2018)

**Pattern (EmojiVote):** Server extracts commands from an uploaded PDF via OCR, then signs the OCR'd byte-string as `sha1(data)` and attaches the signature. Use a shattered-style SHA-1 chosen-prefix collision to produce two PDFs that OCR to different commands but share the same SHA-1 digest.

**Exploit workflow:**
1. Build PDF A that OCR's to a benign command (no `EXECUTE`) and PDF B that OCR's to `EXECUTE <attacker command>`.
2. Pad both with shattered-style suffix data so `sha1(A) == sha1(B)`.
3. Submit A to obtain a valid signature for the shared digest.
4. Replay that signature on B — the server verifies the SHA-1 matches and executes the attacker command.

```bash
# Build the two colliding PDFs (cpc = chosen-prefix collision tool)
./cpc prefix_A prefix_B collision_A.pdf collision_B.pdf
sha1sum collision_A.pdf collision_B.pdf  # identical
# Upload A, capture signature, replay on B
```

**Key insight:** When a protocol signs a message as `sign(sha1(M))` instead of `sign(M)` directly, any SHA-1 collision becomes a signature forgery. Chosen-prefix collisions are practical (cpc/shattered toolkit) — the signer only inspects the digest, never the second preimage.

**References:** DEF CON CTF Qualifier 2018 — writeup 10075

---

## Hash Chain Preimage Authentication Bypass (picoCTF 2017)

**Pattern (hash_chain):** Server authenticates the Nth challenge by asking for `hash^(N-1)(seed)` given `hash^N(seed)`. The seed is derivable from public user data (e.g., `md5(username)`), so any attacker can precompute the whole chain from the start and answer any step.

**Exploit:**
```python
import hashlib

def H(x): return hashlib.md5(x).digest()

seed = H(username.encode())        # public-derived seed
chain = [seed]
for _ in range(TARGET_N + 1):
    chain.append(H(chain[-1]))

# Server sends chain[N]; answer with chain[N-1]
```

**Key insight:** Hash chains are only one-way if the seed is secret. If the seed can be reconstructed from public inputs (username, challenge ID, timestamp), the entire chain is computable forward, and answering "give me the previous hash" is trivial. Treat the seed like a key.

**References:** picoCTF 2017 — writeup 10031

---

## AES-CBC Nonce Strip via Block Boundary Alignment (Trend Micro 2018)

**Pattern:** A server encrypts `nonce | padding | identity | timestamp` with AES-CBC and returns `(iv, ciphertext)`. If the attacker can choose padding such that the first *exactly one* AES block (16 bytes) holds the nonce, then shifting the IV forward by one block — reusing `ciphertext[:16]` as the new IV and `ciphertext[16:]` as the new ciphertext — yields a valid encryption of just `identity | timestamp`. No key is needed because CBC-mode decryption of block 2 is `AES⁻¹(c[16:32]) XOR c[0:16]`, which is exactly the identity-and-timestamp plaintext once the nonce block is promoted to IV.

```python
from Crypto.Cipher import AES
import os

key = os.urandom(16)

# Server builds plaintext and encrypts
def encrypt_with_nonce(identity, timestamp):
    nonce = os.urandom(8)
    padding = b"\x00" * 8          # brings nonce + padding to 16 bytes
    plaintext = nonce + padding + identity + timestamp
    iv = os.urandom(16)
    ct = AES.new(key, AES.MODE_CBC, iv).encrypt(plaintext)
    return iv, ct

iv, ct = encrypt_with_nonce(b"admin___________", b"2018-11-01T00:00")

# Attacker rewrites (iv', ct') to drop the nonce block
new_iv = ct[:16]
new_ct = ct[16:]
recovered = AES.new(key, AES.MODE_CBC, new_iv).decrypt(new_ct)
assert recovered.startswith(b"admin")
```

**Key insight:** CBC's IV is only consulted for the first block — every subsequent block uses the previous ciphertext as its "IV". That means any contiguous slice of a CBC ciphertext is itself a valid CBC ciphertext if you promote the preceding block (or a supplied IV) to the new IV. Whenever a fixed-size header (nonce, magic bytes, counter) occupies exactly one block, the attacker can strip it by reusing that block as an IV. Defend by binding the header into the authentication tag (AEAD) or including its offset in an HMAC.

**References:** Trend Micro CTF 2018 — Offensive-Analysis 400, writeup 11130
