# CTF Misc - Games, VMs & Constraint Solving (Part 4)

Additional CTF-era challenges extracted from 2018+ writeups. For earlier parts, see [games-and-vms.md](games-and-vms.md), [games-and-vms-2.md](games-and-vms-2.md), and [games-and-vms-3.md](games-and-vms-3.md).

## Table of Contents
- [XSLT as Turing-Complete VM for Binary Search (35C3 2018)](#xslt-as-turing-complete-vm-for-binary-search-35c3-2018)
- [JavaScript MAX_SAFE_INTEGER Successor Equality (35C3 2018)](#javascript-max_safe_integer-successor-equality-35c3-2018)
- [Binary Search Oracle in Comparison-Only DSL (35C3 2018)](#binary-search-oracle-in-comparison-only-dsl-35c3-2018)
- [Blind SQLi via Script-Engine Timeout Error (35C3 2018)](#blind-sqli-via-script-engine-timeout-error-35c3-2018)
- [OEIS Sequence Lookup Automation for Recurrence Puzzles (X-MAS CTF 2018)](#oeis-sequence-lookup-automation-for-recurrence-puzzles-x-mas-ctf-2018)
- [QR Code Reassembly from Format-String Structural Constraints (Square CTF 2018)](#qr-code-reassembly-from-format-string-structural-constraints-square-ctf-2018)
- [Matrix Exponentiation for Fibonacci-Like Recurrence (Pwn2Win 2018)](#matrix-exponentiation-for-fibonacci-like-recurrence-pwn2win-2018)
- [Tribonacci Recurrence for Frog Jump Counting (FireShell 2019)](#tribonacci-recurrence-for-frog-jump-counting-fireshell-2019)
- [Selenium + Tesseract for Dynamic Font CAPTCHA (Square CTF 2018)](#selenium--tesseract-for-dynamic-font-captcha-square-ctf-2018)
- [Brainfuck Decodes Piet Image URL — Multi-Layer Polyglot (RITSEC 2018)](#brainfuck-decodes-piet-image-url--multi-layer-polyglot-ritsec-2018)
- [Bytebeat Synth Code Recognition for Hidden Audio (RITSEC 2018)](#bytebeat-synth-code-recognition-for-hidden-audio-ritsec-2018)

---

## XSLT as Turing-Complete VM for Binary Search (35C3 2018)

**Pattern:** Challenge only executes XSLT templates. `<xsl:choose>`, `<xsl:call-template>` with recursion, and `<xsl:variable>` form a full Turing-complete runtime with a stack. Encode a binary-search oracle: `<drinks>` elements hold the stack, `<plate>` elements are instructions, `<course>` blocks act as labels.

```xml
<xsl:template name="step">
  <xsl:param name="lo"/><xsl:param name="hi"/>
  <xsl:variable name="mid" select="($lo + $hi) div 2"/>
  <xsl:choose>
    <xsl:when test="$target = $mid">...found...</xsl:when>
    <xsl:when test="$target &lt; $mid">
      <xsl:call-template name="step">
        <xsl:with-param name="lo" select="$lo"/>
        <xsl:with-param name="hi" select="$mid"/>
      </xsl:call-template>
    </xsl:when>
    ...
  </xsl:choose>
</xsl:template>
```

**Key insight:** Any "pure template" language with named recursion and conditionals is a VM. Build a primitive (binary search, bit extraction, state accumulator) out of its native constructs before trying to escape the sandbox.

**References:** 35C3 CTF 2018 — Juggle, writeup 12803

---

## JavaScript MAX_SAFE_INTEGER Successor Equality (35C3 2018)

**Pattern:** Challenge asserts `x !== x + 1`. For `x = Number.MAX_SAFE_INTEGER + 1 === 9007199254740992`, IEEE 754 rounding makes `x + 1 === x` true, so the assertion passes and the check is bypassed.

```js
let x = 9007199254740992; // 2^53
console.log(x === x + 1); // true
```

**Key insight:** Any numeric invariant that compares `n` to `n + 1` fails at the float boundary. Test with `2^53`, `Infinity`, `NaN`, and `-0 === 0` combinations when a JS check looks like it's making assumptions about arithmetic.

**References:** 35C3 CTF 2018 — Number Error, writeup 12828

---

## Binary Search Oracle in Comparison-Only DSL (35C3 2018)

**Pattern:** Challenge DSL only exposes comparisons against a secret value. Convert it into a full oracle by subtracting decreasing powers of two (`2^30, 2^29, ..., 2^0`) from an initial guess, adding whenever the comparison reports "less than" and subtracting when "greater than".

```python
guess = 0
for shift in range(30, -1, -1):
    guess += 1 << shift
    if oracle(guess) > 0:     # guess too high
        guess -= 1 << shift
```

**Key insight:** Any boolean comparator gives you binary search in `O(log N)` queries. The same trick collapses any comparison-based leak — regex match, timing channel, HTTP status code — into the full value.

**References:** 35C3 CTF 2018 — Juggle, writeup 12803

---

## Blind SQLi via Script-Engine Timeout Error (35C3 2018)

**Pattern:** Server evaluates `eval` of a user-supplied snippet with a tight timeout. Wrap the payload in `if charAt(FLAG, pos) == '?' then pause(10000) end` — correct characters hang until the timeout triggers an error; wrong characters return instantly. Treat the timeout as a truthy bit.

```lua
-- blind timing oracle in Lua eval sandbox
for c in printable do
    send(("if charAt(FLAG, %d) == '%s' then pause(10000) end"):format(i, c))
    if response_time > 5 then flag = flag .. c; break end
end
```

**Key insight:** Script-eval services with timeouts are stateful oracles: any long-running expression leaks a boolean via the wall-clock difference between timeout and instant return.

**References:** 35C3 CTF 2018 — dev/null, writeups 12830, 12871

---

## OEIS Sequence Lookup Automation for Recurrence Puzzles (X-MAS CTF 2018)

**Pattern:** Server asks for the next term in a mathematical sequence. Automate the lookup: query https://oeis.org/search?q=1,1,2,5,14, parse the first result with pyquery, extract the `Next term`, send it back. Wrap around a MD5 captcha brute force for PoW-protected services.

```python
import requests
from pyquery import PyQuery as pq
r = requests.get('https://oeis.org/search', params={'q': ','.join(map(str, seq))})
doc = pq(r.text)
next_term = doc('pre').eq(1).text().split(',')[len(seq)]
```

**Key insight:** Any integer-sequence puzzle is solved in one HTTP request via OEIS. The hard part is the wrapper (captcha, PoW, socket framing) — automate that once and the math stops being the bottleneck.

**References:** X-MAS CTF 2018 — A Weird List of Sequences, writeup 12683

---

## QR Code Reassembly from Format-String Structural Constraints (Square CTF 2018)

**Pattern:** Challenge ships shredded 1-pixel columns of a QR code. Instead of brute-forcing `21!` permutations, anchor on QR invariants: the three finder patterns, the timing pattern between them, the fixed dark module, and the 15-bit format string at column 8 has only 32 valid values (EC level × mask pattern). Filter slices by structural constraints, then permute only the remaining few.

```python
wanted_formats = load_32_valid_qr_formats()
for col in slices:
    if col[:7] in wanted_formats_column_8:
        candidate_cols.append(col)
for perm in itertools.permutations(candidate_cols):
    if decode_qr(np.stack(perm)):
        return perm
```

**Key insight:** Format-specific constraints collapse permutation spaces. QR Version 1 has only 32 possible format strings; anchor on them to prune before brute-forcing.

**References:** Square CTF 2018 — C3: Shredded, writeup 12331

---

## Matrix Exponentiation for Fibonacci-Like Recurrence (Pwn2Win 2018)

**Pattern:** Challenge asks for the `N`-th term of a recurrence `a_{n+1} = f(a_n, a_{n-1})` with `N` up to `10^12`. Naive iteration is impossible. Write the update as a 2×2 matrix product `[a_{n+1}; a_n] = M * [a_n; a_{n-1}]` and compute `M^N` in `O(log N)` with binary exponentiation.

```python
MOD = 10**9 + 7
def matmult(a, b):
    return ((a[0]*b[0] + a[1]*b[2]) % MOD, (a[0]*b[1] + a[1]*b[3]) % MOD,
            (a[2]*b[0] + a[3]*b[2]) % MOD, (a[2]*b[1] + a[3]*b[3]) % MOD)
def matpow(M, n):
    R = (1,0,0,1)
    while n:
        if n & 1: R = matmult(R, M)
        M = matmult(M, M); n >>= 1
    return R
```

**Key insight:** Any linear recurrence over a ring is reducible to matrix exponentiation. Use it whenever the challenge exposes a giant `N` for a classical-looking sequence — Fibonacci, Tribonacci, Lucas, linear Pisano, RNG counters.

**References:** Pwn2Win CTF 2018 — Too Slow, writeup 12501

---

## Tribonacci Recurrence for Frog Jump Counting (FireShell 2019)

**Pattern:** A proof-of-work handshake asks how many ways a frog can reach step `N` if it can jump 1, 2, or 3 steps. That is `f(N) = f(N-1) + f(N-2) + f(N-3)` — the Tribonacci sequence. Precompute modulo the server's modulus; for large `N`, combine with matrix exponentiation above.

```python
def tribonacci(N, MOD=13371337):
    a, b, c = 0, 0, 1
    for _ in range(N):
        a, b, c = b, c, (a + b + c) % MOD
    return c
```

**Key insight:** "Number of ways to climb N stairs with step sizes {1..k}" is always a linear recurrence. Memoize up to the server's max `N`, cache across requests, and keep the tribonacci identity in mind when the challenge text mentions "frog".

**References:** FireShell CTF 2019 — Frogs, writeup 12961

---

## Selenium + Tesseract for Dynamic Font CAPTCHA (Square CTF 2018)

**Pattern:** A CAPTCHA generates math expressions with a random glyph font and rerenders every 5 seconds. Full-window screenshots via Selenium feed Tesseract OCR; clean up Tesseract's common confusions (`x`→`*`, `{`→`(`) before `eval()`.

```python
from selenium import webdriver
from PIL import Image
import pytesseract, io
d = webdriver.Chrome()
d.get(URL); d.execute_script("document.body.style.zoom='450%'")
img = Image.open(io.BytesIO(d.get_screenshot_as_png()))
expr = pytesseract.image_to_string(img).replace('x','*').replace('{','(').replace('}',')')
d.execute_script(f"document.getElementsByName('answer')[0].value={eval(expr)}")
d.find_element_by_tag_name('form').submit()
```

**Key insight:** Dynamic CAPTCHAs are often too short-lived for manual solves but trivial for a 1-second Selenium + Tesseract loop. When OCR alone fails, pair it with a cmap reference library (see ctf-osint/web-and-dns.md).

**References:** Square CTF 2018 — C8, writeups 12160, 12178

---

## Brainfuck Decodes Piet Image URL — Multi-Layer Polyglot (RITSEC 2018)

**Pattern:** Recognise the three most common esolangs stacked together: Brainfuck source outputs a YouTube URL, the video's thumbnail border is a Piet program whose execution prints the flag. Use `bf` → `yt-dlp` → strip border pixels → `npiet` pipeline.

```bash
bf puzzle.bf                          # prints youtube.com/watch?v=XXXX
yt-dlp -x --write-thumbnail "$URL"    # grabs JPG thumbnail
python crop_border.py thumb.jpg > piet.png
npiet piet.png                        # prints the flag
```

**Key insight:** Multi-layer esolangs are recognisable by eye: Brainfuck is `+-<>.,[]`, Piet is colored block grids, Whitespace is invisible. If a challenge description mentions multiple "weird" formats, pipeline the decoders in order.

**References:** RITSEC CTF 2018 — writeup 12224

---

## Bytebeat Synth Code Recognition for Hidden Audio (RITSEC 2018)

**Pattern:** A short C-like one-liner is bytebeat — a generative music format where `t` is a monotonic sample counter. Paste into an online interpreter (http://wry.me/bytebeat/) to hear it; the resulting tune is a recognizable song whose title is the flag.

```c
/* Bytebeat example: output byte = low 8 bits of this expression */
(t * ((t >> 12 | t >> 8) & 63 & t >> 4))
```

**Key insight:** Recognise bytebeat by (a) a `t` variable, (b) bitshifts mixed with modulo, (c) output of size 8-bit unsigned integer. `%`, `|`, `&`, `^`, `>>`, `<<` on `t` are the bytebeat signature. No decoding needed — just play it.

**References:** RITSEC CTF 2018 — writeups 12261, 12268

---
