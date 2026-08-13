---
name: competition-ios-runtime
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for IPA runtime analysis, Frida hooks, Objective-C or Swift method tracing, Keychain
  inspection, SSL pinning bypass, URL scheme handling, and iOS request-signing recovery.
  Use when the user asks to hook an IPA, trace Objective-C or Swift runtime behavior,
  inspect Keychain or plist state, bypass pinning, analyze deeplinks or universal
  links, or replay accepted iOS requests. Use only after `$ctf-sandbox-orchestrator`
  has already established sandbox assumptions and routed here.
---

# Competition iOS Runtime

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when the decisive path runs through live iOS trust boundaries rather than static strings or plist values alone.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Preserve the original IPA, extracted bundle, and any decrypted or re-signed copy as separate artifacts.
2. Start with `Info.plist`, entitlements, URL schemes, frameworks, Keychain usage, and local app storage before broad runtime hooks.
3. Choose the narrowest runtime boundary that proves behavior: signer, trust evaluator, Keychain accessor, Objective-C or Swift method, or network request builder.
4. Correlate static bundle evidence and live hook output before claiming the trust path is understood.
5. Reproduce the accepted request, token, or gated branch from the smallest hook set.

## Workflow

### 1. Static iOS Triage

- Map bundle structure, `Info.plist`, entitlements, URL schemes, universal links, embedded frameworks, and app group paths.
- Record likely trust boundaries: request signers, device binding, certificate checks, jailbreak checks, Keychain access, or local cache loading.
- Note whether sensitive logic sits in Objective-C, Swift, embedded frameworks, or a bundled web surface.

### 2. Hook The Runtime Boundary

- Prefer hooking request builders, crypto helpers, trust evaluators, Keychain reads, or Objective-C selectors instead of broad UI handlers.
- Record plaintext inputs, headers, nonces, signed strings, and outputs at the boundary that changes server acceptance.
- Patch or bypass pinning or environment checks only enough to expose the real request path.

### 3. Replay The Accepted Path

- Rebuild the smallest stateful sequence: local token, device identifier, request body, signature, headers, and trust checks.
- Keep hook logs, bundle paths, plist keys, and local storage artifacts tied to the same session or account state.
- If the task becomes mostly about transform recovery instead of iOS runtime, switch back to the broader crypto or mobile skill.

## Read This Reference

- Load `references/ios-runtime.md` for hook targets, storage checklist, and evidence packaging.

## What To Preserve

- Bundle paths, entitlements, plist keys, selectors, class names, hook points, and header names
- Keychain items, local DB or plist paths, URL schemes, and app-group storage locations
- The smallest replayable request or branch that proves the iOS trust boundary
