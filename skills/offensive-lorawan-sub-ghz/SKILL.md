---
name: offensive-lorawan-sub-ghz
description: LoRaWAN and sub-GHz (433 / 868 / 915 MHz) attack methodology — LoRaWAN
  ABP/OTAA join attack, network/session key reuse, frame counter replay, downlink
  injection on TTN/Helium-style networks, sub-GHz protocol replay (KeeLoq garage doors,
  fixed-code remotes, TPMS spoofing, smart plug telemetry), HackRF / RTL-SDR / Flipper
  Zero workflows, signal analysis with Inspectrum / Universal Radio Hacker, and reconstruction
  of proprietary packet formats. Use for LoRaWAN deployments (smart cities, asset
  tracking, industrial telemetry), or any wireless device using the unlicensed 433/868/915
  MHz bands (garage openers, doorbells, IoT sensors, RC equipment).
---

# LoRaWAN & Sub-GHz Attacks

LoRaWAN provides long-range low-bitrate communication for IoT — common in smart cities, asset tracking, and industrial telemetry. Outside LoRaWAN, the 433 / 868 / 915 MHz ISM bands host garage doors, doorbells, smart plugs, weather stations, and TPMS — most with weak or no crypto.

## Quick Workflow

1. Identify the band + modulation (LoRa CSS vs. simple OOK/FSK)
2. Capture transmissions with appropriate hardware (HackRF / RTL-SDR / Flipper Zero)
3. For LoRaWAN: capture join + uplinks; analyze key derivation
4. For proprietary sub-GHz: demodulate, identify packet format, replay or craft

---

## Hardware

| Tool | Range | Use |
|---|---|---|
| RTL-SDR | RX only, 24 MHz–1.7 GHz | Cheap reconnaissance |
| HackRF One | RX/TX, 1 MHz–6 GHz | Full transceiver |
| Flipper Zero | RX/TX, sub-GHz | Quick replays, fixed-code attacks |
| LimeSDR / BladeRF | RX/TX, wider band | Higher fidelity for LoRaWAN |
| YARD Stick One | TX-focused sub-GHz | Targeted replays |
| LoRa-specific gateway (RAK / Heltec) | LoRaWAN dual-direction | Standards-compliant LoRaWAN testing |

## LoRaWAN

LoRaWAN is a MAC layer over LoRa physical (chirp spread spectrum). Devices either:
- **OTAA** (Over-the-Air Activation) — derive session keys at join
- **ABP** (Activation By Personalization) — pre-flashed keys

### OTAA Join Capture

```bash
# Capture LoRa packets with HackRF + Inspectrum
hackrf_transfer -r capture.iq -f 868000000 -s 1000000 -n 60000000
# Or LoRa-specific: rak_common_for_gateway

# Decode with PHY + MAC stack
git clone https://github.com/Lora-net/LoRaMac-node
# Or use ChirpStack as a sniffing gateway
```

The Join-Request and Join-Accept are encrypted with the device's AppKey. With AppKey (extracted from device firmware — see `offensive-iot`):

- Decrypt Join-Accept → recover NwkSKey, AppSKey
- Subsequent traffic decryption + injection

### ABP — Pre-Flashed Keys

ABP devices have NwkSKey + AppSKey flashed at manufacture. Common flaws:

- Same key across thousands of devices (vendor laziness)
- No frame counter rollover protection → replay any historical uplink
- DevAddr predictability (sequential allocation)

```bash
# If you have NwkSKey + AppSKey + DevAddr, decode/inject with lorawan-test-tools
git clone https://github.com/IoTsec/loraserver-attack-tools
python lora_inject.py --nwkskey <NWKS> --appskey <APPS> --devaddr <ADDR>
```

### Frame Counter Replay

Older LoRaWAN 1.0.x doesn't enforce strict frame counter monotonicity in all stacks. Replay an uplink with a different timestamp → server processes as fresh.

### Downlink Injection

If you control AppSKey + NwkSKey, you can inject downlinks (configuration changes, remote commands) to devices.

## Sub-GHz Proprietary Protocols

### Quick Capture + Replay (Flipper Zero / HackRF)

```bash
# RTL-SDR live monitor
rtl_433 -f 433.92M -A     # auto-decode many devices
gqrx                       # interactive spectrum analyzer

# Flipper Zero Sub-GHz menu: Read → identify modulation → capture → save
# Then replay from the saved file

# HackRF capture
hackrf_transfer -r garage.iq -f 433920000 -s 8000000 -n 80000000
# Inspectrum to visualize, identify OOK / FSK, decode bits
```

### KeeLoq (Old Garage Doors, Some Cars)

KeeLoq uses a 32-bit block cipher with a manufacturer key. The manufacturer key was extracted publicly years ago for major brands. With it:

- Decrypt rolling code → predict next valid code
- Combined with capture-replay, take over the remote

```bash
# rolling-code-tools (research)
git clone https://github.com/AndrewMohawk/RollingPwn
```

Modern KeeLoq deployments (last 5 years) have rotated manufacturer keys, but legacy hardware (older garage doors, some industrial equipment) is in scope.

### Fixed-Code Remotes

Many cheap garage openers, doorbells, and smart plugs use fixed codes — the same packet every time you press the button. Capture once, replay forever.

```bash
# Flipper Zero: Read → Save → Send (from saved file)
# Or with RFCat:
python -c "import rflib; ..."
# OR with HackRF:
hackrf_transfer -t replay.iq -f 433920000 -s 8000000
```

### TPMS Spoofing

Tire-pressure monitoring sensors broadcast at 315/433 MHz with no authentication. Spoof low-pressure alerts:

```bash
# Capture legitimate TPMS
rtl_433 -f 315M -F json | grep TPMS

# Synthesize crafted alerts (custom modulator with HackRF)
# Useful for testing TPMS-aware vehicle systems or as denial-of-trust attack
```

### Reconstruction of Unknown Protocols

```bash
# Universal Radio Hacker (URH) — visual reverse engineering
urh
# Load .iq capture, identify modulation visually,
# auto-detect symbols, decode bits, identify packet structure
```

URH walks you from raw RF to a parsed protocol description, even with no docs.

## Engagement Cheatsheet

```bash
# 1. Identify band + modulation
rtl_433 -f <freq> -A           # auto-detect known protocols
gqrx                           # spectrum view to find activity

# 2. For LoRaWAN
#    - Set up gateway (or HackRF + LoRa decoding)
#    - Capture joins + uplinks
#    - Extract keys from device firmware (see offensive-iot)

# 3. For proprietary sub-GHz
#    - Capture with HackRF / RTL-SDR
#    - Visualize / decode with Inspectrum or URH
#    - Replay or craft

# 4. Document modulation, frequency, packet format, replay viability
```

## Detection

- LoRaWAN networks have server-side anomaly detection (frame counter, signal strength, geographic) — varies widely by operator
- Sub-GHz consumer products typically have no monitoring
- TPMS / industrial equipment has minimal telemetry on RF anomalies

## Reporting

- Identify exact frequency, modulation, baud, and packet format per device
- Distinguish capture-replay vs. crafted-frame attacks
- Note crypto state (cleartext / weak-fixed-key / standards-compliant)
- For LoRaWAN: identify AppKey / NwkSKey / AppSKey storage in firmware

---

## Key References

- rtl_433 protocol database: github.com/merbanan/rtl_433
- Universal Radio Hacker: github.com/jopohl/urh
- RollingPwn (KeeLoq research): github.com/AndrewMohawk/RollingPwn
- LoRaWAN Specification: lora-alliance.org
- Source: https://github.com/SnailSploit/offensive-checklist/blob/main/wireless.md
