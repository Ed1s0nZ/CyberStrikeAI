# CTF Forensics - Signals and Hardware

## Table of Contents
- [VGA Signal Decoding](#vga-signal-decoding)
- [HDMI TMDS Decoding](#hdmi-tmds-decoding)
- [DisplayPort 8b/10b + LFSR Decoding](#displayport-8b10b--lfsr-decoding)
- [Voyager Golden Record Audio (0xFun 2026)](#voyager-golden-record-audio-0xfun-2026)
- [Side-Channel Power Analysis (EHAX 2026)](#side-channel-power-analysis-ehax-2026)
- [Saleae Logic 2 UART Decode (EHAX 2026)](#saleae-logic-2-uart-decode-ehax-2026)
- [Flipper Zero .sub File (0xFun 2026)](#flipper-zero-sub-file-0xfun-2026)
- [Keyboard Acoustic Side-Channel (ApoorvCTF 2026)](#keyboard-acoustic-side-channel-apoorvctf-2026)
- [CD Audio Disc Image Steganography (BSidesSF 2026)](#cd-audio-disc-image-steganography-bsidessf-2026)
- [Caps-Lock LED Morse Code Extraction from Video (STEM CTF 2018)](#caps-lock-led-morse-code-extraction-from-video-stem-ctf-2018)
- [Linux input_event Keylogger Dump Parsing (Pwn2Win 2016)](#linux-input_event-keylogger-dump-parsing-pwn2win-2016)
- [I2C Bus Protocol Decoding (EKOPARTY CTF 2016)](#i2c-bus-protocol-decoding-ekoparty-ctf-2016)
- [IBM-29 Punched Card OCR (EKOPARTY CTF 2016)](#ibm-29-punched-card-ocr-ekoparty-ctf-2016)
- [Serial UART Data Decoding from WAV Audio (EasyCTF 2017)](#serial-uart-data-decoding-from-wav-audio-easyctf-2017)
- [USB MIDI Launchpad Traffic Reconstruction (Sthack 2017)](#usb-midi-launchpad-traffic-reconstruction-sthack-2017)
- [Tektronix Logic-Analyzer CSV Clock-Edge Extraction (35C3 2018)](#tektronix-logic-analyzer-csv-clock-edge-extraction-35c3-2018)

---

## VGA Signal Decoding

**Frame structure:** 800x525 total (640x480 active + blanking). Each sample = 5 bytes: R, G, B, HSync, VSync. Color is 6-bit (0-63).

```python
import numpy as np
from PIL import Image

data = open('vga.bin', 'rb').read()

TOTAL_W, TOTAL_H = 800, 525
ACTIVE_W, ACTIVE_H = 640, 480
BYTES_PER_SAMPLE = 5  # R, G, B, hsync, vsync

# Parse raw samples
samples = np.frombuffer(data, dtype=np.uint8).reshape(-1, BYTES_PER_SAMPLE)
frame = samples.reshape(TOTAL_H, TOTAL_W, BYTES_PER_SAMPLE)

# Extract active region, scale 6-bit to 8-bit
active = frame[:ACTIVE_H, :ACTIVE_W, :3]  # RGB only
img_arr = (active.astype(np.uint16) * 4).clip(0, 255).astype(np.uint8)
Image.fromarray(img_arr).save('vga_output.png')
```

**Key lesson:** Total frame > visible area — always crop blanking. If colors look dark, check if 6-bit (multiply by 4).

---

## HDMI TMDS Decoding

**Structure:** 3 channels (R, G, B), each encoded as 10-bit TMDS (Transition-Minimized Differential Signaling) symbols. Bit 9 = inversion flag, bit 8 = XOR/XNOR mode. Decode is deterministic from MSBs down.

```python
def tmds_decode(symbol_10bit):
    """Decode a 10-bit TMDS symbol to 8-bit pixel value."""
    bits = [(symbol_10bit >> i) & 1 for i in range(10)]
    # bits[9] = inversion flag, bits[8] = XOR/XNOR mode

    # Step 1: undo optional inversion (bit 9)
    if bits[9]:
        d = [1 - bits[i] for i in range(8)]
    else:
        d = [bits[i] for i in range(8)]

    # Step 2: undo XOR/XNOR chain (bit 8 selects mode)
    q = [d[0]]
    if bits[8]:
        for i in range(1, 8):
            q.append(d[i] ^ q[i-1])        # XOR mode
    else:
        for i in range(1, 8):
            q.append(d[i] ^ q[i-1] ^ 1)    # XNOR mode

    return sum(q[i] << i for i in range(8))

# Parse: read 10-bit symbols from binary, group into 3 channels
# Frame is 800x525 total, crop to 640x480 active
```

**Identification:** Binary data with 10-bit aligned structure. Challenge mentions HDMI, DVI, or TMDS.

---

## DisplayPort 8b/10b + LFSR Decoding

**Structure:** 10-bit 8b/10b symbols decoded to 8-bit data, then LFSR-descrambled. Organized in 64-column Transport Units (60 data columns + 4 overhead).

```python
# Standard 8b/10b decode table (partial — full table has 256 entries)
# Use a prebuilt table: map 10-bit symbol -> 8-bit data
# Key: running disparity tracks DC balance

# LFSR descrambler (x^16 + x^5 + x^4 + x^3 + 1)
def lfsr_descramble(data):
    """DisplayPort LFSR descrambler. Resets on control symbols (BS/BE)."""
    lfsr = 0xFFFF  # Initial state
    result = []
    for byte in data:
        out = byte
        for bit_idx in range(8):
            feedback = (lfsr >> 15) & 1
            out ^= (feedback << bit_idx)
            new_bit = ((lfsr >> 15) ^ (lfsr >> 4) ^ (lfsr >> 3) ^ (lfsr >> 2)) & 1
            lfsr = ((lfsr << 1) | new_bit) & 0xFFFF
        result.append(out & 0xFF)
    return bytes(result)

# Transport Unit layout: 64 columns per TU
# Columns 0-59: pixel data (RGB)
# Columns 60-63: overhead (sync, stuffing)
# LFSR resets on control bytes (BS=0x1C, BE=0xFB)
```

**Key lesson:** LFSR scrambler resets on control bytes — identify these to synchronize descrambling. Without reset points, output is garbled.

---

## Voyager Golden Record Audio (0xFun 2026)

**Pattern (11 Lines of Contact):** Analog image encoded as audio. Sync pulses (sharp negative spikes) delimit scan lines. Amplitude between pulses = pixel brightness.

```python
import numpy as np
from scipy.io import wavfile
from PIL import Image

rate, audio = wavfile.read('golden_record.wav')
audio = audio.astype(np.float32)

# Find sync pulses (sharp negative spikes below threshold)
threshold = np.min(audio) * 0.7
sync_indices = np.where(audio < threshold)[0]

# Group consecutive sync samples into pulse starts
pulses = [sync_indices[0]]
for i in range(1, len(sync_indices)):
    if sync_indices[i] - sync_indices[i-1] > 100:
        pulses.append(sync_indices[i])

# Extract scan lines between pulses, resample to fixed width
WIDTH = 512
lines = []
for i in range(len(pulses) - 1):
    line = audio[pulses[i]:pulses[i+1]]
    resampled = np.interp(np.linspace(0, len(line)-1, WIDTH), np.arange(len(line)), line)
    lines.append(resampled)

# Normalize and save as image
img_arr = np.array(lines)
img_arr = ((img_arr - img_arr.min()) / (img_arr.max() - img_arr.min()) * 255).astype(np.uint8)
Image.fromarray(img_arr).save('voyager_image.png')
```

---

## Side-Channel Power Analysis (EHAX 2026)

**Pattern (Power Leak):** Power consumption traces recorded during cryptographic operations. Correct key guesses cause measurably different power consumption at specific sample points.

**Data format:** Typically a multi-dimensional array: `[positions × guesses × traces × samples]`. E.g., 6 digit positions × 10 guesses (0-9) × 20 traces × 50 samples.

**Attack (Differential Power Analysis):**
```python
import numpy as np
import hashlib

# Load power traces: shape = (positions, guesses, traces, samples)
data = np.load('power_traces.npy')  # or parse from CSV/JSON
n_positions, n_guesses, n_traces, n_samples = data.shape

# For each position, find the guess with maximum power at the leak point
key_digits = []
for pos in range(n_positions):
    # Average across traces for each guess
    avg_power = data[pos].mean(axis=1)  # shape: (guesses, samples)

    # Find the sample point with maximum power variance across guesses
    # This is the "leak point" where the correct guess stands out
    variance_per_sample = avg_power.var(axis=0)
    leak_sample = np.argmax(variance_per_sample)

    # The guess with maximum power at the leak point is correct
    best_guess = np.argmax(avg_power[:, leak_sample])
    key_digits.append(best_guess)

key = ''.join(str(d) for d in key_digits)
print(f"Recovered key: {key}")

# Flag may be SHA256 of the key
flag = hashlib.sha256(key.encode()).hexdigest()
```

**Identification:** Challenge mentions "power", "side-channel", "leakage", "traces", or "measurements". Data is a multi-dimensional numeric array with axes for positions/guesses/traces/samples.

**Key insight:** The "leak point" is the sample index where correct vs incorrect guesses show the largest power difference. Average across traces first to reduce noise, then find the sample with maximum variance across guesses.

---

## Saleae Logic 2 UART Decode (EHAX 2026)

**Pattern (Baby Serial):** Saleae Logic 2 `.sal` file (ZIP archive) containing digital channel captures. Data encoded as UART serial.

**File structure:** `.sal` is a ZIP containing `digital-0.bin` through `digital-7.bin` + `meta.json`. Only channel 0 typically has data.

**Binary format (digital-*.bin):**
```text
<SALEAE> magic (8 bytes)
version: u32 = 2
type: u32 = 100 (digital)
initial_state: u32 (0 or 1)
... header fields ...
Delta-encoded transitions (variable-length integers)
```

**Delta encoding:** Each value represents the number of samples between state transitions. The signal alternates between HIGH and LOW at each delta.

**UART decode from deltas:**
```python
import numpy as np

# Parse deltas from binary (after header)
# Reconstruct signal timeline
times = np.cumsum(deltas)
states = []
state = initial_state
for d in deltas:
    states.append(state)
    state ^= 1  # toggle on each transition

# UART decode: detect start bit (HIGH→LOW), sample 8 data bits at bit centers
# Baud rate detection: most common delta ≈ samples_per_bit
# At 1MHz sample rate: 115200 baud ≈ 8.7 samples/bit

def uart_decode(transitions, sample_rate=1_000_000, baud=115200):
    bit_period = sample_rate / baud
    bytes_out = []
    i = 0
    while i < len(transitions):
        # Find start bit (falling edge)
        if transitions[i] == 0:  # LOW = start bit
            byte_val = 0
            for bit in range(8):
                sample_time = (1.5 + bit) * bit_period  # center of each bit
                # Sample signal at this offset from start bit
                bit_val = get_signal_at(sample_time)
                byte_val |= (bit_val << bit)  # LSB first
            bytes_out.append(byte_val)
        i += 1
    return bytes(bytes_out)
```

**Common pitfalls:**
- **Inverted polarity:** UART idle is HIGH (mark). If initial_state=1, the encoding may be inverted — try both
- **Baud rate guessing:** Check common rates: 9600, 19200, 38400, 57600, 115200, 230400
- **Output format:** Decoded bytes may be base64-encoded (containing a PNG image or text)
- **Saleae internal format ≠ export format:** The `.sal` internal binary uses a different encoding than CSV/binary export. Parse the raw delta transitions directly

**Quick approach:** Install Saleae Logic 2, open the `.sal` file, add UART analyzer with auto-baud detection, export decoded data.

---

## Flipper Zero .sub File (0xFun 2026)

RAW_Data binary -> filter noise bytes (0x80-0xFF) -> expand batch variable references -> XOR with hint text.

**Key insight:** Flipper Zero `.sub` files contain raw RF signal data. The RAW_Data field encodes binary as pulse timings. Filter out noise bytes (0x80-0xFF), expand any batch variable references, and XOR with hint text from the challenge to recover the flag.

---

## Keyboard Acoustic Side-Channel (ApoorvCTF 2026)

**Pattern (Author on the Run):** Recover typed text from audio recordings of keystrokes. Reference audio provides labeled samples (known keys), flag audio contains unknown keystrokes to classify.

**Step 1 — Detect keystrokes via energy peaks:**
```python
import numpy as np
from scipy.signal import find_peaks
from scipy.io import wavfile

sr, audio = wavfile.read('flag.wav')
if audio.ndim > 1:
    audio = audio.mean(axis=1)

# Sliding window energy envelope (10ms window)
win = int(0.01 * sr)
energy = np.array([np.sum(audio[i:i+win]**2) for i in range(0, len(audio) - win, win)])

# Find peaks with minimum 175ms separation
min_dist = int(0.175 * sr / win)
peaks, _ = find_peaks(energy, height=0.03 * energy.max(), distance=min_dist)
```

**Step 2 — Extract MFCC features per keystroke:**
```python
import librosa

def extract_features(audio, sr, peak_sample, window_ms=10):
    win = int(window_ms / 1000 * sr)
    start = max(0, peak_sample - win // 2)
    segment = audio[start:start + win]
    mfccs = librosa.feature.mfcc(y=segment.astype(float), sr=sr, n_mfcc=20)
    return np.concatenate([mfccs.mean(axis=1), mfccs.std(axis=1)])  # 40-dim
```

**Step 3 — Classify with KNN against labeled reference:**
```python
from sklearn.neighbors import KNeighborsClassifier

# Build reference from labeled audio (26 keys × 50 presses each)
X_ref, y_ref = [], []
for key_idx, key in enumerate('abcdefghijklmnopqrstuvwxyz'):
    for peak in reference_peaks[key_idx * 50:(key_idx + 1) * 50]:
        X_ref.append(extract_features(ref_audio, sr, peak))
        y_ref.append(key)

knn = KNeighborsClassifier(n_neighbors=5)
knn.fit(X_ref, y_ref)

# Classify flag keystrokes
flag = ''.join(knn.predict([extract_features(flag_audio, sr, p) for p in flag_peaks]))
```

**Key insight:** Window size is critical — 10ms captures the initial impact transient which is most distinctive per key. Larger windows (20-30ms) include key release noise that reduces classification accuracy. Use all individual reference samples rather than averaging, as KNN handles variance better with more data points.

**Detection:** Two audio files provided (reference + target), or challenge mentions "typing", "keyboard", "acoustic".

---

## CD Audio Disc Image Steganography (BSidesSF 2026)

**Pattern (cdimage):** Visual images encoded as pit/land patterns on a CD surface. A `.cdda` file (raw CD Digital Audio) contains only two byte values (e.g., `0x0d` and `0xa8`) representing reflective lands and non-reflective pits. When rendered as a spiral on a disc image, the binary pattern forms readable text or images — similar to LightScribe but using the data layer.

**Key components:**
1. **CIRC de-interleaving** — CD audio data is Cross-Interleaved for error correction. The encoding tool (e.g., [arduinocelentano/cdimage](https://github.com/arduinocelentano/cdimage)) pre-interleaves data to compensate. To decode, reverse the CIRC interleaving before rendering.
2. **Spiral geometry** — bytes per track increases linearly: `tr(n) = tr0 + n * dtr`, physical radius `r(n) = r0 + n * dr`. Default params: `tr0=22951.52`, `dtr=1.387`, `r0=24.5mm`.
3. **Polar-to-Cartesian rendering** — accumulate byte values into a polar grid `(radius_pixel, angle_bin)`, then convert to a circular disc image.

**De-interleaving (CIRC reverse):**

```python
import numpy as np

def deinterleave_cdda(data):
    """Reverse CIRC pre-interleaving from cdimage tool."""
    D = 4
    delays = [
        -24*(3),          -24*(1*D+2)+1,    8-24*(2*D+3),    8-24*(3*D+2)+1,
        16-24*(4*D+3),    16-24*(5*D+2)+1,  2-24*(6*D+3),    2-24*(7*D+2)+1,
        10-24*(8*D+3),    10-24*(9*D+2)+1,  18-24*(10*D+3),  18-24*(11*D+2)+1,
        4-24*(16*D+1),    4-24*(17*D)+1,    12-24*(18*D+1),  12-24*(19*D)+1,
        20-24*(20*D+1),   20-24*(21*D)+1,   6-24*(22*D+1),   6-24*(23*D)+1,
        14-24*(24*D+1),   14-24*(25*D)+1,   22-24*(26*D+1),  22-24*(27*D)+1
    ]
    # Build per-output-index offset: output[g*24+i] came from input[g*24+i + offset[i]]
    offsets = [0] * 24
    for pinf in range(24):
        i = delays[pinf] % 24
        if i < 0:
            i += 24
        dg = (i - delays[pinf]) // 24
        offsets[i] = -(111 - dg) * 24 + (pinf - i)

    total = len(data)
    result = np.zeros(total, dtype=np.uint8)
    for i in range(24):
        out_pos = np.arange(i, total, 24, dtype=np.int64)
        in_pos = out_pos + offsets[i]
        valid = (in_pos >= 0) & (in_pos < total)
        result[in_pos[valid]] = data[out_pos[valid]]
    return result
```

**Rendering de-interleaved data to disc image:**

```python
from PIL import Image

def render_cdda_disc(data, img_size=1024, tr0=22951.52052, dtr=1.3865961805,
                     r0=24.5, rcd=57.5, scale=0.115, n_angle_bins=8192,
                     bright_byte=0x0d):
    """Render de-interleaved CDDA data as a circular disc image."""
    center = img_size // 2
    dr = dtr * r0 / tr0
    polar_sum = np.zeros((img_size, n_angle_bins), dtype=np.float64)
    polar_count = np.zeros((img_size, n_angle_bins), dtype=np.float64)

    tr, r, pos, c_float = tr0, r0, 0, 0.0
    total = len(data)
    while c_float < (800 * 1024 * 1024 - tr) and pos < total:
        itr = int(tr)
        r_px = int(r / scale)
        if 0 <= r_px < img_size:
            end = min(pos + itr, total)
            chunk = data[pos:end]
            n_tb = len(chunk)
            if n_tb > 0:
                angles = (np.arange(n_tb, dtype=np.int64) * n_angle_bins // n_tb) % n_angle_bins
                is_bright = (chunk == bright_byte).astype(np.float64)
                np.add.at(polar_sum[r_px], angles, is_bright)
                np.add.at(polar_count[r_px], angles, 1.0)
        c_float += tr
        ic = pos + itr
        while int(c_float) > ic:
            ic += 1
        pos = ic
        tr += dtr
        r += dr

    density = np.where(polar_count > 0, polar_sum / polar_count, 0)
    ys, xs = np.mgrid[0:img_size, 0:img_size]
    dx, dy = (xs - center).astype(float), (ys - center).astype(float)
    r_arr = np.sqrt(dx * dx + dy * dy).astype(int)
    theta = np.arctan2(-dy, dx)
    theta[theta < 0] += 2 * np.pi
    a_idx = (theta / (2 * np.pi) * n_angle_bins).astype(int) % n_angle_bins
    output = density[np.clip(r_arr, 0, img_size - 1), a_idx]
    output[(r_arr < int(r0 / scale)) | (r_arr > int(rcd / scale))] = 0
    return Image.fromarray((output * 255).astype(np.uint8))

# Full pipeline
data = np.fromfile('flag.cdda', dtype=np.uint8)
deinterleaved = deinterleave_cdda(data)
img = render_cdda_disc(deinterleaved)
img.save('disc_output.png')
```

**Key insight:** Without CIRC de-interleaving, the radial structure (bright/dark rings) is visible but angular detail (text) is completely scrambled. The interleaving spreads each byte across ~108 groups (~2592 bytes), which at typical track lengths (~30K-50K bytes/revolution) shifts angular positions by up to 30 degrees — enough to destroy any readable pattern. The calibration image confirms correct decoding by showing known text.

**Calibration workflow:** The challenge provides `calibrate_img.cdda` with a known output (`calibrate_img.png` showing "Calibrate: 0123456789abc..."). Use this pair to verify geometry parameters (tr0, dtr, r0, scale) before decoding the flag file.

**Detection:** Challenge mentions "album", "CD rip", "CDDA", or provides large (~800MB) files with only 2 unique byte values. The `file` command reports "ISO-8859 text with CR line terminators" because `0x0d` (CR) is one of the two values.

---

## Caps-Lock LED Morse Code Extraction from Video (STEM CTF 2018)

**Pattern:** Extract Morse code from a security camera video by tracking the caps-lock LED pixel on a keyboard using OpenCV frame-by-frame analysis.

```python
import cv2

vidcap = cv2.VideoCapture('SecurityCamera.mp4')
morse = []
while vidcap.isOpened():
    ret, frame = vidcap.read()
    if not ret: break
    r, g, b = frame[58, 686]  # caps-lock LED pixel coordinate
    is_on = r > 200 and g > 200 and b > 200
    morse.append(is_on)

# Convert on/off durations to dots, dashes, and spaces
# Short on = dot, long on = dash, medium off = letter space, long off = word space
durations = []
current = morse[0]
count = 0
for state in morse:
    if state == current:
        count += 1
    else:
        durations.append((current, count))
        current = state
        count = 1
durations.append((current, count))

# Calibrate thresholds from observed durations
# Typical: dot=2-4 frames, dash=6-10 frames, letter gap=4-6 frames, word gap=10+ frames
MORSE_MAP = {
    '.-': 'A', '-...': 'B', '-.-.': 'C', '-..': 'D', '.': 'E',
    '..-.': 'F', '--.': 'G', '....': 'H', '..': 'I', '.---': 'J',
    '-.-': 'K', '.-..': 'L', '--': 'M', '-.': 'N', '---': 'O',
    '.--.': 'P', '--.-': 'Q', '.-.': 'R', '...': 'S', '-': 'T',
    '..-': 'U', '...-': 'V', '.--': 'W', '-..-': 'X', '-.--': 'Y',
    '--..': 'Z', '.----': '1', '..---': '2', '...--': '3',
    '....-': '4', '.....': '5', '-....': '6', '--...': '7',
    '---..': '8', '----.': '9', '-----': '0',
}
```

**Key insight:** Keyboard LEDs (caps lock, num lock, scroll lock) can be programmatically controlled and are visible in security camera footage. Track a specific pixel coordinate across video frames; on/off durations encode Morse code (short=dot, long=dash).

**Detection:** Video of a keyboard where an LED blinks irregularly. Challenge mentions "security camera", "keyboard", "blinking", or "Morse".

---

## Linux input_event Keylogger Dump Parsing (Pwn2Win 2016)

Raw binary dump with 24-byte repeating structure matching Linux's `struct input_event` (`struct timeval` + `__u16 type` + `__u16 code` + `__s32 value`). Filter for `type == EV_KEY (1)` and `value == 1` (key press), map keycodes via Linux kernel's `input-event-codes.h`.

```python
import struct
with open('dump.bin', 'rb') as f:
    while data := f.read(24):
        tv_sec, tv_usec, type_, code, value = struct.unpack('<QQHHi', data)
        if type_ == 1 and value == 1:  # EV_KEY, key press
            print(f"Key code: {code}")  # Map via input-event-codes.h
```

**Key insight:** `/dev/input/event*` captures have a fixed 24-byte `struct input_event` format. Filter EV_KEY type with value=1 for key presses. Map codes using Linux kernel header `input-event-codes.h`.

**Detection:** Binary file size divisible by 24. Challenge mentions keylogger, keyboard, or input device.

---

## I2C Bus Protocol Decoding (EKOPARTY CTF 2016)

Logic analyzer captures of I2C (Inter-Integrated Circuit) bus communications. Decode SDA (data) and SCL (clock) signals to extract transmitted bytes.

```python
def decode_i2c(sda_signal, scl_signal):
    """Decode I2C protocol from logic analyzer capture
    Channel 0 = SDA (data), Channel 1 = SCL (clock)

    I2C framing:
    - START: SDA falls while SCL is high
    - STOP: SDA rises while SCL is high
    - Data: SDA sampled on SCL rising edge
    - ACK: 9th bit (low = ACK, high = NACK)
    """
    bytes_out = []
    current_byte = 0
    bit_count = 0
    in_frame = False

    for i in range(len(scl_signal) - 1):
        # Detect START condition
        if sda_signal[i] == 1 and sda_signal[i+1] == 0 and scl_signal[i] == 1:
            in_frame = True
            bit_count = 0
            current_byte = 0
            continue

        # Detect STOP condition
        if sda_signal[i] == 0 and sda_signal[i+1] == 1 and scl_signal[i] == 1:
            in_frame = False
            continue

        # Sample data on SCL rising edge
        if in_frame and scl_signal[i] == 0 and scl_signal[i+1] == 1:
            if bit_count < 8:
                current_byte = (current_byte << 1) | sda_signal[i+1]
                bit_count += 1
            elif bit_count == 8:
                bytes_out.append(current_byte)
                bit_count = 0
                current_byte = 0

    return bytes_out

# Tools: Saleae Logic 2, sigrok/PulseView, OLS (Open Logic Sniffer)
# Import: File > Open Logic Sniffer capture
# Decode: Analyzers > I2C > Set SDA/SCL channels
```

**Key insight:** I2C uses only 2 wires (SDA + SCL). START/STOP conditions occur when SDA changes while SCL is high. Data bits are sampled on SCL rising edges. Every 9th bit is an ACK. Use logic analyzer software (Saleae, sigrok) for automated decoding.

---

## IBM-29 Punched Card OCR (EKOPARTY CTF 2016)

Decode IBM-29 keypunch card images by detecting hole positions in a standard 80-column x 12-row grid.

```python
from PIL import Image

# IBM-29 character encoding: column punch pattern -> character
IBM_029_MAP = {
    (12,): 'A', (12,1): 'A', (12,2): 'B', (12,3): 'C',  # etc.
    (11,): '-', (11,1): 'J', (11,2): 'K',  # etc.
    (0,): '0', (1,): '1', (2,): '2',  # zone 0 + digit
    # Full mapping: http://www.columbia.edu/cu/computinghistory/029.html
}

def decode_punched_card(image_path, cols=80, rows=12,
                        x_spacing=7, y_spacing=20, x_offset=10, y_offset=10):
    """Detect punches in card image and decode to text"""
    img = Image.open(image_path).convert('L')
    text = ""

    for col in range(cols):
        punches = []
        for row in range(rows):
            x = x_offset + col * x_spacing
            y = y_offset + row * y_spacing
            pixel = img.getpixel((x, y))
            if pixel > 200:  # white = punched hole
                punches.append(row)

        if punches:
            key = tuple(punches)
            text += IBM_029_MAP.get(key, '?')
        else:
            text += ' '

    return text

# Process multiple card images
for i in range(14):
    card_text = decode_punched_card(f'card_{i:02d}.png')
    print(f"Card {i}: {card_text}")
```

**Key insight:** IBM punched cards use a 12-row x 80-column grid. Each character is encoded by 1-3 holes in a column. The grid spacing varies by card reader/scanner resolution -- calibrate by measuring the distance between known reference holes. White/light pixels indicate punched holes.

---

## Serial UART Data Decoding from WAV Audio (EasyCTF 2017)

Audio files can contain serial (UART) data encoded as square wave signals. Decode by sampling amplitude levels and parsing bit timing.

```python
import struct

with open('signal.wav', 'rb') as f:
    f.read(44)  # skip WAV header
    samples = []
    while True:
        data = f.read(2)
        if not data: break
        samples.append(struct.unpack('<h', data)[0])

# Parameters: 9600 baud, 1 start bit, 8 data bits, no parity, 2 stop bits
SAMPLES_PER_BIT = len(samples) // expected_bits  # ~40 for 9600 baud @ 384kHz
THRESHOLD = 0  # above = 1, below = 0

# Convert samples to bits
bits = [1 if s > THRESHOLD else 0 for s in samples]

# Find frames: start bit (0) + 8 data bits + stop bits (1,1)
output = []
i = 0
while i < len(bits) - 11:
    if bits[i] == 0:  # start bit
        byte_bits = bits[i+1:i+9]  # LSB first
        byte_val = sum(b << j for j, b in enumerate(byte_bits))
        output.append(byte_val)
        i += 11  # skip start + 8 data + 2 stop
    else:
        i += 1

print(bytes(output))
```

**Key insight:** UART serial data in audio appears as a square wave with well-defined bit timing. Key parameters to determine: baud rate (samples per bit), frame format (start/stop bits, parity), and bit endianness (UART is LSB-first). The start bit (low) provides synchronization for each byte frame.

**Detection:** WAV file with a clean square wave pattern visible in Audacity. Two distinct amplitude levels with regular timing. Challenge mentions "serial", "UART", "baud", or "RS-232".

---

## USB MIDI Launchpad Traffic Reconstruction (Sthack 2017)

USB traffic from MIDI controller devices (e.g., Novation Launchpad) encodes button presses as MIDI Note On/Off messages that can be reconstructed into visual patterns.

```python
from scapy.all import rdpcap

pkts = rdpcap('capture.pcapng')
# Filter USB bulk transfer packets for MIDI data
# Launchpad MIDI: 0x90 = Note On, 0x80 = Note Off
# Format: [status, key, velocity]
# Key encodes (row, col): key = row*16 + col

characters = []
current_grid = [[0]*8 for _ in range(8)]

for pkt in pkts:
    data = bytes(pkt)
    # Find MIDI messages in USB payload
    if len(data) >= 4:
        status = data[-3]
        key = data[-2]
        velocity = data[-1]

        if status == 0x90 and velocity > 0:  # Note On
            row, col = key // 16, key % 16
            if 0 <= row < 8 and 0 <= col < 8:
                current_grid[row][col] = 1
        elif status == 0x80 or (status == 0x90 and velocity == 0):  # Note Off
            # All-off sequence = character separator
            if all(current_grid[r][c] == 0 for r in range(8) for c in range(8)):
                characters.append(current_grid)
                current_grid = [[0]*8 for _ in range(8)]
```

**Key insight:** MIDI devices use standardized message formats. Novation Launchpad maps its 8x8 grid to MIDI notes where `key = row*16 + col`. Note On (0x90) with velocity > 0 = button lit, Note Off (0x80) = button off. Sequences of all-off messages separate characters displayed on the grid.

**Detection:** USB PCAP with bulk transfer packets containing 3-byte or 4-byte payloads. USB device descriptor shows MIDI class (Audio class, subclass MIDI Streaming). Challenge mentions "MIDI", "Launchpad", "music controller", or "grid".

---

## Tektronix Logic-Analyzer CSV Clock-Edge Extraction (35C3 2018)

**Pattern:** Tektronix logic analyzers export multi-channel captures as ~10-MB CSV files with one column per signal (CLK, R, G, B, ...). Parse the file with Python, detect rising edges on the CLK column, and sample the data columns at each edge to reconstruct the transmitted image/stream.

```python
import csv
with open('capture.csv') as f:
    reader = csv.reader(f)
    prev_clk = 0
    bits = []
    for row in reader:
        try: clk = int(row[1])
        except ValueError: continue
        if prev_clk == 0 and clk == 1:          # rising edge
            bits.append((int(row[2]), int(row[3]), int(row[4])))
        prev_clk = clk
# Reshape bits into image and render with PIL
```

**Key insight:** Logic-analyzer CSV is always edge-sampled. Identify the clock column by its 50%-duty cycle, then sample the data columns synchronously at every rising edge. Works for any synchronous bus (RGB, SPI, I²C clock line).

**References:** 35C3 CTF 2018 — box of blink, writeup 12907
