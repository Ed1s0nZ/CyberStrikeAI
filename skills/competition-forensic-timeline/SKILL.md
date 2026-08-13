---
name: competition-forensic-timeline
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for DFIR chronology, cross-artifact correlation, persistence chains, and incident
  timeline reconstruction. Use when the user asks to build a forensic timeline, correlate
  EVTX, PCAP, registry, disk, memory, mailbox, or browser artifacts, explain the order
  of attacker actions, or pinpoint the stage where the decisive artifact appears.
  Use only after `$ctf-sandbox-orchestrator` has already established sandbox assumptions
  and routed here.
---

# Competition Forensic Timeline

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when the hard part is not finding one artifact, but turning many artifacts into one replayable chronology.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Pick the smallest reliable anchor: first execution, first logon, first network session, first file write, or first mailbox action.
2. Normalize timestamps, time zones, hostnames, users, process IDs, message IDs, and file paths before correlating.
3. Build one minimal chain from foothold to persistence, execution, access, or exfiltration.
4. Separate confirmed event order from inferred gaps.
5. Reproduce the decisive timeline segment that yields the artifact or privilege conclusion.

## Workflow

### 1. Establish Timeline Anchors

- Collect only the active surfaces: EVTX, Sysmon, registry, Amcache, prefetch, browser artifacts, mail traces, PCAPs, memory, or filesystem metadata.
- Record clock source, timezone, and any drift or truncation that could reorder events.
- Link shared identifiers across sources: PID, logon ID, GUID, message ID, hostname, username, IP, or hash.

### 2. Correlate The Execution Graph

- Track process tree, service or task creation, network sessions, file writes, registry changes, mailbox rules, or token use as one path.
- Distinguish causal edges from coincidence by matching identifiers and adjacency, not just nearby timestamps.
- Keep raw artifact and parsed summary side by side so every step can be traced back.

### 3. Compress To The Decisive Story

- Reduce the timeline to the smallest sequence that proves initial access, persistence, lateral movement, collection, or artifact recovery.
- Call out missing validation steps separately instead of mixing them into confirmed chronology.
- If the task becomes mainly about malware config extraction or a Windows pivot edge, switch to the tighter specialized skill.

## Read This Reference

- Load `references/forensic-timeline.md` for anchor selection, cross-source correlation, and evidence packaging.
- If the hard part is packet reassembly, protocol framing, or transferred-object extraction from a capture, prefer `$competition-pcap-protocol`.

## What To Preserve

- Source file paths, event IDs, logon IDs, message IDs, PIDs, hashes, and timestamps with timezone noted
- One compact timeline table or ordered list for the decisive segment
- Raw artifacts, parsed output, and inferred edges kept separate
