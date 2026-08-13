---
name: competition-crypto-mobile
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for crypto, encoding, steganography, APK, IPA, and mobile trust-boundary challenges.
  Use when the user asks to decode a blob, recover a transform chain or key, inspect
  hidden media payloads, hook an APK or IPA signer, inspect app storage, or replay
  mobile request-signing logic. Use only after `$ctf-sandbox-orchestrator` has already
  established sandbox assumptions and routed here.
---

# Competition Crypto Mobile

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when the active challenge depends on recovering a transform chain, hidden media payload, mobile signing path, or local trust boundary.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Decide whether the dominant path is crypto, stego, or mobile.
2. Recover transforms in order; do not jump straight to the fanciest algorithm.
3. Record exact parameters and boundaries that affect the result.
4. Hook the narrowest mobile boundary that proves the behavior.
5. Reproduce the plaintext, payload, signed request, or accepted branch.

## Workflow

### 1. Crypto And Encoding

- Reconstruct the chain step by step: container, compression, encoding, xor or substitution, crypto, integrity, final parse.
- Keep exact keys, IVs, nonces, salts, tags, offsets, and byte order.

### 2. Stego

- Inspect metadata, chunk layout, palettes, alpha planes, LSBs, thumbnails, trailers, and transcoding artifacts.
- Rank decode attempts by evidence, not by brute-force curiosity.

### 3. Mobile

- Start with manifest or plist, exported components, deeplinks, native libs, shared prefs, local DBs, and configs.
- Trace signer logic, token storage, SSL pinning, protobuf or RPC boundaries, and native bridge calls.

## Read This Reference

- Load `references/crypto-mobile.md` for the transform checklist, hook targets, and evidence packaging.
- If the task is specifically about Android dynamic tracing, signer hooks, JNI boundaries, or pinning checks, prefer `$competition-android-hooking`.
- If the task is specifically about iOS runtime tracing, Keychain access, Objective-C or Swift hooks, or pinning checks inside an IPA, prefer `$competition-ios-runtime`.
- If the task is specifically about media carriers, hidden channels, thumbnails, or appended trailers, prefer `$competition-stego-media`.

## What To Preserve

- Decisive bytes proving each decode stage
- Hook points, signed strings, headers, and local storage paths
- Component names, protobuf fields, channel-specific outputs, or trailer offsets
