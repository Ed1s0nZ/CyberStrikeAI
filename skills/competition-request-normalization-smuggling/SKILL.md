---
name: competition-request-normalization-smuggling
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for parser differentials, HTTP normalization gaps, ambiguous headers, path decoding
  drift, transfer-framing mismatches, and request smuggling routes. Use when the user
  asks to trace proxy and backend parse differences, conflicting path normalization,
  Host or forwarded-header ambiguity, CL/TE issues, or routing outcomes that differ
  across hops. Use only after `$ctf-sandbox-orchestrator` has already established
  sandbox assumptions and routed here.
---

# Competition Request Normalization Smuggling

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when request interpretation changes between proxy, middleware, and backend parser layers.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Map every parsing hop: client-facing proxy, gateway, app server, and downstream service.
2. Record path normalization, header canonicalization, transfer framing, and host derivation at each hop.
3. Capture one accepted baseline request and one differential request with minimal delta.
4. Prove which hop interprets the request differently.
5. Reproduce one minimal differential path that yields decisive behavior.

## Workflow

### 1. Map Parse And Routing Boundaries

- Record `Host`, forwarded headers, path decoding, slash collapsing, dot-segment handling, and case behavior.
- Note `Content-Length`, `Transfer-Encoding`, chunk framing, and connection reuse behavior when relevant.
- Keep edge parser and backend parser decisions side by side.

### 2. Prove Differential Interpretation

- Build paired requests that differ in one canonicalization dimension only.
- Capture proxy logs, backend logs, route match, and downstream request shape.
- Show where route, auth scope, or body boundary diverges.

### 3. Reduce To Decisive Smuggling Chain

- Compress to: crafted request -> parser differential across hops -> unintended routed request or hidden endpoint reach -> resulting effect.
- State whether root cause is path normalization drift, header ambiguity, transfer framing differential, or host-derivation confusion.
- If the chain becomes primarily runtime routing without framing tricks, hand off to runtime routing skill.

## Read This Reference

- Load `references/request-normalization-smuggling.md` for parse-differential checklist and evidence packaging.

## What To Preserve

- Raw request pairs, hop-by-hop interpretation, and final routed target
- Exact normalization or framing delta that flips behavior
- One minimal replayable differential request path
