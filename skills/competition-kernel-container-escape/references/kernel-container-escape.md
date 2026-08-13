# Kernel Container Escape Checklist

## First Pass

- Kernel version, runtime type, namespace map, cgroup mode, capability set
- Seccomp profile, AppArmor or SELinux mode, mounts, privileged sockets
- Potential primitives: syscall path, fs boundary, runtime API, namespace leak

## Chain To Reconstruct

1. Isolation baseline recorded
2. Exploit or misconfig primitive triggered
3. Boundary crossover evidence captured
4. Host-relevant capability appears
5. Chain reproduces from reset baseline

## Evidence To Keep Together

- Context side: kernel and runtime config, namespace and capability state
- Primitive side: controllable input, trigger, affected object, observables
- Effect side: identity change, host visibility, privileged action, persistence edge

## Common Pitfalls

- Treating container root as host compromise without crossover proof
- Mixing several primitives before proving one decisive chain
- Claiming escape from crash artifacts without stable capability evidence
