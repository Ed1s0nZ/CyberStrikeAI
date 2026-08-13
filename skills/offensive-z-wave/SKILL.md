---
name: offensive-z-wave
description: Z-Wave attack methodology — sniffing with Z-Force / EZ-Wave / RTL-SDR
  + ZniffMobile, S0 (legacy) network-key derivation flaw and key reuse, S2 (modern)
  ECDH commissioning analysis, replay/injection on unauthenticated nodes, default-key
  brute-force on test deployments, and home-automation hub pivots. Use when targeting
  Z-Wave smart home devices (door locks, sensors, garage controllers) — common in
  mid-2010s smart home deployments still in production.
---

# Z-Wave Attacks

Z-Wave runs in the 800/900 MHz ISM band (US: 908 MHz, EU: 868 MHz). Older networks used the S0 security scheme with a fixed-derivation network key — long-known to be flawed. S2 (mandatory for Z-Wave Plus v2 since 2017) uses ECDH commissioning and is significantly stronger.

## Quick Workflow

1. Identify region (US 908 MHz / EU 868 MHz) — adapter frequency must match
2. Sniff inclusion (commissioning) traffic — that's where keys are exchanged
3. Determine S0 vs S2 from frame format
4. For S0: derive/replay; for S2: analyze ECDH and look for implementation flaws

---

## Hardware

| Adapter | Use |
|---|---|
| Z-Force (legacy, hard to find) | Original research tool |
| EZ-Wave (custom HackRF firmware) | Modern, full transceiver |
| Aeotec Z-Stick | Commercial controller, useful as legitimate node |
| HackRF + open Z-Wave firmware | Multi-band SDR approach |
| RTL-SDR + ZniffMobile (passive only) | Cheap sniffer |

## Sniffing

```bash
# EZ-Wave (HackRF firmware-based)
git clone https://github.com/cureHsu/EZ-Wave
ezwave-sniff -f 908.4MHz -o capture.pcap

# Wireshark with the Z-Wave dissector parses captured frames
wireshark capture.pcap
```

Look for the inclusion phase (controller adding new device) — that's where the network key is exchanged.

## S0 Security Flaw

S0 derives the network key from a fixed all-zero PSK during the inclusion of the first device. That fixed material is well-known — any S0 network you sniff during inclusion can be decrypted offline.

```
S0 commissioning:
  1. New node joins → controller sends key with zero-PSK encryption
  2. Attacker sniffs commissioning frame → derives session key
  3. All future S0 traffic on that network is decryptable
```

If you can:
- Trigger inclusion (factory-reset a node, or wait for legitimate inclusion)
- Sniff during the ~2-second key-exchange window

You own the network key for that mesh.

## S2 (Z-Wave Plus / S2 Authenticated)

S2 fixes S0 by using ECDH for commissioning:
- Each device has a Curve25519 keypair
- Inclusion uses DSK (Device Specific Key) verified out-of-band (sticker/QR)
- Network key never traverses the air in plaintext

S2 attack surface is mostly implementation:
- Inclusion-mode-always-open (controller misconfig)
- Firmware bugs in S2 verification
- Side-channel on ECDH on resource-constrained chips
- DSK printed on a sticker → physical access yields it

## Replay / Injection on Unauthenticated Nodes

Many low-end Z-Wave devices (older sensors, basic switches) don't enforce S0 or S2 — they accept commands in cleartext.

```python
# scapy-zwave (community fork) for crafted frames
from scapy.contrib.zwave import *
frame = ZWave(home_id=0x12345678)/ZWaveBasic(set_value=0xff)
sendp(frame, iface='ezwave0')
```

This unlocks doors / switches lights / unarms sensors when the target lacks authentication.

## Key Brute-Force

For old test deployments using default home IDs / network keys:

```bash
# Try default home IDs
for hid in 0x00000000 0x12345678 ...; do
  ezwave-test --home-id $hid --target-node 1
done
```

Hit rate on production is low; useful only for default-config IoT lab gear.

## Hub Pivots

Z-Wave devices are typically controlled by a hub (SmartThings, Hubitat, Vera, Home Assistant, Z-Wave JS UI). The hub is a Linux device with the Z-Wave PSK in plaintext storage:

- SmartThings Hub: previously cloud-only credentials; modern v3 stores network key locally
- Home Assistant: `~/.homeassistant/zwave_js.json` typically contains keys
- Hubitat: web UI with default password on older versions

Compromise the hub → walk away with the Z-Wave PSK + every paired device's command authority. See `offensive-iot` for hub firmware extraction.

## Engagement Cheatsheet

```bash
# 1. Identify region + frequency
# US: 908.4 MHz; EU: 868.4 MHz; CN: 868.4 MHz

# 2. Sniff
ezwave-sniff -f 908.4MHz -o cap.pcap
wireshark cap.pcap   # filter zwave

# 3. Identify S0 vs S2 from frame format

# 4. For S0: capture inclusion → derive key → decrypt history + control devices

# 5. For S2: focus on hub compromise / DSK theft / implementation bugs

# 6. Test unauthenticated cleartext devices with crafted frames
```

## Detection

- Most Z-Wave deployments have no IDS comparable to Wi-Fi/Zigbee monitoring
- Hub may log unexpected commands but UI rarely surfaces these to users
- Inclusion-mode-open is visible in hub UI but ignored by inattentive admins

## Reporting

- Identify chipset / firmware revision per device (ZW0500 series, ZW7000 series)
- Map S0 vs S2 per node — note any S0 left on a network with S2-capable nodes
- Document hub compromise paths separately

---

## Key References

- EZ-Wave: github.com/cureHsu/EZ-Wave (HackRF-based)
- "Z-Force and the Z-Wave Sniffer" — original research
- Silicon Labs Z-Wave 700-series spec
- Source: https://github.com/SnailSploit/offensive-checklist/blob/main/wireless.md
