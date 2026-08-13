---
name: competition-reverse-pwn
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for reverse engineering, malware, DFIR, firmware, pwnable, and native exploit challenges.
  Use when the user asks to reverse a binary, unpack a sample, inspect a memory dump
  or PCAP, recover malware behavior, debug a crash, or build or verify an exploit
  chain under sandbox assumptions. Use only after `$ctf-sandbox-orchestrator` has
  already established sandbox assumptions and routed here.
---

# Competition Reverse Pwn

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill for binary-heavy challenges where the decisive path runs through artifacts, decoded layers, process behavior, crash state, or exploit primitives.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Preserve the original artifact before unpacking, patching, or instrumenting.
2. Start with passive triage: type, headers, sections, imports, strings, entropy, resources.
3. Decide whether the path is reverse-first, DFIR-first, or exploit-first.
4. Tie every claim to an observable boundary: decode edge, persistence edge, crash edge, or leak edge.
5. Reproduce the artifact or primitive from a clean baseline.

## Workflow

### 1. Reverse Or Forensic Triage

- Separate loader, payload, config, and post-decode behavior.
- Correlate files, memory, logs, registry, services, tasks, IPC, and PCAPs as one graph.
- Keep decoded or dumped artifacts separate from the pristine sample.

### 2. Native And Exploit Path

- Map mitigations, loader behavior, libc or runtime, syscall and IPC surfaces, and protocol framing.
- Record the primitive, controllable bytes, leak source, target object, and final artifact separately.
- Compare host, libc, loader, and framing differences before doubting the primitive.

## Read This Reference

- Load `references/reverse-pwn.md` for triage order, exploit evidence expectations, and common failure modes.
- If the task is specifically about staged payload boundaries, config blobs, beacon parameters, or decoded IOC fields, prefer `$competition-malware-config`.
- If the task is specifically about firmware partitions, boot chains, extracted filesystems, or update-package trust boundaries, prefer `$competition-firmware-layout`.
- If the task is specifically about upload parsing, previews, archive extraction, converters, or deserialization chains, prefer `$competition-file-parser-chain`.
- If the task is specifically about source maps, emitted bundles, chunk registries, or reconstructing hidden runtime structure from served frontend assets, prefer `$competition-bundle-sourcemap-recovery`.
- If the task is specifically about container-to-host boundary crossing, kernel exploit preconditions, namespace or cgroup crossover, or escape primitive verification, prefer `$competition-kernel-container-escape`.
- If the task is specifically about reconstructing protocols, streams, or transferred artifacts from packet captures, prefer `$competition-pcap-protocol`.
- If the task is specifically about a custom binary or text protocol where replay state, message order, or checksum logic is the real blocker, prefer `$competition-custom-protocol-replay`.
- If the task is specifically about reconstructing chronology across EVTX, PCAP, registry, mail, or disk artifacts, prefer `$competition-forensic-timeline`.

## What To Preserve

- Offsets, hashes, section names, imports, config blobs, mutexes, registry keys
- Crash offsets, registers, heap or stack shape, leak addresses, and protocol steps
- Original, decoded, dumped, and instrumented artifacts as separate files
