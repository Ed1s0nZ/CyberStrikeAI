# CTF Forensics - Advanced Steganography

See also: [stego-advanced-2.md](stego-advanced-2.md) for video frame techniques, JPEG XL TOC permutation, Arnold's Cat Map, SSTV FM demodulation, MJPEG steganography, EXIF/Stegano pixel patterns, PDF xref covert channels, ANSI escape code stego, and ECB image recovery.

## Table of Contents
- [FFT Frequency Domain Steganography (Pragyan 2026)](#fft-frequency-domain-steganography-pragyan-2026)
- [SSTV Red Herring + LSB Audio Stego (0xFun 2026)](#sstv-red-herring--lsb-audio-stego-0xfun-2026)
- [DotCode Barcode via SSTV (0xFun 2026)](#dotcode-barcode-via-sstv-0xfun-2026)
- [DTMF Audio Decoding](#dtmf-audio-decoding)
- [Custom Frequency DTMF / Dual-Tone Keypad Encoding (EHAX 2026)](#custom-frequency-dtmf--dual-tone-keypad-encoding-ehax-2026)
- [Multi-Track Audio Differential Subtraction (EHAX 2026)](#multi-track-audio-differential-subtraction-ehax-2026)
- [Cross-Channel Multi-Bit LSB Steganography (ApoorvCTF 2026)](#cross-channel-multi-bit-lsb-steganography-apoorvctf-2026)
- [Audio FFT Musical Note Identification (BYPASS CTF 2025)](#audio-fft-musical-note-identification-bypass-ctf-2025)
- [Audio Metadata Octal Encoding (BYPASS CTF 2025)](#audio-metadata-octal-encoding-bypass-ctf-2025)
- [Nested Tar Archive with Whitespace Encoding (UTCTF 2026)](#nested-tar-archive-with-whitespace-encoding-utctf-2026)
- [DeepSound Audio Steganography with Password Cracking (INShAck 2018)](#deepsound-audio-steganography-with-password-cracking-inshack-2018)
- [Audio Waveform Binary Encoding (BackdoorCTF 2013)](#audio-waveform-binary-encoding-backdoorctf-2013)
- [Audio Spectrogram Hidden QR Code (BaltCTF 2013)](#audio-spectrogram-hidden-qr-code-baltctf-2013)
- [Byte-Reversed .docx ZIP Bidirectional Archive (Security Fest CTF 2018)](#byte-reversed-docx-zip-bidirectional-archive-security-fest-ctf-2018)
- [MIDI Note-On/Note-Off Pitch Pair Encoding (X-MAS CTF 2018)](#midi-note-onnote-off-pitch-pair-encoding-x-mas-ctf-2018)

---

## FFT Frequency Domain Steganography (Pragyan 2026)

**Pattern (H@rDl4u6H):** Image encodes data in frequency domain via 2D FFT.

**Decoding workflow:**
```python
import numpy as np
from PIL import Image

img = np.array(Image.open("image.png")).astype(float)
F = np.fft.fftshift(np.fft.fft2(img))
mag = np.log(1 + np.abs(F))

# Look for patterns: concentric rings, dots at specific positions
# Bright peak = 0 bit, Dark (no peak) = 1 bit
cy, cx = mag.shape[0]//2, mag.shape[1]//2
radii = [100 + 69*i for i in range(21)]  # Example spacing
angles = [0, 22.5, 45, 67.5, 90, 112.5, 135, 157.5]
THRESHOLD = 13.0

bits = []
for r in radii:
    byte_val = 0
    for a in angles:
        fx = cx + r * np.cos(np.radians(a))
        fy = cy - r * np.sin(np.radians(a))
        bit = 0 if mag[int(round(fy)), int(round(fx))] > THRESHOLD else 1
        byte_val = (byte_val << 1) | bit
    bits.append(byte_val)
```

**Identification:** Challenge mentions "transform", poem about "frequency", or image looks blank/noisy. Try FFT visualization first.

---

## SSTV Red Herring + LSB Audio Stego (0xFun 2026)

**Pattern (Melodie):** WAV contains SSTV signal (Scottie 1) that decodes to "SEEMS LIKE A DEADEND". Real flag in 2-bit LSB of audio samples.

```bash
# Decode SSTV (red herring)
qsstv  # Will show decoy message

# Extract real flag from LSB
pip install stego-lsb
stegolsb wavsteg -r -i audio.wav -o out.bin -n 2 -b 1000
```

**Lesson:** Obvious signals may be decoys. Always check LSB even when another encoding is found.

---

## DotCode Barcode via SSTV (0xFun 2026)

**Pattern (Dots):** SSTV decoding produces dot pattern image. Not QR — it's DotCode format.

**Identification:** Dot pattern that isn't a standard QR code. DotCode is a 2D barcode optimized for high-speed printing.

**Tool:** Aspose online DotCode reader (free).

---

## DTMF Audio Decoding

**Pattern (Phone Home):** Audio file contains phone dialing tones encoding data.

```bash
# Decode DTMF tones
sox phonehome.wav -t raw -r 22050 -e signed-integer -b 16 -c 1 - | \
    multimon-ng -t raw -a DTMF -
```

**Post-processing:** Phone number may contain octal-encoded ASCII after delimiter (#):
```python
# Convert octal groups to ASCII
octal_groups = ["115", "145", "164", "141"]  # M, e, t, a
flag = ''.join(chr(int(g, 8)) for g in octal_groups)
```

---

## Custom Frequency DTMF / Dual-Tone Keypad Encoding (EHAX 2026)

**Pattern (Quantum Message):** Audio with dual-tone sequences at non-standard frequencies, aligned at regular intervals (e.g., every 1 second). Hints about "harmonic oscillators" or physics point to custom frequency design.

**Identification:** Spectrogram shows two distinct frequency sets that don't match standard DTMF (697-1633 Hz). Look for evenly-spaced rows/columns of frequency tones.

**Decoding workflow:**
```python
import numpy as np
from scipy.io import wavfile

rate, audio = wavfile.read('challenge.wav')

# 1. Generate spectrogram to identify frequency grid
# Use ffmpeg: ffmpeg -i challenge.wav -lavfi showspectrumpic=s=1920x1080 spec.png

# 2. Map frequencies to keypad (custom grid, NOT standard DTMF)
# Example: rows = [301, 902, 1503, 2104] Hz, cols = [2705, 3306, 3907] Hz
# Forms 4x3 keypad -> digits 0-9 + symbols

# 3. Extract tone pairs per time window
window_size = rate  # 1 second per symbol
for i in range(0, len(audio), window_size):
    segment = audio[i:i+window_size]
    freqs = np.fft.rfftfreq(len(segment), 1/rate)
    magnitude = np.abs(np.fft.rfft(segment))
    # Find two dominant peaks -> map to row/col -> digit

# 4. Convert digit sequence to ASCII
# Split digits into variable-length groups (ASCII range 32-126)
# E.g., "72101108108111" -> [72, 101, 108, 108, 111] -> "Hello"
def digits_to_ascii(digits):
    result, i = [], 0
    while i < len(digits):
        for length in [2, 3]:  # ASCII codes are 2-3 digits
            if i + length <= len(digits):
                val = int(digits[i:i+length])
                if 32 <= val <= 126:
                    result.append(chr(val))
                    i += length
                    break
        else:
            i += 1
    return ''.join(result)
```

**Key insight:** When tones don't match standard DTMF frequencies, generate a spectrogram first to identify the custom frequency grid. The mapping is challenge-specific.

---

## Multi-Track Audio Differential Subtraction (EHAX 2026)

**Pattern (Penguin):** MKV/video file with two nearly-identical audio tracks. Hidden data is embedded as a tiny difference between the tracks, invisible when listening to either individually.

**Identification:**
- `ffprobe` reveals multiple audio streams (e.g., two stereo FLAC tracks)
- Metadata may contain a decoy flag (e.g., in comments)
- Track labels may be misleading (e.g., stereo labeled as "5.1 surround")
- `sox --info` / `sox -n stat` shows nearly identical RMS, amplitude, and frequency statistics for both tracks

**Extraction workflow:**
```bash
# 1. Extract both audio tracks
ffmpeg -i challenge.mkv -map 0:a:0 -c copy track0.flac
ffmpeg -i challenge.mkv -map 0:a:1 -c copy track1.flac

# 2. Convert to WAV for processing
ffmpeg -i track0.flac track0.wav
ffmpeg -i track1.flac track1.wav

# 3. Subtract: invert one track and mix (cancels shared content)
sox -m track0.wav "|sox track1.wav -p vol -1" diff.wav

# 4. Normalize the difference signal
sox diff.wav diff_norm.wav gain -n -3

# 5. Generate spectrogram to read the flag
sox diff_norm.wav -n spectrogram -o spectrogram.png -X 2000 -Y 1000 -z 100 -h

# 6. Optional: filter to isolate flag frequency range
sox diff_norm.wav filtered.wav sinc 5000-12000
sox filtered.wav -n spectrogram -o filtered_spec.png -X 2000 -Y 1000 -z 100 -h
```

**Key insight:** When two audio tracks are nearly identical, subtracting one from the other (phase inversion + mix) cancels shared content and isolates hidden data. The flag is typically encoded as text in the spectrogram of the difference signal, visible in a specific frequency band (e.g., 5-12 kHz).

**Common traps:**
- Decoy flags in metadata/comments — always verify
- Mislabeled channel configurations (stereo as 5.1)
- Flag may only be visible in a narrow time window — use high-resolution spectrogram (`-X 2000+`)

---

## Cross-Channel Multi-Bit LSB Steganography (ApoorvCTF 2026)

**Pattern (Beneath the Armor):** Standard LSB tools (zsteg, stegsolve) fail because different bit positions are used per RGB channel: Red channel bit 0, Green channel bit 1, Blue channel bit 2.

```python
from PIL import Image

img = Image.open("challenge.png")
pixels = img.load()
bits = []
for y in range(img.height):
    for x in range(img.width):
        r, g, b = pixels[x, y][:3]
        bits.append((r >> 0) & 1)  # Red: bit 0
        bits.append((g >> 1) & 1)  # Green: bit 1
        bits.append((b >> 2) & 1)  # Blue: bit 2

# Pack 3 bits per pixel into bytes
data = bytearray()
for i in range(0, len(bits) - 7, 8):
    byte = 0
    for j in range(8):
        byte = (byte << 1) | bits[i + j]
    data.append(byte)
print(data.decode('ascii', errors='ignore'))
```

**Key insight:** When standard LSB tools find nothing, the data may use different bit positions per channel. The hint "cycles" or "modular" suggests cycling through bit positions (0→1→2) across channels. Always try non-standard bit combinations: R[0]G[1]B[2], R[1]G[2]B[0], R[2]G[0]B[1], etc.

**Detection:** Standard `zsteg -a` and `stegsolve` produce no results on an image that metadata hints contain hidden data.

---

## Audio FFT Musical Note Identification (BYPASS CTF 2025)

**Pattern (Piano):** Identify dominant frequencies via FFT (Fast Fourier Transform), map to musical notes (A-G), then read the letter names as a word.

**Technique:** Perform FFT on audio, identify dominant frequencies, map to musical notes.

```python
import numpy as np
from scipy.io import wavfile

rate, audio = wavfile.read('challenge.wav')
if audio.ndim > 1:
    audio = audio[:, 0]  # mono

# FFT to find dominant frequencies
freqs = np.fft.rfftfreq(len(audio), 1/rate)
magnitude = np.abs(np.fft.rfft(audio))

# Find top peaks
peak_indices = np.argsort(magnitude)[-20:]
peak_freqs = sorted(set(round(freqs[i]) for i in peak_indices if freqs[i] > 20))

# Musical note frequency mapping (A4 = 440 Hz)
NOTE_FREQS = {
    'C4': 261.63, 'D4': 293.66, 'E4': 329.63, 'F4': 349.23,
    'G4': 392.00, 'A4': 440.00, 'B4': 493.88,
    'C5': 523.25, 'D5': 587.33, 'E5': 659.25, 'F5': 698.46,
    'G5': 783.99, 'A5': 880.00, 'B5': 987.77,
}

def freq_to_note(freq):
    return min(NOTE_FREQS.items(), key=lambda x: abs(x[1] - freq))[0]

notes = [freq_to_note(f) for f in peak_freqs]
# Extract letter names: B, A, D, F, A, C, E → "BADFACE"
answer = ''.join(n[0] for n in notes)
print(f"Notes: {notes}")
print(f"Answer: {answer}")
```

**Extract and examine audio metadata** using `exiftool audio.mp3` for encoded hints in comment fields (e.g., octal-separated values → base64 → decoded hint).

**Key insight:** Musical note names (A-G) can spell words. When a challenge involves music/piano, identify dominant frequencies via FFT and read the note letter names as text.

---

## Audio Metadata Octal Encoding (BYPASS CTF 2025)

**Pattern (Piano metadata):** Audio file metadata (exiftool comment field) contains underscore-separated numbers representing octal-encoded ASCII values (digits 0-7 only).

```python
# Extract and decode octal metadata
import subprocess, base64

# Get metadata comment
comment = "103_137_63_157_144_145_144_40_162_145_154_151_143"
octal_values = comment.split('_')
decoded = ''.join(chr(int(v, 8)) for v in octal_values)

# May decode to base64, requiring another layer
result = base64.b64decode(decoded).decode()
print(result)
```

**Key insight:** When metadata contains underscore-separated numbers, try octal (digits 0-7 only), decimal, or hex interpretation. Multi-layer encoding (octal → base64 → plaintext) is common.

---

## Nested Tar Archive with Whitespace Encoding (UTCTF 2026)

**Pattern (Silent Archive):** Deeply nested tar archives where data is encoded in whitespace characters (spaces, tabs, newlines) within file names or content.

**Detection:** Archive extracts to another archive (tar-in-tar chain). File content appears empty but contains invisible whitespace characters.

**Decoding workflow:**
```python
import tarfile
import os

# 1. Recursively extract nested tar archives
def extract_all(path, depth=0):
    if depth > 100:  # Guard against infinite nesting
        return
    if tarfile.is_tarfile(path):
        with tarfile.open(path) as tf:
            tf.extractall(f'layer_{depth}')
            for member in tf.getmembers():
                extract_all(f'layer_{depth}/{member.name}', depth + 1)

# 2. Collect whitespace from file names or content
whitespace_data = []
for root, dirs, files in os.walk('layer_0'):
    for f in files:
        path = os.path.join(root, f)
        with open(path, 'rb') as fh:
            content = fh.read()
            # Check for whitespace-only content
            if content.strip() == b'':
                for byte in content:
                    if byte == 0x20:  # space
                        whitespace_data.append('0')
                    elif byte == 0x09:  # tab
                        whitespace_data.append('1')

# 3. Convert binary from whitespace
bits = ''.join(whitespace_data)
message = bytes(int(bits[i:i+8], 2) for i in range(0, len(bits)-7, 8))
print(message.decode(errors='replace'))
```

**Whitespace encoding variants:**
- Space = 0, Tab = 1 (binary encoding)
- Whitespace Steganography: trailing spaces/tabs at end of lines
- Zero-width characters (U+200B, U+200C, U+FEFF) in Unicode text
- Number of spaces between words encodes data

**Key insight:** "Silent" or "invisible" hints point to whitespace encoding. Use `xxd` or `cat -A` to reveal hidden whitespace characters. Deeply nested archives are misdirection — the data is in the whitespace, not the nesting depth.

---

## DeepSound Audio Steganography with Password Cracking (INShAck 2018)

**Pattern:** Two-phase audio steganography: part 1 visible in Audacity spectrogram, part 2 hidden with DeepSound tool (password-protected). Use `deepsound2john.py` to extract the hash, crack with John, then retrieve hidden files.

```bash
# Phase 1: Check spectrogram for visible text
sox audio.wav -n spectrogram -o spec.png

# Phase 2: Extract DeepSound password hash
python3 deepsound2john.py audio.wav > hash.txt

# Crack password
john --wordlist=rockyou.txt hash.txt

# Extract hidden file with DeepSound GUI or CLI using cracked password
```

**DeepSound detection:**
```python
# DeepSound embeds a signature in WAV files
# Check for DeepSound header pattern in audio data
with open('audio.wav', 'rb') as f:
    data = f.read()
    # DeepSound uses specific byte patterns in the audio data section
    # deepsound2john.py from John the Ripper's bleeding-jumbo branch
    # handles detection and hash extraction automatically
```

**Tool installation:**
```bash
# deepsound2john.py is part of John the Ripper bleeding-jumbo
git clone https://github.com/openwall/john.git
# Script located at: john/run/deepsound2john.py

# DeepSound GUI (Windows): http://jpinsoft.net/deepsound/
# For Linux: run under Wine or use the extracted hash + john approach
```

**Key insight:** DeepSound embeds files in WAV audio with optional AES encryption. The password hash is extractable with `deepsound2john.py` from John the Ripper's bleeding-jumbo branch. Always check both spectrogram (visual stego) and DeepSound (data stego) in audio challenges.

**Detection:** WAV file that seems normal but `deepsound2john.py` produces a hash. Challenge has two-part structure where first part is easy (spectrogram) and second part requires a tool. Challenge mentions "layers", "hidden", or "deep".

---

## Audio Waveform Binary Encoding (BackdoorCTF 2013)

**Pattern:** WAV file contains two distinct waveform shapes representing binary 0 and 1. Group 8 bits into bytes and decode as ASCII.

```python
import wave, struct
wf = wave.open('audio.wav', 'rb')
frames = wf.readframes(wf.getnframes())
samples = struct.unpack(f'{len(frames)//2}h', frames)

# Identify two distinct wave patterns (e.g., positive peak vs flat)
# Segment audio into fixed-length windows, classify each as 0 or 1
bits = ''
window = len(samples) // num_bits
for i in range(num_bits):
    segment = samples[i*window:(i+1)*window]
    bits += '1' if max(segment) > threshold else '0'

# Decode binary to ASCII
flag = ''.join(chr(int(bits[i:i+8], 2)) for i in range(0, len(bits)-7, 8))
```

**Key insight:** Open in Audacity and zoom in — two visually distinct wave patterns alternate. Each pattern represents one bit. Count the patterns, group into 8-bit bytes, decode as ASCII.

---

## Audio Spectrogram Hidden QR Code (BaltCTF 2013)

**Pattern:** Audio file contains visual data hidden in the frequency domain, visible only in a spectrogram view.

```bash
# Generate spectrogram image
sox audio.mp3 -n spectrogram -o spec.png
# Or use Sonic Visualiser for interactive exploration

# Look for visual patterns in specific frequency bands (often 5-12 kHz)
# Extract/assemble QR code fragments from spectrogram
# Scan with: zbarimg assembled_qr.png
```

**Key insight:** Use Sonic Visualiser (Layer → Add Spectrogram) with adjustable window size and color mapping. QR codes or text often appear in the 2-15 kHz band. Multiple spectrogram fragments may need to be stitched together in an image editor before scanning.

---

## Byte-Reversed .docx ZIP Bidirectional Archive (Security Fest CTF 2018)

**Pattern (Zion):** Distributed file is a valid `.docx` (ZIP archive). Extract it normally and you see only a decoy document. Reverse the entire file byte-for-byte and the result is *also* a valid ZIP archive — containing a second `word/media/*.png` whose contents are the flag.

**Extraction:**
```bash
# Verify the forward archive
unzip -l doc.docx

# Reverse the byte stream and unpack the mirror archive
python3 -c "import sys;sys.stdout.buffer.write(open('doc.docx','rb').read()[::-1])" > mirror.zip
unzip -l mirror.zip
unzip mirror.zip 'word/media/*' -d mirror/
```

**Why both directions succeed:** ZIP's central directory sits at the end of the archive and the local file headers are parsed only via offsets in that directory. By placing a second set of local headers at the *start* of the file, and a matching central directory at the very end after reversing, the file satisfies the ZIP specification in both reading orders. Python's `zipfile` and `unzip -l` read the central directory from the tail, so they happily open whichever end is presented first.

**Key insight:** Always test container files for byte-reversal, bit-reversal, and byte-interleaving when the forward extraction yields only a decoy. Run `binwalk` on both the forward and reversed copies to surface embedded archives hidden in either direction. The trick generalizes to any format whose parser tolerates trailing garbage (ZIP, RAR, PDF, tar).

**References:** Security Fest CTF 2018 — writeup 10204

---

## MIDI Note-On/Note-Off Pitch Pair Encoding (X-MAS CTF 2018)

**Pattern:** MIDI file plays nothing recognisable but has a strict alternating Note-On/Note-Off pattern. The hidden message is split one byte per *pair*: `ord(char) = note_on_pitch + note_off_pitch`. Sometimes encoded as high/low nibble: `ord(char) = (on << 4) | off`.

```python
import mido
mid = mido.MidiFile('hidden.mid')
ons, offs = [], []
for ev in mid.tracks[0]:
    if ev.type == 'note_on':   ons.append(ev.note)
    elif ev.type == 'note_off': offs.append(ev.note)
flag = ''.join(chr(o + f) for o, f in zip(ons, offs))
```

**Key insight:** MIDI pitch values are 7-bit (0..127), so any byte can be split across two notes. When a MIDI sounds "atonal" but obeys strict note-pair alternation, test `on + off`, `(on<<4)|off`, `off - on`, and XOR combinations before assuming audio stego.

**References:** X-MAS CTF 2018 — A Christmas Carol, writeup 12667

---

