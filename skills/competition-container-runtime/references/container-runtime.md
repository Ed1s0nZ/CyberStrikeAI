# Container Runtime Checklist

## Compare Intent vs Reality

Inspect side by side:

- compose or kube manifests
- image layers and entrypoints
- init containers
- sidecars
- mounted volumes
- runtime env
- live processes and listeners

## Trace The Mount Chain

- who writes the file
- where it is mounted
- which process reads it
- which route or behavior depends on it

## High-Value Runtime Deviations

- rendered secrets written to shared volumes
- init output consumed by the main container
- sidecar-generated config or credentials
- runtime-only env values not visible in checked-in manifests
- reverse-proxy routing that exposes an internal path only after startup

## Evidence To Keep

- one compact block for manifest intent
- one compact block for live mounts, processes, or rendered files
- one compact block for the route or behavior reached only because of runtime state

## Common Pitfalls

- treating checked-in manifests as deployment truth
- stopping at “secret is mounted” without proving the consuming process
- missing sidecar or init output because only the main service was inspected
