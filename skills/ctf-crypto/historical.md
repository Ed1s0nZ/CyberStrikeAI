# CTF Crypto - Historical Ciphers

## Table of Contents
- [Lorenz SZ40/42 (Tunny) Cipher](#lorenz-sz4042-tunny-cipher)
- [Book Cipher Brute Force (Nullcon 2026)](#book-cipher-brute-force-nullcon-2026)

---

## Lorenz SZ40/42 (Tunny) Cipher

The Lorenz cipher uses 12 wheels to encrypt 5-bit ITA2/Baudot characters. With known plaintext, a structured attack recovers all wheel settings.

**Machine structure:**
- 5 χ (chi) wheels: periods 41, 31, 29, 26, 23 — advance every step
- 5 Ψ (psi) wheels: periods 43, 47, 51, 53, 59 — advance only when μ37=1
- μ61 wheel: period 61 — advances every step, controls μ37 stepping
- μ37 wheel: period 37 — advances only when μ61=1, controls Ψ stepping

**Encryption:** `ciphertext[i] = plaintext[i] XOR chi[i] XOR psi[i]` (per 5-bit character)

**CRITICAL: The delta (Δ) approach is the fundamental technique:**

```python
# Step 1: Get keystream from known plaintext
key_stream = [pt[i] ^ ct[i] for i in range(N)]

# Step 2: Compute delta keystream (THE key insight)
delta_k = [key_stream[i] ^ key_stream[i+1] for i in range(N-1)]
# delta_k = delta_chi XOR delta_psi
# Since psi only moves ~25% of the time, delta_k BIASES toward delta_chi

# Step 3: Recover delta_chi via majority vote at each wheel position
# Assume wheels start at position 1
for bit in range(5):
    P = chi_periods[bit]  # [41, 31, 29, 26, 23]
    delta_chi = []
    for phase in range(P):
        # Collect all delta_k values at this wheel phase
        vals = [delta_k_bit[i] for i in range(phase, len(delta_k_bit), P)]
        delta_chi.append(1 if sum(vals) > len(vals)/2 else 0)

# Step 4: Integrate delta_chi to get chi (2 candidates per wheel, start 0 or 1)
chi = [start]  # start = 0 or 1
for i in range(P-1):
    chi.append(chi[-1] ^ delta_chi[i])
# Circular consistency: chi[0] ^ chi[-1] should equal delta_chi[P-1]

# Step 5: Subtract chi from keystream to get psi contribution
# Identify when psi steps: delta_psi = delta_k XOR delta_chi
# When ALL 5 bits of delta_psi are 0 → μ37 was off (psi didn't step)
# (Statistically very rare for all 5 cams to not change when stepping)

# Step 6: From stepping pattern, determine μ61 (period 61)
# μ61[pos] = 1 when we see psi resume stepping after being stopped

# Step 7: Cross-reference to get μ37 (period 37)
# μ37 position advances only when μ61=1

# Step 8: Determine psi wheels from delta_psi values when stepping occurs
# Look for repeating patterns with periods 43, 47, 51, 53, 59

# Step 9: Brute force remaining ambiguity
# Total candidates: 2^5 (chi) × 2^5 (psi) × 61×37 (μ positions) = 2,313,472
# Trivially brutable - decrypt and check if known plaintext matches
```

**Common mistakes to avoid:**
- Do NOT assume psi is "period 2" or just alternating — it has real wheels with periods 43-59
- Do NOT spend time on statistical period-finding for the motor — just use the structured Δ approach
- Do NOT try LFSR analysis on the step sequence — the stepping is from mechanical wheels, not LFSRs
- The "step rate" (~35%) is a consequence of μ37 being on ~50% and μ61 on ~50% = ~25% psi stepping
- Always assume standard wheel periods unless evidence says otherwise
- Total brute force space is tiny (<3M) — don't over-optimize

**ITA2/Baudot encoding (5-bit):**
```python
# Standard ITA2 mapping used in Lorenz challenges
char_to_code = {
    'A': 24, 'B': 19, 'C': 14, 'D': 18, 'E': 16, 'F': 22, 'G': 11,
    'H': 5,  'I': 12, 'J': 26, 'K': 30, 'L': 9,  'M': 7,  'N': 6,
    'O': 3,  'P': 13, 'Q': 29, 'R': 10, 'S': 20, 'T': 1,  'U': 28,
    'V': 15, 'W': 25, 'X': 23, 'Y': 21, 'Z': 17,
    '9': 4,  '5': 27, '8': 31, '3': 8,  '4': 2,  '/': 0,
}
# Code 27 = FIGS shift, Code 31 = LTRS shift
```

---

## Book Cipher Brute Force (Nullcon 2026)

**Pattern (Booking Key):** Book cipher encodes password as list of "steps forward" in reference text.

**Key insight:** Charset constraint drastically reduces candidate starting positions:
```python
def decode_book_cipher(cipher_distances, book_text, valid_chars):
    """Brute-force starting position; filter by charset."""
    candidates = []
    for start_key in range(len(book_text)):
        pos = start_key
        password = []
        valid = True
        for dist in cipher_distances:
            pos = (pos + dist) % len(book_text)
            ch = book_text[pos]
            if ch not in valid_chars:
                valid = False
                break
            password.append(ch)
        if valid:
            candidates.append((start_key, ''.join(password)))
    return candidates  # Typically 3-4 candidates out of ~56k positions
```
