# CTF Crypto - Modern Cipher Attacks (Continued)

Hash-based attacks, protocol-level exploits, ECB oracles, Rabin/RSA parity attacks, and specialized cipher weaknesses. For core AES/CBC/padding oracle techniques, see [modern-ciphers.md](modern-ciphers.md). For stream cipher attacks (LFSR, RC4, XOR), see [stream-ciphers.md](stream-ciphers.md).

## Table of Contents
- [Blum-Goldwasser Bit-Extension Oracle (PlaidCTF 2013)](#blum-goldwasser-bit-extension-oracle-plaidctf-2013)
- [Hash Length Extension Attack (PlaidCTF 2014)](#hash-length-extension-attack-plaidctf-2014)
- [Compression Oracle / CRIME-Style Attack (BCTF 2015)](#compression-oracle--crime-style-attack-bctf-2015)
- [Hash Function Time Reversal via Cycle Detection (BSidesSF 2025)](#hash-function-time-reversal-via-cycle-detection-bsidessf-2025)
- [OFB Mode with Invertible RNG Backward Decryption (BSidesSF 2026)](#ofb-mode-with-invertible-rng-backward-decryption-bsidessf-2026)
- [Weak Key Derivation via Public Key Hash XOR (BSidesSF 2026)](#weak-key-derivation-via-public-key-hash-xor-bsidessf-2026)
- [HMAC-CRC Linearity Attack (Boston Key Party 2016)](#hmac-crc-linearity-attack-boston-key-party-2016)
- [DES Weak Keys in OFB Mode (Boston Key Party 2016)](#des-weak-keys-in-ofb-mode-boston-key-party-2016)
- [SRP (Secure Remote Password) Protocol Bypass via Modular Arithmetic (ASIS CTF Finals 2016)](#srp-secure-remote-password-protocol-bypass-via-modular-arithmetic-asis-ctf-finals-2016)
- [Modified AES S-Box Brute-Force Recovery (H4ckIT CTF 2016)](#modified-aes-s-box-brute-force-recovery-h4ckit-ctf-2016)
- [Square Attack on Reduced-Round AES (0CTF 2016)](#square-attack-on-reduced-round-aes-0ctf-2016)
- [AES-ECB Byte-at-a-Time Chosen Plaintext (ABCTF 2016)](#aes-ecb-byte-at-a-time-chosen-plaintext-abctf-2016)
- [AES-ECB Cut-and-Paste Block Manipulation (NDH Quals 2016)](#aes-ecb-cut-and-paste-block-manipulation-ndh-quals-2016)
- [AES-CBC IV Bit-Flip Authentication Bypass (Google CTF 2016)](#aes-cbc-iv-bit-flip-authentication-bypass-google-ctf-2016)
- [Rabin Cryptosystem LSB Parity Oracle (PlaidCTF 2016)](#rabin-cryptosystem-lsb-parity-oracle-plaidctf-2016)
- [PBKDF2 Pre-Hash Bypass for Long Passwords (BackdoorCTF 2016)](#pbkdf2-pre-hash-bypass-for-long-passwords-backdoorctf-2016)
- [MD5 Multi-Collision via Fastcol (BackdoorCTF 2016)](#md5-multi-collision-via-fastcol-backdoorctf-2016)
- [GHASH Key Recovery over Prime Modulus (nullcon HackIM 2019)](#ghash-key-recovery-over-prime-modulus-nullcon-hackim-2019)
- [SHA-1 Length Extension Plus AES-CBC Cookie Forgery (BSidesSF 2019)](#sha-1-length-extension-plus-aes-cbc-cookie-forgery-bsidessf-2019)

See [modern-ciphers-3.md](modern-ciphers-3.md) for custom hash reversal, CRC32 brute-force, noisy RSA oracle, sponge collisions, CBC IV forgery, padding oracle bit-flip, SPN S-box intersection, AES-CFB IV recovery, three-round XOR, Unicode side channel, SHA-256 basis attack, and HMAC key recovery.

---

## Blum-Goldwasser Bit-Extension Oracle (PlaidCTF 2013)

**Pattern:** Exploit a decryption oracle for Blum-Goldwasser-style encryption by extending ciphertext length by one bit per query to leak plaintext via parity.

**Key insight:** Extend ciphertext by one bit (L+1), shift ciphertext left (`c << 1`), and submit a modified `y` value. The oracle reveals the LSB (parity) of each decrypted chunk. The squaring sequence `y = pow(y, 2, N)` can be manipulated to produce valid extended ciphertexts the server hasn't seen.

```python
# Iterative plaintext recovery via bit-extension
for i in range(msg_length):
    extended_c = original_c << 1        # Shift ciphertext left by 1
    new_y = pow(original_y, 2, N)       # Advance squaring sequence
    response = oracle(extended_c, new_y, msg_length + 1)
    leaked_bit = response & 1           # LSB reveals one plaintext bit
    plaintext_bits.append(leaked_bit)
    original_y = new_y
```

**When to use:** Blum-Goldwasser or BBS-based (Blum Blum Shub) encryption with a decryption oracle that accepts variable-length ciphertexts. The parity leak accumulates one bit per query.

---

## Hash Length Extension Attack (PlaidCTF 2014)

**Pattern:** Server computes `hash(SECRET || user_data)` using MD5, SHA-1, or SHA-256 (Merkle-Damgard constructions). Given a valid hash and the original data, extend it with arbitrary appended data and compute a valid hash — without knowing the secret.

```bash
# Using HashPump (install: apt install hashpump)
hashpump --keylength 8 \
  --signature 'ef16c2bffbcf0b7567217f292f9c2a9a50885e01e002fa34db34c0bb916ed5c3' \
  --data 'original_data' \
  --additional ';admin=true'
# Outputs: new_signature and new_data (with padding bytes)
```

```python
# Python: hashpumpy
import hashpumpy
new_hash, new_data = hashpumpy.hashpump(
    original_hash, original_data, append_data, secret_length
)
```

**Key insight:** Merkle-Damgard hashes (MD5, SHA-1, SHA-256) process data in blocks, and the hash output IS the internal state. Given `H(secret || msg)`, you can compute `H(secret || msg || padding || extension)` without knowing `secret` — just initialize the hash state from the known output and continue hashing. Only HMAC (`H(K XOR opad || H(K XOR ipad || msg))`) is immune. If the secret length is unknown, try lengths 1-32.

*See also [ctf-web/auth-infra.md — Hash Length Extension Attack (ASIS CTF 2017)](../ctf-web/auth-infra.md#hash-length-extension-attack-asis-ctf-2017) for the same primitive applied to a web auth token bypass.*

---

## Compression Oracle / CRIME-Style Attack (BCTF 2015)

**Pattern:** Server compresses plaintext (LZW, zlib, etc.) before encrypting. By observing ciphertext length changes with chosen plaintexts, leak the unknown plaintext character-by-character.

```python
import base64

def oracle(plaintext):
    """Send chosen plaintext, get ciphertext length."""
    resp = send_to_server(plaintext)
    return len(base64.b64decode(resp))

# Baseline: empty input
base_len = oracle("")

# Recover secret byte-by-byte
known = ""
for pos in range(secret_length):
    for c in string.printable:
        candidate = known + c
        length = oracle(candidate)
        if length <= base_len + len(known):  # Compressed = match
            known += c
            break
```

**Key insight:** Compression algorithms (LZW, DEFLATE, zlib) replace repeated sequences with back-references. If `SALT + user_input` is compressed before encryption, sending input that matches part of the salt produces shorter ciphertext (the match compresses). This is the same class as CRIME (TLS), BREACH (HTTP), and HEIST attacks. The oracle is ciphertext length.

---

## Hash Function Time Reversal via Cycle Detection (BSidesSF 2025)

When a system uses iterated hashing as a "time" function (`state_t = H(state_{t-1})`), reverse time by exploiting the finite cycle structure:

1. **Detect cycle:** Use Floyd's tortoise-and-hare or Brent's algorithm to find cycle length L
2. **Compute backward steps:** To go from time T to earlier time T_goal: iterate forward `(L - (T - T_goal)) % L` steps

```python
import hashlib

def hash_step(state):
    return hashlib.md5(state).digest()[:8]  # Truncated hash

def find_cycle(start):
    """Brent's cycle detection: returns (cycle_length, start_of_cycle)"""
    power = lam = 1
    tortoise = start
    hare = hash_step(start)
    while tortoise != hare:
        if power == lam:
            tortoise = hare
            power *= 2
            lam = 0
        hare = hash_step(hare)
        lam += 1
    # lam = cycle length; find cycle start
    tortoise = hare = start
    for _ in range(lam):
        hare = hash_step(hare)
    mu = 0
    while tortoise != hare:
        tortoise = hash_step(tortoise)
        hare = hash_step(hare)
        mu += 1
    return lam, mu  # cycle_length, cycle_start_offset

# Reverse from T_known to T_goal
cycle_len, _ = find_cycle(known_state)
forward_steps = (cycle_len - (t_known - t_goal)) % cycle_len
state = known_state
for _ in range(forward_steps):
    state = hash_step(state)
# state is now the value at t_goal
```

**Key insight:** For truncated hashes (e.g., MD5 -> 64 bits), the expected cycle length is ~2^32, making cycle detection feasible. Going "backward" N steps is equivalent to going forward (cycle_length - N) steps. Assumes the target state is within the main cycle, not on a tail.

---

## OFB Mode with Invertible RNG Backward Decryption (BSidesSF 2026)

**Pattern (randcrypt):** A custom block cipher uses OFB (Output Feedback) mode with a homemade RNG as the keystream generator. The last plaintext block is known (zero padding), leaking one RNG state. If the RNG's state transition function is invertible (bijective), all previous states can be recovered by running the RNG backwards, decrypting the entire ciphertext from the end to the beginning.

```python
def rng_forward(state):
    """Custom RNG state transition (from challenge)."""
    # Example: linear congruential or reversible mixing
    return (state * A + B) % M

def rng_inverse(state):
    """Inverted RNG — recover previous state."""
    return ((state - B) * pow(A, -1, M)) % M

# Last block is zero-padded → ciphertext XOR 0 = keystream = RNG state
leaked_state = int.from_bytes(ciphertext_blocks[-2], 'big')

# Decrypt backwards
state = leaked_state
plaintext_blocks = []
for i in range(len(ciphertext_blocks) - 3, -1, -1):
    state = rng_inverse(state)
    pt = xor_bytes(ciphertext_blocks[i], state.to_bytes(block_size, 'big'))
    plaintext_blocks.insert(0, pt)
```

**Key insight:** OFB mode decouples encryption from the plaintext — the keystream is deterministic from the initial state. If ANY block's plaintext is known (padding, headers, magic bytes), the corresponding RNG state is leaked. An invertible RNG then reveals ALL states. Always check if the RNG transition function has a mathematical inverse.

**When to recognize:** Custom OFB/CTR mode with a non-standard PRNG. Look for: (1) XOR-based encryption, (2) a state-update function that's bijective (no information loss), (3) predictable plaintext in any block position. Files with known padding (PKCS#7 zero-fill, null-terminated strings) are ideal leak points.

---

## Weak Key Derivation via Public Key Hash XOR (BSidesSF 2026)

**Pattern (ran-somewhere):** Hybrid RSA+AES encryption where the AES key is derived as `SHA256(DER_encoded_public_key) XOR seed`, with the seed hardcoded or predictable. Since the public key is public, the AES key is fully recoverable without the RSA private key.

```python
from Crypto.PublicKey import RSA
from Crypto.Cipher import AES
from hashlib import sha256

# Public key is available
pubkey = RSA.import_key(open("public.pem").read())
der_bytes = pubkey.export_key("DER")

# Seed from challenge (hardcoded/predictable)
seed = b'BSidesSFCTF2026!'

# Derive AES key the same way the encryptor did
key_hash = sha256(der_bytes).digest()
aes_key = bytes(a ^ b for a, b in zip(key_hash, seed.ljust(32, b'\x00')))

# Decrypt
ct = open("flag.enc", "rb").read()
iv, ct_body = ct[:16], ct[16:]
cipher = AES.new(aes_key, AES.MODE_CBC, iv)
plaintext = cipher.decrypt(ct_body)
```

**Key insight:** Key derivation that incorporates only public information (public keys, known constants) provides zero security regardless of the hash function used. The "hybrid" design creates a false sense of security — RSA protects nothing if the AES key doesn't depend on the RSA private key.

**When to recognize:** Challenge provides both a public key AND an encrypted file, but no private key or ciphertext for RSA. Look for key derivation code that hashes the public key, uses the public key's modulus/exponent as seed material, or XORs with a constant.

---

## HMAC-CRC Linearity Attack (Boston Key Party 2016)

**Pattern:** HMAC constructed with CRC as the hash function is completely broken because CRC is linear over GF(2). The key is directly recoverable from a single message-MAC pair via polynomial arithmetic over GF(2^64).

```python
# CRC is linear: CRC(a XOR b) = CRC(a) XOR CRC(b)
# HMAC-CRC(key, msg) = CRC(key_opad || CRC(key_ipad || msg))
# Rewrite as polynomial in GF(2): K = known_terms * inverse(x^(128+M) + x^128) mod CRC_POLY
```

**Key insight:** CRC's linearity over GF(2) means HMAC-CRC provides zero security. Always verify the underlying hash function is non-linear before trusting HMAC.

---

## DES Weak Keys in OFB Mode (Boston Key Party 2016)

**Pattern:** DES has 4 weak keys where `E(E(P,K),K) = P` (encryption is self-inverse). In OFB (Output Feedback) mode this causes the keystream to cycle with period 2: even blocks XOR with IV, odd blocks with E(IV,K). Reduces to a 16-byte repeating XOR key.

```python
# DES weak keys: 0x0000000000000000, 0xFFFFFFFFFFFFFFFF,
#                0xE1E1E1E1F0F0F0F0, 0x1E1E1E1E0F0F0F0F
# OFB with weak key: keystream = [IV, E(IV,K), IV, E(IV,K), ...]
# Recovery: try all 4 weak keys; or treat as 16-byte repeating XOR
```

**Key insight:** DES weak keys cause OFB keystream to cycle with period 2. When you see DES+OFB, always try the 4 weak keys first.

---

## Square Attack on Reduced-Round AES (0CTF 2016)

**Pattern:** 4-round AES is vulnerable to the square (integral) attack. Choose 256 plaintexts differing in one byte (a "lambda set"). After 3 rounds, the XOR sum at any byte position equals 0. Guess one byte of the last round key and partially decrypt -- if XOR sum is 0, the guess is correct.

```python
# For each byte position in the last round key:
for candidate in range(256):
    xor_sum = 0
    for ct in ciphertexts:
        xor_sum ^= inv_sub_bytes(ct[pos] ^ candidate)
    if xor_sum == 0:
        key_byte = candidate  # correct guess
# Reduces 2^128 key recovery to ~16 * 256 = 4096 operations
```

**Key insight:** Integral cryptanalysis exploits the "balanced" property (XOR-sum = 0) that propagates through AES rounds. Effective against 4-round AES; 5+ rounds require more sophisticated variants.

---

## SRP (Secure Remote Password) Protocol Bypass via Modular Arithmetic (ASIS CTF Finals 2016)

SRP implementations that only check `A != 0` and `A != N` can be bypassed by sending `A = 2*N`, causing the server to compute a zero session key.

```python
from hashlib import sha256
import hmac

# SRP protocol: server computes session key from A (client's public value)
# S = (A * v^u) ^ b mod N
# If A = 2*N: S = (2*N * v^u) ^ b mod N = 0 (since 2*N mod N = 0)

N = server_modulus
# Send A = 2*N (bypasses checks for A != 0 and A != N)
A_malicious = 2 * N

# Server computes S = 0, so session key K = SHA256(0)
K = sha256(b'\x00').digest()

# Now compute valid HMAC proof with known K
proof = hmac.new(K, salt, sha256).hexdigest()
```

**Key insight:** SRP implementations must validate `A % N != 0`, not just `A != 0` and `A != N`. Sending `A = k*N` for any integer k forces the shared secret to zero, allowing authentication without knowing the password.

---

## Modified AES S-Box Brute-Force Recovery (H4ckIT CTF 2016)

AES implementation with a custom S-Box created by swapping 3 elements of the standard S-Box. Brute-force all C(256,3) * 2 = 5,527,040 possible permutations.

```cpp
// Three elements swapped from standard AES S-Box
// Total permutations: C(256,3) * 2 = ~5.5 million (feasible to brute-force)
#include <openssl/aes.h>

void bruteforce_sbox(uint8_t ciphertext[], uint8_t key[], int ct_len) {
    uint8_t standard_sbox[256]; // standard AES S-Box
    // Try all 3-element swaps
    for (int i = 0; i < 256; i++)
        for (int j = i+1; j < 256; j++)
            for (int k = j+1; k < 256; k++) {
                // Swap pairs: (i,j), (i,k), (j,k)
                uint8_t sbox[256];
                memcpy(sbox, standard_sbox, 256);
                swap(sbox[i], sbox[j]); // try each 2-element swap from the triple
                // Decrypt and check for valid plaintext
                if (try_decrypt_with_sbox(sbox, ciphertext, key, ct_len))
                    return; // found it
            }
}
```

**Key insight:** When a custom AES S-Box differs from standard by only a few element swaps, the search space is small enough to brute-force. For 3 swapped elements: C(256,3) permutation groups times the swap combinations within each group.

---

## AES-ECB Byte-at-a-Time Chosen Plaintext (ABCTF 2016)

**Pattern (Encryption Service):** Server encrypts `user_input || secret_suffix` under AES-ECB. Recover the secret suffix one byte at a time by controlling the input length.

1. Send inputs of decreasing length to push one unknown byte into a known block position
2. For each position, try all 256 byte values and compare the encrypted block:

```python
from pwn import *
import cryptanalib as ca  # FeatherDuster's cryptanalib

def oracle(pt):
    """Send plaintext, receive ECB-encrypted ciphertext."""
    r = remote('target', 7765)
    r.recvuntil('Send me some hex-encoded data to encrypt:\n')
    r.sendline(pt.hex())
    r.recvuntil('Here you go:')
    ct = bytes.fromhex(r.recvline().strip().decode())
    r.close()
    return ct

# Automated byte-at-a-time recovery
flag = ca.ecb_cpa_decrypt(oracle, block_size=16, verbose=True)
print(flag)
```

**Manual approach without library:**
```python
block_size = 16
known = b''

for i in range(len(secret)):
    # Pad so next unknown byte is at end of a block
    pad_len = block_size - 1 - (len(known) % block_size)
    pad = b'A' * pad_len

    # Get target block
    target_ct = oracle(pad)
    target_block_idx = (pad_len + len(known)) // block_size
    target_block = target_ct[target_block_idx*16:(target_block_idx+1)*16]

    # Try all 256 byte values
    for byte_val in range(256):
        test = pad + known + bytes([byte_val])
        test_ct = oracle(test)
        if test_ct[target_block_idx*16:(target_block_idx+1)*16] == target_block:
            known += bytes([byte_val])
            break
```

**Key insight:** ECB mode encrypts identical plaintext blocks to identical ciphertext blocks. By controlling the prefix length, the attacker shifts one unknown byte at a time to a position where it completes a known block prefix. Comparing the target ciphertext block against all 256 possibilities recovers each byte in at most 256 queries. Total queries: ~256 * secret_length. Tool: FeatherDuster's `cryptanalib.ecb_cpa_decrypt()` automates this completely.

---

## AES-ECB Cut-and-Paste Block Manipulation (NDH Quals 2016)

**Pattern (Toil33t):** Server encrypts JSON session data in AES-ECB mode. Fields like `is_admin: false` span predictable block boundaries. Construct chosen plaintext blocks via registration, then splice ciphertext blocks to change `false` to `true`.

1. Detect ECB mode: register with repeating username (e.g., 'A' * 64), look for identical ciphertext blocks
2. Map block boundaries by varying username length until block count changes
3. Determine field ordering by independently varying username and email lengths
4. Craft target block containing `true` by aligning it at a block boundary via padding:

```python
# Align "true" at start of a block using space padding (JSON ignores whitespace)
# Original:  {"username": "AA", "is_admin": false, "email": ""}
# Target:    {"username": "AA", "is_admin":            true, "email": ""}
#                                              ^-- 16-byte block boundary

# Get the "            true" block from:
username = "AAA" + " " * 12 + "true"
# Extract block 2 of the resulting ciphertext

# Get prefix blocks from a short username
# Get suffix block from a padded username
# Concatenate: prefix_blocks + true_block + suffix_block
```

**Key insight:** AES-ECB encrypts each 16-byte block independently with no chaining. Identical plaintext blocks produce identical ciphertext blocks, allowing block-level cut-and-paste. JSON's tolerance for extra whitespace enables block alignment without breaking parsing. The attack requires: (a) detecting ECB via repeated blocks, (b) mapping field layout via length probing, (c) crafting and splicing blocks.

---

## AES-CBC IV Bit-Flip Authentication Bypass (Google CTF 2016)

**Pattern (Eucalypt Forest):** Server encrypts JSON session blob under AES-CBC and returns both IV and ciphertext as a cookie. No integrity check (no MAC/HMAC). Flip bits in the IV to change the first plaintext block.

1. Register with username one bit away from target (e.g., `` `dmin `` instead of `admin` — flip LSB of 'a')
2. Identify the IV byte position corresponding to the target character in the first block
3. Flip the same bit in the IV byte — XOR propagates directly to the plaintext:

```python
import binascii
cookie = binascii.unhexlify(auth_cookie)
iv = bytearray(cookie[:16])
ciphertext = cookie[16:]

# Flip LSB of byte at position where 'a'/'`' appears in first block
# Position depends on JSON structure: {"username":"`dmin"}
# 'a' (0x61) vs '`' (0x60) differ only in bit 0
target_pos = 13  # position of first char of username in block
iv[target_pos] ^= 0x01

forged = binascii.hexlify(bytes(iv) + ciphertext)
```

**Key insight:** AES-CBC decryption XORs the previous ciphertext block (or IV for block 0) with the AES-decrypted block. Flipping bit `i` in the IV flips bit `i` in the first plaintext block with no other side effects. This only works when the server performs no integrity verification (no HMAC, AEAD, or authenticated encryption).

---

## Rabin Cryptosystem LSB Parity Oracle (PlaidCTF 2016)

**Pattern (rabit):** Server encrypts flag with the Rabin cryptosystem (`c = m^2 mod n`) and provides an LSB oracle — for any ciphertext, it returns the least significant bit of the decrypted plaintext. Binary search recovers the full plaintext in `log2(n)` queries.

```python
from Crypto.Util.number import long_to_bytes

def lsb_oracle_attack(enc_flag, N, oracle_fn):
    """Recover plaintext from Rabin/RSA LSB oracle via binary search."""
    lower = 0
    upper = N
    C = enc_flag
    # Rabin: encrypt(2,N) = 4; multiplying ciphertext by 4 doubles plaintext
    e2 = pow(2, 2, N)  # For Rabin; use pow(2, e, N) for RSA

    for i in range(N.bit_length()):
        C = (e2 * C) % N  # Multiply plaintext by 2
        lsb = oracle_fn(C)
        if lsb == 1:
            # 2*m > N (odd remainder after mod), increase lower bound
            lower = (upper + lower) // 2
        else:
            # 2*m < N (even remainder), decrease upper bound
            upper = (upper + lower) // 2
        # Progressive decryption visible:
        print(long_to_bytes(upper))
    return upper
```

**Key insight:** Rabin (and textbook RSA) are multiplicatively homomorphic: multiplying ciphertext by `2^e mod N` doubles the plaintext mod N. Since N is odd, doubling causes a modular wraparound iff the plaintext exceeds `N/2`, which changes the LSB parity. This creates a binary search: each oracle query halves the candidate range, recovering the full plaintext in exactly `log2(N)` queries (~1024 for RSA-1024).

---

## PBKDF2 Pre-Hash Bypass for Long Passwords (BackdoorCTF 2016)

**Pattern (Mindblown):** PBKDF2 (and HMAC generally) pre-hashes passwords longer than the hash block size (64 bytes for SHA-1/SHA-256). If the target password exceeds 64 bytes, `PBKDF2(password)` equals `PBKDF2(SHA1(password))`, enabling authentication with the hash instead of the original password.

```python
import hashlib

original_password = "complexPasswordWhichContainsManyCharactersWithRandomSuffixeghjrjg"
# len > 64, so HMAC pre-hashes it
equivalent = hashlib.sha1(original_password.encode()).digest()
# Login with equivalent — PBKDF2 produces the same derived key
```

**Key insight:** HMAC's inner construction is `H((K XOR ipad) || message)`. When the key (password) exceeds the hash block size, HMAC first reduces it via `K = H(password)`. This means `HMAC(long_password, ...)` equals `HMAC(H(long_password), ...)`. Any system using PBKDF2/HMAC with a `!==` identity check after hash comparison is vulnerable when passwords exceed 64 bytes. This is a HMAC specification behavior, not an implementation bug.

---

## MD5 Multi-Collision via Fastcol (BackdoorCTF 2016)

**Pattern (Forge):** Generate 2^k files with identical MD5 hashes by chaining `fastcol` (Marc Stevens' tool). Each run produces two suffixes (A, B) that when appended yield the same MD5. Chain 3 runs to produce 8 collisions:

```text
[prefix][suffix1A][suffix2A][suffix3A]  \
[prefix][suffix1A][suffix2A][suffix3B]   |
[prefix][suffix1A][suffix2B][suffix3A]   |-- all have same MD5
[prefix][suffix1A][suffix2B][suffix3B]   |
[prefix][suffix1B][suffix2A][suffix3A]   |
[prefix][suffix1B][suffix2B][suffix3B]  /
```

```bash
# Install: git clone https://github.com/cr-marcstevens/hashclash
# Generate one collision pair (~minutes on modern CPU):
./fastcol -o suffix1A.bin suffix1B.bin < prefix.bin
# Chain: append suffix1A to prefix, run fastcol again for suffix2A/2B, etc.
```

**Key insight:** MD5 collision generation is practical with `fastcol` (~minutes per pair). Because MD5 uses Merkle-Damgard construction, collisions compose: if `H(A||X) == H(A||Y)`, then `H(A||X||Z) == H(A||Y||Z)` for any suffix Z. Chaining k collision pairs produces 2^k files with identical MD5. For CRC32 collisions, append bytes after PNG IEND chunk (parsers ignore trailing data) and brute-force the 4-byte CRC adjustment.

---

## GHASH Key Recovery over Prime Modulus (nullcon HackIM 2019)

**Pattern (GenuineCounterMode):** A custom GCM-like scheme computes `tag = c + sum(b_i * H^(i+1)) mod n` where `n` is a 128-bit prime (not `GF(2^128)`). The 12-byte nonce has 10 bytes fixed from the session ID plus 2 random bytes, so nonce collisions arrive in roughly 256 encryption queries (birthday bound). With two colliding nonces, the equations for `tag1` and `tag2` share the same `c = E_K(nonce || counter)`, so subtracting eliminates `c` and leaves a linear equation in `H` modulo prime `n` — solvable by a single modular inverse, not `GF(2^128)` polynomial factoring.

```python
from Crypto.Util.number import bytes_to_long, long_to_bytes, inverse

n = 327989969870981036659934487747327553919  # prime modulus (not GF(2^128))

# 1. Request encryptions with single-block messages until two share a nonce
# 2. With colliding (nonce, ct1, tag1) and (nonce, ct2, tag2):
m1 = bytes_to_long(ct1)  # single 16-byte block
m2 = bytes_to_long(ct2)
t1 = bytes_to_long(tag1)
t2 = bytes_to_long(tag2)
H = ((t1 - t2) * inverse(m1 - m2, n)) % n

# 3. Forge: encrypt "may i please have the galf", flip CTR bytes to 'flag'
#    then recompute tag using recovered H and c = tag - sum(b_i * H^(i+1))
c0 = (t1 - sum(bytes_to_long(b) * pow(H, i + 1, n) for i, b in enumerate(blocks1))) % n
forged_tag = (c0 + sum(bytes_to_long(b) * pow(H, i + 1, n) for i, b in enumerate(forged_blocks))) % n
```

**Key insight:** GCM's security rests on `GHASH` operating over `GF(2^128)` where inversion is hard without the key. Swapping the modulus to a plain prime `n` collapses the authentication to textbook linear algebra mod `n` — two nonce-colliding tags give one linear equation per unknown, solved with `inverse(m1 - m2, n)`. Short-nonce (2 random bytes) designs guarantee birthday collisions in ~256 queries. Contrast with the `GF(2^128)` AES-GCM forbidden attack in [modern-ciphers.md](modern-ciphers.md#aes-gcm-nonce-reuse--forbidden-attack), which needs polynomial factoring over binary fields.

---

## SHA-1 Length Extension Plus AES-CBC Cookie Forgery (BSidesSF 2019)

**Pattern (decrypto):** Cookie stores `user = iv || AES-CBC(key, plaintext)` plus a separate `signature = SHA1(secret || decrypt(ct))` tag. The session cookie leaks the AES key (e.g. trailing 32 bytes of a base64 session blob). To forge `UID 0`, length-extend the signature with `\nUID 0\n`, decrypt the current ciphertext to learn the plaintext, append hashpump's padding + extension, re-encrypt with the known key, and send both updated `user` and `signature`.

```python
import hashpumpy, binascii, base64, urllib
from Crypto.Cipher import AES

# Extract AES key from leaked rack.session cookie
key = base64.b64decode(urllib.unquote(cookies['rack.session'].split('--')[0]))[-32:]
user = binascii.unhexlify(cookies['user'])
iv, ct = user[:16], user[16:]

def decrypt(c): return AES.new(key, AES.MODE_CBC, iv).decrypt(c).rstrip(b'\x10\x0f\x0e...')
def encrypt(p): pad = 16 - len(p) % 16; return AES.new(key, AES.MODE_CBC, iv).encrypt(p + bytes([pad])*pad)

# Length-extend signature (secret length guessed = 8)
new_sig, new_plain = hashpumpy.hashpump(cookies['signature'], decrypt(ct), b'\nUID 0\n', 8)
cookies['signature'] = new_sig
cookies['user']      = binascii.hexlify(iv + encrypt(new_plain))
```

**Key insight:** Hash length extension applies whenever the MAC is `H(secret || data)` over a Merkle-Damgard hash (MD5/SHA-1/SHA-256). If the *same* `data` also lives inside a *separately* keyed cipher whose key is recoverable, you can combine primitives: hashpumpy produces the extended plaintext and new tag, then you re-encrypt with the leaked AES key so the server's CBC decryption matches the extended string the signature now covers. Parse-order quirks (later fields overriding earlier ones) let the appended `\nUID 0\n` win.

---

See [modern-ciphers-3.md](modern-ciphers-3.md) for custom hash reversal, CRC32 brute-force, noisy RSA LSB oracle, sponge collisions, CBC IV forgery + block truncation, padding oracle + bit-flip command injection, SPN S-box intersection, AES-CFB IV recovery, three-round XOR, Unicode decode side channel, SHA-256 basis attack, MAC forgery via XOR block cancellation, and bit-by-bit HMAC key recovery.
