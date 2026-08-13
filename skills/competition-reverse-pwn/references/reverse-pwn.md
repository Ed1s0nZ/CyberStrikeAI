# Reverse, DFIR, And Pwn Playbook

## Reverse And DFIR

- Start with file type, headers, sections, imports, strings, resources, embedded archives, entropy
- Reconstruct process trees, persistence, startup paths, IPC, and network beacons as one execution graph
- Explain when each recovered value becomes plaintext and where it came from

## Pwn And Native

- Record mitigations, loader path, libc build, protocol framing, syscall surface
- Separate exploit prerequisites from exploit proof
- Tie allocator layout, crash state, and target object to observed execution

## Evidence To Keep

- One compact block for offsets, hashes, imports, config fields, or registry paths
- One compact block for crash state, leaks, allocator state, and protocol steps
- Minimal but sufficient logs proving the decoded layer or privilege effect

## Common Pitfalls

- Mixing original and modified artifacts
- Treating one string hit as proof of the full behavior
- Naming a primitive without replaying it from a clean baseline
