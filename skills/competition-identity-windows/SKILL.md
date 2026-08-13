---
name: competition-identity-windows
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for Active Directory, Kerberos, LDAP, OAuth, enterprise messaging, Windows host
  forensics, credential material, and lateral-movement challenges. Use when the user
  asks to trace tickets or tokens, inspect mailbox rules, analyze Windows host evidence,
  understand an AD trust path, or explain a lateral-movement chain across sandbox-linked
  nodes. Use only after `$ctf-sandbox-orchestrator` has already established sandbox
  assumptions and routed here.
---

# Competition Identity Windows

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when the challenge revolves around identity flow, replayable credentials, Windows host artifacts, enterprise mail, or lateral movement.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Map the identity or pivot chain before diving into every host artifact.
2. Separate credential possession from accepted privilege.
3. Correlate identity evidence, host evidence, and mail evidence on one timeline.
4. Keep tickets, SIDs, event IDs, mailbox rules, and pivot hosts in compact evidence blocks.
5. Reproduce the privilege edge or mail effect from the smallest viable chain.

## Workflow

### 1. Identity And AD

- Trace principal origin, sync path, token or ticket minting, claims transformation, group resolution, and accepting service.
- When Kerberos matters, record ticket type, SPN, delegation mode, PAC or group data, encryption type, and cache location.

### 2. Windows Host And Pivoting

- Correlate SAM, SECURITY, SYSTEM, NTDS, DPAPI, LSA secrets, ETW, Sysmon, PowerShell, services, tasks, WMI, WinRM, SMB, and RDP as one pivot graph.
- Express movement as a concrete chain: foothold -> recovered artifact -> replay path -> pivot host -> resulting capability.

### 3. Enterprise Messaging

- Keep phishing lures, consent logs, mailbox rules, and identity-provider events tied together so the mail path and privilege path stay connected.

## Read This Reference

- Load `references/identity-windows.md` for the ticket, host, and enterprise-messaging checklist.
- If the task is primarily a host-to-host pivot, Kerberos replay, or Windows privilege chain, prefer `$competition-windows-pivot`.
- If the task is specifically about constrained delegation, unconstrained delegation, RBCD, S4U, or ticket-acceptance proof, prefer `$competition-kerberos-delegation`.
- If the task is specifically about AD CS, certificate templates, EKUs, enrollment rights, PKINIT, or cert-based privilege, prefer `$competition-ad-certificate-abuse`.
- If the task is specifically about OAuth or OIDC claims, callback flow, scopes, consent, or accepted login identity, prefer `$competition-oauth-oidc-chain`.
- If the task is specifically about DPAPI masterkeys, vault blobs, browser or vault secrets, backup-key use, or protected-secret-to-access chains, prefer `$competition-dpapi-credential-chain`.
- If the task is specifically about LSASS memory, ticket caches, LUID-linked material, DPAPI context, or replayable host credential artifacts, prefer `$competition-lsass-ticket-material`.
- If the task is specifically about mailbox rules, forwarding, OAuth consent, delegate access, or transport-level mail abuse, prefer `$competition-mailbox-abuse`.
- If the task is specifically about forced authentication, relay targets, or proving which service accepts relayed auth, prefer `$competition-relay-coercion-chain`.

## What To Preserve

- SIDs, SPNs, ticket fields, event IDs, mailbox rules, and replay points
- Exact host-to-host pivot order and the service that accepts the credential or ticket
- Raw artifacts, parsed summaries, and derived timelines as separate outputs
