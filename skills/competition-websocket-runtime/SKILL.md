---
name: competition-websocket-runtime
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for WebSocket and SSE handshakes, auth material, subscription state, realtime message
  schemas, reconnect behavior, and frame-driven runtime effects. Use when the user
  asks to inspect a WebSocket or SSE handshake, decode frames, trace subscriptions,
  follow reconnect logic, inspect auth material sent during realtime setup, or explain
  how live frames change rendered or persisted state. Use only after `$ctf-sandbox-orchestrator`
  has already established sandbox assumptions and routed here.
---

# Competition WebSocket Runtime

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when the decisive behavior is carried by realtime handshake and frame flow rather than one-shot HTTP alone.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Map the handshake first: origin, path, headers, cookies, query, auth token, and upgrade response.
2. Separate connection setup, subscription messages, keepalives, server pushes, and reconnect logic.
3. Record message schema, topic or channel identity, and state side effects in one chain.
4. Tie frames to rendered, stored, or backend-visible effects.
5. Reproduce the smallest handshake-plus-frame sequence that reaches the decisive state change.

## Workflow

### 1. Map The Realtime Handshake

- Record the initial HTTP or SSE request, upgrade headers, cookies, tokens, query params, origin checks, and negotiated protocol.
- Note whether auth material is carried by headers, cookies, query strings, or initial application frames.
- Keep route, subscription endpoint, and session identity tied together.

### 2. Decode Message Flow

- Separate subscribe, unsubscribe, ack, heartbeat, server push, reconnect, and terminal frames.
- Recover message types, channel IDs, schema fields, and sequencing that matter to behavior.
- Distinguish transport keepalive from application-level business messages.

### 3. Reduce To The Decisive Realtime Path

- Compress the result to the smallest sequence: handshake -> auth or subscribe frame -> pushed or accepted frame -> resulting state change.
- Keep canonical frame order and any replayed minimal order side by side.
- If the hard part is generic protocol reassembly without runtime UI or app-state linkage, switch back to the tighter protocol skill.

## Read This Reference

- Load `references/websocket-runtime.md` for the handshake checklist, frame checklist, and evidence packaging.

## What To Preserve

- Handshake headers, cookies, query params, auth material, negotiated subprotocol, and channel IDs
- Frame schemas, subscription messages, server pushes, reconnect flow, and resulting state changes
- The smallest replayable realtime sequence that proves the decisive branch
