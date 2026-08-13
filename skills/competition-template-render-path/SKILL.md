---
name: competition-template-render-path
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for SSR, template rendering, route loaders, hydration payloads, server-client render
  boundaries, and template-to-handler enforcement gaps. Use when the user asks to
  inspect SSR or template routes, trace render context or hydration data, compare
  template gating with handler enforcement, explain preview or hidden-route rendering,
  or connect render pipeline behavior to the decisive branch. Use only after `$ctf-sandbox-orchestrator`
  has already established sandbox assumptions and routed here.
---

# Competition Template Render Path

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when the decisive bug or artifact lives in route resolution, server render context, template data, or hydration handoff rather than in a plain JSON API alone.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Map the render chain in order: route resolution, loader or data fetch, template or component context, response HTML, hydration payload, and client takeover.
2. Record host, route params, preview toggles, tenant or host switches, and server-only variables before mutating anything.
3. Compare template gating with loader or handler enforcement.
4. Preserve one successful render and one failing render path with the smallest delta.
5. Reproduce the smallest request-to-render branch that proves the decisive behavior.

## Workflow

### 1. Map Route To Render Context

- Record host, path, route match, loader, template, layout, hydration blob, and client boot chunk for the active view.
- Note whether the response is SSR HTML, static HTML plus hydration, edge-rendered content, or a template fragment used by another route.
- Keep server-only context and client-visible context separate.

### 2. Trace Template And Enforcement Boundaries

- Show where permissions, feature flags, preview state, tenant selection, or host-based switches are applied.
- Compare template-level gating, loader-level gating, and backend handler enforcement instead of trusting any one layer.
- Record hidden fields, inline data, hydration JSON, meta tags, or alternate partials that expose the decisive branch.

### 3. Reduce To The Decisive Render Path

- Compress the result to the smallest sequence: request -> route match -> loader or template context -> rendered output or hidden data -> resulting effect.
- State clearly whether the decisive weakness lives in route selection, template context construction, server-client hydration handoff, or mismatched enforcement between render and handler.
- If the task becomes mostly emitted bundle recovery or source map reconstruction, hand off to the tighter bundle skill.

## Read This Reference

- Load `references/template-render-path.md` for the render checklist, hydration checklist, and evidence packaging.

## What To Preserve

- Route names, loader names, templates, layouts, hydration keys, and host or preview switches
- One success or failure pair that shows where render-layer behavior diverges
- One minimal request-to-render sequence that reaches the decisive branch
