---
name: competition-graphql-rpc-drift
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for GraphQL schemas, persisted queries, RPC manifests, generated clients, OpenAPI
  drift, hidden operations, and contract-to-handler mismatches. Use when the user
  asks to inspect GraphQL or RPC requests, compare client contracts to live handlers,
  recover hidden operations, trace generated clients, or explain how schema or contract
  drift produces the decisive behavior. Use only after `$ctf-sandbox-orchestrator`
  has already established sandbox assumptions and routed here.
---

# Competition Graphql Rpc Drift

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when the hard part is matching declared contracts with live handlers to find hidden, stale, or privileged operations.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Collect the declared contract surface first: schema, manifest, generated client, persisted query map, or OpenAPI spec.
2. Record actual request shapes, operation names, variables, method, path, and auth context before mutating anything.
3. Compare declared contract, generated client behavior, and live handler behavior side by side.
4. Preserve one accepted operation and one drifted or hidden operation with the smallest delta.
5. Reproduce the smallest contract-to-handler mismatch that proves the decisive branch.

## Workflow

### 1. Map The Declared Contract Surface

- Record GraphQL schema, introspection output, persisted query ids, RPC manifests, generated clients, or OpenAPI documents that define the intended surface.
- Note versioned endpoints, client-only guards, hidden enums, optional fields, and operation naming conventions.
- Keep document source and generation path tied to the observed requests.

### 2. Prove Live Handler Behavior

- Capture the real request and response pairs, including operation name, variables, headers, cookies, and status.
- Compare client-side validation, schema expectations, and live handler normalization or fallback behavior.
- Record hidden operations, stale fields, undocumented methods, or handler-only branches that still execute.

### 3. Reduce To The Decisive Drift Path

- Compress the result to the smallest sequence: declared contract -> actual request -> handler branch -> resulting capability.
- State clearly whether the decisive drift lives in generated client assumptions, persisted query mapping, schema version skew, RPC manifest mismatch, or handler-side hidden logic.
- If the task shifts into generic JWT, OAuth, or queue behavior after acceptance, hand off to the tighter specialized skill.

## Read This Reference

- Load `references/graphql-rpc-drift.md` for the contract checklist, live-handler checklist, and evidence packaging.

## What To Preserve

- Schemas, manifests, generated clients, persisted query ids, operation names, and version markers
- One accepted and one drifted request pair that proves the mismatch
- One minimal contract-to-handler sequence that reaches the decisive effect
