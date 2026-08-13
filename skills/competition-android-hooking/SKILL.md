---
name: competition-android-hooking
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for Android APK hooking, Frida tracing, request-signing recovery, SSL pinning bypass,
  JNI boundary inspection, and app trust-boundary analysis. Use when the user asks
  to hook an APK, inspect signer logic, trace Java or native boundaries, bypass pinning
  or root checks, inspect shared prefs or app databases, or replay accepted mobile
  requests. Use only after `$ctf-sandbox-orchestrator` has already established sandbox
  assumptions and routed here.
---

# Competition Android Hooking

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when the decisive path runs through an Android app's live trust boundary rather than static strings alone.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Preserve the original APK, extracted resources, and decompiled output before patching or resigning.
2. Start with manifest, exported components, deeplinks, native libs, prefs, local DBs, and bundled configs.
3. Decide the narrowest runtime boundary to hook: signer, crypto helper, JNI bridge, WebView bridge, or request builder.
4. Correlate static evidence and dynamic traces before claiming a trust edge is understood.
5. Reproduce the signed request, accepted token, or gated branch from the smallest hook set.

## Workflow

### 1. Static Triage Before Hooks

- Map package structure, exported activities, services, receivers, providers, and deeplink handlers.
- Note SSL pinning logic, root checks, feature flags, token storage, shared prefs, SQLite tables, and protobuf or RPC boundaries.
- Identify whether the sensitive logic sits in Java, Kotlin, JNI, or a bundled WebView.

### 2. Hook The Narrowest Boundary

- Prefer hooking request signers, crypto helpers, keystore access, protobuf encode or decode, or JNI marshaling instead of broad UI hooks.
- Record plaintext inputs, signed strings, headers, nonces, and outputs at the boundary that actually changes trust.
- If pinning or environment checks block progress, patch or hook only enough to expose the real request path.

### 3. Replay The Accepted Path

- Rebuild the smallest sequence that reaches the accepted server-side branch: local state, nonce, request body, signature, and headers.
- Keep hook logs, captured request shapes, and local storage paths tied to the same account or session state.
- If the challenge becomes more about transform recovery than Android runtime, switch back to the broader crypto or mobile skill.

## Read This Reference

- Load `references/android-hooking.md` for hook targets, storage checklist, and evidence packaging.

## What To Preserve

- Hook points, class names, JNI symbols, signer inputs and outputs, and header names
- Shared prefs, local DB rows, deeplinks, exported components, and token storage paths
- The smallest replayable request or branch that proves the trust boundary
