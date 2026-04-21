# EdgeTX Radio Control

## Auto-Detection by USB VID:PID
| USB ID | Mode | Device | Capability |
|--------|------|--------|-----------|
| 1209:4f54 | Joystick | /dev/input/js0 | Read 8+ RC channels |
| 0483:5740 | Serial VCP | /dev/ttyACM0 | CLI, passthrough, module control |
| 0483:5720 | Storage | /dev/sd* | SD card access (configs, Lua, firmware) |
| 0483:df11 | DFU | - | Flash firmware via dfu-util |
| 10c4:ea60 | ELRS USB | /dev/ttyUSB0 | Direct ESP32 module (esptool) |

Auto-controller: `~/combat/tx12_auto.py`

## CLI Commands (Serial VCP Mode)
```
set pulses 0/1              - freeze/resume RF output
set rfmod 0 power off/on    - module power
set rfmod 0 bootpin 1/0     - ESP32 boot mode
serialpassthrough rfmod 0 <baud> - direct UART bridge
ls /path                    - SD card listing
reboot                      - reboot (DFU if USB connected)
```

## Firmware (TX12 MkII, EdgeTX 2.9.4)
- STM32F405, 1MB flash
- Bootloader: 0x08000000 (64KB), Main: 0x08010000 (960KB)
- Baud table at 0x0807aed4: index 0=115200, 1=400000, 2=921600, 3=1870000
- radio.yml `internalModuleBaudrate: 2` = 921600 baud
- DFU flash: `dfu-util -a 0 -s 0x08010000:leave -D firmware.bin`

## CRSF Access
- NOT available as AUX serial mode in EdgeTX 2.9.4
- Use Lua API: `crossfireTelemetryPush/Pop()` + `serialWrite/Read()`
- Or Telemetry Mirror (S.Port format, 57600 baud - re-encoded, not raw)
- Or serialpassthrough for direct module UART access

## Joystick Channel Reading
```python
import struct
fd = open('/dev/input/js0', 'rb')
axes = [0]*8  # Roll,Pitch,Thr,Yaw,SA,SB,SC,SD (-32767 to +32767)
while True:
    ts, val, etype, num = struct.unpack('IhBB', fd.read(8))
    if etype & 0x02 and num < 8: axes[num] = val
```

## Safety
- NEVER scan baud rates on serial port - locks EdgeTX, kills RC link
- NEVER write before confirming data flow from radio
- Model must have internalModule configured for CRSF
- USB disconnect recovers stuck radio

## Attack Escalation Path
Joystick (read-only) → Serial (CLI control) → Storage (plant Lua/modify config) → DFU (flash firmware) → Full control. Each mode enables access to the next. Plant persistence at every opportunity.
