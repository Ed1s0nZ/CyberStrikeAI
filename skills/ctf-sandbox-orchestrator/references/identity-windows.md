# Identity, Windows, AD, And Enterprise Messaging

Use this reference for Kerberos, LDAP, NTLM, WinRM, RDP, SMB, OAuth, SAML/OIDC, Exchange, mailbox rules, Windows host forensics, credential material, and lateral movement.

## Identity Flow

1. Map principal origin, sync or enrollment path, token/ticket minting, claims transformation, group resolution, and final consumer.
2. Separate credential material from usable privilege.
3. Express movement as a concrete chain: host -> recovered artifact -> replay path -> pivot host -> resulting capability.

## Windows Host And Lateral Movement

Inspect the active path across:

- SAM, SECURITY, SYSTEM, NTDS, DPAPI, LSA secrets
- PowerShell history, ETW, Sysmon, event logs, prefetch, jump lists, Amcache, SRUM, shimcache
- Services, tasks, WMI, WinRM, SMB, RDP, remote registry, admin shares, PsExec-like behavior

When Kerberos matters, record ticket type, SPN, delegation mode, PAC/group data, encryption type, cache location, and the accepting service.

When AD privilege edges matter, inspect ACLs, GPO links, SIDHistory, delegation, certificate templates, service accounts, and replication rights.

## Enterprise Messaging

Correlate phishing lures, attachment chains, consent logs, login traces, message-trace logs, and mailbox-rule changes so the mail path and identity path stay connected.

## Evidence To Keep

- SIDs, group names, event IDs, logon IDs, ticket caches, SPNs, delegation flags
- Mail headers, consent records, mailbox-rule changes, and downstream forwarding effects
- Exact replay point showing where the credential or ticket becomes effective

## Common Pitfalls

- Treating possession of a hash or ticket as proof of resulting privilege
- Describing "domain compromise" without a reproducible edge-by-edge chain
- Mixing mailbox evidence, identity evidence, and host evidence without a timeline
