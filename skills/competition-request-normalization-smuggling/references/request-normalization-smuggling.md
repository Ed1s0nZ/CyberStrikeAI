# Request Normalization Smuggling Checklist

## First Pass

- Front proxy, gateway, backend parser, internal service chain
- Path decode rules, slash and dot handling, case normalization, host derivation
- Header canonicalization, duplicate header behavior, CL/TE handling, chunk parsing

## Chain To Reconstruct

1. Baseline request path captured
2. Differential request crafted with one delta
3. Parser or router divergence observed
4. Unintended route or request body boundary reached
5. Decisive effect reproduced

## Evidence To Keep Together

- Request side: raw baseline and differential requests
- Hop side: each parser decision and route match per hop
- Effect side: hidden endpoint access, auth bypass branch, or state mutation

## Common Pitfalls

- Changing multiple fields at once and losing root-cause attribution
- Looking only at frontend proxy logs without backend route evidence
- Reporting parser mismatch without reproducing final effect
