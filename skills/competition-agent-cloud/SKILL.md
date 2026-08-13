---
name: competition-agent-cloud
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for AI-agent, prompt-injection, MCP or toolchain, cloud, container, CI/CD, and supply-chain
  challenges. Use when the user asks to analyze prompt-to-tool flows, retrieval poisoning,
  mounted secrets, deployment drift, runtime-vs-manifest mismatches, registry provenance,
  or CI-produced artifacts under sandbox assumptions. Use only after `$ctf-sandbox-orchestrator`
  has already established sandbox assumptions and routed here.
---

# Competition Agent Cloud

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when the challenge path is driven by prompt-to-tool execution, retrieval and memory boundaries, deployment drift, or build and release provenance.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Decide whether the dominant path is agentic or infrastructure-driven.
2. Map one minimal control chain: untrusted input -> visible context -> tool or deployment side effect.
3. Distinguish checked-in intent from live runtime truth.
4. Keep prompts, tool args, manifests, mounts, and provenance steps in compact evidence blocks.
5. Reproduce the exploit or misconfiguration with minimal context and minimal instrumentation.

## Workflow

### 1. Agent And Prompt Injection

- Treat prompts, tool schemas, retrieved chunks, planner notes, memory files, and handoffs as challenge artifacts.
- Prove one minimal chain from untrusted content to model-visible instruction to tool side effect.
- Distinguish claimed capability from runtime-exposed capability.

### 2. Cloud, Containers, And CI/CD

- Split build-time, deploy-time, and runtime.
- Reconcile compose or kube manifests with live mounts, env, logs, and traffic.
- Trace provenance from source to dependency resolution to build to publish to runtime consumer.

## Read This Reference

- Load `references/agent-cloud.md` for the control-stack checklist, deployment-truth checklist, and evidence packaging.
- If the task is specifically about prompt-boundary abuse or retrieved-content-to-tool drift, prefer `$competition-prompt-injection`.
- If the task is specifically about CI, dependency provenance, registry drift, or shipped artifacts, prefer `$competition-supply-chain`.
- If the task is specifically about queue payloads, async worker drift, retries, or worker-only runtime state, prefer `$competition-queue-worker-drift`.
- If the task is specifically about SSRF to internal control surfaces, metadata endpoints, or metadata-derived token pivots, prefer `$competition-ssrf-metadata-pivot`.
- If the task is specifically about proxy-upstream parse differentials, ambiguous headers, path normalization drift, or request smuggling behavior, prefer `$competition-request-normalization-smuggling`.
- If the task is specifically about metadata-service access, instance or workload identity, link-local token paths, or metadata-derived privilege, prefer `$competition-cloud-metadata-path`.
- If the task is specifically about kube API permissions, service-account trust, admission behavior, controller drift, or cluster secret exposure, prefer `$competition-k8s-control-plane`.
- If the task is specifically about live mounts, sidecars, init containers, or runtime-only secret exposure, prefer `$competition-container-runtime`.
- If the task is specifically about container-to-host boundary crossing, kernel-surface prerequisites, or escape primitive verification, prefer `$competition-kernel-container-escape`.

## What To Preserve

- Prompt snippets, retrieved chunks, planner transitions, and final tool args
- Compose or Kubernetes fragments tied to live mounts or routes
- Artifact hashes, dependency drift, CI steps, and the resulting runtime consumer
