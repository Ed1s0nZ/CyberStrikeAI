# Signal Modulation Identification

## Tool
`~/combat/signal_identify.py` - automatic modulation classifier from SDR capture

## Usage
```bash
python3 ~/combat/signal_identify.py --freq 5807              # Live HackRF capture
python3 ~/combat/signal_identify.py --file capture.raw        # From IQ file
python3 ~/combat/signal_identify.py --scan 5650 5950          # Scan band
python3 ~/combat/signal_identify.py --freq 750 --json         # JSON output
```

## Detectable Modulations
| Type | Indicators |
|------|-----------|
| CW | BW<50kHz, FM dev<50kHz, peak/avg>20dB |
| FM_VIDEO | FM dev>1MHz, PAL(64μs)/NTSC(63.5μs) line sync |
| OFDM | Cyclic prefix correlation, flat spectrum, BW>1MHz |
| LORA | Chirp autocorrelation at 2^SF/BW symbol period |
| FM | FM dev>0.5MHz, no video sync |
| FSK | 2-8 discrete spectral peaks |
| AM | High amplitude variation, low FM dev |
| DIGITAL_WIDEBAND | Flat spectrum >5MHz, no OFDM CP |

## Three-Layer Feature Extraction
1. **Spectral**: BW, SNR, flatness, peak count, peak-to-average ratio
2. **Temporal**: FM deviation, AM index, amplitude/phase entropy
3. **Cyclostationary**: autocorrelation at OFDM CP lags, video line periods, LoRa chirp periods

## Field Application
- Detect enemy drone VTX type before engaging (analog vs digital)
- Identify ELRS/LoRa control link on any frequency
- Scan 5.8GHz band to find all active FPV transmitters
- Classify unknown signals from captured equipment
