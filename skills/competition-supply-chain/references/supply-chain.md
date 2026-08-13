# Supply Chain And CI Checklist

## Provenance Chain

Track this order explicitly:

1. Source checkout
2. Dependency declaration
3. Lockfile or resolver result
4. Build step
5. Packaging or signing step
6. Publish target
7. Runtime consumer

## High-Value Evidence

- Version drift between source, lockfile, and fetched artifact
- Registry or mirror pulls that differ from expected origin
- Build scripts with preinstall, prepare, postinstall, or codegen steps
- Signed or packaged artifact hash compared to runtime-loaded artifact

## Common Pitfalls

- Stopping at lockfile drift without proving runtime consumption
- Treating checked-in manifests as equivalent to live pipeline behavior
- Losing the earliest divergence point inside a wall of CI logs
