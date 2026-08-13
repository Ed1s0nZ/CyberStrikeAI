# AD Certificate Abuse Checklist

## Core Surfaces

- Enterprise CA configuration, issuance policy, enrollment permissions, manager approval, authorized signatures
- Template flags, EKUs, subject or SAN controls, enrollment agent settings, superseded templates
- PKINIT, smartcard logon, Schannel, certificate mapping, relay paths, accepting services

## Abuse Chain To Reconstruct

1. Principal with enrollment or relay path identified
2. Template or CA weakness established
3. Certificate issued with exploitable identity material
4. Service or logon path accepts the cert
5. Resulting privilege or account effect confirmed

## Evidence To Keep Together

- Template side: name, rights, EKUs, flags, SAN controls, issuance requirements
- Cert side: subject, SAN, serial, validity, issuer, thumbprint
- Acceptance side: PKINIT or service mapping, target account, resulting privilege

## Common Pitfalls

- Stopping at template misconfiguration without proving cert issuance
- Proving issuance without proving where the cert is actually accepted
- Mixing certificate abuse with Kerberos delegation when the decisive edge is the template or mapping itself
