# Relay Coercion Checklist

## Chain Segments

- Coercion source and trigger
- Captured or forwarded authentication
- Relay target and protocol
- Acceptance point and resulting effect

## Reconstruction Order

1. What triggers the source to authenticate
2. Which identity leaves the source
3. Where that authentication is relayed
4. Which service actually accepts it
5. What privilege, enrollment, or artifact results

## Evidence To Keep Together

- Source side: host, service, trigger, protocol, coerced principal
- Relay side: listener, transcript, target host, protocol, response
- Effect side: accepted service, resulting account effect, privilege, or issued artifact

## Common Pitfalls

- Stopping at “forced auth happened” without proving relay acceptance
- Proving relay acceptance without showing what capability it produced
- Mixing several candidate relay targets without isolating the one that actually accepted the relayed auth
