# Competition Sandbox Router Matrix

Use this file after the sandbox model is active and one dominant evidence type or blocker is visible.

## Default Rule

Start in `ctf-sandbox-orchestrator`. Route to a child skill only when the decisive unknown is narrower than the generic sandbox workflow.
All child `competition-*` skills are downstream-only modules. They should not be entered implicitly before `ctf-sandbox-orchestrator` has already established sandbox assumptions.

## Stay In Competition Sandbox When

- The task is still ambiguous across domains.
- You are only building the first node map or the first minimal path.
- Several domains are involved but none dominates yet.
- The main blocker is deciding which evidence matters first.

## Route By Dominant Surface

### Web And Runtime

- General site, API, auth, session, upload, worker, or hidden route behavior -> `$competition-web-runtime`
- Browser storage, service workers, Cache Storage, IndexedDB -> `$competition-browser-persistence`
- WebSocket or SSE handshake, subscriptions, reconnect logic, realtime frames -> `$competition-websocket-runtime`
- Host headers, vhost routing, reverse proxies, route-to-service resolution -> `$competition-runtime-routing`
- SSR loaders, templates, hydration payloads, render-layer enforcement drift -> `$competition-template-render-path`
- Source maps, build manifests, emitted bundles, chunk loading, runtime recovery -> `$competition-bundle-sourcemap-recovery`
- GraphQL schema, persisted query, RPC manifest, generated client, contract drift -> `$competition-graphql-rpc-drift`
- OAuth or OIDC redirect, callback, PKCE, claim acceptance -> `$competition-oauth-oidc-chain`
- JWT header, `kid`, `alg`, issuer, audience, role or claim confusion -> `$competition-jwt-claim-confusion`
- Upload parsing, archive extraction, preview conversion, deserialization -> `$competition-file-parser-chain`
- Queue payloads, retries, cron drift, worker-only behavior -> `$competition-queue-worker-drift`
- SSRF reachability, internal fetch pivots, metadata token extraction, token acceptance -> `$competition-ssrf-metadata-pivot`
- Race windows, ordering bugs, idempotency failures, timing-dependent state drift -> `$competition-race-condition-state-drift`
- Proxy/backend parse differentials, path normalization drift, framing ambiguity, smuggling -> `$competition-request-normalization-smuggling`

### Reverse, Exploit, DFIR, Binary

- Binary triage, exploit primitives, crash behavior, native reversing -> `$competition-reverse-pwn`
- Malware config, staged payload, beacon parameters, IOC decoding -> `$competition-malware-config`
- Firmware partitions, boot chains, update packages, extracted filesystem -> `$competition-firmware-layout`
- Packet captures, protocol reconstruction, stream rebuilding -> `$competition-pcap-protocol`
- Custom protocol replay, state machine, checksums, accepted session reproduction -> `$competition-custom-protocol-replay`
- Timeline reconstruction across disk, logs, PCAP, registry, mailbox -> `$competition-forensic-timeline`

### Crypto, Stego, Mobile

- Encoding chain, crypto boundary, stego media, mobile trust boundary -> `$competition-crypto-mobile`
- Image, audio, video, document, trailer, metadata stego -> `$competition-stego-media`
- Android APK, Frida, signer logic, SSL pinning, native bridge -> `$competition-android-hooking`
- iOS IPA, Objective-C or Swift runtime, Keychain, pinning, deeplink -> `$competition-ios-runtime`

### Agent, Cloud, Container, Supply Chain

- Prompt injection, retrieval poisoning, planner drift, tool misuse -> `$competition-prompt-injection`
- Agent, cloud, CI/CD, container, supply-chain as a broad cluster -> `$competition-agent-cloud`
- Image layers, sidecars, mounts, runtime-vs-manifest mismatch -> `$competition-container-runtime`
- Registry provenance, dependency drift, build or release tampering -> `$competition-supply-chain`
- K8s API, service account, RBAC, admission/controller behavior -> `$competition-k8s-control-plane`
- Metadata service, workload identity, link-local credential path -> `$competition-cloud-metadata-path`
- Container-to-host boundary crossing, kernel preconditions, namespace or cgroup escape proof -> `$competition-kernel-container-escape`

### Identity And Windows

- Identity chain, Windows host evidence, enterprise messaging, mixed AD task -> `$competition-identity-windows`
- Host-to-host pivot, WinRM, SMB, RDP, accepted replay edge -> `$competition-windows-pivot`
- LSASS memory, ticket caches, replayable session material -> `$competition-lsass-ticket-material`
- DPAPI masterkeys, vault blobs, browser secret stores, unwrap-to-access chain -> `$competition-dpapi-credential-chain`
- Kerberos delegation, S4U, RBCD, delegated ticket acceptance -> `$competition-kerberos-delegation`
- AD CS template, PKINIT, enrollment rights, cert-based privilege -> `$competition-ad-certificate-abuse`
- Coercion primitive, relay target, forced auth acceptance -> `$competition-relay-coercion-chain`
- Mailbox rules, forwarding, OAuth consent, delegate abuse -> `$competition-mailbox-abuse`
- Linux secret replay, socket trust edges, SSH or token pivots, host-to-host Linux movement -> `$competition-linux-credential-pivot`

## Re-Route Rules

- If the current child skill stops matching the dominant blocker, return to `ctf-sandbox-orchestrator` and pick a narrower path.
- If the path broadens from a narrow child skill into a mixed-domain chain, return to `ctf-sandbox-orchestrator` and rebuild the route from the earliest uncertain step.
- Keep only one primary child skill active unless a second one is clearly needed for the next decisive boundary.
