# CTF Crypto - Classic Ciphers

## Table of Contents
- [Vigenere Cipher](#vigenere-cipher)
- [Atbash Cipher](#atbash-cipher)
- [Polybius Square Cipher (Qiwi-Infosec 2016)](#polybius-square-cipher-qiwi-infosec-2016)
- [Substitution Cipher with Rotating Wheel](#substitution-cipher-with-rotating-wheel)
- [Kasiski Examination for Key Length](#kasiski-examination-for-key-length)
- [XOR Variants](#xor-variants)
  - [Multi-Byte XOR Key Recovery via Frequency Analysis](#multi-byte-xor-key-recovery-via-frequency-analysis)
  - [Cascade XOR (First-Byte Brute Force)](#cascade-xor-first-byte-brute-force)
  - [XOR with Rotation: Power-of-2 Bit Isolation (Pragyan 2026)](#xor-with-rotation-power-of-2-bit-isolation-pragyan-2026)
  - [Weak XOR Verification Brute Force (Pragyan 2026)](#weak-xor-verification-brute-force-pragyan-2026)
- [Deterministic OTP with Load-Balanced Backends (Pragyan 2026)](#deterministic-otp-with-load-balanced-backends-pragyan-2026)
- [OTP Key Reuse / Many-Time Pad XOR (BYPASS CTF 2025)](#otp-key-reuse--many-time-pad-xor-bypass-ctf-2025)
- [Book Cipher](#book-cipher)
- [Variable-Length Homophonic Substitution (ASIS CTF Finals 2013)](#variable-length-homophonic-substitution-asis-ctf-finals-2013)
- [Grid Permutation Cipher Keyspace Reduction (BSidesSF 2026)](#grid-permutation-cipher-keyspace-reduction-bsidessf-2026)
- [Image-Based Caesar Shift Ciphers (BSidesSF 2026)](#image-based-caesar-shift-ciphers-bsidessf-2026)
  - [Variant A — Vertical Strip Shift (caesar1)](#variant-a--vertical-strip-shift-caesar1)
  - [Variant B — Horizontal Shift with ASCII Encoding (caesar2)](#variant-b--horizontal-shift-with-ascii-encoding-caesar2)
- [XOR Key Recovery via File Format Headers (MetaCTF Flash 2026)](#xor-key-recovery-via-file-format-headers-metactf-flash-2026)
- [3D Vigenere Palindrome Symmetry Key Recovery (SECCON 2017)](#3d-vigenere-palindrome-symmetry-key-recovery-seccon-2017)
- [Nihilist Cipher Double-Crib Key Recovery (Security Fest CTF 2018)](#nihilist-cipher-double-crib-key-recovery-security-fest-ctf-2018)
- [16-Byte XOR Block Cipher Structural Reversal (h4ckc0n 2018)](#16-byte-xor-block-cipher-structural-reversal-h4ckc0n-2018)
- [Flag Semaphore Photo Decoding (DefCamp CTF 2018)](#flag-semaphore-photo-decoding-defcamp-ctf-2018)
- [Two-Byte Nibble Reassembly with Random Padding (Trend Micro 2018)](#two-byte-nibble-reassembly-with-random-padding-trend-micro-2018)

---

## Vigenere Cipher

**Known Plaintext Attack (most common in CTFs):**
```python
def vigenere_decrypt(ciphertext, key):
    result = []
    key_index = 0
    for c in ciphertext:
        if c.isalpha():
            shift = ord(key[key_index % len(key)].upper()) - ord('A')
            base = ord('A') if c.isupper() else ord('a')
            result.append(chr((ord(c) - base - shift) % 26 + base))
            key_index += 1
        else:
            result.append(c)
    return ''.join(result)

def derive_key(ciphertext, plaintext):
    """Derive key from known plaintext (e.g., flag format CCOI26{)"""
    key = []
    for c, p in zip(ciphertext, plaintext):
        if c.isalpha() and p.isalpha():
            c_val = ord(c.upper()) - ord('A')
            p_val = ord(p.upper()) - ord('A')
            key.append(chr((c_val - p_val) % 26 + ord('A')))
    return ''.join(key)
```

### Kasiski Examination for Key Length

When no known plaintext is available, determine the Vigenere key length using Kasiski examination: find repeated sequences in the ciphertext and compute the GCD of their distances.

```python
from math import gcd
from functools import reduce
from collections import Counter

def kasiski_examination(ciphertext, min_seq=3):
    """Find repeating sequences and compute likely key lengths."""
    ct = ''.join(c.upper() for c in ciphertext if c.isalpha())
    distances = []

    # Find repeated trigrams and their distances
    for seq_len in range(min_seq, 6):
        seen = {}
        for i in range(len(ct) - seq_len):
            seq = ct[i:i+seq_len]
            if seq in seen:
                for prev_pos in seen[seq]:
                    distances.append(i - prev_pos)
                seen[seq].append(i)
            else:
                seen[seq] = [i]

    # Key length is likely the GCD of distances
    if distances:
        key_len = reduce(gcd, distances)
        print(f"Likely key length: {key_len}")
        print(f"All distances: {sorted(set(distances))}")
        return key_len
    return None

def frequency_attack(ciphertext, key_length):
    """Break Vigenere by frequency analysis on each key-position group."""
    ct = [c.upper() for c in ciphertext if c.isalpha()]
    english_freq = [0.082,0.015,0.028,0.043,0.127,0.022,0.020,0.061,0.070,
                   0.002,0.008,0.040,0.024,0.067,0.075,0.019,0.001,0.060,
                   0.063,0.091,0.028,0.010,0.023,0.002,0.020,0.001]
    key = []

    for i in range(key_length):
        group = [ct[j] for j in range(i, len(ct), key_length)]
        # Try each shift, score by English letter frequency
        best_shift, best_score = 0, -1
        for shift in range(26):
            decrypted = [chr((ord(c) - ord('A') - shift) % 26 + ord('A')) for c in group]
            freq = Counter(decrypted)
            score = sum(freq.get(chr(j+65), 0) / len(group) * english_freq[j]
                       for j in range(26))
            if score > best_score:
                best_score = score
                best_shift = shift
        key.append(chr(best_shift + ord('A')))

    return ''.join(key)
```

**Key insight:** Repeated sequences in Vigenere ciphertext occur at distances that are multiples of the key length. The GCD of all such distances reveals the key length, after which each position becomes a simple Caesar cipher solvable by frequency analysis.

**When standard keys don't work:**
1. Key may not repeat - could be as long as message
2. Key derived from challenge theme (character names, phrases)
3. Key may have "padding" - repeated letters (IICCHHAA instead of ICHA)
4. Try guessing plaintext words from theme, derive full key

---

## Atbash Cipher

Simple substitution: A<->Z, B<->Y, C<->X, etc.

```python
def atbash(text):
    return ''.join(
        chr(ord('Z') - (ord(c.upper()) - ord('A'))) if c.isalpha() else c
        for c in text
    )
```

**Identification:** Challenge name hints ("Abashed" = Atbash), preserves spaces/punctuation, 1-to-1 substitution.

---

## Polybius Square Cipher (Qiwi-Infosec 2016)

5x5 grid cipher where each letter maps to a two-digit coordinate (row, column). I/J typically share a cell.

```python
import string

def polybius_decrypt(ciphertext, key="ABCDEFGHIKLMNOPQRSTUVWXYZ"):
    """Decrypt Polybius square cipher (pairs of digits 1-5)"""
    grid = {}
    for i, ch in enumerate(key):
        row, col = i // 5 + 1, i % 5 + 1
        grid[(row, col)] = ch

    digits = [int(d) for d in ciphertext if d.isdigit()]
    plaintext = ""
    for i in range(0, len(digits), 2):
        plaintext += grid.get((digits[i], digits[i+1]), '?')
    return plaintext

# Example: "5211251521531412" -> pairs (5,2)(1,1)(2,5)(1,5)(2,1)(5,3)(1,4)(1,2)
print(polybius_decrypt("5211251521531412"))
```

**Key insight:** Polybius ciphers produce digit-only ciphertext with values 1-5. The 5x5 grid merges I/J into one cell. Custom key alphabets change the grid layout but the two-digit coordinate structure remains constant.

---

## Substitution Cipher with Rotating Wheel

**Pattern (Wheel of Mystery):** Physical cipher wheel with inner/outer alphabets.

**Automated solver:** Use [quipqiup.com](https://quipqiup.com/) for general substitution ciphers — it uses word pattern matching and language entropy to solve without knowing the key.

**Brute force all rotations:**
```python
outer = "ABCDEFGHIJKLMNOPQRSTUVWXYZ{}"
inner = "QNFUVWLEZYXPTKMR}ABJICOSDHG{"  # Given

for rotation in range(len(outer)):
    rotated = inner[rotation:] + inner[:rotation]
    mapping = {outer[i]: rotated[i] for i in range(len(outer))}
    decrypted = ''.join(mapping.get(c, c) for c in ciphertext)
    if decrypted.startswith("METACTF{"):
        print(decrypted)
```

---

## XOR Variants

### Multi-Byte XOR Key Recovery via Frequency Analysis

**Pattern:** Ciphertext XOR'd with a repeating multi-byte key. Key length unknown.

**Step 1 — Determine key length:** Try each candidate length, split ciphertext into groups by position modulo key length, score each group's byte frequency against English text (space = 0x20 is the most common byte).

**Step 2 — Recover each key byte:** For each position, brute-force all 256 byte values and select the one producing the most English-like decrypted text.

```python
from collections import Counter

def score_english(data):
    """Score how English-like a byte sequence is."""
    freq = Counter(data)
    # Space is the most common character in English text
    return freq.get(ord(' '), 0) + sum(freq.get(c, 0) for c in range(ord('a'), ord('z')+1))

def find_key_length(ciphertext, max_len=40):
    """Test key lengths by scoring single-byte XOR on each column."""
    best_len, best_score = 1, 0
    for kl in range(1, max_len + 1):
        total = 0
        for col in range(kl):
            group = ciphertext[col::kl]
            best_col_score = max(
                score_english(bytes(b ^ k for b in group))
                for k in range(256)
            )
            total += best_col_score
        if total > best_score:
            best_score = total
            best_len = kl
    return best_len

def recover_key(ciphertext, key_length):
    """Recover each key byte via frequency analysis."""
    key = []
    for col in range(key_length):
        group = ciphertext[col::key_length]
        best_k = max(range(256), key=lambda k: score_english(bytes(b ^ k for b in group)))
        key.append(best_k)
    return bytes(key)

ct = open('encrypted.bin', 'rb').read()
kl = find_key_length(ct)
key = recover_key(ct, kl)
print(f"Key ({kl} bytes): {key}")
print(bytes(c ^ key[i % len(key)] for i, c in enumerate(ct)))
```

**Key insight:** Multi-byte repeating XOR splits into `key_length` independent single-byte XOR problems. English text frequency (especially space = 0x20) reliably identifies correct key bytes. Works best with ciphertext longer than ~100 bytes.

### Cascade XOR (First-Byte Brute Force)

**Pattern (Shifty XOR):** Each byte XORed with previous ciphertext byte.

```python
# c[i] = p[i] ^ c[i-1] (or similar cascade)
# Brute force first byte, rest follows deterministically
for first_byte in range(256):
    flag = [first_byte]
    for i in range(1, len(ct)):
        flag.append(ct[i] ^ flag[i-1])
    if all(32 <= b < 127 for b in flag):
        print(bytes(flag))
```

### XOR with Rotation: Power-of-2 Bit Isolation (Pragyan 2026)

**Pattern (R0tnoT13):** Given `S XOR ROTR(S, k)` for multiple rotation offsets k, recover S.

**Key insight:** When ALL rotation offsets are powers of 2 (2, 4, 8, 16, 32, 64), even-indexed and odd-indexed bits NEVER mix across any frame. This reduces N-bit recovery to just 2 bits of brute force.

**Algorithm:**
1. Express every bit of S in terms of two unknowns (s_0 for even bits, s_1 for odd bits) using the k=2 frame
2. Only 4 candidate states -> try all, verify against all frames
3. XOR valid state with ciphertext -> plaintext

### Weak XOR Verification Brute Force (Pragyan 2026)

**Pattern (Dor4_Null5):** Verification XORs all comparison bytes into a single byte instead of checking each individually.

**Vulnerability:** Any fixed response has 1/256 probability of passing. With enough interaction budget (e.g., 4919 attempts), brute-force succeeds with ~256 expected attempts.

```python
for attempt in range(3000):
    r.sendlineafter(b"prompt: ", b"00" * 8)  # Fixed zero response
    result = r.recvline()
    if b"successful" in result:
        break
```

---

## Deterministic OTP with Load-Balanced Backends (Pragyan 2026)

**Pattern (DumCows):** Service encrypts data with deterministic keystream that resets per connection. Multiple backends with different keystreams behind a load balancer.

**Attack:**
1. Send known plaintext (e.g., 18 bytes of 'A'), XOR with ciphertext -> recover keystream
2. XOR keystream with target ciphertext -> decrypt secret
3. **Backend matching:** Must connect to same backend for keystream to match. Retry connections until patterns align.

```python
def recover_keystream(known, ciphertext):
    return bytes(k ^ c for k, c in zip(known, ciphertext))

def decrypt(keystream, target_ct):
    return bytes(k ^ c for k, c in zip(keystream, target_ct))
```

**Key insight:** When encryption is deterministic per connection with no nonce/IV, known-plaintext attack is trivial. The challenge is matching backends.

---

## OTP Key Reuse / Many-Time Pad XOR (BYPASS CTF 2025)

**Pattern (Once More Unto the Same Wind):** Two ciphertexts encrypted with the same OTP key. Known plaintext for one message enables recovery of the other.

**XOR property:** `C1 XOR C2 = P1 XOR P2` (key cancels). When one plaintext (P1) is known, recover the other: `P2 = C1 XOR C2 XOR P1`.

```python
from pwn import xor

c1 = bytes.fromhex("7713283f5e9979...")
c2 = bytes.fromhex("740b393f4c8b67...")

# If one plaintext is known (or guessable, e.g., padded 'A' chars)
known_plaintext = b"A" * len(c1)
flag = xor(xor(c1, c2), known_plaintext)
print(flag)
```

**When plaintext is unknown — crib dragging:**
```python
def crib_drag(c1, c2, crib, max_pos=None):
    """Slide known word across XOR of two ciphertexts."""
    xored = xor(c1[:min(len(c1), len(c2))], c2[:min(len(c1), len(c2))])
    for pos in range(len(xored) - len(crib)):
        candidate = xor(xored[pos:pos+len(crib)], crib)
        if all(32 <= b < 127 for b in candidate):
            print(f"pos {pos}: {candidate}")
```

**Key insight:** OTP (One-Time Pad) XOR encryption is only secure when the key is truly one-time. Reusing the key on two messages leaks `P1 XOR P2` — exploit with known plaintext or crib dragging.

---

## Book Cipher

**Pattern (Booking Key, Nullcon 2026):** Book cipher with "steps forward" encoding. Brute-force starting position with charset filtering reduces ~56k candidates to 3-4.

See [historical.md](historical.md) for full implementation.

---

## Variable-Length Homophonic Substitution (ASIS CTF Finals 2013)

**Pattern (Rookie Agent):** Ciphertext uses alphanumeric characters grouped in blocks of 5. Single-character frequency analysis shows non-uniform distribution. N-gram analysis reveals repeated multi-character groups mapping to single plaintext characters, with different plaintext characters encoded by groups of different lengths (1-4 characters).

**Analysis workflow:**

1. Collapse whitespace and compute n-gram frequencies (1 through 6):
```python
from collections import Counter

ct = "6di16ovhtmnzslsxqcjo8fkdmtyrbn..."  # cleaned ciphertext
for n in range(1, 7):
    ngrams = [ct[i:i+n] for i in range(len(ct)-n+1)]
    freq = Counter(ngrams).most_common(20)
    print(f"{n}-grams: {freq[:10]}")
```

2. Identify constant-frequency groups — if `8f`, `fk`, and `kd` each appear exactly 36 times, check whether `8fkd` also appears 36 times. If so, it is a single substitution unit:
```python
# Iteratively replace most-frequent fixed groups with single symbols
substitutions = {
    '8fkd': 'E', '4bg9': 'I', 'lsxq': 'A', 'fmrk': 'B',
    '9gle': 'C', 'mtyr': 'D', 'cjo': 'F', 'htm': 'G',
    # ... continue for all identified groups
}
reduced = ct
for pattern, symbol in sorted(substitutions.items(), key=lambda x: -len(x[0])):
    reduced = reduced.replace(pattern, symbol)
```

3. The reduced text is now a monoalphabetic substitution — solve with [quipqiup.com](https://quipqiup.com/) or statistical analysis on English.

4. When some characters remain ambiguous after decryption, brute-force permutations against a known hash of the flag:
```python
from itertools import permutations
from hashlib import sha256

partial_flag = '3c6a1c371b381c943065864b95ae5546'
ambiguous_chars = '12456789x'  # chars with uncertain mapping
known_hash = '9f2a579716af14400c9ba1de8682ca52c17b3ed4235ea17ac12ae78ca24876ef'

for p in permutations(ambiguous_chars):
    mapping = dict(zip(ambiguous_chars, p))
    candidate = ''.join(mapping.get(c, c) for c in partial_flag)
    if sha256(('ASIS_' + candidate).encode()).hexdigest() == known_hash:
        print(f"Flag: ASIS_{candidate}")
        break
```

**Key insight:** Variable-length homophonic substitution hides letter frequencies by mapping common plaintext letters to longer codegroups. The attack reverses this: find n-grams that always appear as a unit (identical frequency for all sub-n-grams), replace them with single symbols, then solve the resulting monoalphabetic substitution. When the flag format provides a hash for verification, brute-force remaining ambiguous character permutations offline.

---

## Grid Permutation Cipher Keyspace Reduction (BSidesSF 2026)

**Pattern (ghostcrypt):** A substitution cipher built on a 5x5 grid where the key permutes rows and columns independently. Row permutations and column permutations commute — applying all row swaps then all column swaps gives the same result regardless of order. This collapses the keyspace from potentially huge to just 5! x 5! = 14,400 combinations, making brute-force trivial.

```python
from itertools import permutations

# 5x5 grid substitution cipher — brute force row+column permutations
grid_size = 5
ciphertext = "..."  # encrypted text
wordlist = set(open("/usr/share/dict/words").read().split())

for row_perm in permutations(range(grid_size)):
    for col_perm in permutations(range(grid_size)):
        # Apply inverse permutation to grid
        decrypted = apply_grid_permutation(ciphertext, row_perm, col_perm)
        words = decrypted.split()
        if sum(1 for w in words if w.lower() in wordlist) > len(words) * 0.5:
            print(f"Key: rows={row_perm}, cols={col_perm}")
            print(decrypted)
            break
```

**Key insight:** Row and column permutations on a grid are independent operations that commute. The total keyspace is the product of row permutations x column permutations (n!^2), NOT the factorial of total cells. For a 5x5 grid: 120 x 120 = 14,400 — brute-forceable in milliseconds.

**When to recognize:** Challenge uses a grid-based cipher, mentions "row/column shuffling", or provides a substitution table that looks like a permuted matrix. Any grid cipher where rows and columns are shuffled independently has this n!^2 keyspace property.

---

## Image-Based Caesar Shift Ciphers (BSidesSF 2026)

Two variants of applying Caesar cipher concepts to 2D image data:

### Variant A — Vertical Strip Shift (caesar1)

Each vertical strip of pixels is shifted downward by `(column / strip_width) * multiplier mod height`. The multiplier is a small integer (1-50), making it brute-forceable.

```python
from PIL import Image
import sys

img = Image.open("shifted.png")
w, h = img.size
pixels = img.load()
strip_width = 10  # Determined by visual inspection

for multiplier in range(1, 51):
    out = Image.new("RGB", (w, h))
    out_px = out.load()
    for x in range(w):
        shift = (x // strip_width) * multiplier % h
        for y in range(h):
            out_px[x, (y - shift) % h] = pixels[x, y]
    out.save(f"attempt_{multiplier}.png")
```

### Variant B — Horizontal Shift with ASCII Encoding (caesar2)

Each row is shifted horizontally by a different amount. The shift value for each strip directly encodes an ASCII character of the flag.

```python
from PIL import Image

original = Image.open("original.png")
shifted = Image.open("shifted.png")
w, h = original.size

flag = ""
prev_shift = -1
for y in range(h):
    orig_row = [original.getpixel((x, y)) for x in range(w)]
    shift_row = [shifted.getpixel((x, y)) for x in range(w)]
    # Find shift by comparing rows
    for offset in range(128):
        if all(orig_row[(x + offset) % w] == shift_row[x] for x in range(min(20, w))):
            if offset != prev_shift:
                flag += chr(offset)
                prev_shift = offset
            break
print(flag)
```

**Key insight:** Image pixel shifts are a visual form of Caesar cipher. When comparing an original and shifted image, the shift amount per row/column directly encodes hidden data. Always compare row-by-row or column-by-column when given two versions of the same image.

**When to recognize:** Challenge provides one or two image files with visible horizontal or vertical "shearing" artifacts. If an original image is provided alongside a shifted version, compute per-row or per-column offsets and check if they decode as ASCII.

---

## XOR Key Recovery via File Format Headers (MetaCTF Flash 2026)

**Pattern (In The Door):** A file claims to be a known format (e.g., PDF, PNG, ZIP) but `file` reports it as "data". The file has been XOR-encrypted with a repeating key. Recover the key by XOR-ing the encrypted bytes against the expected file format header, then extend the key using known structural elements at the end of the file.

```python
# Step 1: XOR first bytes against expected header to derive key start
encrypted = open('encrypted.pdf', 'rb').read()

# PDF files always start with %PDF-1.
expected_header = b'%PDF-1.'
key_start = bytes(a ^ b for a, b in zip(encrypted[:len(expected_header)], expected_header))
print(f"Key prefix: {key_start}")  # e.g., b'h4ck4ll'

# Step 2: Extend key using known trailer structures
# PDF files end with %%EOF (possibly followed by newline)
# Try known trailer patterns at the end of the file
pdf_trailers = [b'%%EOF\n', b'%%EOF\r\n', b'%%EOF']
for trailer in pdf_trailers:
    tail = encrypted[-len(trailer):]
    key_tail = bytes(a ^ b for a, b in zip(tail, trailer))
    print(f"Key tail candidate: {key_tail}")

# Step 3: Once key length is known, combine fragments
# Common structures to anchor: 'startxref', 'trailer', 'endobj'
key = b'h4ck4llth3cryp70'  # 16-byte repeating key
key_len = len(key)

# Step 4: Decrypt entire file
decrypted = bytes(encrypted[i] ^ key[i % key_len] for i in range(len(encrypted)))
with open('decrypted.pdf', 'wb') as f:
    f.write(decrypted)

# Verify
import subprocess
result = subprocess.run(['file', 'decrypted.pdf'], capture_output=True, text=True)
print(result.stdout)  # Should show: PDF document
```

**Key insight:** Every file format has known byte sequences at fixed positions -- magic bytes at the start, structural markers throughout, and trailer signatures at the end. XOR with a repeating key is fully recoverable when you know enough plaintext at known offsets. For a key of length N, you need N bytes of known plaintext at known positions (they need not be contiguous, but you must know their offset modulo the key length).

**Common file format anchors for key recovery:**

| Format | Header | Trailer/Footer |
|--------|--------|----------------|
| PDF | `%PDF-1.` | `%%EOF` |
| PNG | `\x89PNG\r\n\x1a\n` | `IEND\xaeB\x60\x82` |
| ZIP | `PK\x03\x04` | `PK\x05\x06` (EOCD) |
| JPEG | `\xff\xd8\xff\xe0` | `\xff\xd9` |
| ELF | `\x7fELF` | -- |
| GIF | `GIF89a` or `GIF87a` | `\x3b` (trailer) |

**When to recognize:** Challenge provides a file that should be a known format (filename extension or description says so) but `file` reports "data" or wrong type. Hex dump shows no recognizable magic bytes. XOR the first few bytes against the expected header -- if the result looks like an ASCII string or repeating pattern, it is a repeating XOR key.

**Determining key length:** If the header-derived key fragment repeats or the key is a readable string, try common lengths (8, 16, 32). Alternatively, XOR the file against itself shifted by candidate key lengths and look for low-entropy output (many null bytes indicate correct shift = key length).

**References:** MetaCTF Flash CTF 2026 "In The Door"

---

## 3D Vigenere Palindrome Symmetry Key Recovery (SECCON 2017)

**Pattern:** When k2 = reverse(k1) in a 3D Vigenere cipher, the encryption depends only on `k1[i] + k1[key_len-1-i]` (symmetric sums). Only half the key needs recovery:

```python
# Encryption: ct[i] = table[k1[i%kl]][k2[i%kl]][pt[i]]
# With k2 = reverse(k1): ct[i] depends on k1[i%kl] + k1[(kl-1-i)%kl]
# Known-plaintext with flag prefix recovers kl/2 sum values
# Then brute-force one half of the key (the sums constrain the other)
for c1 in range(len(s)):
    for c2 in range(len(s)):
        if (c1 + c2) % len(s) == known_sum:
            # test this key pair
```

**Key insight:** Palindrome key structure (k2 = reverse(k1)) halves the effective keyspace. Each plaintext position depends on the sum of two key characters at mirrored positions. Known plaintext of length >= key_length/2 fully constrains these sums, reducing the remaining brute-force dramatically. This generalizes to any polyalphabetic cipher where key symmetry reduces independent key variables.

**References:** SECCON CTF 2017

---

## Nihilist Cipher Double-Crib Key Recovery (Security Fest CTF 2018)

**Pattern (Mission Impossible):** Nihilist cipher encrypts via Polybius square lookup + additive numeric key. Each plaintext letter becomes a two-digit Polybius coordinate, then the key's digit stream is added to produce ciphertext digits.

**Key insight:** The flag format `midnight{...}` contains two `i` characters at known positions. Both must produce the same Polybius coordinates (e.g., `24`), so their ciphertext differences directly reveal two key digit pairs — enough to constrain the additive key. Every valid Polybius coordinate must be in range 1-5, which prunes invalid key candidates aggressively.

**Recovery approach:**
```python
# For each pair of key digits (k1, k2) in 1..9:
#   for each ciphertext two-digit group:
#     plain = ((c1 - k1) % 10, (c2 - k2) % 10)
#     if plain[0] not in 1..5 or plain[1] not in 1..5: reject
#   otherwise test frequency (most common letter => 'e') and look up Polybius
```
Use the two repeated-character constraints from the flag prefix to cut the keyspace to a handful, then brute-force the Polybius square from remaining ciphertext frequency.

**References:** Security Fest CTF 2018 — writeup 10210

---

## 16-Byte XOR Block Cipher Structural Reversal (h4ckc0n 2018)

**Pattern (custom XOR block cipher):** Encryption operates on 16-byte blocks split into four 4-byte lanes. Each output byte is an XOR of several input bytes from the same block; one lane can be recovered as the XOR of the other three.

**Exploit:** Because every encrypted byte is a linear combination of plaintext bytes in the same block, XOR three lanes together to reconstruct the fourth. No key recovery needed — the algorithm is self-inverting once the linear structure is identified.

```python
def decrypt(ciphertext):
    out = bytearray()
    for i in range(0, len(ciphertext), 16):
        for j in range(4):
            xorsum = 0
            for k in range(4):
                if k != j:
                    for l in range(i + k*4, i + k*4 + 4):
                        xorsum ^= ciphertext[l]
            for m in range(i + j*4, i + j*4 + 4):
                out.append(ciphertext[m] ^ xorsum)
    return bytes(out)
```

**Key insight:** Any unkeyed cipher built from XOR-only operations over fixed-size blocks is a linear system. If each output bit is a linear combination of input bits, invert by solving for the dependent lane — no key or brute force required.

**References:** h4ckc0n 2018 — writeup 10806

---

## Flag Semaphore Photo Decoding (DefCamp CTF 2018)

**Pattern:** Challenge provides photos of a person holding flags in fixed positions. Map each pose to a letter using the standard flag semaphore chart (8 compass directions × 2 arms = ~32 letters). Two special poses — `J` (shift to "letters") and `#` (shift to "numbers") — switch between modes; maintain a mode flag while decoding.

```python
SEMAPHORE = {
    ('NW','N'): 'A', ('NW','NE'): 'B', ('NW','E'): 'C', ('NW','SE'): 'D',
    # ... full table on Wikipedia
    ('N','NE'):  'J',  # letters shift
    ('NE','SE'): '#',  # numbers shift
}
letters = [SEMAPHORE[pose] for pose in pose_sequence]
```

**Key insight:** Semaphore is often disguised as "person dancing" or "art installation" photos. The presence of exactly two outstretched limbs per frame is the tell.

**References:** DefCamp CTF 2018 — Multiple Flags, writeup 12005

---

## Two-Byte Nibble Reassembly with Random Padding (Trend Micro 2018)

**Pattern:** Custom encoding outputs two bytes for every input byte, where each input byte is split into high and low nibbles, and each nibble is padded with a random high nibble. Recover the original by masking low nibbles and recombining.

```python
def decode(src):
    return bytes(((src[2*i] & 0xf) << 4) | (src[2*i+1] & 0xf)
                 for i in range(len(src)//2))
```

**Key insight:** Random padding in the *high* nibble can be ignored entirely — only the low nibbles carry signal. Spot the pattern when the encoded length is exactly 2x the plaintext length and histograms show uniform high-nibble distribution.

**References:** Trend Micro CTF 2018 — J1, writeup 12874
