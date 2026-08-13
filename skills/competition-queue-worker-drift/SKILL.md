---
name: competition-queue-worker-drift
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for queues, async workers, cron jobs, delayed tasks, retry behavior, worker-only
  config drift, and payload-to-side-effect chains. Use when the user asks to trace
  a queue payload, inspect async job execution, explain worker-only behavior, follow
  retries or dead-letter handling, or connect an enqueued item to a later file, cache,
  email, or privilege-bearing side effect. Use only after `$ctf-sandbox-orchestrator`
  has already established sandbox assumptions and routed here.
---

# Competition Queue Worker Drift

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when the decisive effect happens after enqueue, inside a worker, or only under async runtime state that differs from the request path.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Map the async chain first: enqueue point, queue payload, worker consumer, retries, and final side effect.
2. Keep request-time state separate from worker-time state.
3. Record queue name, message shape, worker config, retry policy, and downstream store in one chain.
4. Compare synchronous path and async path when behavior diverges.
5. Reproduce the smallest enqueue-to-side-effect flow that proves the decisive async drift.

## Workflow

### 1. Map Enqueue And Worker Identity

- Record queue names, topics, cron schedules, delayed jobs, dead-letter queues, worker processes, and consumer groups.
- Note which config, env vars, feature flags, or credentials exist only in the worker environment.
- Keep enqueue request, stored payload, and worker identity tied together.

### 2. Trace Worker-Only State And Retries

- Show how worker runtime differs from the request path: different env, files, mounts, caches, permissions, or clocks.
- Record retry count, backoff, dedupe keys, failure handling, dead-letter flow, and idempotency behavior.
- Distinguish immediate request success from eventual worker success or failure.

### 3. Reduce To The Decisive Async Chain

- Compress the result to the smallest sequence: enqueue -> worker runtime -> retry or branch -> resulting effect.
- State clearly whether the decisive difference lives in payload shape, worker config, retry path, or downstream consumer.
- If the issue is really about the file parser invoked by the worker, switch back to the tighter file-parser skill.

## Read This Reference

- Load `references/queue-worker-drift.md` for the queue checklist, retry checklist, and evidence packaging.

## What To Preserve

- Queue names, payloads, worker identities, retry metadata, dead-letter edges, and downstream effects
- The exact worker-only config or state that changes behavior
- One minimal enqueue-to-side-effect reproduction chain
