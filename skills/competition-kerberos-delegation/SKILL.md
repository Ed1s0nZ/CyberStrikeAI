---
name: competition-kerberos-delegation
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for Kerberos delegation, SPN trust edges, S4U abuse, RBCD, constrained or unconstrained
  delegation, and service-ticket acceptance. Use when the user asks about constrained
  delegation, unconstrained delegation, RBCD, S4U, SPNs, ticket acceptance, or how
  a Kerberos trust edge turns into effective privilege under sandbox assumptions.
  Use only after `$ctf-sandbox-orchestrator` has already established sandbox assumptions
  and routed here.
---

# Competition Kerberos Delegation

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when the hard part is not "is there Kerberos here," but which delegation edge exists, which ticket is being minted, and which service really accepts it.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Write the trust chain first: principal -> delegation edge -> ticket type -> target SPN -> accepting service -> resulting privilege.
2. Separate ticket possession from accepted privilege.
3. Keep SPNs, delegation mode, PAC/group data, encryption type, and service acceptance in one compact evidence block.
4. Reproduce one minimal delegation chain before broadening into variants.
5. Tie every privilege claim to a specific accepted ticket or service-side effect.

## Workflow

### 1. Identify The Delegation Edge

- Determine whether the path is constrained delegation, unconstrained delegation, resource-based constrained delegation, protocol transition, or another trust edge.
- Inspect SPNs, ACLs, service accounts, SIDHistory, certificate templates, and replication rights only when they affect the active path.

### 2. Trace Ticket Minting And Acceptance

- Record TGT/TGS type, S4U steps when relevant, delegation flags, PAC or group data, encryption type, cache location, and target SPN.
- Prove which service actually accepts the ticket and what capability appears after acceptance.

### 3. Report The Effective Edge

- Compress the chain into one replayable path, not a vague "domain compromise" statement.
- Separate candidate edges from the edge that really lands privilege.

## Read This Reference

- Load `references/kerberos-delegation.md` for the delegation checklist, ticket fields to preserve, and common proof mistakes.

## What To Preserve

- SPN, ticket type, delegation mode, PAC/group data, encryption type, cache location, accepting service
- Service-side logs, event IDs, logon session changes, or group changes proving effective privilege
- The exact trust edge that makes the ticket replayable
