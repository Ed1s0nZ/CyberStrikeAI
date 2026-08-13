---
name: offensive-bluetooth-classic
description: Bluetooth Classic (BR/EDR) attack methodology — device discovery, service
  enumeration via SDP, LMP/L2CAP layer attacks, legacy PIN cracking (BlueBorne / KNOB),
  Bluetooth file-transfer abuse (BlueSnarfing legacy), unauthenticated profile abuse
  (HSP, HFP, OPP), and modern relevance against older industrial / automotive / accessory
  targets. Use when in-scope devices use Bluetooth Classic (Bluetooth ≤ 4.0 BR/EDR)
  — common in legacy car kits, industrial sensors, older medical devices, and audio
  accessories.
---

# Bluetooth Classic (BR/EDR) Attacks

Older than BLE, less commonly attacked today, but still present in cars, industrial sensors, audio gear, and legacy enterprise hardware. Many of the well-known historic attacks (BlueSnarf, BlueBug) are mitigated; KNOB and the BlueBorne family remain relevant against unpatched devices.

## Quick Workflow

1. Discover devices with `hcitool` / `bluetoothctl` / `redfang`
2. Enumerate exposed services via SDP
3. Test each service profile for unauth access
4. Check pairing crypto (KNOB applicability)
5. Proximity-physical attacks for legacy / unpatched

---

## Discovery

```bash
# Modern adapter (built-in or USB Bluetooth 4.0+)
sudo hciconfig hci0 up
sudo hcitool inq                       # inquiry
sudo hcitool scan --length=12          # 12-second scan

# bluetoothctl interactive
bluetoothctl
> scan on
> devices

# Discoverable-mode-only devices appear; non-discoverable need address brute
sudo redfang -r 00:00:00:00:00:00-FF:FF:FF:FF:FF:FF
# (very slow — ~7 hours per OUI prefix)
```

## Service Discovery (SDP)

```bash
# List all services on a device
sdptool browse AA:BB:CC:DD:EE:FF
sdptool records AA:BB:CC:DD:EE:FF
```

Common profiles and their attack relevance:

| Profile | UUID | Attack |
|---|---|---|
| OBEX Object Push (OPP) | 0x1105 | BlueSnarf/BlueBug on legacy phones (mostly extinct) |
| OBEX File Transfer (FTP) | 0x1106 | Browse / write filesystem on legacy devices |
| Headset (HSP/HFP) | 0x1108 / 0x111E | Eavesdrop active call audio |
| Serial Port Profile (SPP) | 0x1101 | Industrial/IoT debug ports — often unauthenticated |
| HID | 0x1124 | Keyboard/mouse impersonation |
| Audio Sink/Source (A2DP) | 0x110B / 0x110A | Audio injection/eavesdrop |

## SPP Abuse

The Serial Port Profile (SPP) tunnels arbitrary data over Bluetooth as a virtual COM port. Industrial / IoT devices use it for debug or telemetry, often without authentication.

```bash
# Connect to SPP service, channel typically 1
sudo rfcomm bind /dev/rfcomm0 AA:BB:CC:DD:EE:FF 1
sudo screen /dev/rfcomm0 9600
# Then interact with the device's CLI / debug menu
```

## KNOB (CVE-2019-9506)

Forces Bluetooth pairing to negotiate a 1-byte encryption key — making the link key trivially brute-forceable.

```bash
# Test with internalblue (requires Broadcom firmware patch)
git clone https://github.com/seemoo-lab/internalblue
internalblue
> log keys
# Patch firmware to allow 1-byte key; pair with target; observe weak key
```

Patched in firmware on most modern devices. Still works against:
- Older Broadcom-based devices (pre-2019 BCM chipsets)
- Embedded automotive Bluetooth stacks
- Cheap consumer audio gear

## BlueBorne (CVE-2017-1000251 et al.)

A family of buffer overflows / info leaks in major Bluetooth stacks (Linux BlueZ, Android, Windows, iOS). Mostly patched 2017–2018, but unpatched embedded Linux devices are common.

```bash
# Armis blueborne-scanner — checks for patch-level
git clone https://github.com/ArmisSecurity/blueborne
python blueborne_scanner.py AA:BB:CC:DD:EE:FF
```

## HID Spoofing (PoC)

If pairing succeeds via Just Works or weak PIN, you can register as a HID device — keystroke injection on an unattended Bluetooth-paired host.

```bash
# bdaddr + HID example — register custom HID on rfcomm
hcitool dev
hciconfig hci0 class 0x000540   # HID device class
sdptool add HID
# Use a HID descriptor crafted as keyboard, send keystrokes
```

## Audio Eavesdropping

If a target has Bluetooth headset paired and active, and you can re-pair (PIN brute or KNOB):

- HSP/HFP profiles let you become the peer and receive audio
- Some firmware allows simultaneous peer connections — eavesdrop without disrupting

## Engagement Cheatsheet

```bash
# 1. Discover
sudo hcitool inq

# 2. Enumerate services per device
sdptool browse <MAC>

# 3. SPP (industrial/IoT) — connect and explore
sudo rfcomm bind /dev/rfcomm0 <MAC> 1
sudo screen /dev/rfcomm0 9600

# 4. Patch-level scan
python blueborne_scanner.py <MAC>

# 5. KNOB testing (with adapter that supports internalblue)
internalblue → log keys → re-pair target

# 6. Document profiles, auth state, exposed commands per device
```

## Detection

- No native Bluetooth Classic IDS in most environments
- Active inquiry visible to nearby Bluetooth-aware monitoring (rare)
- Re-pairing prompts on target devices may surface to users

## Reporting

- Identify chipset + firmware version per device (often visible in service records)
- Map CVE applicability (BlueBorne, KNOB, BlueFrag, et al.)
- Document specific profile abuses (SPP exposed without auth, HID spoofing successful, etc.)

---

## Key References

- internalblue: github.com/seemoo-lab/internalblue
- KNOB attack: knobattack.com
- BlueBorne: armis.com/blueborne
- Bluetooth Core Spec — Volume 2 (BR/EDR Controller)
- Source: https://github.com/SnailSploit/offensive-checklist/blob/main/wireless.md
