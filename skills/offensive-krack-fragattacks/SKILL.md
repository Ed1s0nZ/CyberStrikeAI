---
name: offensive-krack-fragattacks
description: KRACK (CVE-2017-13077..082) and FragAttacks (CVE-2020-24586..588 + 26139-26147)
  — key reinstallation, fragmentation, and aggregation attacks against WPA2 supplicants.
  Covers Vanhoef's test scripts, viability against modern patched stacks (mostly mitigated
  post-2021), residual unpatched embedded devices and IoT vendors, and the practical
  limitations of these attacks in modern engagements. Use when assessing legacy supplicants,
  embedded clients, or vendors with poor patch cadence.
---

# KRACK & FragAttacks

Two attack families against WPA2 client implementations. Both well-disclosed (KRACK 2017, FragAttacks 2021) and largely patched on modern OSes — but the embedded/IoT long tail keeps them in scope for many engagements.

## When These Apply

| Family | Target | Patch Status |
|---|---|---|
| KRACK | WPA2 supplicants in 4-way handshake / GTK / FT / TDLS | Major OSes patched 2017–2018 |
| FragAttacks | Frame fragmentation/aggregation across WPA2/3 | Most stacks patched 2021–2022 |

Probability of success today is high only against:
- Embedded OEM devices (cameras, sensors, point-of-sale)
- Old Android phones (<8 unpatched)
- Industrial / SCADA Wi-Fi clients
- Wi-Fi-enabled toys, smart bulbs, no-name IoT

Modern Win11 / iOS 16+ / Android 13+ / hostapd-2.10 are mitigated.

## KRACK — Key Reinstallation

The 4-way handshake's M3 retransmission causes the supplicant to reinstall the same PTK with reset nonce/replay counters. Frames encrypted under the reused keystream become decryptable.

```bash
# Vanhoef's official test scripts
git clone https://github.com/vanhoefm/krackattacks-scripts
cd krackattacks-scripts/krackattack
sudo ./krack-test-client.py --interface wlan0
# Tests the supplicant on a connected client
```

Output identifies which CVE variants the client is vulnerable to.

### Practical Outcomes

When successful:
- Decryption of WPA2-encrypted frames between client and AP
- TKIP downgrade enables packet injection
- Recovery of session keys for the duration of the affected key cycle

Not a PSK recovery — you don't get the wireless password from KRACK.

## FragAttacks — Frame Splicing

FragAttacks abuse 802.11 fragmentation and aggregation to inject frames that mix encrypted and plaintext fragments, or to splice attacker-controlled fragments into legitimate frames.

```bash
git clone https://github.com/vanhoefm/fragattacks
cd fragattacks
sudo ./test-fragattacks.py wlan0 --interface wlan0
# Suite of ~12 tests covering each variant
```

| CVE | Mechanism |
|---|---|
| CVE-2020-24588 | A-MSDU spoofing — inject crafted A-MSDU subframes |
| CVE-2020-24587 | Mixed-key fragment cache poisoning |
| CVE-2020-24586 | Decoupled fragment cache → reuse |
| CVE-2020-26139 | Forwarding plaintext frames before authentication |
| CVE-2020-26140 | Accepting plaintext frames in protected network |

### Practical Outcomes

- Inject malicious frames that the client treats as legitimate (HTTP redirect, DNS poison)
- Read decrypted fragments from cached state
- Cross-protect data exfil via crafted A-MSDU

## Targeting Workflow

1. Identify the in-scope client (MAC, OS, vendor)
2. Estimate patch likelihood — if modern OS, likely patched; if embedded, likely vulnerable
3. Run the test suite from a controlled AP setup
4. Report each vulnerable variant separately with the matching CVE

```bash
# Rogue AP that drives the test
sudo hostapd-mana /tmp/krack_test_ap.conf

# Force client to associate (deauth from real AP, or social-engineer)
sudo aireplay-ng --deauth 5 -a <real-BSSID> -c <client-MAC> wlan0mon

# Run test once associated
sudo ./krack-test-client.py --interface wlan0
```

## Detection

- WIPS may flag deauth-driven roams to attacker AP
- Test scripts generate distinctive frame patterns; modern WIPS recognizes Vanhoef's tooling
- Successful exploitation is essentially silent at protocol level

## Reporting

For each vulnerable CVE:

- Client model + firmware version (be specific)
- Variant tested + result (vulnerable / patched / partial)
- Practical impact in the engagement context (decryption only, or injection viable?)
- Remediation: vendor patch URL, mitigation (WPA3 + PMF blocks most)

---

## Key References

- KRACK: krackattacks.com (Vanhoef)
- FragAttacks: fragattacks.com (Vanhoef)
- Original papers: USENIX Security 2017 (KRACK), USENIX Security 2021 (FragAttacks)
- CISA advisories tracking embedded vendor patches
- Source: https://github.com/SnailSploit/offensive-checklist/blob/main/wireless.md
