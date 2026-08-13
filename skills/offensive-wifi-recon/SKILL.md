---
name: offensive-wifi-recon
description: Wi-Fi reconnaissance methodology — adapter selection, monitor mode and
  packet injection setup, regulatory domain handling, multi-band airspace mapping,
  hidden SSID discovery, BSSID/ESSID/channel/PMF/encryption fingerprinting, client
  probe analysis, vendor OUI lookup, war-driving with Kismet/airodump-ng/Wigle, and
  structured airspace data capture for downstream attacks. Use at the start of any
  wireless engagement to build the target map before active attacks; covers 2.4 GHz,
  5 GHz, and 6 GHz (Wi-Fi 6E) bands and adapter compatibility for each.
---

# Wi-Fi Reconnaissance

The first phase of any wireless engagement. Build a complete picture of the airspace before you deauth, evil-twin, or capture handshakes — every later attack depends on knowing the right BSSID, channel, encryption, and client population.

## Quick Workflow

1. Pick the right adapter for the target's band(s) and PHY
2. Verify monitor mode + injection actually work
3. Set the regulatory domain (legal channels and TX power)
4. Sweep all bands passively
5. Drill down on each in-scope BSSID for client population and PMF status
6. Record everything in a structured target list before any active attack

---

## Adapter Selection

| Chipset | Strengths | Notes |
|---------|-----------|-------|
| Atheros AR9271 (Alfa AWUS036NHA) | Solid 2.4 GHz monitor + injection | 802.11n only |
| Realtek RTL8812AU (AWUS036ACH) | Dual-band, injection | Driver: aircrack-ng/rtl8812au |
| MediaTek MT7612U (AWUS036ACM) | Stable dual-band | In-tree driver on modern kernels |
| MediaTek MT7921AU | Wi-Fi 6 monitor (limited) | Patched drivers required |
| AWUS036AXML / AXM | Wi-Fi 6E (6 GHz) | Bleeding edge — verify per release |

```bash
# Identify your radio
lsusb | grep -iE "(atheros|realtek|mediatek|alfa)"
iw dev
iw list | grep -A 8 "Supported interface modes"
iw list | grep -E "Frequencies:" -A 30
```

## Monitor Mode Setup

```bash
# Kill conflicting services
sudo airmon-ng check kill

# Enable monitor mode
sudo airmon-ng start wlan0
# Or manually
sudo ip link set wlan0 down
sudo iw wlan0 set monitor control
sudo ip link set wlan0 up

# Verify monitor mode + injection
sudo aireplay-ng --test wlan0mon
```

The injection test should report 30/30 ack rates against nearby APs. Lower scores indicate driver, antenna, or position issues.

## Regulatory Domain

```bash
# Check current
iw reg get

# Set explicitly (us = United States, jp = Japan extended, etc.)
sudo iw reg set US
```

Setting the right regdomain unlocks legitimate channels (US: 1–11 on 2.4, 36–165 on 5; JP adds 12–13 + 184+ DFS) and TX power. **Operate within the regdomain you're authorized to use.**

## Passive Multi-Band Sweep

```bash
# All bands
sudo airodump-ng wlan0mon --band abg

# 5 GHz only (helps see UNII bands)
sudo airodump-ng wlan0mon --band a

# 6 GHz (requires 6E-capable adapter and updated airodump-ng)
sudo airodump-ng wlan0mon --band ax

# Hop only specific channels
sudo airodump-ng wlan0mon -c 1,6,11,36,40,44,48
```

Capture to file for later analysis:

```bash
sudo airodump-ng wlan0mon --band abg --write recon --output-format pcap,csv
```

## Targeted Capture

Once you've identified an in-scope BSSID:

```bash
sudo airodump-ng -c 6 --bssid AA:BB:CC:DD:EE:FF -w target wlan0mon
```

Pin to the channel — channel-hopping during a focused capture loses frames.

## Hidden SSIDs

Hidden APs broadcast beacons with empty ESSID. The name leaks during client probes (active scan) or association requests:

```bash
# Wait for legitimate client to associate, ESSID appears in airodump output
# Or, if a client is already associated, deauth them once to force reassociation:
sudo aireplay-ng --deauth 1 -a AA:BB:CC:DD:EE:FF -c 11:22:33:44:55:66 wlan0mon
```

(Only deauth with explicit authorization — see `offensive-deauth-disassoc`.)

## Kismet for War-Driving

```bash
sudo kismet -c wlan0mon
# Open https://localhost:2501 for the dashboard
```

Kismet handles GPS integration, plots APs to a map, fingerprints by IE order, identifies probable IoT vendors from OUI prefixes, and tags known-vulnerable models.

For long-running captures, drop `--no-ncurses` and run headless under `tmux`.

## Wigle Submission

If the engagement permits:

```bash
# Export Kismet's .kismet → CSV → Wigle import format
kismetdb_dump_devices --in capture.kismet --out devices.csv
```

(Wigle aggregates wireless network observations geographically — useful for mapping but check ROE.)

## Vendor / OUI Identification

```bash
# Quick OUI lookup
echo "AA:BB:CC" | wireshark-tools/manuf-lookup
# Or check the airodump CSV's BSSID prefix against /usr/share/wireshark/manuf
```

Vendor identification informs:
- Likely default credentials (router brand → known defaults)
- Known firmware bugs (CVE per chipset)
- Whether WPS is likely vulnerable (Pixie Dust per chipset)
- Whether KRACK / FragAttacks patches are likely applied (vendor patch cadence)

## Data to Record per Target

| Field | Why |
|-------|-----|
| BSSID | Required for every active attack |
| ESSID | Match against PNL probes; client probe correlation |
| Channel + width | Pin radio for capture |
| Band | Adapter selection |
| Encryption | WPA2-PSK / WPA2-Enterprise / WPA3-SAE / Open / WEP |
| PMF (Protected Management Frames) | Whether deauth works |
| RSSI | Position planning |
| Beacon interval / TIM | Anomaly detection vs. evil-twin defenders |
| Vendor (OUI) | Likely default creds, known bugs |
| Client list (MACs + RSSI) | Targets for deauth/relay |
| WPS enabled? | Pixie Dust candidate |

## Detection Considerations

A defender's WIDS sees:
- New device entering the airspace (probe requests reveal even before association)
- Channel hopping patterns of monitor-mode interfaces
- Non-standard probe behavior (KARMA-style universal responses, see `offensive-evil-twin`)

Pure passive recon (no probes from your radio) is invisible to most WIDS deployments. Stay passive until you're committed to the active phase.

## Engagement Cheatsheet

```bash
# 1. Setup
sudo airmon-ng check kill && sudo airmon-ng start wlan0
sudo iw reg set US
sudo aireplay-ng --test wlan0mon          # confirm injection (skip if pure passive)

# 2. Sweep all bands, write to file
sudo airodump-ng wlan0mon --band abg --write recon --output-format pcap,csv

# 3. Kismet for sustained map (optional)
sudo kismet -c wlan0mon --no-ncurses --daemonize

# 4. Per BSSID drill-down
sudo airodump-ng -c <ch> --bssid <BSSID> -w <name> wlan0mon

# 5. Build target list with all fields above
```

---

## Key References

- IEEE 802.11-2020 (combined spec)
- aircrack-ng documentation: aircrack-ng.org
- Kismet documentation: kismetwireless.net
- WIGLE: wigle.net (read the API ToS before automated submissions)
- Source: https://github.com/SnailSploit/offensive-checklist/blob/main/wireless.md
