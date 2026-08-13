# LSASS Ticket Material Checklist

## First Pass

- Logon sessions, LUIDs, package names, ticket caches, SSP artifacts, DPAPI context, account names
- Material types: plaintext, NTLM hash, TGT, service ticket, delegated ticket, DPAPI secret, gMSA material
- Acceptance candidates: SMB, WinRM, service ticket use, Schannel, DPAPI unwrap, service logon

## Chain To Reconstruct

1. Host or memory artifact located
2. Credential or ticket material extracted
3. Replay or unwrap path identified
4. Service or host accepts it
5. Resulting logon, token, or privilege confirmed

## Evidence To Keep Together

- Host side: process, dump or cache source, LUID, package, account
- Material side: type, ticket flags, SPN, encryption, DPAPI context, cache location
- Acceptance side: target service, target host, resulting session or privilege change

## Common Pitfalls

- Treating every extracted secret as replayable without proving an accepting service
- Mixing several logon sessions and ticket caches into one vague story
- Describing ticket presence without tying it to a concrete privilege or pivot edge
