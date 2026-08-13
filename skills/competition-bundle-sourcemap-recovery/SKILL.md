---
name: competition-bundle-sourcemap-recovery
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for source maps, build manifests, chunk registries, emitted bundles, obfuscated
  loader flow, and frontend runtime recovery. Use when the user asks to reconstruct
  served JavaScript structure, inspect source maps or chunk maps, trace bundle loading,
  recover hidden routes or APIs from emitted assets, or explain runtime behavior from
  built frontend artifacts. Use only after `$ctf-sandbox-orchestrator` has already
  established sandbox assumptions and routed here.
---

# Competition Bundle Sourcemap Recovery

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when runtime truth lives in built assets, source maps, chunk tables, or obfuscated loader flow rather than in checked-in source alone.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Start from the served artifact set: entry HTML, build manifest, bootstrap bundle, chunk map, and source maps.
2. Record chunk ids, route chunks, loader functions, endpoint strings, and config keys before broad manual deobfuscation.
3. Reconstruct the smallest runtime graph that explains which asset executes now.
4. Keep served artifact truth separate from repository source unless parity is proven.
5. Reproduce the smallest asset-to-runtime boundary that proves the decisive behavior.

## Workflow

### 1. Map The Served Artifact Set

- Record entry HTML, script tags, preload hints, manifest files, asset map, chunk registry, and source map URLs.
- Note framework-specific artifacts such as route manifests, client reference manifests, or lazy-loader tables when present.
- Keep emitted filenames, hash suffixes, and route ownership tied together.

### 2. Reconstruct Runtime Structure

- Follow bootstrap code, chunk loaders, module registry, string decoders, and lazy import boundaries.
- Use source maps, manifest files, and stable symbol clusters to recover route names, API calls, feature flags, and hidden panels.
- Distinguish build-time intent from the bundle that is actively served now.

### 3. Reduce To The Decisive Bundle Path

- Compress the result to the smallest sequence: served asset -> loader path -> module or symbol -> runtime effect.
- State clearly whether the decisive weakness lives in manifest drift, chunk loading, hidden route code, string decoding, or stale source assumptions.
- If the task shifts from built assets to SSR or template enforcement, hand back to the tighter template-render skill.

## Read This Reference

- Load `references/bundle-sourcemap-recovery.md` for the artifact checklist, deobfuscation checklist, and evidence packaging.

## What To Preserve

- Served filenames, chunk ids, manifest entries, source map paths, recovered symbols, and endpoint strings
- The exact executing bundle or module that proves the runtime branch
- One minimal asset-to-runtime sequence that reaches the decisive effect
