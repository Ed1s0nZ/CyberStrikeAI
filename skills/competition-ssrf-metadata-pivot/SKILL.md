---
name: competition-ssrf-metadata-pivot
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for SSRF reachability, internal route probing, metadata-service access, credential
  pivoting, and token-to-accepted-privilege chains. Use when the user asks to trace
  SSRF sources, internal hosts, metadata endpoints, link-local tokens, service-account
  credentials, or explain how a server-side fetch edge turns into accepted access.
  Use only after `$ctf-sandbox-orchestrator` has already established sandbox assumptions
  and routed here.
---

# Competition SSRF Metadata Pivot

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when the decisive path runs through server-side request capability, internal service reachability, or metadata-derived credentials.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Separate the SSRF source, forwarding layer, reachable target, and accepted downstream credential edge.
2. Record request method, URL construction, header behavior, redirects, DNS or host overrides, and response shaping before mutation.
3. Map internal host, metadata endpoint, token extraction, and accepting service as one chain.
4. Distinguish read-only reachability from credential-bearing access.
5. Reproduce the smallest SSRF-to-accepted-access path.

## Workflow

### 1. Map SSRF Reachability

- Record source primitive: URL parameter, webhook, image fetcher, importer, proxy endpoint, or backend callback.
- Note normalization steps: scheme filtering, host allowlists, redirects, DNS resolution, path rewrite, and header injection.
- Keep target host, protocol, and response behavior tied to the exact SSRF source.

### 2. Trace Metadata And Credential Pivot

- Show whether metadata endpoints, internal control APIs, or workload identity services are reachable.
- Record token fields, role scope, service account, expiration, and where the token is accepted.
- Distinguish credential extraction success from accepted privilege at a downstream service.

### 3. Reduce To Decisive SSRF Chain

- Compress to: SSRF source -> internal or metadata target -> credential or sensitive response -> accepted replay or API access.
- State whether the decisive edge is parser bypass, allowlist bypass, redirect abuse, header confusion, or metadata trust.
- If the task becomes mostly cloud identity policy analysis, hand off to the tighter cloud metadata skill.

## Read This Reference

- Load `references/ssrf-metadata-pivot.md` for SSRF checklists, metadata pivots, and evidence packaging.

## What To Preserve

- SSRF source point, URL construction rules, reachable hosts, and response deltas
- Extracted token or credential fields, scope, and accepting service
- One minimal SSRF-to-accepted-access replay path
