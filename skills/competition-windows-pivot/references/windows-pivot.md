# Windows Pivot And Kerberos Checklist

## Pivot Chain

Write the chain explicitly:

- source host
- recovered artifact
- replay protocol or service
- destination host
- resulting capability

## Kerberos Fields To Keep

- Ticket type
- SPN
- Delegation mode
- PAC or group data
- Encryption type
- Cache location
- Accepting service

## Host Evidence To Keep

- Event IDs, logon IDs, process creation, service creation, task creation
- WinRM, SMB, RDP, WMI, admin-share, and remote-registry traces
- Group membership or token changes on the destination host

## Common Pitfalls

- Treating possession of a ticket as proof of accepted privilege
- Saying a pivot worked without showing the destination-side effect
- Mixing several hops into one vague statement instead of a reproducible chain
