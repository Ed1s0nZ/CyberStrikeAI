# Internal Network Discovery & Enumeration

## Phase 1: Host Discovery
```bash
# Ping sweep
nmap -sn -T4 SUBNET

# ARP scan (more reliable)
sudo nmap -PR -sn SUBNET

# UPnP/SSDP
python3 -c "import socket; s=socket.socket(socket.AF_INET,socket.SOCK_DGRAM); s.settimeout(5); s.sendto(b'M-SEARCH * HTTP/1.1\r\nHost:239.255.255.250:1900\r\nST:ssdp:all\r\nMX:3\r\nMan:\"ssdp:discover\"\r\n\r\n',('239.255.255.250',1900))"
```

## Phase 2: Service Enumeration
```bash
# Top ports with version detection
nmap -sV -sC -T4 --top-ports 500 --open TARGET_LIST

# IoT-specific ports
nmap -sV -p 5555,554,1883,8883,8080,8443,49152,37787 --open SUBNET

# SMB sweep
nmap -p 445,139 --open SUBNET
```

## Phase 3: Target Identification
| Port | Service | Attack Vector |
|------|---------|---------------|
| 5555 | ADB | Connect directly, often no auth |
| 22 | SSH | Brute with harvested creds |
| 23 | Telnet | Default creds |
| 80/8080 | HTTP | Default creds, CVEs |
| 445 | SMB | Null session, EternalBlue, brute |
| 3389 | RDP | Brute, BlueKeep |
| 554 | RTSP | Default creds on cameras |
| 1883 | MQTT | Usually no auth |
| 8443 | HTTPS | Router admin panels |

## Phase 4: Device Fingerprinting
```bash
# MAC vendor lookup
nmap -sn SUBNET | grep MAC  # First 3 octets = vendor

# Common vendors:
# Sony Interactive = PlayStation
# Intel Corporate = Laptop/PC
# Shenzhen Hikeen = Chinese IoT (TV, camera)
# Dell = Server/workstation
# Apple = iPhone/Mac
# Samsung = Phone/TV
```

## Phase 5: Prioritize Targets
1. Open ADB (5555) - instant shell
2. IoT with default creds (cameras, routers)
3. SMB with weak/reused passwords
4. Unpatched services (EternalBlue, BlueKeep)
5. Web panels with known CVEs
