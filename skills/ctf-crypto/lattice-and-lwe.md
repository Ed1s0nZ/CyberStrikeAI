# CTF Crypto - Lattice and LWE Attacks

## Table of Contents
- [Quick Triage: Is This a Lattice Problem?](#quick-triage-is-this-a-lattice-problem)
- [Core Tools: LLL, BKZ, Babai, CVP, SVP](#core-tools-lll-bkz-babai-cvp-svp-asis-ctf-finals-2015-ctfzone-2017)
  - [LLL](#lll)
  - [BKZ](#bkz)
  - [Babai nearest plane](#babai-nearest-plane)
  - [CVP vs SVP](#cvp-vs-svp)
- [Hidden Number Problem (HNP): Partial Nonce / Biased Nonce](#hidden-number-problem-hnp-partial-nonce--biased-nonce-nullcon-hackim-2020-ledger-donjon-ctf-2020)
  - [Minimal ECDSA partial-nonce workflow](#minimal-ecdsa-partial-nonce-workflow)
- [LCG and Truncated Output as a Lattice Problem](#lcg-and-truncated-output-as-a-lattice-problem-x-mas-ctf-2018-fwordctf-2020)
  - [Minimal truncated-LCG workflow](#minimal-truncated-lcg-workflow)
- [LWE via Embedding and CVP](#lwe-via-embedding-and-cvp-plaidctf-2016-aero-ctf-2020)
  - [Embedding-style lattice](#embedding-style-lattice)
  - [For ternary or sparse secrets](#for-ternary-or-sparse-secrets)
- [Ring-LWE / Module-LWE Recognition Notes](#ring-lwe--module-lwe-recognition-notes-plaidctf-2016-dicectf-2022)
  - [Flattening Ring-LWE to plain LWE](#flattening-ring-lwe-to-plain-lwe)
- [Orthogonal Lattices: HSSP / AHSSP Style Recovery](#orthogonal-lattices-hssp--ahssp-style-recovery-zer0pts-ctf-2022)
- [Subset Sum / Knapsack via Lattice Reduction](#subset-sum--knapsack-via-lattice-reduction-hitcon-ctf-2017-backdoorctf-2023)
- [Common Failure Modes](#common-failure-modes)
- [Quick Checklist Before You Commit to Lattices](#quick-checklist-before-you-commit-to-lattices)

---

## Quick Triage: Is This a Lattice Problem?

Use lattice tools when the challenge gives you:

- many modular equations plus a promise that the hidden values are small, sparse, or close to each other
- partial leakage of a secret nonce, seed, or state bits
- linear relations with bounded error terms
- vectors or matrices over `Z_q` where the true solution should be unusually short
- a subset-sum or knapsack instance that "looks too structured"

Typical CTF phrasing:

- "high bits of k are known"
- "the error is small"
- "the secret coefficients are in {-1,0,1}"
- "recover seed from truncated outputs"
- "find a short vector"
- "solve noisy linear equations modulo q"

**First question to ask:** what is supposed to be small?

- the secret itself
- the error vector
- the nonce difference
- a subset indicator vector in `{0,1}^n`
- a correction term caused by modular wraparound

That "small thing" is usually what the lattice is trying to expose.

---

## Core Tools: LLL, BKZ, Babai, CVP, SVP (ASIS CTF Finals 2015, CTFZone 2017)

### LLL

Default first move. Fast, easy, often enough for CTF-sized parameters.

Use it when:

- dimensions are moderate
- the hidden vector is very short
- the challenge author clearly expects a standard embedding attack
- you want structure first, exact recovery second

```python
from sage.all import Matrix, ZZ

M = Matrix(ZZ, basis_rows)
R = M.LLL()
print(R[0])
```

### BKZ

Use when LLL almost works but not quite.

- better for harder CVP/SVP instances
- useful when the gap between the target vector and random lattice vectors is small
- in CTFs, `BKZ(block_size=20..35)` is often already enough

```python
R = M.BKZ(block_size=25)
```

### Babai nearest plane

Good for approximate CVP after reduction.

- reduce basis with `LLL` or `BKZ` first
- then apply Babai to recover the nearby vector
- often enough for ternary or small-error LWE

```python
from fpylll import IntegerMatrix, CVP

# After building and reducing the lattice basis:
closest = CVP.babai(B, target)
```

### CVP vs SVP

- **SVP:** "find an unusually short non-zero lattice vector"
- **CVP:** "find the lattice vector closest to a target"

Rule of thumb:

- if you only know "some relation must be very short", think SVP / embedding
- if you already have a target vector and want the nearest valid lattice point, think CVP / Babai

---

## Hidden Number Problem (HNP): Partial Nonce / Biased Nonce (nullcon HackIM 2020, Ledger Donjon CTF 2020)

**Pattern:** signatures or RNG equations leak a few bits of a hidden value `k`, or `k` is sampled from a small / biased range.

This is the classic route from:

- ECDSA partial nonce leakage
- Schnorr biased nonce leakage
- custom congruence systems where only high bits or low bits are known

Generic shape:

`a_i * x + b_i ≡ e_i (mod q)`

where:

- `x` is the secret key
- `e_i` is small or partially known

That "small error" is what turns the problem into a lattice instance.

**When to use:**

- repeated signatures with leaked high bits / low bits of `k`
- same signing scheme with biased or short nonces
- LCG-like recurrence where each output leaks only part of the internal state

**Practical workflow:**

1. normalize all equations so the secret key is the same unknown in every row
2. isolate the bounded error term
3. scale rows so all coordinates have comparable size
4. run `LLL`
5. test the candidate secret against the original equations

Skeleton:

```python
from sage.all import Matrix, ZZ

def build_hnp_lattice(q, coeffs, bounds):
    n = len(coeffs)
    rows = []
    for i in range(n):
        row = [0] * (n + 1)
        row[i] = q
        rows.append(row)

    last = [c for c in coeffs] + [bounds]
    rows.append(last)
    return Matrix(ZZ, rows)
```

**Key insight:** HNP attacks usually do not require a perfect lattice model. In CTFs, once the true secret produces a vector much shorter than random noise, `LLL` often exposes it directly or gets you close enough to brute-force the last few bits.

### Minimal ECDSA partial-nonce workflow

If a challenge leaks the top bits of each nonce `k_i`, write:

`k_i = leaked_i * 2^t + delta_i`

where `delta_i` is small. For ECDSA:

`s_i * k_i - h_i ≡ r_i * d (mod q)`

Substitute the leaked form of `k_i`:

`r_i * d - s_i * delta_i ≡ s_i * leaked_i * 2^t - h_i (mod q)`

Now the unknowns are:

- the private key `d`
- a set of small corrections `delta_i`

That is the lattice hook.

Minimal starter code:

```python
from sage.all import Matrix, ZZ

def build_ecdsa_partial_nonce_lattice(q, rs, ss, hs, leaked, t):
    n = len(rs)
    M = Matrix(ZZ, n + 2, n + 2)

    for i in range(n):
        M[i, i] = q

    for i in range(n):
        M[n, i] = ss[i]
        M[n + 1, i] = (hs[i] - ss[i] * leaked[i] * (1 << t)) % q

    M[n, n] = 1
    M[n + 1, n + 1] = q // (1 << t)
    return M
```

What to do next:

1. build the lattice
2. run `LLL`
3. inspect short rows for a plausible `d`
4. verify `d` against all signatures
5. if one or two bits are off, brute-force the remaining uncertainty

**When this works best:** many signatures, enough leaked bits per nonce, and a single long-term signing key shared across all samples.

---

## LCG and Truncated Output as a Lattice Problem (X-MAS CTF 2018, FwordCTF 2020)

**Pattern:** internal state follows an affine recurrence, but you only see:

- high bits
- low bits
- several states with unknown parameters
- several consecutive outputs plus a small hidden correction

Typical examples:

- unknown seed, known modulus
- known modulus, known `a`, known `b`, only top bits of outputs
- unknown `a`, unknown `b`, several exact or truncated outputs

The trick is to rewrite:

`state_i = observed_i * 2^t + hidden_i`

where `hidden_i` is small. Then the recurrence becomes a modular linear relation in those small hidden values.

**When to use:**

- high-bit leakage from LCG states
- recurrence modulo a large prime
- multiple consecutive outputs
- exact algebra seems messy but every step differs only by a small hidden remainder

**Key insight:** truncated-state recovery is often just HNP wearing different clothes. If the unknown carries per row are small enough, the lattice will expose them.

### Minimal truncated-LCG workflow

Suppose:

`x_{i+1} = a*x_i + b (mod m)`

but the service leaks only the high bits:

`y_i = x_i >> t`

Then write:

`x_i = y_i * 2^t + z_i`

where `z_i` is the hidden low-bit part and is small.

Plugging into the recurrence gives:

`y_{i+1} * 2^t + z_{i+1} ≡ a*(y_i * 2^t + z_i) + b (mod m)`

Rearrange:

`z_{i+1} - a*z_i ≡ a*y_i*2^t + b - y_{i+1}*2^t (mod m)`

Now the unknowns are the small `z_i`. That is exactly the kind of bounded modular relation lattices like.

Minimal starter code:

```python
from sage.all import Matrix, ZZ

def build_truncated_lcg_lattice(m, a, b, ys, t):
    n = len(ys) - 1
    M = Matrix(ZZ, n + 1, n + 1)

    for i in range(n):
        M[i, i] = m

    for i in range(n):
        rhs = (a * ys[i] * (1 << t) + b - ys[i + 1] * (1 << t)) % m
        M[n, i] = rhs

    M[n, n] = 1 << t
    return M
```

What to do next:

1. use several consecutive outputs
2. run `LLL`
3. recover candidate low bits `z_i`
4. reconstruct full states `x_i`
5. verify the recurrence exactly

**When this works best:** modulus is known, leakage is consecutive, and the hidden low part is much smaller than the modulus.

---

## LWE via Embedding and CVP (PlaidCTF 2016, Aero CTF 2020)

**Pattern:** given `A`, `b`, modulus `q`, and the promise:

`b = A*s + e (mod q)`

where:

- `s` is small or sparse
- `e` is small

This is the standard LWE shape.

**Immediate checks:**

- are coefficients of `s` in `{-1,0,1}` or a tiny range?
- is the error noticeably smaller than `q`?
- does the challenge give many rows and only a few columns?
- does solving over the integers almost work except for modular wraparound?

### Embedding-style lattice

```python
from sage.all import Matrix, ZZ, identity_matrix, zero_matrix, block_matrix

def lwe_embedding(A, q):
    m, n = A.nrows(), A.ncols()
    top = block_matrix([[q * identity_matrix(m), zero_matrix(ZZ, m, n)]])
    bottom = block_matrix([[A.transpose(), identity_matrix(n)]])
    return block_matrix([[top], [bottom]])
```

Then:

- reduce the basis
- use Babai / nearest-plane on the target
- recover the short secret / error pair

### For ternary or sparse secrets

After CVP:

- map near-zero values back into `{-1,0,1}`
- test both endian choices
- test both "row vectors" and "column vectors" conventions

**Key insight:** many CTF LWE instances are intentionally below the "real cryptography" hardness line. The challenge is usually not defeating production-grade LWE, but noticing that the secret or error was chosen tiny enough for `LLL + Babai` to work.

---

## Ring-LWE / Module-LWE Recognition Notes (PlaidCTF 2016, DiceCTF 2022)

You should suspect Ring-LWE / Module-LWE when:

- objects are polynomials modulo `x^n ± 1`
- multiplication is cyclic or negacyclic convolution
- samples look like `(a(x), b(x)=a(x)s(x)+e(x))`
- coefficients are reduced modulo `q`

In many CTFs, the intended shortcut is not a full Ring-LWE attack, but one of these:

- coefficients are tiny enough to lift to integers directly
- the ring structure decouples into easier scalar problems
- the service leaks enough evaluations to turn the problem into plain LWE
- one representation bug breaks the intended hardness

**Practical advice:**

- first try to flatten the polynomial problem into vectors
- test coefficient embedding before chasing deeper algebra
- check whether NTT / inverse NTT is used incorrectly
- check sign conventions, endian order, and whether coefficients were centered into `[-q/2, q/2]`

### Flattening Ring-LWE to plain LWE

```python
from sage.all import Matrix, ZZ, vector

def ring_lwe_to_matrix(a_poly, n, q):
    """Flatten a(x) in Z_q[x]/(x^n+1) to its negacyclic rotation matrix."""
    coeffs = list(a_poly) + [0] * (n - len(list(a_poly)))
    rows = []
    for i in range(n):
        row = [0] * n
        for j in range(n):
            idx = (i - j) % n
            sign = -1 if (i - j) < 0 and ((i - j) % n) != 0 else 1
            # negacyclic: x^n = -1
            if j <= i:
                row[j] = coeffs[i - j]
            else:
                row[j] = -coeffs[n + i - j]
        rows.append(row)
    return Matrix(ZZ, rows)
# After flattening, treat as plain LWE: b_vec = A_mat * s_vec + e_vec (mod q)
```

**Key insight:** most Ring-LWE / Module-LWE CTF challenges are weakened by implementation mistakes, tiny errors, or over-structured secrets. Flatten to plain LWE first and check whether standard lattice tools solve it before pursuing ring-specific attacks.

---

## Orthogonal Lattices: HSSP / AHSSP Style Recovery (zer0pts CTF 2022)

**Pattern:** you do not directly know the secret matrix or subset, but you can construct vectors that should be orthogonal to it modulo `M` or `p`.

This often appears in hidden-subset style problems:

- recover a hidden binary matrix
- recover a hidden low-weight subspace
- reconstruct unknown rows from modular inner-product relations

Core workflow:

1. build a lattice whose short vectors represent orthogonal relations
2. reduce it
3. recover the orthogonal lattice
4. take the kernel / orthogonal complement
5. reduce again to expose the hidden binary or short basis

```python
from sage.all import Matrix, ZZ, identity_matrix, block_matrix

def orthogonal_lattice_recovery(H, M):
    """Recover hidden binary basis from h = alpha * A (mod M).

    H: observed matrix (k x n) over Z_M
    M: modulus
    Returns: LLL-reduced orthogonal lattice whose kernel reveals A.
    """
    k, n = H.nrows(), H.ncols()
    # Build lattice: [M*I_k | 0; H^T | I_n]
    top = block_matrix([[M * identity_matrix(k), Matrix(ZZ, k, n)]])
    bot = block_matrix([[H.change_ring(ZZ).transpose(), identity_matrix(n)]])
    L = block_matrix([[top], [bot]])
    L_reduced = L.LLL()
    # Short rows in the bottom-right block are orthogonal to the hidden basis
    return L_reduced
```

**When to use:**

- challenge gives `h = αA` or affine variants of that relation
- unknown matrix entries are in `{0,1}` or another tiny alphabet
- direct solving fails because the structure lives in an unknown subspace

**Key insight:** in these problems, the shortest vectors are not the answer itself. They are the doorway to the answer: first recover the orthogonal space, then turn back and reconstruct the hidden basis.

---

## Subset Sum / Knapsack via Lattice Reduction (HITCON CTF 2017, BackdoorCTF 2023)

**Pattern:** recover a binary vector `x_i ∈ {0,1}` such that:

`sum(a_i * x_i) = target`

This is the classic subset-sum / knapsack lattice setup.

Use it when:

- the instance is intentionally low-density
- the hidden vector is binary
- direct meet-in-the-middle is still too large

Skeleton:

```python
from sage.all import Matrix, ZZ

def knapsack_lattice(weights, target):
    n = len(weights)
    M = Matrix(ZZ, n + 1, n + 1)
    for i in range(n):
        M[i, i] = 1
        M[i, n] = weights[i]
    M[n, n] = -target
    return M
```

Then:

- run `LLL`
- look for a row whose last coordinate is `0`
- check whether the remaining coordinates are in `{0,1}` or `{−1,0,1}`

**Key insight:** the lattice is built so that the correct subset produces a vector with an abnormally small final coordinate. In easy CTF instances, that vector survives reduction.

---

## Common Failure Modes

- **Wrong scaling:** one coordinate dominates the basis and hides the short vector.
- **Wrong centering:** values should be mapped to `[-q/2, q/2]`, not kept in `[0, q)`.
- **Wrong orientation:** rows vs columns are swapped.
- **Too few samples:** the lattice exists, but not enough equations pin the secret down.
- **Noise too large:** `LLL` is not enough; try `BKZ`, better scaling, or a different embedding.
- **Mistaken problem type:** what looks like LWE may actually be plain linear algebra, CRT, or a bugged encoding problem.
- **Forgot brute-force finish:** lattice often gets you "almost correct"; the last few bits or signs may still need a tiny brute force.

---

## Quick Checklist Before You Commit to Lattices

- Can I write the unknown as "small secret" or "small error"?
- Is there a bounded term that should make one vector much shorter than random?
- Did I try centering coefficients?
- Did I test both row/column conventions?
- Did I try `LLL` first before building something more exotic?
- If `LLL` almost works, did I try `BKZ` or Babai?
- If the instance is polynomial-based, did I first flatten it into coefficient vectors?

If most answers are "yes", the challenge is very likely meant to be solved with lattice reduction.
