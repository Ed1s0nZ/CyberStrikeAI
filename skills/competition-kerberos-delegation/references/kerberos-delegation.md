# Kerberos Delegation Checklist

## Write The Chain Explicitly

- source principal
- delegation edge
- ticket minted or transformed
- target SPN
- accepting service
- resulting privilege

## Ticket Fields To Preserve

- TGT or TGS type
- SPN
- delegation mode
- S4U step if present
- PAC or group data
- encryption type
- cache location
- accepting service

## Trust Edges To Inspect

- constrained delegation
- unconstrained delegation
- resource-based constrained delegation
- protocol transition
- SIDHistory or ACL-based privilege edges when they affect ticket usability

## Common Pitfalls

- Treating a minted ticket as proof of accepted privilege
- Reporting the delegation mode without naming the accepting service
- Expanding into every AD edge before one replayable chain is proven
