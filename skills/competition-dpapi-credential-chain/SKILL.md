---
name: competition-dpapi-credential-chain
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for DPAPI masterkeys, vault blobs, browser credential stores, protected secrets,
  domain backup keys, and secret-to-acceptance replay chains. Use when the user asks
  to inspect DPAPI blobs or masterkeys, recover browser or vault credentials, trace
  DPAPI context or backup-key use, or explain how protected Windows secrets become
  accepted access or privilege. Use only after `$ctf-sandbox-orchestrator` has already
  established sandbox assumptions and routed here.
---

# Competition Dpapi Credential Chain

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when the decisive Windows secret is DPAPI-protected and the hard part is proving which context unwraps it and where the plaintext is accepted.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Separate protected blob, masterkey, decrypting context, and final accepting service.
2. Record SID, user or machine context, masterkey path, vault or browser store, and target replay point before broad conclusions.
3. Keep DPAPI source artifact, unwrap step, plaintext secret, and acceptance edge in one chain.
4. Distinguish local user DPAPI, machine DPAPI, domain backup key use, and application-specific wrapping.
5. Reproduce the smallest DPAPI-to-accepted-access path that proves the decisive edge.

## Workflow

### 1. Map Protected Secret And DPAPI Context

- Record blob source, masterkey location, SID, protector scope, profile path, credential store, and any application wrapper such as browser encryption or vault metadata.
- Note whether the decisive value lives in Credential Manager, Vault, browser cookies, browser passwords, Wi-Fi profiles, RDP files, or custom app storage.
- Keep protected artifact, masterkey candidate, and account or machine context tied together.

### 2. Prove Unwrap And Acceptance

- Show how the secret is decrypted: user logon material, machine context, domain backup key, or another recovered protector.
- Record plaintext type, target host or service, replay method, and resulting session, token, or data access.
- Distinguish successful blob decryption from actual accepted access.

### 3. Reduce To The Decisive DPAPI Chain

- Compress the result to the smallest sequence: protected artifact -> masterkey or unwrap context -> plaintext secret -> accepted replay or access -> resulting capability.
- State clearly whether the decisive edge lives in masterkey recovery, DPAPI scope confusion, application wrapper handling, or the service that accepts the recovered secret.
- If the task broadens into generic LSASS ticket material or full Windows pivoting, hand back to the tighter host or pivot skill.

## Read This Reference

- Load `references/dpapi-credential-chain.md` for the blob checklist, masterkey checklist, and evidence packaging.

## What To Preserve

- Blob paths, masterkey paths, SIDs, protector scope, store names, and application wrapper details
- The exact accepting service or dataset unlocked by the recovered plaintext
- One minimal protected-artifact-to-accepted-access sequence that proves the edge
