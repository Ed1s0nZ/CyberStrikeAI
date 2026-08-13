---
name: competition-windows-pivot
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for Kerberos, WinRM, SMB, RDP, Windows credential material, replayable tickets,
  delegation edges, and host-to-host pivot chains. Use when the user asks to replay
  Kerberos material, trace a WinRM, SMB, or RDP pivot, understand host-to-host privilege
  movement, or prove which Windows service accepted a credential or ticket. Use only
  after `$ctf-sandbox-orchestrator` has already established sandbox assumptions and
  routed here.
---

# Competition Windows Pivot

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when the challenge path is dominated by host-to-host movement, replayable ticket material, or Windows privilege edges.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Compress the pivot into a concrete chain: foothold -> recovered artifact -> replay path -> pivot host -> resulting capability.
2. Separate stored credential material from usable privilege.
3. Keep host evidence, ticket evidence, and privilege effect on one timeline.
4. Record the exact accepting service or host for every replayed artifact.
5. Reproduce the smallest pivot that still proves the privilege edge.

## Workflow

### 1. Recover The Replay Material

- Inspect SAM, SECURITY, SYSTEM, NTDS, DPAPI, LSA secrets, browser stores, PowerShell history, ETW, Sysmon, and event logs in the active path.
- Distinguish password, hash, ticket, cookie, vault blob, or gMSA material by where it can actually be used.

### 2. Trace The Pivot Chain

- Map the protocol actually used: WinRM, SMB, RDP, WMI, admin shares, remote registry, or service control.
- When Kerberos matters, record SPN, delegation, PAC or group data, encryption type, and the accepting service.
- When AD edges matter, inspect ACLs, GPO links, SIDHistory, delegation, certificate templates, and replication rights.

### 3. Report The Edge

- Keep the pivot path concrete and replayable.
- State what artifact crossed which boundary and what capability appeared on the destination host.

## Read This Reference

- Load `references/windows-pivot.md` for the pivot checklist, Kerberos evidence block, and common replay mistakes.
- If the task is specifically about DPAPI masterkeys, browser or vault stores, protected blobs, or proving where a recovered DPAPI secret is accepted, prefer `$competition-dpapi-credential-chain`.
- If the task is specifically about LSASS memory, ticket caches, replayable session material, or host-resident credential extraction, prefer `$competition-lsass-ticket-material`.
- If the task is specifically about delegation edges, SPN trust, S4U flow, or which service accepts the delegated ticket, prefer `$competition-kerberos-delegation`.
- If the hard part is forced authentication, coercion primitives, relay targets, or the service that accepts relayed auth, prefer `$competition-relay-coercion-chain`.

## What To Preserve

- Host names, logon IDs, SIDs, SPNs, ticket fields, service names, and event IDs
- Exact replay point and resulting logon session, token, or group change
- Raw host artifacts and derived timeline separately
