# Linux Credential Pivot Checklist

## First Pass

- SSH keys, agent sockets, kubeconfigs, cloud tokens, service-account secrets
- Env vars, config files, systemd units, sudoers, capabilities, setuid binaries
- Namespace context, container runtime sockets, control-plane endpoints

## Chain To Reconstruct

1. Credential artifact recovered
2. Accepting service identified
3. Replay or auth path executed
4. New session, token, or privilege gained
5. Pivot or lateral effect reproduced

## Evidence To Keep Together

- Artifact side: file or socket path, owner, scope, lifetime
- Replay side: target host, service, protocol, principal
- Effect side: privilege change, new shell, control-plane action, lateral movement

## Common Pitfalls

- Listing secrets without proving acceptance on a target service
- Mixing local privilege escalation and lateral movement in one vague chain
- Ignoring namespace or socket ownership context
