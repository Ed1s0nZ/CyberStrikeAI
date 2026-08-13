---
name: competition-browser-persistence
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for browser cookies, localStorage, sessionStorage, IndexedDB, Cache Storage, service
  workers, offline caches, and client-side session persistence. Use when the user
  asks to inspect browser state, replay cached auth or session behavior, explain why
  a page behaves differently after load, or trace how stored client state changes
  requests, rendering, or access. Use only after `$ctf-sandbox-orchestrator` has already
  established sandbox assumptions and routed here.
---

# Competition Browser Persistence

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when the decisive branch lives in browser-held state rather than only in visible HTML or backend source.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Identify the active persistence surface first: cookie jar, localStorage, sessionStorage, IndexedDB, Cache Storage, or service worker.
2. Record origin, scope, domain, path, expiry, and key names before mutating state.
3. Tie stored state to one concrete effect: request header, rendered branch, cached response, offline behavior, or hidden route access.
4. Separate boot-time state from runtime-mutated state.
5. Reproduce the smallest stateful sequence that reaches the decisive branch.

## Workflow

### 1. Map Browser State Surfaces

- Inspect cookies, storage buckets, service worker registrations, cache entries, and transient globals exposed during boot.
- Record which origin, host, route, or feature flag each state item actually applies to.
- Keep auth tokens, refresh material, CSRF state, cached responses, and feature toggles in separate evidence blocks.

### 2. Tie State To Runtime Behavior

- Show how stored state becomes request headers, role derivation, route visibility, cached API data, or offline fallback behavior.
- Compare clean-state and mutated-state runs with one variable changed at a time.
- Distinguish UI-only state from backend-accepted state.

### 3. Reduce To The Decisive Persistence Chain

- Compress the result to the smallest chain: initial page or login -> state persisted -> subsequent request or render branch -> resulting capability.
- Keep extracted storage, service worker scripts, and replay steps tied to the same origin and route.
- If the problem broadens into general web routing or worker behavior outside browser persistence, switch back to the broader web-runtime skill.

## Read This Reference

- Load `references/browser-persistence.md` for the browser-state checklist, service-worker checklist, and evidence packaging.

## What To Preserve

- Cookie attributes, storage keys, database names, cache keys, service worker scopes, and origin boundaries
- The exact request or render effect caused by each decisive state item
- Clean-state vs mutated-state reproduction steps for the smallest working path
