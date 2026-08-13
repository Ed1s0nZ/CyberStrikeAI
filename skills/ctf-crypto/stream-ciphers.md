# CTF Crypto - Stream Cipher Attacks

LFSR, RC4, and XOR-based stream cipher attacks. For block cipher attacks (AES, padding oracle, MAC forgery), see [modern-ciphers.md](modern-ciphers.md).

## Table of Contents
- [LFSR Stream Cipher Attacks](#lfsr-stream-cipher-attacks)
  - [Berlekamp-Massey Algorithm](#berlekamp-massey-algorithm)
  - [Correlation Attack](#correlation-attack)
  - [Known-Plaintext on LFSR Keystream](#known-plaintext-on-lfsr-keystream)
  - [Galois vs Fibonacci LFSR](#galois-vs-fibonacci-lfsr)
  - [Common LFSR Lengths and Polynomials](#common-lfsr-lengths-and-polynomials)
  - [Galois LFSR Tap Recovery via Autocorrelation (BSidesSF 2026)](#galois-lfsr-tap-recovery-via-autocorrelation-bsidessf-2026)
- [RC4 Second-Byte Bias Distinguisher (Hackover CTF 2015)](#rc4-second-byte-bias-distinguisher-hackover-ctf-2015)
- [XOR Consecutive Byte Correlation Attack (Defcamp 2015)](#xor-consecutive-byte-correlation-attack-defcamp-2015)
- [Fibonacci Stream Cipher Position-Shifting Oracle (EKOPARTY 2017)](#fibonacci-stream-cipher-position-shifting-oracle-ekoparty-2017)
- [Z3 Constraint Solving for Custom Stream Ciphers (Tokyo Westerns 2017)](#z3-constraint-solving-for-custom-stream-ciphers-tokyo-westerns-2017)
- [Keystream Recovery via Run-Length Encoding Collisions (Google CTF Quals 2018)](#keystream-recovery-via-run-length-encoding-collisions-google-ctf-quals-2018)
- [LFSR Filter Linear Annihilator Attack (Hack.lu 2018)](#lfsr-filter-linear-annihilator-attack-hacklu-2018)
- [Hostname-as-XOR-Key Leaked via DNS Capture (SECCON 2018)](#hostname-as-xor-key-leaked-via-dns-capture-seccon-2018)

---

## LFSR Stream Cipher Attacks

Linear Feedback Shift Registers generate keystreams from an initial state and feedback polynomial. Common in CTF crypto challenges and lightweight/custom ciphers.

**Detection:** Look for bit-level operations (XOR, shift, AND with tap mask), short repeating keystreams, or challenge descriptions mentioning "stream cipher", "LFSR", "shift register", or "linear recurrence".

### Berlekamp-Massey Algorithm

**Pattern:** Given a portion of known keystream (from known plaintext XOR), recover the minimal LFSR that generates it. Once you have the feedback polynomial and state, predict all future (and past) output.

**Key insight:** Berlekamp-Massey finds the shortest LFSR producing a given sequence in O(n^2). If you have 2L consecutive keystream bits (where L is the LFSR length), you can fully recover the LFSR.

```python
from sage.all import *

# Known keystream bits (from known plaintext XOR ciphertext)
keystream = [1, 0, 1, 1, 0, 0, 1, 0, 1, 1, 1, 0, 0, 1]

# Berlekamp-Massey in SageMath
F = GF(2)
seq = [F(b) for b in keystream]
R = berlekamp_massey(seq)  # Returns the feedback polynomial
print(f"LFSR polynomial: {R}")
print(f"LFSR length: {R.degree()}")

# Recover initial state from first L bits
L = R.degree()
state = keystream[:L]

# Generate future keystream
def lfsr_next(state, taps):
    """taps = list of tap positions from polynomial"""
    new_bit = 0
    for t in taps:
        new_bit ^= state[t]
    return state[1:] + [new_bit]
```

### Correlation Attack

**Pattern:** Combined LFSR generator (multiple LFSRs combined through a nonlinear function). If the combining function has correlation bias toward one LFSR's output, attack that LFSR independently.

**Key insight:** If `P(output = LFSR_i output) > 0.5`, brute-force LFSR_i's initial state (2^L candidates for length-L LFSR) and check correlation with known keystream. Much faster than brute-forcing the full combined state.

```python
# Correlation attack on a single biased LFSR
def correlation_attack(keystream_bits, lfsr_length, taps, threshold=0.6):
    """Try all 2^L initial states, keep those with high correlation"""
    best_corr, best_state = 0, None
    for seed in range(2**lfsr_length):
        state = [(seed >> i) & 1 for i in range(lfsr_length)]
        matches = 0
        s = state[:]
        for i, bit in enumerate(keystream_bits):
            if s[0] == bit:
                matches += 1
            s = lfsr_next(s, taps)
        corr = matches / len(keystream_bits)
        if corr > best_corr:
            best_corr, best_state = corr, seed
    return best_state, best_corr
```

### Known-Plaintext on LFSR Keystream

**Pattern:** XOR known plaintext with ciphertext to get keystream. With >=2L keystream bits, solve the linear system directly.

```python
import numpy as np

# Given 2L keystream bits, solve for L-bit state + L feedback taps
# Keystream relation: k[i+L] = c[0]*k[i] + c[1]*k[i+1] + ... + c[L-1]*k[i+L-1] (mod 2)
def solve_lfsr(keystream, L):
    """Solve for LFSR feedback from 2L keystream bits over GF(2)"""
    # Build matrix: each row is [k[i], k[i+1], ..., k[i+L-1]] = k[i+L]
    A = []
    b = []
    for i in range(L):
        A.append(keystream[i:i+L])
        b.append(keystream[i+L])
    # Solve over GF(2) using SageMath
    from sage.all import matrix, vector, GF
    M = matrix(GF(2), A)
    v = vector(GF(2), b)
    coeffs = M.solve_right(v)
    return list(coeffs)
```

### Galois vs Fibonacci LFSR

Two equivalent representations — same keystream, different wiring:
- **Fibonacci:** feedback from multiple taps XOR'd into last position (most common in CTFs)
- **Galois:** feedback distributed across the register (faster in hardware)

Conversion: Galois polynomial is the reciprocal of Fibonacci polynomial. Most CTF tools assume Fibonacci form.

### Common LFSR Lengths and Polynomials

| Bits | Common primitive polynomial | Period |
|------|---------------------------|--------|
| 16 | x^16 + x^14 + x^13 + x^11 + 1 | 65535 |
| 32 | x^32 + x^22 + x^2 + x + 1 | 2^32 - 1 |
| 64 | x^64 + x^4 + x^3 + x + 1 | 2^64 - 1 |

**Maximal-length LFSR:** Primitive polynomial -> period = 2^L - 1 (visits all nonzero states).

### Galois LFSR Tap Recovery via Autocorrelation (BSidesSF 2026)

**Pattern (lfstream):** A PNG file is encrypted by XORing each N-bit block with the current state of a Galois LFSR (right-shift model). The LFSR length, seed, and tap mask are unknown. Recover all three from the known 16-byte PNG header.

**Step 1 — Recover keystream via known plaintext:**

```bash
# PNG header is always: 89 50 4e 47 0d 0a 1a 0a 00 00 00 0d 49 48 44 52
# XOR first 16 encrypted bytes with this header to get 128 keystream bits
```

**Step 2 — Find LFSR length via autocorrelation sliding:**

Slide the 128-bit keystream against itself at increasing offsets. The offset where most bits align reveals the LFSR period. For a right-shift Galois LFSR, the keystream repeats with a one-bit shift per step, so the autocorrelation peak occurs at offset = LFSR length + 1.

```python
def find_lfsr_length(bits, min_len=8, max_len=64, step=8):
    """Slide keystream bits against themselves to find LFSR period."""
    best = None
    for n in range(min_len, max_len + 1, step):
        # Split keystream into n-bit state windows
        states = [int(bits[i*n:(i+1)*n], 2) for i in range(len(bits) // n)]
        if len(states) < 2:
            continue

        # For each transition, check Galois right-shift consistency
        mask_votes = {}
        mismatches = 0
        for i in range(len(states) - 1):
            s, nxt = states[i], states[i + 1]
            base = s >> 1  # Right-shift without feedback
            if s & 1:      # LSB was 1 → feedback applied
                derived_mask = base ^ nxt
                mask_votes[derived_mask] = mask_votes.get(derived_mask, 0) + 1
            else:           # LSB was 0 → no feedback, next = base
                if nxt != base:
                    mismatches += 1

        if mask_votes:
            best_mask, support = max(mask_votes.items(), key=lambda kv: kv[1])
            if mismatches == 0:
                print(f"Length {n}: tap_mask=0x{best_mask:0{n//4}x}, "
                      f"support={support}, mismatches=0 ← MATCH")
```

**Step 3 — Decrypt with recovered parameters:**

```python
def galois_lfsr_step(state, tap_mask, bits):
    """Single step of right-shift Galois LFSR."""
    out = state & 1
    state >>= 1
    if out:
        state ^= tap_mask
    return state & ((1 << bits) - 1)

# Seed = first keystream block (LFSR state before first step)
seed = int(keystream_bits[:lfsr_bits], 2)
state = seed

with open("flag.png.enc_lfsr", "rb") as f_in, open("flag.png", "wb") as f_out:
    block_size = lfsr_bits // 8
    while True:
        chunk = f_in.read(block_size)
        if not chunk:
            break
        key = state.to_bytes(block_size, "big")
        f_out.write(bytes(b ^ k for b, k in zip(chunk, key)))
        state = galois_lfsr_step(state, tap_mask, lfsr_bits)
```

**Key insight:** For a Galois right-shift LFSR (`state >>= 1; if lsb: state ^= tap_mask`), the tap mask is directly computable from any two consecutive states where the outgoing LSB is 1: `tap_mask = (state >> 1) XOR next_state`. This is more direct than Berlekamp-Massey (which assumes Fibonacci form) and requires no algebraic libraries. The autocorrelation approach to find the LFSR length works because correct-length windows produce consistent tap masks with zero mismatches, while incorrect lengths produce contradictory masks.

**When to recognize:** Challenge encrypts a file with known headers (PNG, PDF, ZIP, ELF) using XOR with an unknown "stream cipher" or "PRNG". Filename or description mentions "LFSR", "shift register", or "stream". The encrypted file preserves the original length (no padding), indicating a stream cipher. Try Galois tap recovery first — it's faster and simpler than Berlekamp-Massey for right-shift implementations.

**Known file headers for keystream recovery:**

| Format | Header bytes | Usable bits |
|--------|-------------|-------------|
| PNG | `89 50 4e 47 0d 0a 1a 0a 00 00 00 0d 49 48 44 52` | 128 |
| PDF | `25 50 44 46 2d` ("%PDF-") | 40 |
| ZIP | `50 4b 03 04` | 32 |
| ELF | `7f 45 4c 46` | 32 |
| JFIF | `ff d8 ff e0` | 32 |

---

## RC4 Second-Byte Bias Distinguisher (Hackover CTF 2015)

**Pattern:** Distinguish RC4 output from true random data by exploiting RC4's second-byte bias. The second output byte of RC4 is biased toward `0x00` with probability 1/128 (vs expected 1/256).

```python
count_zero = 0
for sample in all_samples:
    if sample[1] == 0x00:  # second byte
        count_zero += 1

# Expected: random = N/256, RC4 = N/128 (2x more zeros)
if count_zero > threshold:
    print("RC4")
else:
    print("Random")
```

**Key insight:** RC4's key scheduling creates a well-known bias where `P(second_byte == 0) = 1/128` instead of `1/256`. With ~2048 samples, RC4 produces ~16 zero second-bytes vs ~8 for random. Other RC4 biases: bytes 3-255 show weaker biases; long-term biases exist at every 256th position.

---

## XOR Consecutive Byte Correlation Attack (Defcamp 2015)

When a cipher XORs consecutive ciphertext bytes, the relationship between two ciphertexts reveals plaintext differences without knowing the key:

```python
# Observation: xorct[i] = ct[i] ^ ct[i+1]
# For two ciphertext/plaintext pairs:
# plain2[i] ^ plain1[i] == xorct1[i] ^ xorct2[i]

# With one known plaintext, decrypt the other:
for i in range(len(ct2)):
    xorct1 = ct1[i] ^ ct1[i+1]
    xorct2 = ct2[i] ^ ct2[i+1]
    plain2_char = xorct1 ^ xorct2 ^ plain1[i]
```

**Key insight:** XOR of consecutive bytes cancels key material, leaving only plaintext-dependent differences. One known plaintext breaks all subsequent messages.

---

## Fibonacci Stream Cipher Position-Shifting Oracle (EKOPARTY 2017)

**Pattern:** Custom cipher encrypts byte at position `k` as `Fib(seed + k) + plaintext_byte`. The seed is encoded in the first byte of each query. Incrementing the first byte by N shifts the Fibonacci starting position by 1, turning the server into an oracle: given any 2-byte query, the server returns the Fibonacci value at an arbitrary position XOR'd with the corresponding plaintext byte.

**Attack (flag recovery via oracle):**
1. Send queries of the form `[seed_offset][target_byte_position]` to request specific positions in the target ciphertext
2. For each position, try all 256 candidate plaintext values: `candidate_byte + Fib(adjusted_seed + pos)` should match the observed server output
3. Compare against the known target ciphertext byte to identify the correct plaintext

```python
# Oracle: server returns Fib(seed + k) XOR plaintext[k]
# Shift seed by 1 per byte offset increment
for pos in range(flag_length):
    for candidate in range(256):
        # Query with adjusted seed to reach this position
        oracle_output = query(seed_offset=pos, position=0)
        fib_val = oracle_output ^ candidate
        if matches_target_ciphertext(fib_val, pos):
            flag_bytes.append(candidate)
            break
```

**Key insight:** When keystream depends on position in a predictable mathematical way and the starting position is controllable, the server becomes a decryption oracle. Complexity is O(n * 256) queries where n is the flag length — linear in the target size.

**References:** EKOPARTY CTF 2017

---

## Z3 Constraint Solving for Custom Stream Ciphers (Tokyo Westerns 2017)

**Pattern:** Custom stream cipher with algebraic mixing: `encrypted[i] = (message[i] + key[i%13] + encrypted[i-1]) % 128`. Known plaintext prefix (e.g., `TWCTF{`) anchors the first several constraints. Z3 solver encodes each step as an integer constraint and directly recovers both the unknown key and the remaining flag characters.

```python
from z3 import *

key_len = 13
flag_len = len(encrypted)

key = [Int(f'k{i}') for i in range(key_len)]
flag = [Int(f'f{i}') for i in range(flag_len)]

s = Solver()

# Cipher recurrence: enc[i] = (flag[i] + key[i%13] + enc[i-1]) % 128
for i in range(flag_len):
    prev = encrypted[i-1] if i > 0 else 0
    s.add(encrypted[i] == (flag[i] + key[i % key_len] + prev) % 128)

# Key and flag must be printable ASCII
for k in key:
    s.add(k >= 32, k <= 126)
for f in flag:
    s.add(f >= 32, f <= 126)

# Anchor with known plaintext prefix
for i, c in enumerate(b'TWCTF{'):
    s.add(flag[i] == c)

if s.check() == sat:
    m = s.model()
    recovered = bytes([m[flag[i]].as_long() for i in range(flag_len)])
    print(recovered)
```

**Key insight:** Stream ciphers with algebraic (addition-based) mixing are directly amenable to Z3 constraint solving. Encode each step as an integer constraint, add known-plaintext anchors for the flag prefix, and let the solver recover key and remaining plaintext simultaneously. This avoids any manual analysis of the cipher structure.

**References:** Tokyo Westerns CTF 2017

---

## Keystream Recovery via Run-Length Encoding Collisions (Google CTF Quals 2018)

**Pattern (DogeStore):** Server computes `sha3(rle_decode(decrypt(xor(input, keystream))))`. Two different inputs can produce the same decoded plaintext when RLE has multiple valid encodings of the same byte run (e.g., `a\x02` vs `a\x01a\x00` both decode to `aa`). Equal hashes across different ciphertexts therefore expose XOR relationships between keystream bytes.

**Exploit:** For each candidate byte position pair `(i, i+2)` and each byte value `x`, submit two ciphertexts that differ only at those positions by `x`. If the SHA3 outputs match, `keystream[i] XOR keystream[i+2] == x`.

```python
def probe(i, x):
    # Builds two ciphertexts that differ by x at positions i and i+2
    c1 = baseline_cipher(i, 0, 0)
    c2 = baseline_cipher(i + 2, x, x)
    return sha3(server_decode(c1)) == sha3(server_decode(c2))

diffs = {}
for i in range(keystream_len - 2):
    for x in range(256):
        if probe(i, x):
            diffs[i] = x  # k[i] ^ k[i+2] = x
            break
```

Chain the recovered differences to reconstruct the entire keystream once any single byte is known (or constrained by a flag-prefix crib).

**Key insight:** Whenever a protocol post-processes plaintext with a lossy or many-to-one step (RLE, normalization, lowercase), a hash oracle over the post-processed output leaks equalities between unknown plaintext bytes — and therefore keystream bytes — without ever decrypting.

**References:** Google CTF Quals 2018 — writeup 10370

---

## LFSR Filter Linear Annihilator Attack (Hack.lu 2018)

**Pattern:** Keystream is generated by passing the LFSR state through a nonlinear filter function `f`. If `f` has a small linear annihilator `g` (i.e. `g(f(x)) = 0` over GF(2)), every ciphertext byte produces a linear equation in the LFSR state. Enough bytes yield a solvable GF(2) system that recovers the initial state.

```python
from sage.all import *
R = PolynomialRing(GF(2), 'x')
F = GF(2)
# 1. Build matrix A: each row is g(f(state_at_t)) expressed linearly in bits of state_0
# 2. Solve A * state_0 = 0 (kernel gives candidate seeds)
# 3. Filter candidates whose plaintext is printable
for cand in A.right_kernel():
    pt = decrypt(cipher, cand)
    if all(0x20 <= b < 0x7f for b in pt): print(pt)
```

**Key insight:** Any LFSR-based stream cipher with a nonlinear filter is only as strong as its filter's algebraic immunity. If `f` has an annihilator of low degree, the effective keystream is linear, and Gaussian elimination recovers the state. Check the filter function against BoolFunction databases before trusting it.

**References:** Hack.lu CTF 2018 — LFSR StreamCipher, writeup 12084

---

## Hostname-as-XOR-Key Leaked via DNS Capture (SECCON 2018)

**Pattern:** Binary fetches `gethostbyaddr()` on its own IP, reverses the resulting hostname, and uses it as an XOR key for a file at `/flag.txt.encrypted`. The hostname is long and unusual (`cur10us4ndl0ngh0stn4m3`), but appears *in cleartext* in captured DNS traffic during the reverse lookup.

```python
hostname = b"cur10us4ndl0ngh0stn4m3"[::-1]  # recovered from pcap
with open('flag.txt.encrypted','rb') as f: ct = f.read()
flag = bytes(b ^ hostname[i % len(hostname)] for i, b in enumerate(ct))
```

**Key insight:** DNS queries, HTTP `Host` headers, and TLS SNI often leak secrets that the binary treats as confidential. Always pcap the challenge binary during execution — the "key" may never even touch memory you can inspect.

**References:** SECCON 2018 — Boguscrypt, writeup 12054
