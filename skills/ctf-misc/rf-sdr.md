# CTF Misc - RF / SDR / IQ Signal Processing

Techniques for Software-Defined Radio (SDR) signal processing using In-phase/Quadrature (IQ) data.

## IQ File Formats
- **cf32** (complex float 32): GNU Radio standard, `np.fromfile(path, dtype=np.complex64)`
- **cs16** (complex signed 16-bit): `np.fromfile(path, dtype=np.int16).reshape(-1,2)`, then `I + jQ`
- **cu8** (complex unsigned 8-bit): RTL-SDR raw format

## Analysis Pipeline
```python
import numpy as np
from scipy import signal

# 1. Load IQ data
iq = np.fromfile('signal.cf32', dtype=np.complex64)

# 2. Spectrum analysis - find occupied bands
fft_data = np.fft.fftshift(np.fft.fft(iq[:4096]))
freqs = np.fft.fftshift(np.fft.fftfreq(4096))
power_db = 20*np.log10(np.abs(fft_data)+1e-10)

# 3. Identify symbol rate via cyclostationary analysis
x2 = np.abs(iq_filtered)**2  # squared magnitude
fft_x2 = np.abs(np.fft.fft(x2, n=65536))
# Peak in fft_x2 = symbol rate (samples_per_symbol = 1/peak_freq)

# 4. Frequency shift to baseband
center_freq = 0.14  # normalized frequency of band center
t = np.arange(len(iq))
baseband = iq * np.exp(-2j * np.pi * center_freq * t)

# 5. Low-pass filter to isolate band
lpf = signal.firwin(101, bandwidth/2, fs=1.0)
filtered = signal.lfilter(lpf, 1.0, baseband)
```

## QAM-16 Demodulation with Carrier + Timing Recovery
QAM-16 (Quadrature Amplitude Modulation) — the key challenge is carrier frequency offset causing constellation rotation (circles instead of points).

**Decision-directed carrier recovery + Mueller-Muller timing:**
```python
# Loop parameters (2nd order PLL)
carrier_bw = 0.02  # wider BW = faster tracking, more noise
damping = 1.0
theta_n = carrier_bw / (damping + 1/(4*damping))
Kp = 2 * damping * theta_n      # proportional gain
Ki = theta_n ** 2                # integral gain

carrier_phase = 0.0
carrier_freq = 0.0

for each symbol sample:
    # De-rotate by current phase estimate
    symbol = raw_sample * np.exp(-1j * carrier_phase)

    # Find nearest constellation point (decision)
    nearest = min(constellation, key=lambda p: abs(symbol - p))

    # Phase error (decision-directed)
    error = np.imag(symbol * np.conj(nearest)) / (abs(nearest)**2 + 0.1)

    # Update 2nd order loop
    carrier_freq += Ki * error
    carrier_phase += Kp * error + carrier_freq
```

**Mueller-Muller timing error detector:**
```python
timing_error = (Re(y[n]-y[n-1]) * Re(d[n-1]) - Re(d[n]-d[n-1]) * Re(y[n-1]))
             + (Im(y[n]-y[n-1]) * Im(d[n-1]) - Im(d[n]-d[n-1]) * Im(y[n-1]))
# y = received symbol, d = decision (nearest constellation point)
```

## Key Insights for RF CTF Challenges
- **Circles in constellation** = constant frequency offset (points rotate at fixed rate, forming a ring)
- **Spirals** = frequency offset that drifts over time (ring radius changes as amplitude/AGC also drifts). If you see points tracing outward arcs rather than closed circles, suspect combined frequency + gain instability
- **Blobs on grid** = correct sync, just noise
- **4-fold ambiguity**: DD carrier recovery can lock with 0/90/180/270 rotation - try all 4
- **Bandwidth vs symbol rate**: BW = Rs x (1 + alpha), where alpha is roll-off factor (0 to 1)
- **RC vs RRC**: "RC pulse shaping" at TX means receiver just samples (no matched filter needed); "RRC" means apply matched RRC filter at RX
- **Cyclostationary peak at Rs** confirms symbol rate even without knowing modulation order
- **AGC**: normalize signal power to match constellation power: `scale = sqrt(target_power / measured_power)`
- **GNU Radio's QAM-16 default mapping** is NOT Gray code - always check the provided constellation map

## Common Framing Patterns
- Idle/sync pattern repeating while link is idle
- Start delimiter (often a single symbol like 0)
- Data payload (nibble pairs for QAM-16: high nibble first, low nibble)
- End delimiter (same as start, e.g., 0)
- The idle pattern itself may contain the delimiter value - distinguish by context (is it part of the 16-symbol repeating pattern?)
