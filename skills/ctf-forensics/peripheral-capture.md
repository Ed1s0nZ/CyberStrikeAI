# CTF Forensics - Peripheral Capture Analysis

USB, HID, and Bluetooth peripheral traffic reconstruction from packet captures. For general network PCAP forensics (DNS/TCP/ICMP/SMB/RADIUS/RC4), see [network-advanced.md](network-advanced.md). For basic network forensics, see [network.md](network.md).

## Table of Contents
- [USB HID Mouse/Pen Drawing Recovery (EHAX 2026)](#usb-hid-mousepen-drawing-recovery-ehax-2026)
- [USB HID Keyboard Capture Decoding (EKOPARTY CTF 2016)](#usb-hid-keyboard-capture-decoding-ekoparty-ctf-2016)
- [USB Keyboard LED Morse Code Exfiltration (BITSCTF 2017)](#usb-keyboard-led-morse-code-exfiltration-bitsctf-2017)
- [USB HID Keyboard Arrow Key Navigation Tracking (HackIT 2017)](#usb-hid-keyboard-arrow-key-navigation-tracking-hackit-2017)
- [Bluetooth RFCOMM Packet Reassembly (HITCON 2018)](#bluetooth-rfcomm-packet-reassembly-hitcon-2018)
- [GBA USB URB_INTERRUPT Framebuffer Extraction (hxp 2018)](#gba-usb-urb_interrupt-framebuffer-extraction-hxp-2018)

---

## USB HID Mouse/Pen Drawing Recovery (EHAX 2026)

**Pattern (Painter):** PCAP contains USB HID interrupt transfers from a mouse/pen device. Drawing data encoded as relative movements with multiple draw modes.

**Packet format (7-byte HID reports):**
| Byte | Field | Notes |
|------|-------|-------|
| 0 | Button state | 0x01 = pressed (may be constant) |
| 1 | Mode/pad | 0=hover, 1=draw mode 1, 2=draw mode 2 |
| 2-3 | dx (int16 LE) | Relative X movement |
| 4-5 | dy (int16 LE) | Relative Y movement |
| 6 | Wheel | Usually 0 |

**Extraction and rendering:**
```python
import struct
from PIL import Image, ImageDraw

# Extract HID data
# tshark -r capture.pcap -Y "usb.transfer_type==1" -T fields -e usb.capdata

packets = []
with open('hid_data.txt') as f:
    for line in f:
        raw = bytes.fromhex(line.strip().replace(':', ''))
        if len(raw) >= 7:
            btn = raw[0]
            mode = raw[1]
            dx = struct.unpack('<h', raw[2:4])[0]
            dy = struct.unpack('<h', raw[4:6])[0]
            packets.append((btn, mode, dx, dy))

# Accumulate positions per mode
SCALE = 5
positions = {0: [], 1: [], 2: []}
x, y = 0, 0
for btn, mode, dx, dy in packets:
    x += dx
    y += dy
    positions[mode].append((x, y))

# Render each mode separately (different colors = different text layers)
for mode in [1, 2]:
    pts = positions[mode]
    if not pts:
        continue
    min_x = min(p[0] for p in pts) - 100
    min_y = min(p[1] for p in pts) - 100
    max_x = max(p[0] for p in pts) + 100
    max_y = max(p[1] for p in pts) + 100
    w = (max_x - min_x) * SCALE
    h = (max_y - min_y) * SCALE
    img = Image.new('RGB', (w, h), 'white')
    draw = ImageDraw.Draw(img)
    for i in range(1, len(pts)):
        x0 = (pts[i-1][0] - min_x) * SCALE
        y0 = (pts[i-1][1] - min_y) * SCALE
        x1 = (pts[i][0] - min_x) * SCALE
        y1 = (pts[i][1] - min_y) * SCALE
        # Skip long jumps (pen lifts)
        if abs(pts[i][0]-pts[i-1][0]) < 50 and abs(pts[i][1]-pts[i-1][1]) < 50:
            draw.line([(x0,y0),(x1,y1)], fill='black', width=3)
    img.save(f'mode_{mode}.png')
```

**Key techniques:**
- **Separate modes:** Different button/mode values draw different text layers — render each independently
- **Skip pen lifts:** Large dx/dy jumps indicate pen was lifted, not drawn — filter by distance threshold
- **High resolution:** Scale 5-8x with margins for readable handwriting
- **Time gradient:** Color points by temporal order (rainbow gradient) to trace stroke direction
- **Character segmentation:** Group consecutive same-mode points by large X gaps to isolate characters

**Alternative: AWK extraction + SVG rendering (faster pipeline):**
```bash
# Extract capdata and convert to signed deltas in one pass
tshark -r pref.pcap -Y "usb.transfer_type==0x01 && usb.endpoint_address==0x81 && usb.capdata" \
  -T fields -e usb.capdata > capdata.txt

awk '
function hexval(c){ return index("0123456789abcdef",tolower(c))-1 }
function hex2dec(h, n,i){ n=0; for(i=1;i<=length(h);i++) n=n*16+hexval(substr(h,i,1)); return n }
function s16(u){ return (u>=32768)?u-65536:u }
{ d=$1; if(length(d)!=14) next
  btn=hex2dec(substr(d,3,2))
  x=s16(hex2dec(substr(d,7,2) substr(d,5,2)))
  y=s16(hex2dec(substr(d,11,2) substr(d,9,2)))
  print btn, x, y }' capdata.txt > deltas.txt
```
Then render with SVG (Python) — filter on pen-down state (button=2), accumulate deltas, flip Y axis, draw strokes between consecutive pen-down points.

**Difference from keyboard HID:** Mouse HID uses relative movements (accumulated), keyboard uses keycodes (direct). Mouse drawing requires rendering; keyboard requires keymap lookup.

---

## USB HID Keyboard Capture Decoding (EKOPARTY CTF 2016)

USB keyboard captures contain HID scan codes that map to keystrokes. Decode the capture to reconstruct typed text.

```python
# USB HID keyboard report format:
# Byte 0: Modifier keys (Shift, Ctrl, Alt)
# Byte 1: Reserved (0x00)
# Bytes 2-7: Up to 6 simultaneous key codes

# HID scan code to character mapping (partial)
HID_MAP = {
    0x04: 'a', 0x05: 'b', 0x06: 'c', 0x07: 'd', 0x08: 'e',
    0x09: 'f', 0x0a: 'g', 0x0b: 'h', 0x0c: 'i', 0x0d: 'j',
    0x0e: 'k', 0x0f: 'l', 0x10: 'm', 0x11: 'n', 0x12: 'o',
    0x13: 'p', 0x14: 'q', 0x15: 'r', 0x16: 's', 0x17: 't',
    0x18: 'u', 0x19: 'v', 0x1a: 'w', 0x1b: 'x', 0x1c: 'y',
    0x1d: 'z', 0x1e: '1', 0x1f: '2', 0x20: '3', 0x21: '4',
    0x22: '5', 0x23: '6', 0x24: '7', 0x25: '8', 0x26: '9',
    0x27: '0', 0x28: '\n', 0x2c: ' ', 0x2d: '-', 0x2e: '=',
    0x2f: '[', 0x30: ']', 0x33: ';', 0x34: "'", 0x36: ',',
    0x37: '.', 0x38: '/',
}

SHIFT_MAP = {
    'a': 'A', 'b': 'B', '1': '!', '2': '@', '3': '#', '4': '$',
    '5': '%', '6': '^', '7': '&', '8': '*', '9': '(', '0': ')',
    '-': '_', '=': '+', '[': '{', ']': '}', ';': ':', "'": '"',
    ',': '<', '.': '>', '/': '?',
}

def decode_hid_keyboard(capture_data):
    """Decode USB HID keyboard capture to text"""
    text = ""
    for report in capture_data:
        modifier = report[0]
        keycode = report[2]  # first key in report

        if keycode == 0:
            continue

        char = HID_MAP.get(keycode, '')
        if modifier & 0x22:  # Left or Right Shift
            char = SHIFT_MAP.get(char, char.upper())

        text += char
    return text

# Extract from Wireshark: tshark -r capture.pcapng -T fields -e usb.capdata
# Or from text dump: parse +XX/-XX format (+ = keydown, - = keyup)
```

**Key insight:** USB HID keyboards send 8-byte reports where byte 0 is modifiers (Shift/Ctrl/Alt) and bytes 2-7 are active key scan codes. In Wireshark, filter with `usb.transfer_type == 1` and extract `usb.capdata`. Ignore reports where byte 2 is 0x00 (key release).

---

## USB Keyboard LED Morse Code Exfiltration (BITSCTF 2017)

**Pattern (Ghost in the Machine):** A pcap of USB keyboard traffic contains host-to-device packets with alternating `0x01`/`0x03` values controlling the Caps Lock LED state. Timing differences between LED state changes encode Morse code: durations >300ms represent dashes, shorter durations represent dots. Decode the Morse sequence to recover the flag.

```python
from scapy.all import rdpcap
import struct

packets = rdpcap('usb_capture.pcap')
signals = []

for p in packets:
    raw = bytes(p)
    # USB HID SET_REPORT to keyboard (host -> device)
    if len(raw) >= 35 and raw[30] in (0x01, 0x03):
        timestamp = p.time
        led_state = raw[30]  # 0x01 = LED off, 0x03 = LED on
        signals.append((timestamp, led_state))

# Convert timing to Morse
morse = ''
for i in range(0, len(signals) - 1, 2):
    duration = signals[i+1][0] - signals[i][0]
    if duration > 0.3:
        morse += '-'
    else:
        morse += '.'
    # Gap between signals indicates letter/word boundary
```

**Key insight:** Data exfiltration via keyboard LED state changes captured in USB pcap. The LED control packets use HID SET_REPORT class requests. Timing analysis of on/off transitions reveals Morse code patterns. Tools: Wireshark USB dissector, filter on `usb.transfer_type == 0x02` (interrupt) and direction host→device.

---

## USB HID Keyboard Arrow Key Navigation Tracking (HackIT 2017)

USB HID keyboard traffic from an Apple Keyboard requires tracking arrow key navigation. Decode HID keycodes using the USB HID usage table. Modifier byte `0x02` = Shift (uppercase). Track cursor position via up/down arrow presses to determine which line contains the flag.

```bash
tshark -r capture.pcap -T fields -e usb.capdata | \
  python3 decode_hid.py  # Must track arrow keys for line position
```

Arrow key HID codes to track:
- `0x4F` = Right Arrow
- `0x50` = Left Arrow
- `0x51` = Down Arrow (next line)
- `0x52` = Up Arrow (previous line)

```python
# Skeleton: track line position during HID decode
line = 0
lines = {0: ""}
for report in hid_reports:
    modifier = report[0]
    keycode = report[2]
    if keycode == 0x51:    # Down arrow
        line += 1; lines.setdefault(line, "")
    elif keycode == 0x52:  # Up arrow
        line -= 1; lines.setdefault(line, "")
    elif keycode in HID_MAP:
        char = HID_MAP[keycode]
        if modifier & 0x22:
            char = char.upper()
        lines[line] += char
# Flag is on a specific line determined by arrow navigation
```

**Key insight:** USB keyboard captures must account for cursor movement keys (arrows, backspace). Track cursor line position to reconstruct text typed on each line separately — the flag may be on a non-zero line that arrow keys navigated to.

---

## Bluetooth RFCOMM Packet Reassembly (HITCON 2018)

**Pattern:** A Lego EV3-over-Bluetooth capture contains RFCOMM frames whose payloads are EV3 direct commands. Packets are 32–34 bytes long, have an 8-byte RFCOMM header, and carry an `order` byte plus a `group_number` byte that together reorder into a coherent binary. Reassemble by (1) filtering `btrfcomm` in Wireshark, (2) sorting packets first by `group_number` then by `order`, and (3) concatenating the data fields after the header.

```python
# Python with pyshark
import pyshark
cap = pyshark.FileCapture("capture.pcap", display_filter="btrfcomm")
frames = []
for pkt in cap:
    raw = bytes.fromhex(pkt.btrfcomm.payload.replace(":", ""))
    # RFCOMM header size varies: 4 (UIH) or 5 (with length extension)
    hdr_len = 4 if raw[2] & 0x01 == 0 else 5
    body = raw[hdr_len:]
    order, group = body[0], body[1]
    frames.append((group, order, body[2:]))
frames.sort()
binary = b"".join(chunk for _, _, chunk in frames)
open("payload.bin", "wb").write(binary)
```

**Key insight:** RFCOMM is a TCP-like serial port emulation layered on L2CAP; it fragments application payloads when they exceed the MTU. CTF challenges love to split flags across many frames because most pcap walkthroughs stop at TCP/UDP and skip the Bluetooth link layer. Use Wireshark filters `btrfcomm.channel`, `btl2cap`, or `btsnoop_hci` to isolate the relevant flows, then sort by any available order/group bytes before concatenating. Similar logic applies to USB bulk transfers (`usb.transfer_type == 0x03`) and MIDI-over-BLE traffic.

**References:** HITCON CTF 2018 — EV3 Basic, writeup 11902

---

## GBA USB URB_INTERRUPT Framebuffer Extraction (hxp 2018)

**Pattern:** pcap contains USB `URB_INTERRUPT` packets from a Game Boy Advance debug adapter. The GBA framebuffer is `240 × 160` at RGB565 (2 bytes/pixel = 76 800 bytes). Block type 6 carries memory dumps; split each packet's payload into the framebuffer grid and convert RGB565 to 8-bit RGB tuples.

```python
from PIL import Image
from scapy.all import rdpcap
pkts = [p for p in rdpcap('cap.pcap') if p.haslayer('Raw')]
img = Image.new('RGB', (240, 160))
for p in pkts:
    if p.Raw.load[3] == 0x06:        # type 6 = memory dump
        data = p.Raw.load[4:]
        for i in range(76800 // 2):
            rgb = int.from_bytes(data[2*i:2*i+2], 'little')
            r = (rgb & 0xF800) >> 8
            g = (rgb & 0x07E0) >> 3
            b = (rgb & 0x001F) << 3
            img.putpixel((i % 240, i // 240), (r, g, b))
img.save('screen.png')
```

**Key insight:** Handheld console debug protocols usually wrap memory dumps in typed blocks. When you see GBA/NDS/PSP USB traffic, grep for block type 6 (framebuffer) or type 7 (audio) before parsing the rest.

**References:** hxp CTF 2018 — cheatquest of hxpschr 2, writeup 12591
