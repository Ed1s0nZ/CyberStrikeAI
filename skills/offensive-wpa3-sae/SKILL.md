---
name: offensive-wpa3-sae
description: WPA3 / SAE (Simultaneous Authentication of Equals) attack methodology
  — transition-mode (mixed WPA2/WPA3) downgrade, Dragonblood side-channel attacks
  (CVE-2019-9494, 9495, 13377, 13456), SAE auth flooding for AP CPU exhaustion, Hash-to-Element
  (H2E) timing analysis, group downgrade, and 6 GHz / Wi-Fi 6E spec implications (PMF
  mandatory, no transition mode allowed). Use when target advertises WPA3-SAE or WPA3-Personal/Enterprise,
  or operates in 6 GHz where WPA3 + PMF are required by spec.
---

# WPA3 / SAE Attacks

WPA3 fixes the offline-handshake-cracking weakness of WPA2 by replacing the 4-way PSK exchange with SAE (a Dragonfly-derived password-authenticated key exchange). The straightforward offline crack disappears — but transition-mode misconfigurations and the original SAE implementation's side-channel leaks open new paths.

## Quick Workflow

1. Verify the target advertises WPA3 (RSN IE shows AKM SAE = 8)
2. Check for transition-mode (mixed WPA2 + WPA3) — easiest path
3. If pure WPA3, fingerprint the AP's hostapd version for Dragonblood applicability
4. Side-channel timing or cache attacks if reachable
5. Otherwise, accept that offline cracking isn't viable — pivot to other surfaces

---

## Transition-Mode Downgrade

If the AP advertises both WPA2-PSK and WPA3-SAE (transition mode for mixed-client networks), older clients can be forced onto WPA2:

```bash
# Identify transition mode in beacon frames
sudo airodump-ng wlan0mon -c <ch> --bssid <BSSID>
# Encryption column shows WPA2 WPA3 (both)
```

Steps:

1. Spoof a beacon advertising **only RSN-WPA2** with the same BSSID/SSID
2. Client roams to your beacon, performs WPA2 4-way handshake
3. Capture handshake exactly like `offensive-wpa2-psk`

```bash
# Use hostapd-mana or airbase-ng for the WPA2-only AP advertisement
airbase-ng -e CorpWiFi -c 6 -W 1 wlan0mon
# -W 1 enables WPA, configure for WPA2-only RSN element
```

**Why this works:** WPA3-SAE clients fall back to WPA2-PSK if the AP only advertises WPA2 — there's no protected downgrade defense in transition mode. WPA3-only mode (no transition) blocks this.

**Mitigation defenders use:** WPA3-only networks (no WPA2). Wi-Fi 6E (6 GHz) mandates WPA3-only by spec.

## Dragonblood (CVE-2019-9494 / 9495 / 13377 / 13456)

Side-channel and downgrade attacks against the SAE Hunting-and-Pecking algorithm in pre-2.10 hostapd / wpa_supplicant.

### Cache-Based Side-Channel

The original SAE password-element derivation iterates a variable number of times depending on the password and MAC. Cache hits leak the iteration count.

```bash
git clone https://github.com/vanhoefm/dragonblood
cd dragonblood

# Cache-based attack (requires co-located malicious code on target host — limited)
python3 dragontime.py --bssid AA:BB:CC:DD:EE:FF --iface wlan0mon
```

### Timing Side-Channel

The same iteration count leaks via observable timing of the SAE commit phase from outside.

```bash
python3 dragontime.py --bssid AA:BB:CC:DD:EE:FF --iface wlan0mon --mode timing
```

### Downgrade to Weak Group

Some implementations accept SAE with the deprecated MODP group 5 if the client requests it. Combined with cache/timing side channels, this enables offline dictionary attack.

```bash
python3 dragondrain.py wlan0mon AA:BB:CC:DD:EE:FF
```

### Patched Versions

| Implementation | Fixed |
|---|---|
| hostapd / wpa_supplicant | 2.10 (April 2022) |
| Apple iOS / macOS | 2019 patches |
| Windows | KB-batched 2019-2020 |
| Embedded routers | Often unpatched — high hit rate on consumer SOHO |

## Hash-to-Element (H2E)

WPA3 R2 introduced H2E to replace the iteration-leaky Hunting-and-Pecking. H2E is constant-time. If the AP advertises H2E in the RSNXE element, Dragonblood-class attacks don't apply.

```bash
# Wireshark filter
wlan.rsnx.field.h2e
```

If H2E is present and required (no Hunting-and-Pecking fallback), only the spec is left to attack — abandon SAE attacks and pivot to other surfaces (PMF check, evil-twin via EAP if Enterprise, supply-chain via management frames).

## SAE Auth Flooding (DoS)

SAE's commit phase requires the AP to do heavy elliptic-curve work per association attempt. Floods can exhaust CPU on lower-end APs, denying service to legitimate clients.

```bash
sudo mdk4 wlan0mon a -a AA:BB:CC:DD:EE:FF -m -s 1024
# Auth attack mode -a, multiple per second -s 1024
```

**This is a DoS — only with explicit authorization.** Modern enterprise APs use anti-clogging tokens to throttle SAE-flood attacks; consumer routers often don't.

## 6 GHz / Wi-Fi 6E Implications

The 6 GHz band (Wi-Fi 6E, channels 1–233 in the 5925–7125 MHz range) requires:

- **WPA3-only** (no transition mode)
- **PMF (802.11w) mandatory** (deauth/disassoc protected)
- **OWE (Opportunistic Wireless Encryption)** for open networks

Net effect: most pre-WPA3 attacks (deauth, transition-mode downgrade) don't apply on 6 GHz. Pure SAE side-channel, evil-twin, or out-of-band attacks remain viable.

## Detection Considerations

WPA3 is enterprise-defended much like WPA2 — WIDS catches:

- Beacon spoofing (transition-mode downgrade) via fingerprint mismatch (IE order, vendor-specific, beacon timing)
- SAE flood via association rate per source MAC
- Repeated SAE commit failures (timing attack telemetry)

Successful Dragonblood-class attacks against patched modern hostapd are unlikely. Consumer SOHO and embedded APs are still in scope.

## Engagement Cheatsheet

```bash
# 1. Identify mode
sudo airodump-ng wlan0mon -c <ch> --bssid <BSSID>
# Encryption: WPA2 + WPA3 → transition; WPA3-only → SAE-only

# 2. Transition-mode downgrade attempt
sudo airbase-ng -e <ESSID> -c <ch> -W 1 -z 4 wlan0mon  # WPA2-RSN advertised only

# 3. Wait for client roam, capture WPA2 handshake (handoff to offensive-wpa2-psk)

# 4. If pure WPA3, fingerprint hostapd
# (passive analysis of beacon IE order + version-specific behaviors)

# 5. Run Dragonblood test scripts if pre-2.10 hostapd suspected
python3 dragondrain.py wlan0mon <BSSID>
python3 dragontime.py --bssid <BSSID> --iface wlan0mon

# 6. Document residual viable attacks; pivot to evil-twin / EAP / RF if pure WPA3 R2
```

---

## Key References

- Dragonblood: dragonblood.net (Vanhoef + Ronen)
- IEEE 802.11-2020 (combined spec including WPA3)
- WFA WPA3 Specification
- hostapd 2.10 release notes
- Source: https://github.com/SnailSploit/offensive-checklist/blob/main/wireless.md
