# CTF Crypto - Modern Cipher Attacks (Part 3)

Custom hash reversal, CRC brute-force, noisy RSA oracles, sponge collisions, CBC/padding oracle tricks, SPN recovery, AES-CFB, three-round XOR, Unicode side channels, SHA-256 basis attacks, MAC forgery, HMAC bit oracles. For Blum-Goldwasser, hash length extension, compression oracles, OFB/HMAC-CRC/DES weak keys, SRP, square attack, AES-ECB/CBC oracles, Rabin, PBKDF2, and MD5 multi-collision, see [modern-ciphers-2.md](modern-ciphers-2.md).

## Table of Contents
- [Custom Hash State Reversal via Known Intermediates (BackdoorCTF 2016)](#custom-hash-state-reversal-via-known-intermediates-backdoorctf-2016)
- [CRC32 Brute-Force for Small Payloads (BackdoorCTF 2016)](#crc32-brute-force-for-small-payloads-backdoorctf-2016)
- [Noisy RSA LSB Oracle with Post-Hoc Error Correction (SharifCTF 7 2016)](#noisy-rsa-lsb-oracle-with-post-hoc-error-correction-sharifctf-7-2016)
- [Sponge Hash Collision via Meet-in-the-Middle on Partial State (BKP 2017)](#sponge-hash-collision-via-meet-in-the-middle-on-partial-state-bkp-2017)
- [CBC IV Forgery + Block Truncation for Authentication Bypass (0CTF 2017)](#cbc-iv-forgery--block-truncation-for-authentication-bypass-0ctf-2017)
- [Padding Oracle to CBC Bitflip Command Injection (BSidesSF 2017)](#padding-oracle-to-cbc-bitflip-command-injection-bsidessf-2017)
- [SPN Cipher Partial Key Recovery via S-box Intersection (SharifCTF 7 2016)](#spn-cipher-partial-key-recovery-via-s-box-intersection-sharifctf-7-2016)
- [AES-CFB IV Recovery from Timestamp-Seeded PRNG (SHA2017)](#aes-cfb-iv-recovery-from-timestamp-seeded-prng-sha2017)
- [Three-Round XOR Protocol Key Cancellation (HITB 2017)](#three-round-xor-protocol-key-cancellation-hitb-2017)
- [AES-CBC UnicodeDecodeError Side-Channel Oracle (Kaspersky 2017)](#aes-cbc-unicodedecodeerror-side-channel-oracle-kaspersky-2017)
- [SHA-256 Basis Attack for XOR-Aggregate Hash Bypass (34C3 CTF 2017)](#sha-256-basis-attack-for-xor-aggregate-hash-bypass-34c3-ctf-2017)
- [Custom MAC Forgery via XOR Block Cancellation with Key Rotation (PlaidCTF 2018)](#custom-mac-forgery-via-xor-block-cancellation-with-key-rotation-plaidctf-2018)
- [Bit-by-Bit HMAC Key Recovery via XOR Plus Addition Arithmetic (Midnight Sun CTF 2018)](#bit-by-bit-hmac-key-recovery-via-xor-plus-addition-arithmetic-midnight-sun-ctf-2018)
- [CBC IV Recovery from Block-2 Known Plaintext (RITSEC 2018)](#cbc-iv-recovery-from-block-2-known-plaintext-ritsec-2018)
- [Iterated SHA-256 Timing Oracle on Character Match (35C3 2018)](#iterated-sha-256-timing-oracle-on-character-match-35c3-2018)
- [GF(p) Linear-System AES Key Recovery from PCAP Matrix (35C3 Junior 2018)](#gfp-linear-system-aes-key-recovery-from-pcap-matrix-35c3-junior-2018)
- [SHA-1 Length Extension with UTF-8 High-Byte Bypass (OTW Advent 2018)](#sha-1-length-extension-with-utf-8-high-byte-bypass-otw-advent-2018)
- [Cross-Session Cube-Root Recovery via CRT (X-MAS 2018)](#cross-session-cube-root-recovery-via-crt-x-mas-2018)
- [CBC Previous-Block Byte Flipping for Cookie Privilege Escalation (picoCTF 2018)](#cbc-previous-block-byte-flipping-for-cookie-privilege-escalation-picoctf-2018)

---

## Custom Hash State Reversal via Known Intermediates (BackdoorCTF 2016)

**Pattern (Collision Course):** Custom hash processes 4-byte blocks, updating state with XOR and rotations. If intermediate states are printed, reverse each block's hash by computing `hash(block) = s(i) XOR ROL(s(i+1), 7)`. Then brute-force 4-byte printable inputs matching each hash value.

```python
def reverse_hash_states(states):
    """Given intermediate hash states, recover per-block hash values."""
    blocks = []
    for i in range(len(states) - 1):
        # state_update: s(i+1) = ROR(s(i) ^ hash(block), 7)
        # Therefore:    hash(block) = s(i) ^ ROL(s(i+1), 7)
        h = states[i] ^ rol32(states[i+1], 7)
        blocks.append(h)
    return blocks

def rol32(val, n):
    return ((val << n) | (val >> (32 - n))) & 0xFFFFFFFF

# Brute-force printable 4-byte blocks matching each hash
import itertools, string
for target_hash in block_hashes:
    for chars in itertools.product(string.printable, repeat=4):
        block = bytes(ord(c) for c in chars)
        if custom_hash(block) == target_hash:
            print(f"Found: {block}")
            break
```

**Key insight:** When a custom hash function leaks intermediate states (after each block), each block becomes an independent 4-byte brute-force problem (~2^32 worst case, reduced to ~10^8 for printable ASCII). Inverting the state update equation isolates per-block targets. This pattern appears whenever iterative hashes expose partial state.

---

## CRC32 Brute-Force for Small Payloads (BackdoorCTF 2016)

**Pattern (CRC):** Encrypted ZIP files store CRC32 of uncompressed contents. For very small files (5 bytes), brute-force all printable 5-character strings, compute CRC32, and match against the stored value. Multiple matches are common but context resolves ambiguity.

```python
import binascii, itertools, string, zipfile

# Extract CRC from ZIP without decrypting
with zipfile.ZipFile('encrypted.zip') as z:
    crc = z.infolist()[0].CRC

# Brute-force 5-byte printable content
for chars in itertools.product(string.printable[:95], repeat=5):
    candidate = ''.join(chars).encode()
    if binascii.crc32(candidate) & 0xFFFFFFFF == crc:
        print(f"Match: {candidate}")
```

**Key insight:** CRC32 stored in ZIP headers is not encrypted — it's always accessible even for password-protected ZIPs. For small files (≤ 6 bytes of printable ASCII), the search space is feasible. A C implementation is ~100x faster than Python. Multiple CRC collisions are expected for 5+ byte payloads; combine with language analysis or cross-reference multiple encrypted files to disambiguate.

---

## Noisy RSA LSB Oracle with Post-Hoc Error Correction (SharifCTF 7 2016)

**Pattern:** Extension of the RSA LSB oracle binary search when the oracle occasionally returns incorrect results. Run the standard LSB oracle attack, then inspect decoded bytes. Non-ASCII or unexpected charset values indicate an oracle error within the last ~8 bits. Try single bit-flips at nearby oracle positions; the correct flip fixes the entire remaining decryption.

```python
def lsb_oracle_attack(ciphertext, e, n, oracle_fn, flips=None):
    """Recover plaintext from RSA LSB oracle, with optional error correction."""
    flips = flips or []
    lower, upper = 0, n
    mult = 1
    for i in range(n.bit_length()):
        ciphertext = (ciphertext * pow(2, e, n)) % n
        result = oracle_fn(ciphertext)
        if i in flips:
            result = not result  # correct known oracle error
        mid = (lower + upper) // 2
        if result == 0:
            upper = mid
        else:
            lower = mid
    return lower
```

**Key insight:** Sparse oracle errors produce localized corruption in the recovered plaintext. By inspecting character validity (e.g., expecting hex digits), the error position can be identified and corrected by flipping the oracle result at that query index.

---

## Sponge Hash Collision via Meet-in-the-Middle on Partial State (BKP 2017)

**Pattern:** A custom sponge hash uses AES with a known key, XORing 10-byte message blocks into a 16-byte state. Since only 10 of 16 state bytes are controllable per block, a direct preimage requires ~2^48 work. Meet-in-the-middle reduces this: precompute 2^24 forward AES encryptions keyed on their last 6 bytes, then search backward decryptions for matches in those 6 bytes.

```python
from Crypto.Cipher import AES
import os

aes = AES.new(b'\x00' * 16, AES.MODE_ECB)
forward = {}

# Forward: compute AES(random_10_bytes || 0x00*6), key on last 6 bytes
for _ in range(2**24):
    block = os.urandom(10) + b'\x00' * 6
    enc = aes.encrypt(block)
    forward[enc[-6:]] = block

# Backward: compute AES_dec(target XOR random_c), check last 6 bytes
target_state = b'\x77\x40\x56\x0a\x1d\x64'  # target hash
for _ in range(2**40):
    c_block = os.urandom(10) + target_state
    dec = aes.decrypt(c_block)
    if dec[-6:] in forward:
        a_block = forward[dec[-6:]]
        b_block = xor(aes.encrypt(a_block), dec)  # middle block
        break
```

**Key insight:** When a sponge rate is smaller than the state size, the uncontrolled bytes create a meet-in-the-middle opportunity. Precompute one direction, search the other — reducing 2^48 to 2^24 space + 2^24 time.

---

## CBC IV Forgery + Block Truncation for Authentication Bypass (0CTF 2017)

**Pattern:** Service encrypts `MD5(padded_name) || padded_name` with AES-CBC. The MD5 serves as an integrity check on login. Two attacks combine: (1) IV manipulation: XOR IV bytes to change the decrypted first block from the source MD5 to the target MD5. (2) Block truncation: register with `pad("admin") + 16_junk_bytes`, then strip trailing ciphertext blocks — AES-CBC has no length field, so shorter ciphertext decrypts validly if PKCS7 padding is correct.

```python
# Forge IV to flip MD5 from registered user to "admin"
source_md5 = md5(pad("admin") + b"A"*16)
target_md5 = md5(pad("admin"))
new_iv = bytes(a ^ b ^ c for a, b, c in zip(original_iv, source_md5, target_md5))

# Strip last 2 blocks (junk + PKCS padding block)
forged_token = new_iv + ciphertext[16:-32]
```

**Key insight:** AES-CBC decryption has no built-in length integrity. Truncating ciphertext blocks from the end is valid as long as the new last block decrypts to valid PKCS7 padding. Combined with IV manipulation of block 0, this forges arbitrary first-block content.

---

## Padding Oracle to CBC Bitflip Command Injection (BSidesSF 2017)

**Pattern:** Encrypted commands passed via URL parameter. Error messages reveal padding validity (padding oracle). Chain two attacks: (1) Padding oracle recovers the plaintext of the encrypted command. (2) CBC bitflipping modifies a ciphertext block to inject shell metacharacters (`;$(cmd)`) into the decrypted command, achieving RCE through crypto manipulation alone.

```python
# Step 1: Padding oracle recovers plaintext
plaintext = padding_oracle_decrypt(ciphertext, oracle_fn)

# Step 2: CBC bitflip — modify block N-1 to change decrypted block N
target_block = 5
desired = b';$(cat *.txt)   '  # 16 bytes, pad with spaces
original = plaintext[target_block * 16:(target_block + 1) * 16]
ct = bytearray(bytes.fromhex(ciphertext))
for i in range(16):
    ct[(target_block - 1) * 16 + i] ^= original[i] ^ desired[i]
forged = ct.hex()
```

**Key insight:** Padding oracle and CBC bitflipping are usually taught separately. Chaining them converts a pure cryptographic weakness into full command injection: the oracle recovers plaintext needed to compute the XOR mask, and the bitflip injects the payload.

---

## SPN Cipher Partial Key Recovery via S-box Intersection (SharifCTF 7 2016)

**Pattern:** A 3-round substitution-permutation network with 36-bit blocks and 6-bit S-boxes. Attack using chosen-plaintext pairs: for each pair of 6-bit sub-keys (rounds 2 and 3), partially decrypt through the last two rounds and check if the intermediate S-box input matches. Intersecting candidate key sets across ~200 plaintext-ciphertext pairs uniquely identifies each 6-bit sub-key, reducing a 108-bit brute force to six independent 12-bit searches.

```python
def recover_subkeys(pairs, sbox, perm):
    """Recover 6-bit subkeys via intersection across plaintext-ciphertext pairs."""
    for sbox_pos in range(6):  # 6 S-boxes per round
        candidates = None
        for pt, ct in pairs:
            valid = set()
            for k2 in range(64):  # 6-bit subkey round 2
                for k3 in range(64):  # 6-bit subkey round 3
                    # Partial decrypt through rounds 3 and 2
                    intermediate = inv_sbox[ct_bits[sbox_pos] ^ k3]
                    intermediate = inv_perm(intermediate)
                    if inv_sbox[intermediate ^ k2] == expected_from_pt:
                        valid.add((k2, k3))
            candidates = valid if candidates is None else candidates & valid
        assert len(candidates) == 1  # unique key pair
```

**Key insight:** SPN structures allow divide-and-conquer key recovery. Each S-box position can be attacked independently, and the intersection of valid key candidates across multiple plaintext-ciphertext pairs converges to a unique solution.

---

## AES-CFB IV Recovery from Timestamp-Seeded PRNG (SHA2017)

**Pattern:** Ransomware encrypts files with AES-CFB using a hardcoded password from bash_history. The IV is derived from `random.choice()` seeded with `int(time())` at encryption time. The file's mtime (preserved by the filesystem) equals the exact seed used, enabling full decryption without the private key.

```python
import random, os, string, base64
from Crypto.Cipher import AES

password = b'hardcoded_password_from_bash_history'
img = 'encrypted_file.enc'

# File mtime IS the random seed used at encryption time
random.seed(int(os.stat(img).st_mtime))
iv = ''.join(random.choice(string.letters + string.digits) for _ in range(16))

aes = AES.new(password, AES.MODE_CFB, iv.encode())
with open(img, 'rb') as f:
    ciphertext = base64.b64decode(f.read())
plaintext = aes.decrypt(ciphertext)
```

**Key insight:** PRNG seeded with `time()` at encryption time leaks the seed via the filesystem mtime. Always check Python version compatibility — Python 2 and Python 3 have different `random` module implementations producing different sequences from the same seed. The `-it` flag on `cp`/`mv` may reset mtime; work from the original unmodified file.

**References:** SHA2017

---

## Three-Round XOR Protocol Key Cancellation (HITB 2017)

**Pattern:** A custom protocol performs a three-message XOR key exchange:
1. Client sends `c1 = msg XOR clientKey`
2. Server responds `c2 = c1 XOR serverKey`
3. Client sends `c3 = c2 XOR clientKey`

All three ciphertexts are observable in a PCAP or network capture. Computing `c1 XOR c2 XOR c3` directly recovers the original `msg` because all key material cancels:

```python
# c1 = msg ^ clientKey
# c2 = msg ^ clientKey ^ serverKey
# c3 = msg ^ serverKey
# c1 ^ c2 ^ c3 = msg ^ clientKey ^ msg ^ clientKey ^ serverKey ^ msg ^ serverKey
#              = msg   (all keys cancel via XOR)
plaintext = bytes(a ^ b ^ c for a, b, c in zip(c1, c2, c3))
```

**Key insight:** Three-message XOR key exchange where the client applies its key twice creates an algebraic weakness: XOR of all three ciphertexts directly recovers the original message without knowledge of either key. Any protocol where the same key is applied an even number of times is trivially broken.

**References:** HITB 2017

---

## AES-CBC UnicodeDecodeError Side-Channel Oracle (Kaspersky 2017)

**Pattern:** Server decrypts AES-CBC ciphertext and attempts to UTF-8 decode the result. Invalid UTF-8 sequences raise a `UnicodeDecodeError` (or equivalent). This error is distinguishable from other errors (e.g., application-level errors), creating a decryption oracle analogous to a padding oracle.

**Attack:** Standard CBC bit-flip oracle technique, using UTF-8 validity as the distinguisher:
1. For each target plaintext byte at position `i` in block `b`, modify byte `i` in block `b-1`
2. Cycle through all 256 XOR values; when the decrypted byte produces valid UTF-8 in context, the server returns a non-`UnicodeDecodeError` response
3. From the XOR value that passes and the known modification to `c[b-1][i]`, recover `plaintext[b][i]`

```python
# CBC bit-flip oracle using UTF-8 validity
for guess in range(256):
    modified = bytearray(prev_block)
    modified[pos] = known_intermediate[pos] ^ guess  # produce desired output byte
    if not unicode_error(modified_block + target_block):
        plaintext_byte = guess  # valid UTF-8 at this position
        break
```

**Key insight:** Any error that distinguishes valid from invalid plaintext content serves as a decryption oracle — not just PKCS#7 padding errors. UTF-8 validity, base64 decodability, JSON parsability, and ASCII-only constraints are all valid oracle conditions. The only requirement is a server-side distinguishable response.

**References:** Kaspersky CTF 2017

---

## SHA-256 Basis Attack for XOR-Aggregate Hash Bypass (34C3 CTF 2017)

**Pattern:** Find 256 files whose SHA-256 hashes form a basis for Z_2^256. Then for any target hash, compute which subset of basis files XORs to produce the desired hash difference. This breaks systems that verify integrity via `XOR(sha256(file_i)) == expected`.

```python
# 1. Generate ~300 random valid Python files
# 2. Compute SHA-256 of each -> 256-bit vectors over GF(2)
# 3. Gaussian elimination to find 256 linearly independent vectors
# 4. Target: h_new XOR (XOR of sha256(basis_files)) = h_orig
# 5. Solve the linear system to find which basis files to include
from sage.all import GF, matrix
M = matrix(GF(2), [hash_to_bits(sha256(f)) for f in basis_files])
target = hash_to_bits(sha256(malicious_zip)) ^ hash_to_bits(original_hash)
solution = M.solve_left(target)
```

**Key insight:** SHA-256 hashes are 256-bit vectors over GF(2). Given ~256 random hashes, they almost certainly span the full space, meaning you can XOR-combine them to produce any target 256-bit value. This breaks XOR-based aggregate hash verification: if the system checks `XOR(sha256(file_i)) == expected`, you can replace files while maintaining the aggregate. The attack does NOT find SHA-256 collisions -- it exploits the linearity of XOR aggregation over non-linear hash outputs.

**References:** 34C3 CTF 2017

---

### Custom MAC Forgery via XOR Block Cancellation with Key Rotation (PlaidCTF 2018)

**Pattern:** Custom MAC uses AES-ECB with key stream that repeats every 128 blocks. Craft three queries where 2048-byte filler blocks cancel via XOR between queries, leaving only the target command's MAC. (PlaidCTF 2018)

```python
mac1 = fmac("tag " + tag_cmd(cmdline))      # tag AAA...
mac2 = fmac("tag " + expand_cmd(cmdline))    # tag BBB...(2048) + cmd_padded
mac3 = fmac("tag " + expand_cmd(tag_cmd(cmdline)))  # tag BBB...(2048) + tagAAA_padded
forged_mac = mac1 ^ mac2 ^ mac3  # XOR cancellation = fmac(cmdline)
```

**Key insight:** When a MAC's internal key stream repeats periodically, arrange message blocks so that identical blocks at the same key-stream positions cancel via XOR across multiple queries. Three queries suffice to forge any target command's MAC.

---

### Bit-by-Bit HMAC Key Recovery via XOR Plus Addition Arithmetic (Midnight Sun CTF 2018)

**Pattern:** Flawed HMAC computes `sha256((key XOR msg) + msg)` where `+` is bitwise addition (not concatenation). Sending `msg=0` gives `sha256(key)`. For bit position `i`, sending `msg=2^i`: if key bit `i` is set, XOR clears it and addition restores it, giving the same hash. (Midnight Sun CTF 2018)

```python
key_hash = get_digest(b'\x00')  # sha256(key + 0) = sha256(key)
key = 0
for i in range(key_bits):
    digest = get_digest(int_to_bytes(2**i))
    if digest == key_hash:
        key |= (1 << i)  # bit i is set in key
```

**Key insight:** When XOR and addition interact, setting bit `i` in the message XORs it away from the key but adds it back. If key bit `i` was already set, `XOR(1,1)=0` and `0+1=1`, restoring the original value. If key bit `i` was 0, `XOR(0,1)=1` and `1+1=0` with carry, changing the hash. This creates a per-bit oracle.

---

### CBC IV Recovery from Block-2 Known Plaintext (RITSEC 2018)

**Pattern:** AES-CBC given: full ciphertext, known plaintext from block 2 onward, partial key. Recover the missing IV by first brute-forcing missing key bytes via block 2 (which does not depend on the IV), then XOR plaintext[0] with `AES_decrypt(ct[0], K)` to get the IV.

```python
for tail in itertools.product(string.printable, repeat=2):
    K = base_key + ''.join(tail).encode()
    if AES.new(K, AES.MODE_ECB).decrypt(ct)[16:32] == plaintext[16:32]:
        raw = AES.new(K, AES.MODE_ECB).decrypt(ct[:16])
        IV = bytes(a ^ b for a, b in zip(raw, plaintext[:16]))
        break
```

**Key insight:** Block 2 of CBC decrypts with `prev_ct XOR raw_decrypt` where `prev_ct` is from the ciphertext itself — IV-independent. Use it to recover the key first, then XOR back to the IV.

**References:** RITSEC CTF 2018 — Who drew on my program, writeup 12269

---

### Iterated SHA-256 Timing Oracle on Character Match (35C3 2018)

**Pattern:** Server validates password character-by-character, and each correct character triggers an additional `sha256` iterated 9999 times. Correct characters therefore make the server respond ~0.66 s slower. Brute-force each position by timing responses.

```python
for ch in string.printable:
    t = time.time()
    send(prefix + ch)
    dt = time.time() - t
    if dt > baseline + 0.3:
        prefix += ch; break
```

**Key insight:** Any early-exit or variable-work validator using heavy hashing leaks position-by-position through total wall-time. Measure baseline vs. correct-char time, not absolute times.

**References:** 35C3 CTF 2018 — ultra secret, writeup 12820

---

### GF(p) Linear-System AES Key Recovery from PCAP Matrix (35C3 Junior 2018)

**Pattern:** Service sends 40 plaintext/ciphertext pairs over the network. Extract from pcap with tshark, build a 40×40 matrix `A` and vector `b` over `GF(p)`, then solve for the unknown AES round-key bytes.

```python
from sage.all import matrix, GF
A = matrix(GF(p), 40, A_rows)
key = A.solve_right(vector(GF(p), b))
```

Use `tshark -r file.pcap -Y 'data.len>0' -T fields -e data` to dump the packet bytes, parse into rows, feed to Sage.

**Key insight:** Any protocol that reveals multiple "key applied to known input" samples collapses to linear algebra when the transformation is linear (or linear in a subfield). Sage's `solve_right` handles the rest.

**References:** 35C3 Junior CTF 2018 — pretty-linear, writeups 12788, 12789

---

### SHA-1 Length Extension with UTF-8 High-Byte Bypass (OTW Advent 2018)

**Pattern:** Server checks that all appended bytes to a length-extendable SHA-1 MAC are `< 0x80`. Standard `hashpumpy`/`hlextend` output contains `0x80` and padding bytes that fail the check. Rewrite the padding region using valid multi-byte UTF-8 sequences (e.g., `\xc2\x80` → U+0080) that survive the filter but SHA-1 treats identically.

```python
import hlextend
h = hlextend.new('sha1')
forged = h.extend(b';cat flag', b'A'*msg_len, key_len, old_mac)
# Replace any 0x80-0xFF bytes with UTF-8 two-byte equivalents
safe = forged.replace(b'\x80', b'\xc2\x80')
```

**Key insight:** ASCII-only filters can be bypassed by substituting multi-byte Unicode sequences whose byte values stay below `0x80`. Any length-extension attack behind an ASCII validator is still exploitable with UTF-8 creativity.

**References:** OverTheWire Advent Bonanza 2018 — Day 16, writeup 12754

---

### Cross-Session Cube-Root Recovery via CRT (X-MAS 2018)

**Pattern:** Service exposes `m^3 mod N_i` across multiple sessions with different moduli but the same small plaintext. Because `m^3 < N_1 * N_2 * N_3` for small `m`, Chinese Remainder Theorem recovers `m^3` as an integer, then `iroot` gives `m`.

```python
from sympy.ntheory.modular import crt
from gmpy2 import iroot
m_cubed, _ = crt([N1, N2, N3], [c1, c2, c3])
m, exact = iroot(int(m_cubed), 3)
assert exact
```

**Key insight:** Håstad broadcast attack for `e = 3` generalises to any scenario where you see `m^e mod N_i` across enough moduli that `m^e < prod(N_i)`. CRT joins them; integer root extraction finishes.

**References:** X-MAS CTF 2018 — Santa's list 2.0, writeup 12659

---

## CBC Previous-Block Byte Flipping for Cookie Privilege Escalation (picoCTF 2018)

**Pattern (Secured Logon):** Server stores `{"username": "...", "admin": 0, ...}` encrypted with AES-CBC + base64, returned as a cookie. No MAC. To escalate from `admin: 0` to `admin: 1`, locate the byte offset of `'0'` inside some plaintext block `P_{n+1}`, then XOR the corresponding byte in the *previous* ciphertext block `C_n` with `ord('0') ^ ord('1')`. The attacker block `C_n` decrypts to garbage (which may break the preceding JSON field), but the targeted byte in `P_{n+1}` flips cleanly because `P_{n+1} = AES_dec(C_{n+1}) XOR C_n`.

```python
from base64 import b64encode, b64decode

cookie = b64decode(stolen_cookie)              # IV || C1 || C2 || ... (or C0||... )
buf    = bytearray(cookie)

# Block layout for plaintext {'username': '', 'admin': 0, 'password': ''}:
#   block 1: "{'username': '',"      <- will decrypt to garbage after flip
#   block 2: " 'admin': 0, 'pa"      <- target byte 10 (the '0')
#   block 3: "ssword': ''}"

# To flip byte 10 of plaintext block 2, XOR byte 10 of ciphertext block 1.
# With IV-prefixed layout: index = 16 (IV) + 0*16 (C1) + 10 = 26
#   (or 0*16 + 10 = 10 if there is no separate IV prepended)
offset = 10
buf[offset] ^= ord('0') ^ ord('1')             # 0x30 ^ 0x31 = 0x01

forged_cookie = b64encode(bytes(buf)).decode()
```

**Key insight:** In AES-CBC, `P_{n+1} = AES_dec(C_{n+1}) XOR C_n`. Flipping byte `i` of `C_n` flips byte `i` of `P_{n+1}` with zero side effects on `P_{n+1}`, but turns `P_n` (which was `AES_dec(C_n) XOR C_{n-1}`) into pseudo-random garbage. Works whenever the server (a) uses CBC without integrity checks, (b) parses the JSON/cookie leniently enough to tolerate a corrupted earlier block (unknown-key field, ignored garbage, lenient JSON parser), and (c) exposes the block boundary offset of the target byte. Contrast with [AES-CBC IV Bit-Flip (Google CTF 2016)](modern-ciphers-2.md#aes-cbc-iv-bit-flip-authentication-bypass-google-ctf-2016), which targets block 0 by flipping the IV and leaves all later blocks intact.
