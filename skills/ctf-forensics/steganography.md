# CTF Forensics - Steganography

Non-image steganography techniques (PDF, SVG, terminal, text, compression, spreadsheet) and general-purpose image stego patterns (PNG structure, file overlays, GIF, autostereograms, interleaving). For image-specific steganography (JPEG DQT/F5/slack, BMP bitplane, PNG palette, pixel permutation, edge matching), see [stego-image.md](stego-image.md). For advanced techniques (FFT, SSTV, audio, video, JPEG XL), see [stego-advanced.md](stego-advanced.md) and [stego-advanced-2.md](stego-advanced-2.md).

## Table of Contents
- [Quick Tools](#quick-tools)
- [Binary Border Steganography](#binary-border-steganography)
- [Multi-Layer PDF Steganography (Pragyan 2026)](#multi-layer-pdf-steganography-pragyan-2026)
- [Advanced PDF Steganography (Nullcon 2026 rdctd series)](#advanced-pdf-steganography-nullcon-2026-rdctd-series)
- [SVG Animation Keyframe Steganography (UTCTF 2024)](#svg-animation-keyframe-steganography-utctf-2024)
- [PNG Chunk Reordering (0xFun 2026)](#png-chunk-reordering-0xfun-2026)
- [File Format Overlays (0xFun 2026)](#file-format-overlays-0xfun-2026)
- [Nested PNG with Iterating XOR Keys (VuwCTF 2025)](#nested-png-with-iterating-xor-keys-vuwctf-2025)
- [GIF Frame Differential + Morse Code (BaltCTF 2013)](#gif-frame-differential--morse-code-baltctf-2013)
- [GZSteg + Spammimic Text Steganography (VolgaCTF 2013)](#gzsteg--spammimic-text-steganography-volgactf-2013)
- [Spreadsheet Frequency Analysis Binary Recovery (Sharif CTF 2016)](#spreadsheet-frequency-analysis-binary-recovery-sharif-ctf-2016)
- [Kitty Terminal Graphics Protocol Decoding (BSidesSF 2026)](#kitty-terminal-graphics-protocol-decoding-bsidessf-2026)
- [ANSI Escape Sequence Steganography in Terminal Art (BSidesSF 2026)](#ansi-escape-sequence-steganography-in-terminal-art-bsidessf-2026)
- [Autostereogram / Magic Eye Solving (BSidesSF 2026)](#autostereogram--magic-eye-solving-bsidessf-2026)
- [Two-Layer Byte+Line Interleaving (BSidesSF 2026)](#two-layer-byteline-interleaving-bsidessf-2026)
- [Progressive PNG Layered XOR Decryption (OpenCTF 2016)](#progressive-png-layered-xor-decryption-openctf-2016)
- [Multi-Stream Video Container Steganography (BSidesSF 2026)](#multi-stream-video-container-steganography-bsidessf-2026)
- [APNG (Animated PNG) Frame Extraction (IceCTF 2016)](#apng-animated-png-frame-extraction-icectf-2016)
- [PNG Height/CRC Manipulation for Hidden Content (H4ckIT CTF 2016)](#png-heightcrc-manipulation-for-hidden-content-h4ckit-ctf-2016)
- [QR Code Reconstruction from Curved Glass Reflection in Video (PlaidCTF 2018)](#qr-code-reconstruction-from-curved-glass-reflection-in-video-plaidctf-2018)
- [GIF Palette Manipulation for QR Code Reconstruction (3DSCTF 2017)](#gif-palette-manipulation-for-qr-code-reconstruction-3dsctf-2017)
- [Angecryption: AES-CBC Encrypting One Valid File into Another (34C3 CTF 2017)](#angecryption-aes-cbc-encrypting-one-valid-file-into-another-34c3-ctf-2017)
- [SVG Micro-Coordinate Steganography (SharifCTF 8)](#svg-micro-coordinate-steganography-sharifctf-8)

---

## Quick Tools

```bash
steghide extract -sf image.jpg
zsteg image.png              # PNG/BMP analysis
stegsolve                    # Visual analysis

# Steghide brute-force (0xFun 2026)
stegseek image.jpg rockyou.txt  # Faster than stegcracker
# Common weak passphrases: "simple", "password", "123456"
```

---

## Binary Border Steganography

**Pattern (Framer, PascalCTF 2026):** Message encoded as black/white pixels in 1-pixel border around image.

```python
from PIL import Image

img = Image.open('output.jpg')
w, h = img.size
bits = []

# Read border clockwise: top → right → bottom (reversed) → left (reversed)
for x in range(w): bits.append(0 if sum(img.getpixel((x, 0))[:3]) < 384 else 1)
for y in range(1, h): bits.append(0 if sum(img.getpixel((w-1, y))[:3]) < 384 else 1)
for x in range(w-2, -1, -1): bits.append(0 if sum(img.getpixel((x, h-1))[:3]) < 384 else 1)
for y in range(h-2, 0, -1): bits.append(0 if sum(img.getpixel((0, y))[:3]) < 384 else 1)

# Convert bits to ASCII
msg = ''.join(chr(int(''.join(map(str, bits[i:i+8])), 2)) for i in range(0, len(bits)-7, 8))
```

---

## Multi-Layer PDF Steganography (Pragyan 2026)

**Pattern (epstein files):** Flag hidden across multiple layers in a PDF.

**Layer checklist:**
1. `strings file.pdf | grep -i hidden` -- hidden comments in PDF objects
2. Extract hex strings, try XOR with theme-related keywords
3. Check bytes **after `%%EOF`** marker -- may contain GPG/encrypted data
4. Try ROT18 (ROT13 on letters + ROT5 on digits) as final decode layer

```bash
# Extract post-EOF data
python3 -c "
data = open('file.pdf','rb').read()
eof = data.rfind(b'%%EOF')
print(data[eof+5:].hex())
"
```

---

## Advanced PDF Steganography (Nullcon 2026 rdctd series)

Six distinct hiding techniques in a single PDF:

**1. Invisible text separators:** Underscores rendered as invisible line segments. Extract with `pdftotext -layout` and normalize whitespace to underscores.

**2. URI annotations with escaped braces:** Link annotations contain flag in URI with `\{` and `\}` escapes:
```python
import pikepdf
pdf = pikepdf.Pdf.open(pdf_path)
for page in pdf.pages:
    for annot in (page.get("/Annots") or []):
        obj = annot.get_object()
        if obj.get("/Subtype") == pikepdf.Name("/Link"):
            uri = str(obj.get("/A").get("/URI")).replace(r"\{", "{").replace(r"\}", "}")
            # Check for flag pattern
```

**3. Blurred/redacted image with Wiener deconvolution:**
```python
from skimage.restoration import wiener
import numpy as np

def gaussian_psf(sigma):
    k = int(sigma * 6 + 1) | 1
    ax = np.arange(-(k//2), k//2 + 1, dtype=np.float32)
    xx, yy = np.meshgrid(ax, ax)
    psf = np.exp(-(xx**2 + yy**2) / (2 * sigma * sigma))
    return psf / psf.sum()

img_arr = np.asarray(img.convert("L")).astype(np.float32) / 255.0
deconv = wiener(img_arr, gaussian_psf(3.0), balance=0.003, clip=False)
```

**4. Vector rectangle QR code:** Hundreds of tiny filled rectangles (e.g., 1.718x1.718 units) forming a QR code. Parse PDF content stream for `re` operators, extract centers, render as grid, decode with `zbarimg`.

**5. Compressed object streams:** Use `mutool clean -d -c -m input.pdf output.pdf` to decompress all streams, then `strings` to search.

**6. Document metadata:** Check Producer, Author, Keywords fields: `pdfinfo doc.pdf` or `exiftool doc.pdf`.

**Official writeup details (Nullcon 2026 rdctd 1-6):**
- **rdctd 1:** Flag is visible in plain text (Section 3.4)
- **rdctd 2:** Flag in hyperlink URI with escaped braces (`\{`, `\}`)
- **rdctd 3:** LSB stego in Blue channel, **bit plane 5** (not bit 0!). Use `zsteg` with all planes: `zsteg -a extracted.ppm | grep ENO`
- **rdctd 4:** QR code hidden under black redaction box. Use Master PDF Editor to remove the box, scan QR
- **rdctd 5:** Flag in FlateDecode compressed stream (not visible with `strings`):
  ```python
  import re, zlib
  pdf = open('file.pdf', 'rb').read()
  for s in re.findall(b'stream[\r\n]+(.*?)[\r\n]+endstream', pdf, re.S):
      try:
          dec = zlib.decompress(s)
          if b'ENO{' in dec: print(dec)
      except: pass
  ```
- **rdctd 6:** Flag in `/Producer` metadata field

**Comprehensive PDF flag hunt checklist:**
1. `strings -a file.pdf | grep -o 'FLAG_FORMAT{[^}]*}'`
2. `exiftool file.pdf` (all metadata fields)
3. `pdfimages -all file.pdf img` + `zsteg -a img-*.ppm`
4. Open in PDF editor, check for overlay/redaction boxes hiding content
5. Decompress FlateDecode streams and search
6. Parse link annotations for URIs with escaped characters
7. `mutool clean -d file.pdf clean.pdf && strings clean.pdf`

---

## SVG Animation Keyframe Steganography (UTCTF 2024)

**Pattern (Insanity Check):** SVG favicon contains animation keyframes with alternating fill colors.

**Encoding:** `#FFFF` = 1, `#FFF6` = 0. Timing intervals (~0.314s or 3x0.314s) encode Morse code dots/dashes.

**Detection:** SVG files with `<animate>` tags, `keyTimes`/`values` attributes. Check favicon.svg and other vector assets. Two-value alternation patterns encode binary or Morse.

---

## APNG (Animated PNG) Frame Extraction (IceCTF 2016)

APNG files contain multiple frames within a standard PNG container. Tools like `tweakpng` or `apngdis` extract individual frames that may contain hidden data.

```bash
# Check if PNG is actually APNG (contains acTL chunk)
python3 -c "
import struct
with open('image.png', 'rb') as f:
    data = f.read()
    if b'acTL' in data:
        print('APNG detected!')
        idx = data.index(b'acTL')
        num_frames = struct.unpack('>I', data[idx+4:idx+8])[0]
        print(f'Number of frames: {num_frames}')
"

# Extract frames using apngdis
apngdis image.apng  # produces frame_01.png, frame_02.png, ...

# Alternative: use PHP or Python libraries
# pip install apng
python3 -c "
from apng import APNG
im = APNG.open('image.apng')
for i, (png, control) in enumerate(im.frames):
    png.save(f'frame_{i:02d}.png')
"
```

**Key insight:** Regular PNG viewers display only the first frame of an APNG. Hidden data can be in any subsequent frame. The `acTL` chunk signals APNG format; `fcTL`/`fdAT` chunks contain additional frame data.

---

## PNG Height/CRC Manipulation for Hidden Content (H4ckIT CTF 2016)

PNG images with incorrect IHDR dimensions hide content below the visible area. Brute-force the correct height by matching the IHDR CRC.

```python
import struct, zlib

def fix_png_height(filename):
    with open(filename, 'rb') as f:
        data = bytearray(f.read())

    # IHDR chunk starts at offset 8 (after 8-byte PNG signature)
    # IHDR layout: width(4) height(4) bitdepth(1) colortype(1) ...
    ihdr_start = 8 + 4  # skip signature + chunk length
    ihdr_data = data[ihdr_start:ihdr_start + 17]  # "IHDR" + 13 bytes
    stored_crc = struct.unpack('>I', data[ihdr_start + 17:ihdr_start + 21])[0]

    width = struct.unpack('>I', ihdr_data[4:8])[0]

    # Brute-force correct height
    for h in range(1, 4096):
        test_ihdr = ihdr_data[:8] + struct.pack('>I', h) + ihdr_data[12:]
        if zlib.crc32(test_ihdr) & 0xffffffff == stored_crc:
            print(f"Correct height: {h} (was: {struct.unpack('>I', ihdr_data[8:12])[0]})")
            data[ihdr_start + 8:ihdr_start + 12] = struct.pack('>I', h)
            with open('fixed_' + filename, 'wb') as f:
                f.write(data)
            return h

    # If no CRC match, the CRC itself may need fixing after setting height
    # Manual approach: set height larger, fix CRC
    return None
```

**Key insight:** PNG stores image dimensions in the IHDR chunk with a CRC. If the height is reduced, data below the visible area is hidden but still present in IDAT chunks. Brute-forcing the height against the stored CRC reveals the correct dimensions. If the CRC was also modified, try increasing the height and recalculating the CRC.

---

## PNG Chunk Reordering (0xFun 2026)

**Pattern (Spectrum):** Invalid PNG has chunks out of order.

**Fix:** Reorder to: `signature + IHDR + (ancillary chunks) + (all IDAT in order) + IEND`.

```python
import struct

with open('broken.png', 'rb') as f:
    data = f.read()

sig = data[:8]
chunks = []
pos = 8
while pos < len(data):
    length = struct.unpack('>I', data[pos:pos+4])[0]
    chunk_type = data[pos+4:pos+8]
    chunk_data = data[pos+8:pos+8+length]
    crc = data[pos+8+length:pos+12+length]
    chunks.append((chunk_type, length, chunk_data, crc))
    pos += 12 + length

# Sort: IHDR first, IEND last, IDATs in original order
ihdr = [c for c in chunks if c[0] == b'IHDR']
idat = [c for c in chunks if c[0] == b'IDAT']
iend = [c for c in chunks if c[0] == b'IEND']
other = [c for c in chunks if c[0] not in (b'IHDR', b'IDAT', b'IEND')]

with open('fixed.png', 'wb') as f:
    f.write(sig)
    for typ, length, data, crc in ihdr + other + idat + iend:
        f.write(struct.pack('>I', length) + typ + data + crc)
```

---

## File Format Overlays (0xFun 2026)

**Pattern (Pixel Rehab):** Archive appended after PNG IEND, but magic bytes overwritten with PNG signature.

**Detection:** Check bytes after IEND for appended data. Compare magic bytes against known formats.

```python
# Find IEND, check what follows
data = open('image.png', 'rb').read()
iend_pos = data.find(b'IEND') + 8  # After IEND + CRC
trailer = data[iend_pos:]
# Replace first 6 bytes with 7z magic if they match PNG sig
if trailer[:4] == b'\x89PNG':
    trailer = b'\x37\x7a\xbc\xaf\x27\x1c' + trailer[6:]
    open('hidden.7z', 'wb').write(trailer)
```

---

## Nested PNG with Iterating XOR Keys (VuwCTF 2025)

**Pattern (Matroiska):** Each PNG layer XOR-encrypted with incrementing keys ("layer2", "layer3", etc.).

**Identification:** Matryoshka/nested hints. Try incrementing key patterns for recursive extraction.

---

## GIF Frame Differential + Morse Code (BaltCTF 2013)

**Pattern:** Animated GIF contains hidden dots visible only when comparing frames against originals. Dots encode Morse code.

```bash
# Extract frames from animated GIF
convert animated.gif frame_%03d.gif

# Compare each frame against its base using ImageMagick
for i in $(seq 1 100); do
    compare -fuzz 10% -compose src stego_$i.gif original_$i.gif diff_$i.gif
done

# Inspect diff images — dots appear at specific positions
# Map dot patterns to Morse: small dot = dit, large dot = dah
```

**Key insight:** `compare -fuzz 10%` reveals subtle single-pixel modifications invisible to the eye. The diff images show isolated dots whose timing/spacing encodes Morse code. Decode dots → dashes/dots → letters → flag.

---

## GZSteg + Spammimic Text Steganography (VolgaCTF 2013)

**Pattern:** Data hidden within gzip compression metadata, decoded through spammimic.com.

1. Apply GZSteg patches to gzip 1.2.4 source, compile, extract with `gzip --s` flag
2. Extracted text resembles spam email — submit to [spammimic.com](https://www.spammimic.com/) decoder
3. Decoded output is the flag

**Key insight:** GZSteg exploits redundancy in the gzip DEFLATE compression format to embed covert data. The extracted payload often uses a second steganographic layer (spammimic encodes data as innocuous-looking spam text). Look for `.gz` files larger than expected for their content.

---

## Spreadsheet Frequency Analysis Binary Recovery (Sharif CTF 2016)

When spreadsheet cells contain numbers with varying frequencies, the frequency rank may encode binary data:

1. **Count occurrences** of each unique value
2. **Sort by frequency** to create a mapping: value -> frequency rank (0-255)
3. **Replace each cell** with its frequency rank to recover raw bytes

```python
from collections import Counter

# Count frequency of each value
freq = Counter(all_cell_values)

# Create mapping: value -> index in frequency-sorted list
sorted_vals = sorted(freq.keys(), key=lambda x: freq[x])
mapping = {v: i for i, v in enumerate(sorted_vals)}

# Apply mapping to recover binary
binary = bytes(mapping[v] for v in all_cell_values)
# Result is typically an ELF binary or image
```

**Key insight:** 256 unique values suggest byte-level encoding. The frequency distribution of the mapped output should resemble typical binary file statistics.

---

## Kitty Terminal Graphics Protocol Decoding (BSidesSF 2026)

**Pattern (kitty):** A file contains Kitty terminal graphics protocol escape sequences (`ESC_G`) that embed zlib-compressed RGB image data in base64-encoded chunks.

**Protocol format:**
```text
\x1b_Ga=T,q=2,f=24,o=z,m=1,s=WIDTH,v=HEIGHT;BASE64DATA\x1b\\
```

**Header fields:**
- `a=T` — action: transmit
- `q=2` — quiet mode (suppress responses)
- `f=24` — format: 24-bit RGB
- `o=z` — compression: zlib
- `m=1` — more chunks follow; `m=0` — final chunk
- `s=WIDTH,v=HEIGHT` — image dimensions (present in first chunk only)

**Decoding workflow:**
```python
import re
import base64
import zlib
from PIL import Image

# Read the raw file
data = open('kitty_output.bin', 'rb').read()

# Extract all base64 payloads from escape sequences
# Pattern: \x1b_G...;BASE64\x1b\\
chunks = re.findall(rb'\x1b_G([^;]*);([^\x1b]*)\x1b\\\\', data)

# Parse dimensions from first chunk's header
first_header = chunks[0][0].decode()
width = int(re.search(r's=(\d+)', first_header).group(1))
height = int(re.search(r'v=(\d+)', first_header).group(1))

# Concatenate all base64 payloads
b64_data = b''.join(chunk[1] for chunk in chunks)
compressed = base64.b64decode(b64_data)
raw_rgb = zlib.decompress(compressed)

# Reconstruct image
img = Image.frombytes('RGB', (width, height), raw_rgb)
img.save('recovered.png')
```

**Key insight:** Kitty graphics protocol is a modern terminal image display mechanism. The data is invisible when viewed in non-Kitty terminals but can be decoded from the raw escape sequences. Multi-chunk messages (`m=1` followed by continuation chunks) must be concatenated before base64 decoding.

**Detection:** Binary file containing `\x1b_G` sequences. `strings` output shows base64-like data interspersed with escape codes. Challenge mentions "kitty", "terminal graphics", or "meow".

**References:** BSidesSF 2026 "kitty"

---

## ANSI Escape Sequence Steganography in Terminal Art (BSidesSF 2026)

**Pattern (roar):** Flag text is interleaved between ANSI color escape codes and Unicode braille characters in terminal art. When rendered in a terminal, the art displays normally while the flag characters are invisible (zero-width or same-color-as-background). However, the flag is extractable by stripping all escape sequences and non-ASCII characters.

**Extraction:**
```python
import re

data = open('art.txt', 'rb').read().decode('utf-8', errors='replace')

# Strip ANSI escape sequences
clean = re.sub(r'\x1b\[[0-9;]*[a-zA-Z]', '', data)

# Extract only printable ASCII (flag characters)
flag_chars = [c for c in clean if 32 <= ord(c) <= 126 and c not in ' \t\n']

# Or: filter out braille unicode block (U+2800-U+28FF) and other non-ASCII
flag_chars = [c for c in clean if ord(c) < 128 and c.isprintable() and c != ' ']

print(''.join(flag_chars))
```

**Alternative approach — diff against rendered output:**
```bash
# Render with ANSI codes, capture visible text
cat art.txt | col -b > rendered.txt
# Compare raw vs rendered to find hidden characters
```

**Key insight:** ANSI escape sequences control terminal colors, cursor position, and text attributes. Flag characters inserted between escape codes are technically present in the file but invisible when rendered because they're either: (a) the same color as the background, (b) followed by a cursor-move-back sequence, or (c) overwritten by subsequent characters. Raw byte extraction bypasses all rendering tricks.

**Detection:** File with many `\x1b[` sequences (ANSI codes), Unicode braille characters (U+2800-U+28FF), and unexpectedly large file size for the visible content. Challenge mentions "terminal", "art", "ANSI", or shows ASCII/Unicode art.

**References:** BSidesSF 2026 "roar"

---

### Autostereogram / Magic Eye Solving (BSidesSF 2026)

**Pattern (stereotype):** Challenge image is an autostereogram (Magic Eye). The hidden 3D content (flag text) is revealed by viewing with crossed/divergent eyes or programmatically via layer difference.

**Programmatic solve (GIMP or Python):**
1. Duplicate the image as a second layer
2. Set the top layer's blending mode to "Difference"
3. Slide the top layer horizontally by the repeat width (~100 pixels)
4. The hidden depth pattern appears as bright lines on a dark background

```python
from PIL import Image
import numpy as np

img = np.array(Image.open('stereogram.png'))
shift = 100  # Repeat width — try values 80-120
diff = np.abs(img[:, shift:].astype(int) - img[:, :-shift].astype(int))
Image.fromarray(diff.astype(np.uint8)).save('revealed.png')
```

**Finding the shift value:** The repeat width is the horizontal distance between identical vertical strips. Autocorrelate a single row: `np.correlate(row, row, mode='full')` — the first peak after center is the shift.

**Key insight:** Autostereograms encode depth via horizontal pixel displacement relative to a repeating pattern. Subtracting the image from a shifted copy of itself cancels the repeating background and reveals the depth variation as the flag text.

**When to recognize:** Image has a repeating texture/pattern, challenge mentions "eyes", "seeing", "3D", "magic", or "stereogram".

**References:** BSidesSF 2026 "stereotype"

---

### Two-Layer Byte+Line Interleaving (BSidesSF 2026)

**Pattern (seeing-double):** Two PNG files are interleaved at the byte level into a single file. After byte-level deinterlacing, the resulting images have their scanlines interleaved, requiring a second round of line-level deinterlacing.

**Step 1 — Byte deinterleave:**
```python
data = open('interleaved.ppnngg', 'rb').read()
file_a = bytes(data[i] for i in range(0, len(data), 2))  # Even bytes
file_b = bytes(data[i] for i in range(1, len(data), 2))  # Odd bytes
# file_a and file_b are valid PNGs
```

**Step 2 — Line deinterleave (if needed):**
```python
from PIL import Image
import numpy as np

img = np.array(Image.open('file_a.png'))
# Even lines form one sub-image, odd lines form another
sub1 = img[0::2]  # Lines 0, 2, 4, ...
sub2 = img[1::2]  # Lines 1, 3, 5, ...
Image.fromarray(sub1).save('final_a.png')
Image.fromarray(sub2).save('final_b.png')
```

**Key insight:** The two-layer interleaving (first bytes, then scanlines) means simple deinterleaving at one level produces garbled results. Recognize multi-layer interleaving by: (1) deinterleaved file is a valid image but content looks "striped" or has alternating line artifacts, (2) file extension hints (`.ppnngg` = two PNGs interleaved).

**Detection:** File has double-extension or unusual extension. `file` command may identify it as data or as one format. Even/odd byte extraction produces valid file headers (e.g., both halves start with PNG magic `89 50 4E 47`).

**References:** BSidesSF 2026 "seeing-double"

---

### Multi-Stream Video Container Steganography (BSidesSF 2026)

**Pattern (ads):** An MP4 video container holds multiple video streams. The default (stream 0:0) plays normally, but a second stream (0:1) contains the flag. Most video players only show the first/default stream. The secondary stream uses AV1 codec which has poor support in many tools, adding friction.

```bash
# Detect multiple streams
ffprobe -hide_banner flag.mp4
# Look for Stream #0:1 — a second video stream

# Extract second stream to its own file
ffmpeg -i flag.mp4 -map 0:1 -c copy second_stream.mp4

# Or extract just the first frame from stream 1
ffmpeg -i flag.mp4 -map 0:1 -frames:v 1 flag.jpg
```

**Key insight:** MP4/MKV containers can hold multiple video, audio, and subtitle tracks. Most players default to stream 0:0. Always run `ffprobe` or `mediainfo` to enumerate ALL streams. The `-map 0:N` flag in ffmpeg selects specific streams. VLC can also switch tracks via Video → Video Track menu.

**When to recognize:** Challenge provides a video file where the visible content seems irrelevant or is a red herring. `ffprobe` shows multiple `Stream` entries. Check metadata fields like `handler_name` for hints (e.g., "CTF Trickery").

**Detection checklist:**
1. `ffprobe -hide_banner file.mp4` — count Stream lines
2. `mediainfo file.mp4` — check track count
3. VLC → Video → Video Track → try all tracks

**References:** BSidesSF 2026 "ads"

---

## Progressive PNG Layered XOR Decryption (OpenCTF 2016)

**Pattern (Progressive Encryption):** PNG contains standard `IDAT` chunk (coarse first scan) plus custom `scRT` chunks. Each `scRT` chunk is XOR-encrypted with a multi-byte key. Decrypting reveals another `IDAT` chunk plus another `scRT`, forming nested layers.

1. Extract the custom `scRT` chunk data from the PNG
2. Use xortool to guess the XOR key (expected most frequent byte: `\xFF` for image data):
```bash
# Extract scRT chunk contents
python3 -c "
import struct
with open('image.png', 'rb') as f:
    data = f.read()
# Parse PNG chunks, find scRT
pos = 8  # skip PNG signature
while pos < len(data):
    length = struct.unpack('>I', data[pos:pos+4])[0]
    chunk_type = data[pos+4:pos+8]
    if chunk_type == b'scRT':
        with open('layer.bin', 'wb') as out:
            out.write(data[pos+8:pos+8+length])
    pos += 12 + length
"

# Guess XOR key
xortool -c ff layer.bin
# Output: key = 'nacho'
```

3. Decrypt and split: the decrypted data contains a valid `IDAT` chunk followed by another `scRT`
4. Repeat for each layer until all `scRT` chunks are decrypted
5. Reassemble: concatenate PNG header + all decrypted `IDAT` chunks + `IEND`

**Layer keys in this challenge:** `nacho`, `savages`, `president`, `kilobits`, `monkey`, `butler`

**Shortcut:** Open the raw PNG bytes as a raw image in GraphBitStreamer (32 bpp, width matching original). Weak XOR encryption preserves visual patterns (like ECB-encrypted images), making the flag readable without full decryption.

**Key insight:** Custom PNG chunks (non-standard 4-letter types) often contain hidden data. The PNG spec allows arbitrary ancillary chunks — parsers ignore unknown types. When multiple layers use different XOR keys, each must be cracked independently using frequency analysis. The shortcut works because XOR with a short repeating key preserves large-scale pixel patterns, similar to ECB mode's visual leakage.

---

### QR Code Reconstruction from Curved Glass Reflection in Video (PlaidCTF 2018)

**Pattern:** QR code visible only as a curved reflection on a glass sphere in surveillance footage (~100px wide). Manual reconstruction required flipping, de-warping, identifying as Version 2 (25x25), decoding format string for ECC level, and pixel-by-pixel reconstruction using known flag prefix to fix initial data bytes.

**Steps:**
1. Extract best frame from video, crop the reflection
2. Flip horizontally (mirror reflection) and apply de-warping
3. Identify QR version (25x25 = Version 2) and decode format bits for ECC level and mask pattern
4. Begin manual pixel transcription of the 25x25 grid
5. When initial decode fails, use known plaintext (flag prefix "PCTF{") to fix first data bytes
6. High ECC level (Q = 25% recovery) corrects remaining pixel errors

```python
from PIL import Image
import numpy as np

# Step 1: Extract and flip the reflection
frame = Image.open('best_frame.png')
reflection = frame.crop((x1, y1, x2, y2))
flipped = reflection.transpose(Image.FLIP_LEFT_RIGHT)

# Step 2: Scale up for manual transcription
scaled = flipped.resize((500, 500), Image.NEAREST)
scaled.save('reflection_scaled.png')

# Step 3: Manual 25x25 grid transcription (Version 2 QR)
# After manual pixel identification, create the QR matrix
qr_matrix = np.zeros((25, 25), dtype=np.uint8)
# Fill in identified modules from visual inspection...
# qr_matrix[row][col] = 1  # dark module
# qr_matrix[row][col] = 0  # light module

# Step 4: Render as clean QR image for scanning
cell_size = 20
qr_img = Image.new('L', (25 * cell_size, 25 * cell_size), 255)
for r in range(25):
    for c in range(25):
        if qr_matrix[r][c]:
            for dy in range(cell_size):
                for dx in range(cell_size):
                    qr_img.putpixel((c * cell_size + dx, r * cell_size + dy), 0)
qr_img.save('reconstructed_qr.png')

# Step 5: Scan with zbarimg or use known prefix to fix errors
# zbarimg reconstructed_qr.png
```

**Key insight:** QR codes with high ECC levels (Q or H) can tolerate significant reconstruction errors. When a QR code is partially visible (reflection, damage, low resolution), manually reconstruct what you can, use known plaintext to fix early data modules, and let ECC correct the rest.

---

### GIF Palette Manipulation for QR Code Reconstruction (3DSCTF 2017)

GIF with 108,900 single-pixel frames. Each frame has identical pixel data but different palette entries. Map palette color to black/white to reconstruct a 330x330 QR code:

```python
from PIL import Image
gif = Image.open('challenge.gif')
width = int(gif.n_frames ** 0.5)  # sqrt(108900) = 330
pixels = []
for i in range(gif.n_frames):
    gif.seek(i)
    palette = gif.getpalette()
    # First palette entry: yellow=(255,255,0) or green=(0,255,0)
    pixels.append(0 if palette[0] > 128 else 255)  # black or white

out = Image.new('L', (width, width))
out.putdata(pixels)
out.save('qr.png')
# zbarimg qr.png
```

**Key insight:** GIF frames with identical pixel data but different color palettes encode binary data through palette manipulation. The number of frames is a perfect square, giving the side length of the hidden image. Each frame represents one pixel; the palette's first entry determines its color. When a GIF has an unusually large number of frames whose count is a perfect square, check for palette-based encoding.

---

### Angecryption: AES-CBC Encrypting One Valid File into Another (34C3 CTF 2017)

Based on Ange Albertini's technique: a crafted AES-CBC key and IV can encrypt one valid image file into another valid image file:

```python
from Crypto.Cipher import AES
key = bytes.fromhex('...')  # provided or recovered
iv = bytes.fromhex('...')
aes = AES.new(key, AES.MODE_CBC, iv)
encrypted = aes.encrypt(open('flag.png', 'rb').read())
# encrypted is ALSO a valid PNG (a mask image)
# Overlay the mask on the original to reveal hidden content
```

**Key insight:** Angecryption exploits the fact that file format headers have enough degrees of freedom to survive AES-CBC encryption with chosen key/IV. The technique crafts the IV so that decrypting the "mask" file header produces a valid "flag" file header. When you find two valid image files and an AES key/IV in a challenge, try encrypting one — the result may be the other, and visual comparison reveals the flag.

---

### SVG Micro-Coordinate Steganography (SharifCTF 8)

SVG contains a visible graphic plus a second `<g>` element with extremely small coordinate values (e.g., 450.xxxxx, 835.xxxxx). Apply SVG transform to zoom in:

```xml
<svg viewBox="448.75 834.69 2 2" width="2000" height="2000">
  <!-- or apply transform: -->
  <g transform="scale(200, 200) translate(-448.75, -834.69)">
    <!-- hidden content becomes visible -->
  </g>
</svg>
```

**Key insight:** SVG coordinates with many decimal places hide micro-scale drawings invisible at normal zoom. Check for `<g>` elements with coordinate values that cluster in a tiny range. The fractional parts of the coordinates define the hidden image. Scale up by 100-1000x and translate to the cluster center to reveal. When SVG file size is unexpectedly large for the visible content, inspect coordinate precision in `<path>`, `<line>`, or `<g>` elements.
