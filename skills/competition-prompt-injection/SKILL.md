---
name: competition-prompt-injection
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for prompt-injection, retrieval poisoning, memory contamination, planner drift,
  MCP or tool-boundary abuse, and agent exfiltration challenges. Use when the user
  asks to analyze prompt injection, retrieval poisoning, memory contamination, planner
  drift, tool-argument corruption, or secret exposure caused by an agent chain. Use
  only after `$ctf-sandbox-orchestrator` has already established sandbox assumptions
  and routed here.
---

# Competition Prompt Injection

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when the challenge is primarily about trust boundaries inside an agentic system.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Identify the first untrusted content that becomes model-visible.
2. Map the chain from retrieval, memory, or transcript into planner or executor behavior.
3. Record the exact point where text becomes a tool argument, file path, network target, or secret request.
4. Prove one minimal exploit chain before exploring variants.
5. Keep prompt snippets and tool transitions in compact evidence blocks.

## Workflow

### 1. Map The Control Stack

- Track system, developer, user, retrieved, memory, planner, and tool-response layers separately.
- Distinguish claimed capability from runtime-exposed capability.
- Note what the model can actually call, read, or mutate.

### 2. Prove The Boundary Crossing

- Reproduce one chain from untrusted text to changed planner behavior, changed tool args, or secret exposure.
- Keep the decisive transcript compact: source chunk, rewritten planner state, final tool invocation.
- Prefer the smallest transcript that still demonstrates the bug.

### 3. Report By Boundary

- State which layer failed: retrieval, summarizer, planner, executor, tool normalization, or output post-processing.
- Separate instruction drift from actual side effect.

## Read This Reference

- Load `references/prompt-injection.md` for the checklist, evidence layout, and common prompt-boundary pitfalls.

## What To Preserve

- Original malicious chunk or prompt
- Intermediate summary or planner drift if it matters
- Final tool args, file paths, or exposed secret surface
