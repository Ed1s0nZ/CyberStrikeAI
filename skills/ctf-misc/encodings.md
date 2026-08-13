# CTF Misc - Encodings & Media

## Table of Contents
- [Common Encodings](#common-encodings)
  - [Base64](#base64)
  - [Base32](#base32)
  - [Hex](#hex)
  - [IEEE 754 Floating Point Encoding](#ieee-754-floating-point-encoding)
  - [UTF-16 Endianness Reversal (LACTF 2026)](#utf-16-endianness-reversal-lactf-2026)
  - [BCD (Binary-Coded Decimal) Encoding (VuwCTF 2025)](#bcd-binary-coded-decimal-encoding-vuwctf-2025)
  - [Multi-Layer Encoding Detection (0xFun 2026)](#multi-layer-encoding-detection-0xfun-2026)
  - [URL Encoding](#url-encoding)
  - [ROT13 / Caesar](#rot13--caesar)
  - [Caesar Brute Force](#caesar-brute-force)
- [QR Codes](#qr-codes)
  - [Basic Commands](#basic-commands)
  - [QR Structure](#qr-structure)
  - [Repairing Damaged QR](#repairing-damaged-qr)
  - [Finder Pattern Template](#finder-pattern-template)
  - [QR Code Chunk Reassembly (LACTF 2026)](#qr-code-chunk-reassembly-lactf-2026)
  - [QR Code Chunk Reassembly via Indexed Directories (UTCTF 2026)](#qr-code-chunk-reassembly-via-indexed-directories-utctf-2026)
- [Multi-Stage URL Encoding Chain (UTCTF 2026)](#multi-stage-url-encoding-chain-utctf-2026)
- [Esoteric Languages](#esoteric-languages)
  - [Whitespace Language Parser (BYPASS CTF 2025)](#whitespace-language-parser-bypass-ctf-2025)
  - [Custom Brainfuck Variants (Themed Esolangs)](#custom-brainfuck-variants-themed-esolangs)
  - [Multi-Layer Esoteric Language Chains (Break In 2016)](#multi-layer-esoteric-language-chains-break-in-2016)
- [base65536 CJK Unicode Binary Encoding (IceCTF 2018)](#base65536-cjk-unicode-binary-encoding-icectf-2018)

See also: [encodings-advanced.md](encodings-advanced.md) - Verilog/HDL, Gray code, binary tree encoding, RTF custom tags, SMS PDU decoding, multi-encoding solvers, UTF-9, pixel binary encoding, hex Sudoku + QR, TOPKEK, MaxiCode

---

## Common Encodings

### Base64
```bash
echo "encoded" | base64 -d
# Charset: A-Za-z0-9+/=
```

### Base32
```bash
echo "OBUWG32DKRDHWMLUL53TI43OG5PWQNDSMRPXK3TSGR3DG3BRNY4V65DIGNPW2MDCGFWDGX3DGBSDG7I=" | base32 -d
# Charset: A-Z2-7= (no lowercase, no 0,1,8,9)
```

### Hex
```bash
echo "68656c6c6f" | xxd -r -p
```

### IEEE 754 Floating Point Encoding

Numbers that encode ASCII text when viewed as raw IEEE 754 bytes:

```python
import struct

values = [240600592, 212.2753143310547, 2.7884192016691608e+23]

# Each float32 packs to 4 ASCII bytes
for v in values:
    packed = struct.pack('>f', v)  # Big-endian single precision
    print(f"{v} -> {packed}")      # b'Meta', b'CTF{', b'fl04'

# For double precision (8 bytes per value):
# struct.pack('>d', v)
```

**Key insight:** If challenge gives a list of numbers (mix of integers, decimals, scientific notation), try packing each as IEEE 754 float32 (`struct.pack('>f', v)`) — the 4 bytes often spell ASCII text.

### UTF-16 Endianness Reversal (LACTF 2026)

**Pattern (endians):** Text "turned to Japanese" -- mojibake from UTF-16 endianness mismatch.

**Fix:** Reverse the encoding/decoding order:
```python
# If encoded as UTF-16-LE but decoded as UTF-16-BE:
fixed = mojibake.encode('utf-16-be').decode('utf-16-le')

# If encoded as UTF-16-BE but decoded as UTF-16-LE:
fixed = mojibake.encode('utf-16-le').decode('utf-16-be')
```

**Identification:** Text appears as CJK characters (Japanese/Chinese), challenge mentions "translation" or "endian".

### BCD (Binary-Coded Decimal) Encoding (VuwCTF 2025)

**Pattern:** Challenge name hints at ratio (e.g., "1.5x" = 1.5:1 byte ratio). Each nibble encodes one decimal digit.

```python
def bcd_decode(data):
    """Decode BCD: each byte = 2 decimal digits."""
    return ''.join(f'{(b>>4)&0xf}{b&0xf}' for b in data)

# Then convert decimal string to ASCII
ascii_text = ''.join(chr(int(decoded[i:i+2])) for i in range(0, len(decoded), 2))
```

### Multi-Layer Encoding Detection (0xFun 2026)

**Pattern (139 steps):** Recursive decoding with troll flags as decoys.

**Critical rule:** When data is all hex chars (0-9, a-f), decode as **hex FIRST**, not base64 (which also accepts those chars).

```python
def auto_decode(data):
    while True:
        data = data.strip()
        if data.startswith('REAL_DATA_FOLLOWS:'):
            data = data.split(':', 1)[1]
        # Prioritize hex when ambiguous
        if all(c in '0123456789abcdefABCDEF' for c in data) and len(data) % 2 == 0:
            data = bytes.fromhex(data).decode('ascii', errors='replace')
        elif set(data) <= set('ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/='):
            data = base64.b64decode(data).decode('ascii', errors='replace')
        else:
            break
    return data
```

**Ignore troll flags** — check for "keep decoding" or "REAL_DATA_FOLLOWS:" markers.

### URL Encoding
```python
import urllib.parse
urllib.parse.unquote('hello%20world')
```

### ROT13 / Caesar
```bash
echo "uryyb" | tr 'a-zA-Z' 'n-za-mN-ZA-M'
```

**ROT13 patterns:** `gur` = "the", `synt` = "flag"

### Caesar Brute Force
```python
text = "Khoor Zruog"
for shift in range(26):
    decoded = ''.join(
        chr((ord(c) - 65 - shift) % 26 + 65) if c.isupper()
        else chr((ord(c) - 97 - shift) % 26 + 97) if c.islower()
        else c for c in text)
    print(f"{shift:2d}: {decoded}")
```

---

## QR Codes

### Basic Commands
```bash
zbarimg qrcode.png           # Decode
zbarimg -S*.enable qr.png    # All barcode types
qrencode -o out.png "data"   # Encode
```

### QR Structure

**Finder patterns (3 corners):** 7x7 modules at top-left, top-right, bottom-left

**Version formula:** `(version * 4) + 17` modules per side

### Repairing Damaged QR

```python
from PIL import Image
import numpy as np

img = Image.open('damaged_qr.png')
arr = np.array(img)

# Convert to binary
gray = np.mean(arr, axis=2)
binary = (gray < 128).astype(int)

# Find QR bounds
rows = np.any(binary, axis=1)
cols = np.any(binary, axis=0)
rmin, rmax = np.where(rows)[0][[0, -1]]
cmin, cmax = np.where(cols)[0][[0, -1]]

# Check finder patterns
qr = binary[rmin:rmax+1, cmin:cmax+1]
print("Top-left:", qr[0:7, 0:7].sum())  # Should be ~25
```

### Finder Pattern Template
```python
finder_pattern = [
    [1,1,1,1,1,1,1],
    [1,0,0,0,0,0,1],
    [1,0,1,1,1,0,1],
    [1,0,1,1,1,0,1],
    [1,0,1,1,1,0,1],
    [1,0,0,0,0,0,1],
    [1,1,1,1,1,1,1],
]
```

### QR Code Chunk Reassembly (LACTF 2026)

**Pattern (error-correction):** QR code split into grid of chunks (e.g., 5x5 of 9x9 pixels), shuffled.

**Solving approach:**
1. **Fix known chunks:** Use structural patterns -- finder patterns (3 corners), timing patterns, alignment patterns -- to place ~50% of chunks
2. **Extract codeword constraints:** For each candidate payload length, use QR spec to identify which pixels are invariant across encodings
3. **Backtracking search:** Assign remaining chunks under pixel constraints until QR decodes successfully

**Tools:** `segno` (Python QR library), `zbarimg` for decoding.

### QR Code Chunk Reassembly via Indexed Directories (UTCTF 2026)

**Pattern (QRecreate):** QR code split into numbered chunks stored in separate directories. Directory names encode the chunk index as base64 (e.g., `MDAx` → `001` → index 1).

**Solving approach:**
1. Decode each directory name from base64 to get the numeric index
2. Sort chunks by decoded index
3. Arrange in a grid (e.g., 100 chunks → 10x10) and stitch into a single image
4. Decode the reconstructed QR code

```python
import os, base64, math
from PIL import Image

# 1. Decode directory names to get indices
chunks = []
for dirname in os.listdir('chunks/'):
    index = int(base64.b64decode(dirname).decode())
    tile = Image.open(f'chunks/{dirname}/tile.png')
    chunks.append((index, tile))

# 2. Sort by index and arrange in grid
chunks.sort(key=lambda x: x[0])
n = len(chunks)
side = int(math.isqrt(n))
tile_w, tile_h = chunks[0][1].size

canvas = Image.new("RGB", (side * tile_w, side * tile_h), (255, 255, 255))
for i, (_, tile) in enumerate(chunks):
    r, c = divmod(i, side)
    canvas.paste(tile, (c * tile_w, r * tile_h))

canvas.save('reconstructed_qr.png')
# 3. Decode with zbarimg or pyzbar
```

**Key insight:** Unlike the LACTF variant (shuffled chunks requiring structural analysis), indexed chunks just need sorting. The challenge is recognizing that directory names are base64-encoded indices. Check `base64 -d` on folder names when they look like random strings.

---

## Multi-Stage URL Encoding Chain (UTCTF 2026)

**Pattern (Breadcrumbs):** Flag is hidden behind a chain of URLs, each encoded differently. Follow the breadcrumbs across external resources (GitHub Gists, Pastebin, etc.), decoding at each hop.

**Common encoding layers per hop:**
1. **Base64** → URL to next resource
2. **Hex** → URL to next resource (e.g., `68747470733a2f2f...` = `https://...`)
3. **ROT13** → final flag

**Decoding workflow:**
```python
import base64, codecs

# Hop 1: Base64
hop1 = "aHR0cHM6Ly9naXN0Lmdp..."
url2 = base64.b64decode(hop1).decode()

# Hop 2: Hex-encoded URL
hop2 = "68747470733a2f2f..."
url3 = bytes.fromhex(hop2).decode()

# Hop 3: ROT13-encoded flag
hop3 = "hgsynt{...}"
flag = codecs.decode(hop3, 'rot_13')
```

**Key insight:** Each resource contains a hint about the next encoding (e.g., "Three letters follow" hints at 3-character encoding like hex). Look for contextual clues in surrounding text (poetry, comments, filenames) that indicate the encoding type.

**Detection:** Challenge mentions "trail", "breadcrumbs", "follow", or "scavenger hunt". First resource contains what looks like encoded data rather than a direct flag.

---

## Esoteric Languages

| Language | Pattern |
|----------|---------|
| Brainfuck | `++++++++++[>+++++++>` |
| Whitespace | Only spaces, tabs, newlines (or S/T/L substitution) |
| Ook! | `Ook. Ook? Ook!` |
| Malbolge | Extremely obfuscated |
| Piet | Image-based |

### Whitespace Language Parser (BYPASS CTF 2025)

**Pattern (Whispers of the Cursed Scroll):** File contains only S (space), T (tab), L (linefeed) characters — or visible substitutes. Stack-based virtual machine (VM) with PUSH, OUTPUT, and EXIT instructions.

**Instruction set (IMP = Instruction Modification Parameter):**
| Instruction | Encoding | Action |
|-------------|----------|--------|
| PUSH | `S S` + sign + binary + `L` | Push number to stack (S=0, T=1, L=terminator) |
| OUTPUT CHAR | `T L S S` | Pop stack, print as ASCII character |
| EXIT | `L L L` | Halt program |

```python
def solve_whitespace(content):
    # Convert to S/T/L tokens (handle both raw whitespace and visible chars)
    if any(c in content for c in 'STL'):
        code = [c for c in content if c in 'STL']
    else:
        code = [{'\\s': 'S', '\\t': 'T', '\\n': 'L'}.get(c, '') for c in content]
        code = [c for c in code if c]

    stack, output, i = [], "", 0

    while i < len(code):
        if code[i:i+2] == ['S', 'S']:  # PUSH
            i += 2
            sign = 1 if code[i] == 'S' else -1
            i += 1
            val = 0
            while i < len(code) and code[i] != 'L':
                val = (val << 1) + (1 if code[i] == 'T' else 0)
                i += 1
            i += 1  # skip terminator L
            stack.append(sign * val)
        elif code[i:i+4] == ['T', 'L', 'S', 'S']:  # OUTPUT CHAR
            i += 4
            if stack:
                output += chr(stack.pop())
        elif code[i:i+3] == ['L', 'L', 'L']:  # EXIT
            break
        else:
            i += 1

    return output
```

**Identification:** File with only whitespace characters, or challenge mentions "invisible code", "blank page", or uses S/T/L substitution. Try [Whitespace interpreter online](https://vii5ard.github.io/whitespace/) for quick testing.

---

### Custom Brainfuck Variants (Themed Esolangs)

**Pattern:** File contains repetitive themed words (e.g., "arch", "linux", "btw") used as substitutes for Brainfuck operations. Common in Easy/Misc CTF challenges.

**Identification:**
- File is ASCII text with very long lines of repeated words
- Small vocabulary (5-8 unique words)
- One word appears as a line terminator (maps to `.` output)
- Two words are used for increment/decrement (one has many repeats per line)
- Words often relate to a meme or theme (e.g., "I use Arch Linux BTW")

**Standard Brainfuck operations to map:**
| Op | Meaning | Typical pattern |
|----|---------|-----------------|
| `+` | Increment cell | Most repeated word (defines values) |
| `-` | Decrement cell | Second most repeated word |
| `>` | Move pointer right | Short word, appears alone or with `.` |
| `<` | Move pointer left | Paired with `>` word |
| `[` | Begin loop | Appears at start of lines with `]` counterpart |
| `]` | End loop | Appears at end of same lines as `[` |
| `.` | Output char | Line terminator word |

**Solving approach:**
```python
from collections import Counter
words = content.split()
freq = Counter(words)
# Most frequent = likely + or -, line-ender = likely .

# Map words to BF ops, translate, run standard BF interpreter
mapping = {'arch': '+', 'linux': '-', 'i': '>', 'use': '<',
           'the': '[', 'way': ']', 'btw': '.'}
bf = ''.join(mapping.get(w, '') for w in words)
# Then execute bf string with a standard Brainfuck interpreter
```

**Real example (0xL4ugh CTF - "iUseArchBTW"):** `.archbtw` extension, "I use Arch Linux BTW" meme theme.

**Tips:** Try swapping `+`/`-` or `>`/`<` if output is not ASCII. Verify output starts with known flag format.

---

### Multi-Layer Esoteric Language Chains (Break In 2016)

Challenges may stack multiple esoteric languages requiring sequential interpretation:

1. **Piet:** Visual programming language using colored pixel blocks. Execute PNG images as code:
```bash
npiet challenge.png         # npiet interpreter
# Or: java -jar PietDev.jar challenge.png
```

2. **Malbolge:** Extremely difficult esoteric language. Decode output from previous layer:
```bash
# Piet output → base64 decode → Malbolge source
echo "piet_output" | base64 -d > program.mal
malbolge program.mal        # Or use online interpreter
```

Common esoteric chains: Piet → base64 → Malbolge, Brainfuck → Ook → Whitespace, JSFuck → standard JS.

**Key insight:** When a PNG file doesn't contain obvious visual stego, try interpreting it as Piet code. Use `file` + visual inspection to identify the first layer, then decode sequentially.

---

## base65536 CJK Unicode Binary Encoding (IceCTF 2018)

**Pattern:** A blob that looks like a wall of Chinese characters (CJK Unified Ideographs) is actually a **base65536** encoding: each character carries two bytes of data, mapping 0x0000..0xFFFF to a picked subset of 65,536 Unicode codepoints. Detect by `file` reporting "Unicode text, UTF-8" with mostly CJK codepoints; decode with the `base65536` npm package or the Python port.

```bash
# Node.js / npm path
npm install -g base65536
echo -n "宝䀈䀋..." | base65536 --decode > out.bin

# Python port
pip install base65536
python3 - <<'PY'
import base65536, sys
sys.stdout.buffer.write(base65536.decode(open("blob.txt").read()))
PY > out.bin

file out.bin
# common outcome: "Zip archive data" or "ELF 64-bit"
```

**Key insight:** base64 expands 3 bytes → 4 chars; base65536 expands 2 bytes → 1 *Unicode codepoint*, and since a codepoint renders as 1–4 UTF-8 bytes the encoded stream actually *expands* by ~2× on disk — but visually it looks compact, which is the CTF trick. Any wall of Unicode that lacks variance across the Basic Multilingual Plane and is dominated by CJK, Hangul, or Tibetan is a candidate. Also check base1024 (BMP), base2048, base4096, and base32768 for related tricks.

**References:** IceCTF 2018 — Rabbit Hole, writeup 11421
