# CTF Forensics - Advanced Steganography (Part 2)

See also: [stego-advanced.md](stego-advanced.md) for audio steganography (FFT frequency domain, DTMF, SSTV, LSB audio, musical notes, metadata encoding, waveform binary, spectrogram QR) and whitespace/archive encoding.

## Table of Contents
- [Video Frame Accumulation for Hidden Image (ASIS CTF Finals 2013)](#video-frame-accumulation-for-hidden-image-asis-ctf-finals-2013)
- [Reversed Audio Hidden Message (ASIS CTF Finals 2013)](#reversed-audio-hidden-message-asis-ctf-finals-2013)
- [Video Frame Averaging for Hidden Content (SECCON 2015)](#video-frame-averaging-for-hidden-content-seccon-2015)
- [JPEG XL TOC Permutation Steganography (BSidesSF 2026)](#jpeg-xl-toc-permutation-steganography-bsidessf-2026)
- [Arnold's Cat Map Image Descrambling (Nuit du Hack 2017)](#arnolds-cat-map-image-descrambling-nuit-du-hack-2017)
- [High-Resolution SSTV Custom FM Demodulation (PlaidCTF 2017)](#high-resolution-sstv-custom-fm-demodulation-plaidctf-2017)
- [MJPEG Extra Bytes After FFD9 Steganography (PoliCTF 2017)](#mjpeg-extra-bytes-after-ffd9-steganography-polictf-2017)
- [EXIF Zlib Data with Non-Default LSB Pixel Pattern (ASIS CTF Finals 2017)](#exif-zlib-data-with-non-default-lsb-pixel-pattern-asis-ctf-finals-2017)
- [PDF Cross-Reference Table Covert Channel (SEC-T CTF 2017)](#pdf-cross-reference-table-covert-channel-sec-t-ctf-2017)
- [ANSI Escape Code Steganography in Network Capture (Square CTF 2017)](#ansi-escape-code-steganography-in-network-capture-square-ctf-2017)
- [Pixel-Wise ECB Deduplication for Image Recovery (BackdoorCTF 2017)](#pixel-wise-ecb-deduplication-for-image-recovery-backdoorctf-2017)
- [Multi-Color QR Code Binary Mapping Brute Force (STEM CTF 2019)](#multi-color-qr-code-binary-mapping-brute-force-stem-ctf-2019)

---

## Video Frame Accumulation for Hidden Image (ASIS CTF Finals 2013)

**Pattern:** Video shows small images (icons, shapes) flashing briefly at different screen positions. Individual frames appear random, but the positions trace out a hidden pattern (QR code, text, image) when all frames are composited together.

**Extraction workflow:**

1. Extract individual frames from the video:
```bash
ffmpeg -i challenge.mp4 -vsync 0 frames/frame_%04d.png
```

2. Composite all frames by taking the maximum (or union) of all pixel values:
```python
from PIL import Image
import os

frames_dir = 'frames'
frame_files = sorted(os.listdir(frames_dir))

# Load first frame as base
base = Image.open(os.path.join(frames_dir, frame_files[0])).convert('L')

# Accumulate: take maximum pixel value across all frames
import numpy as np
accumulated = np.array(base, dtype=np.float64)
for f in frame_files[1:]:
    frame = np.array(Image.open(os.path.join(frames_dir, f)).convert('L'), dtype=np.float64)
    accumulated = np.maximum(accumulated, frame)

result = Image.fromarray(accumulated.astype(np.uint8))
result.save('accumulated.png')
```

3. Alternative: convert to GIF and delete the black background frame in GIMP to see all positions overlaid.

4. Clean up the revealed pattern (e.g., QR code) — select foreground, grow/shrink selection, flood fill, scale to expected dimensions (e.g., 21x21 for Version 1 QR):
```bash
# Scan for QR code
zbarimg accumulated.png
```

**Key insight:** When a video shows objects flashing at seemingly random positions, composite all frames together. The positions themselves encode the hidden data — each frame contributes one pixel/cell to a larger image. Convert to GIF for frame-by-frame inspection in GIMP, or use PIL/NumPy to take per-pixel maximum across all frames.

---

## Reversed Audio Hidden Message (ASIS CTF Finals 2013)

**Pattern:** Audio track (standalone or extracted from video) sounds garbled or unintelligible. Playing it in reverse reveals speech, numbers, or other meaningful content.

**Extraction and reversal:**
```bash
# Extract audio from video
ffmpeg -i challenge.mp4 -vn -acodec pcm_s16le audio.wav

# Reverse audio
sox audio.wav reversed.wav reverse
# Or: ffmpeg -i audio.wav -af areverse reversed.wav

# Play to hear hidden message
play reversed.wav
```

**Alternative:** Open in Audacity -> Effect -> Reverse. Listen for speech, numbers, or encoded data.

**Key insight:** Reversed audio is one of the simplest audio steganography techniques. If audio sounds like garbled speech with recognizable cadence, try reversing it first. The hidden content is often a numeric string (e.g., an MD5 hash) or instructions for the next step of the challenge. Check both the audio and video tracks of multimedia files independently.

---

## Video Frame Averaging for Hidden Content (SECCON 2015)

Extract content hidden across multiple video frames by temporal averaging:

```python
import numpy as np
from PIL import Image
import glob

frames = sorted(glob.glob('frames/*.png'))
N = len(frames)

# Accumulate frames as floating-point to preserve precision
acc = np.zeros(np.array(Image.open(frames[0])).shape, dtype=np.float64)
for f in frames:
    acc += np.array(Image.open(f), dtype=np.float64) / N

# Convert back to uint8
result = Image.fromarray(np.round(acc).astype(np.uint8))
result.save('averaged.png')
```

Use histogram equalization to enhance contrast if the averaged image is faint:

```python
from PIL import ImageOps
enhanced = ImageOps.equalize(result.convert('L'))
enhanced.save('enhanced.png')
```

**Key insight:** Content obscured by motion, noise, or rapid changes across frames becomes visible when averaged. Extract frames with `ffmpeg -i video.mp4 frames/%04d.png` first. Works for hidden QR codes, text, and watermarks.

---

## JPEG XL TOC Permutation Steganography (BSidesSF 2026)

**Pattern (image-progress):** JPEG XL's Table of Contents (TOC) supports a permutation field that reorders how AC groups (progressive scan tiles) are stored in the file. The convergence order during progressive decoding — which 256x256 tiles appear first as you truncate the file at increasing offsets — encodes the flag.

**Decoding approach:**
1. **Progressive truncation:** Truncate the JXL file at increasing byte offsets (e.g., every 1KB)
2. **Decode each truncation:** Use `djxl` to decode each truncated file
3. **Measure tile convergence:** Compare each decoded truncation against the full decode to determine which 256x256 tiles have converged (match the final image)
4. **Read convergence order:** The order in which tiles reach their final state spells the flag

```python
import subprocess
import numpy as np
from PIL import Image

# Full decode as reference
subprocess.run(['djxl', 'flag.jxl', 'full.png'])
full = np.array(Image.open('full.png'))
h, w = full.shape[:2]
tile_size = 256
tiles_x = (w + tile_size - 1) // tile_size
tiles_y = (h + tile_size - 1) // tile_size

# Track when each tile converges
converged = {}
jxl_data = open('flag.jxl', 'rb').read()

for offset in range(1000, len(jxl_data), 1000):
    # Write truncated file
    with open('/tmp/trunc.jxl', 'wb') as f:
        f.write(jxl_data[:offset])

    # Try to decode (may fail for very short truncations)
    result = subprocess.run(['djxl', '/tmp/trunc.jxl', '/tmp/trunc.png'],
                          capture_output=True)
    if result.returncode != 0:
        continue

    partial = np.array(Image.open('/tmp/trunc.png'))

    # Check which tiles match the full decode
    for ty in range(tiles_y):
        for tx in range(tiles_x):
            tile_id = ty * tiles_x + tx
            if tile_id in converged:
                continue
            y0, y1 = ty * tile_size, min((ty+1) * tile_size, h)
            x0, x1 = tx * tile_size, min((tx+1) * tile_size, w)
            if np.array_equal(partial[y0:y1, x0:x1], full[y0:y1, x0:x1]):
                converged[tile_id] = offset

# Sort tiles by convergence order
order = sorted(converged.items(), key=lambda x: x[1])
flag_chars = [chr(tile_id) for tile_id, _ in order]
print('Flag:', ''.join(flag_chars))
```

**Alternative — direct TOC extraction:**
```bash
# Modified djxl with debug prints can extract TOC permutation directly
# Look for the permutation array in the JXL frame header
# The TOC permutation maps: stored_order[i] -> logical_group[i]
# Inverse gives: logical_group -> stored_order (convergence priority)
```

**JPEG XL progressive structure:**
- **DC groups:** Low-frequency data (converges first, gives blurry preview)
- **AC groups:** High-frequency detail, stored per 256x256 tile
- **TOC permutation:** Reorders the storage of AC groups — controls which tiles get detail first during progressive loading
- **Lehmer code:** JXL encodes the permutation as a Lehmer code sequence in the TOC header

**Key insight:** JPEG XL's TOC permutation is a legitimate feature for progressive rendering optimization (prioritize important image regions). As a steganographic channel, it's invisible — the fully decoded image looks identical regardless of permutation. The hidden data is only revealed by observing the progressive convergence order, which requires truncating the file at multiple points.

**Detection:** JXL file where progressive rendering shows tiles appearing in an unusual order (e.g., spelling text). Challenge mentions "progressive", "convergence", or "order matters".

**References:** BSidesSF 2026 "image-progress"

---

## Arnold's Cat Map Image Descrambling (Nuit du Hack 2017)

Arnold's Cat Map is a chaotic area-preserving transformation that is periodic — iterating it enough times restores the original image. When an image appears scrambled with a noise-like pattern but retains the correct dimensions and color histogram, suspect a Cat Map scramble.

```python
from PIL import Image
import numpy as np

img = np.array(Image.open('scrambled.png'))
N = img.shape[0]  # Must be square

def arnold_cat_map(image, n):
    """Apply Arnold's Cat Map transformation"""
    result = np.zeros_like(image)
    for x in range(n):
        for y in range(n):
            nx = (2*x + y) % n
            ny = (x + y) % n
            result[nx, ny] = image[x, y]
    return result

# Iterate until original image reappears (period depends on N)
current = img.copy()
for i in range(1, N * N):
    current = arnold_cat_map(current, N)
    Image.fromarray(current).save(f'frame_{i:04d}.png')
    # Check if we've returned to original (or visually inspect)
```

**Key insight:** Arnold's Cat Map is periodic with period dividing `3*N` for most image sizes. Iterating the forward transform eventually restores the original. For large images, compute the period analytically via `lcm` of matrix eigenvalue orders in `Z/NZ` rather than brute-forcing all iterations.

**Detection:** Square image that looks like uniformly scrambled noise but has a plausible color distribution. Challenge mentions "cat", "Arnold", "chaotic", or "permutation".

---

## High-Resolution SSTV Custom FM Demodulation (PlaidCTF 2017)

When a WAV file contains an SSTV signal at higher-than-standard sample rate (e.g., 96kHz vs standard 2.3kHz bandwidth), standard SSTV decoders fail on the high-frequency content. Use custom FM demodulation.

```python
# Method 1: GNU Radio
# Hilbert Transform -> Quadrature Demod -> low-pass filter

# Method 2: Manual arccos + derivative (handles clipping)
import numpy as np
from scipy.io import wavfile

rate, data = wavfile.read('signal.wav')
# Normalize to [-1, 1]
data = data / np.max(np.abs(data))
# Clamp to valid arccos range
data = np.clip(data, -0.999, 0.999)
# Instantaneous frequency via arccos derivative
phase = np.arccos(data)
freq = np.diff(phase) * rate / (2 * np.pi)
# Map frequency to pixel intensity (1500-2300Hz typical SSTV range)
pixels = np.clip((freq - 1500) / 800 * 255, 0, 255).astype(np.uint8)
```

**Key insight:** Standard SSTV decoders (QSSTV, MMSSTV) assume standard bandwidth (~2.3kHz). High-sample-rate recordings may contain wider-bandwidth signals that these decoders truncate. Manual FM demodulation via `arccos` + differentiation (avoiding Hilbert transform artifacts on clipped signals) recovers the full frequency range.

**Detection:** WAV file at unusually high sample rate (48kHz, 96kHz) where standard SSTV decoders produce garbled or partial output. Spectrogram shows frequency-modulated signal structure.

---

## MJPEG Extra Bytes After FFD9 Steganography (PoliCTF 2017)

MJPEG video frames that contain extra bytes after the JPEG end-of-image marker (FFD9) hide data in the padding.

```python
# Split MJPEG into individual frames
frames = open('video.mjpeg', 'rb').read().split(b'\xff\xd8')

hidden = b""
for frame in frames:
    if not frame: continue
    frame = b'\xff\xd8' + frame
    # Find JPEG EOI marker
    eoi = frame.find(b'\xff\xd9')
    if eoi != -1:
        extra = frame[eoi + 2:]  # bytes after FFD9
        if extra:
            hidden += extra

print(hidden.decode(errors='ignore'))
```

**Key insight:** JPEG decoders stop at the FFD9 (End of Image) marker and ignore trailing bytes. In MJPEG streams, each frame is a complete JPEG — appending 1+ extra bytes after each frame's FFD9 creates a covert channel invisible to video players.

**Detection:** MJPEG file where individual frames are slightly larger than expected. `binwalk` on raw MJPEG may show repeated JPEG headers. Hex dump shows non-zero data between FFD9 and the next FFD8.

---

## EXIF Zlib Data with Non-Default LSB Pixel Pattern (ASIS CTF Finals 2017)

A JPG's EXIF `ImageDescription` field contains zlib-compressed then base64-encoded data. Detect via the `\x78\x9C` zlib magic bytes after base64 decoding. After decompression, the hint references the Stegano Python library with a `triangular_numbers` generator for non-sequential pixel selection (positions 1, 3, 6, 10, ...).

```bash
# Step 1: Extract EXIF ImageDescription
exiftool -ImageDescription image.jpg
# Or:
python3 -c "
from PIL import Image
img = Image.open('image.jpg')
desc = img._getexif()[270]  # Tag 270 = ImageDescription
print(repr(desc))
"

# Step 2: Base64-decode, then zlib-decompress
python3 -c "
import base64, zlib
desc = '<exif_description_value>'
decoded = base64.b64decode(desc)
print(zlib.decompress(decoded).decode())
"

# Step 3: Extract hidden data using Stegano with triangular_numbers generator
python3 -c "
from stegano import lsb
from stegano.lsb import generators
print(lsb.reveal('image.png', generators.triangular_numbers()))
"
```

**Key insight:** Standard LSB tools (zsteg, stegsolve) fail with non-sequential pixel patterns. The Stegano library supports custom generators; always check EXIF metadata for hints about which generator to use. The `\x78\x9C` bytes are the deflate magic — a reliable indicator of zlib-compressed content.

---

## PDF Cross-Reference Table Covert Channel (SEC-T CTF 2017)

PDF xref table entries normally use generation number 0 (live objects) or 65535 (free/deleted). Non-standard generation numbers encode data: read each non-zero, non-65535 generation number in order, interpret as hex -> ASCII characters (may need to reverse the string).

```bash
# Inspect raw xref entries with pdf-parser.py
python pdf-parser.py --stats suspicious.pdf
python pdf-parser.py --type /XRef suspicious.pdf

# Or read the raw xref table directly
python3 -c "
with open('suspicious.pdf', 'rb') as f:
    data = f.read().decode('latin-1')

# Find xref section
xref_idx = data.rfind('xref')
xref_section = data[xref_idx:xref_idx+2000]
gen_numbers = []
for line in xref_section.splitlines():
    parts = line.split()
    if len(parts) == 3 and parts[2] in ('n', 'f'):
        gen = int(parts[1])
        if gen not in (0, 65535):
            gen_numbers.append(gen)

# Convert hex values to ASCII
flag = bytes.fromhex(''.join(f'{g:02x}' for g in gen_numbers)).decode()
print(flag)
# Also try reversed: print(flag[::-1])
"
```

**Key insight:** PDF xref generation numbers are rarely validated by viewers, making them a low-noise steganographic channel. Any value other than 0 (live) or 65535 (deleted) is suspicious. Use `pdf-parser.py --raw` to inspect raw xref entries without parser normalization.

---

## ANSI Escape Code Steganography in Network Capture (Square CTF 2017)

Network packet data contains ANSI escape sequences (color codes, cursor movement). Raw hex and strings tools show garbled output. Pipe raw bytes through a terminal pager (`more`, `less -r`) to render the escape codes — the flag becomes visible as colored or positioned text.

```bash
# Extract raw TCP stream payload
tshark -r capture.pcap -q -z "follow,tcp,raw,0" | \
  tail -n +7 | tr -d '\n' | xxd -r -p > stream.bin

# Render ANSI escape codes (simplest approach)
more stream.bin
# or
cat stream.bin | less -r

# Alternative: extract data field directly
tshark -r capture.pcap -T fields -e data | xxd -r -p | more
```

ANSI escape patterns to recognize:
- `\x1b[<n>m` — color/attribute codes
- `\x1b[<row>;<col>H` — cursor position
- `\x1b[<n>A/B/C/D` — cursor movement (up/down/right/left)

**Key insight:** ANSI escape sequences encode visual information only revealed by terminal rendering. Always try `more` or `less -r` if content looks like terminal output. Cursor-positioning sequences can spell out text that only appears correct on a terminal.

---

## Pixel-Wise ECB Deduplication for Image Recovery (BackdoorCTF 2017)

An image is encrypted by replacing each pixel's value with a hash (ECB-mode pixel encryption). Since the pixel value space is small (256 for grayscale, or limited palette), precompute a hash-to-pixel lookup table and remap each hash value back to the original pixel.

```python
from PIL import Image
import hashlib

img = Image.open('encrypted.png').convert('L')  # Grayscale
pixels = list(img.getdata())

# Build lookup table: hash(pixel) -> pixel value
# The encryption maps each unique pixel value to a unique hash
# Since the space is small (256 values), enumerate all possible originals
lookup = {}
for original_val in range(256):
    # Determine which hash function was used (MD5, SHA1, etc.)
    h = hashlib.md5(bytes([original_val])).hexdigest()
    lookup[h] = original_val

# Reconstruct: each "pixel" in encrypted image is actually a hash index
# For palette-based images, map color index -> original pixel
unique_colors = list(set(pixels))
color_map = {}
for i, color in enumerate(unique_colors):
    # ECB: identical pixels -> identical cipher values
    # Count unique values to confirm small space
    pass

# Simpler: if encrypted values are small integers (0-255 remapped)
# The structure is preserved — just find the right permutation
reconstructed = Image.new('L', img.size)
# Map each encrypted value back using the lookup
```

**Key insight:** ECB-mode pixel encryption leaks structure via identical ciphertexts for identical plaintext pixels. With only 256 possible grayscale values, the full lookup table is trivial to precompute. The encrypted image will show the same shapes/edges as the original — recognizable structure confirms ECB mode.

---

## Multi-Color QR Code Binary Mapping Brute Force (STEM CTF 2019)

**Pattern:** A QR-like image uses N colors instead of black/white. A valid QR code requires only two states (black=1, white=0), so each color must map to one of those. With N non-trivial colors, iterate all 2^N binary partitions and try to decode each candidate. Typical N=6 produces 64 candidates; 3 of the 64 often decode (redundancy baked into QR error correction).

```python
from PIL import Image
from itertools import product
import subprocess, os

img = Image.open('QvR.png').convert('RGB')
px = img.load()
w, h = img.size

# Collect distinct non-pure colors (ignore black/white which are unambiguous)
palette = set()
for y in range(h):
    for x in range(w):
        c = px[x, y]
        if c not in ((0, 0, 0), (255, 255, 255)):
            palette.add(c)
palette = sorted(palette)                       # deterministic order
print(f'{len(palette)} variable colors -> {2**len(palette)} attempts')

for bits in product([0, 1], repeat=len(palette)):
    mapping = dict(zip(palette, bits))
    out = Image.new('1', (w, h), 1)
    op = out.load()
    for y in range(h):
        for x in range(w):
            c = px[x, y]
            if c == (0, 0, 0):       v = 0
            elif c == (255, 255, 255): v = 1
            else:                     v = mapping[c]
            op[x, y] = v
    fn = f'try_{"".join(map(str, bits))}.png'
    out.save(fn)
    r = subprocess.run(['zbarimg', '-q', fn], capture_output=True, text=True)
    if r.stdout.strip():
        print(fn, '->', r.stdout.strip())
```

**Key insight:** QR codes are strictly binary — any multi-color image that "looks like" a QR is hiding a 2^N coloring. Because QR has heavy Reed-Solomon error correction, multiple partitions can decode (each carries a different message in the same physical grid). Always try all 2^N mappings; with N<=8 the brute force is negligible and `zbarimg` filters the valid ones automatically.

**References:** STEM CTF: Cyber Challenge 2019 — QvR Code, writeup 13375
