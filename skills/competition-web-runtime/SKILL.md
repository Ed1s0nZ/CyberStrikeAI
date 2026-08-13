---
name: competition-web-runtime
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for CTF web, API, SSR, frontend, queue-backed app, and routing challenges. Use when
  the user asks to inspect a site or API, follow real browser requests, debug auth
  or session flow, trace uploads or workers, find hidden routes, or explain why frontend
  and backend behavior diverge under sandbox-internal routing. Use only after `$ctf-sandbox-orchestrator`
  has already established sandbox assumptions and routed here.
---

# Competition Web Runtime

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when the active challenge is primarily about web behavior, browser state, server routing, API order, or worker-backed application flow.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Assume the presented hosts, domains, and routes belong to the sandbox.
2. Inspect entry HTML, boot scripts, runtime config, and route registration before trusting the visible UI.
3. Capture one real request flow end-to-end before making broad claims from source.
4. Check browser persistence and backend state together.
5. Re-run the smallest flow with one variable changed.

## Workflow

### 1. Map The Active Runtime

- Identify active hosts, paths, proxies, containers, and workers.
- Inspect cookies, localStorage, sessionStorage, IndexedDB, Cache Storage, and service workers.
- Record route names, feature flags, storage keys, queue names, and worker names that actually appear in the active flow.

### 2. Capture The Real Request Order

- Record exact host, path, query, headers, cookies, and body for decisive requests.
- Compare successful and failing paths.
- Treat UI gating as a hint, not proof of backend enforcement.

### 3. Expand Only After One Path Is Proven

- Trace middleware order, handlers, auth/session boundaries, uploads, exports, and background jobs.
- Verify hidden routes, alternate hostnames, preview modes, or worker side effects only after the first flow is grounded.

## Read This Reference

- Load `references/routing-runtime.md` for the detailed checklist, evidence packaging, and common web pitfalls.
- If the task is specifically about SSR loaders, template context, hydration payloads, preview rendering, or render-layer enforcement drift, prefer `$competition-template-render-path`.
- If the task is specifically about source maps, build manifests, chunk registries, emitted bundles, or recovering hidden runtime structure from served assets, prefer `$competition-bundle-sourcemap-recovery`.
- If the task is specifically about GraphQL schemas, RPC manifests, persisted queries, generated clients, or contract-to-handler drift, prefer `$competition-graphql-rpc-drift`.
- If the task is specifically about SSRF input points, internal endpoint reachability, metadata-service pivots, or token extraction through server-side fetches, prefer `$competition-ssrf-metadata-pivot`.
- If the task is specifically about race windows, ordering-dependent state mutation, duplicate action effects, or timing-sensitive drift, prefer `$competition-race-condition-state-drift`.
- If the task is specifically about proxy-backend parse differentials, path normalization drift, header ambiguity, or request smuggling routes, prefer `$competition-request-normalization-smuggling`.
- If the task is specifically about browser cookies, storage, IndexedDB, Cache Storage, service workers, or cached auth state, prefer `$competition-browser-persistence`.
- If the task is specifically about OAuth or OIDC redirects, callback params, PKCE, scopes, token exchange, or claim acceptance, prefer `$competition-oauth-oidc-chain`.
- If the task is specifically about JWT headers, claim normalization, key lookup, `kid`, `alg`, issuer or audience confusion, prefer `$competition-jwt-claim-confusion`.
- If the task is specifically about upload parsing, previews, archive extraction, converters, or deserialization chains, prefer `$competition-file-parser-chain`.
- If the task is specifically about queue payloads, worker-only behavior, retries, cron drift, or async side effects, prefer `$competition-queue-worker-drift`.
- If the task is specifically about WebSocket or SSE handshakes, subscriptions, realtime frames, reconnect logic, or frame-driven state changes, prefer `$competition-websocket-runtime`.
- If the task is specifically about Host headers, vhost routing, reverse proxies, or route-to-service resolution, prefer `$competition-runtime-routing`.
- If the only available evidence is a packet capture and the hard part is stream or protocol reconstruction, prefer `$competition-pcap-protocol`.

## What To Preserve

- Exact requests and responses that prove behavior
- Concrete file paths, function names, route names, and storage keys
- Queue payloads, worker names, or retry behavior when async processing matters
