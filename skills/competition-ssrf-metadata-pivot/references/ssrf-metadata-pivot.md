# SSRF Metadata Pivot Checklist

## First Pass

- SSRF source parameter, URL builder, method, headers, redirect handling
- Host allowlist or denylist logic, DNS behavior, scheme checks, proxy layers
- Reachable internal services, metadata endpoints, token responses

## Chain To Reconstruct

1. Server-side fetch primitive is confirmed
2. Internal or metadata endpoint is reachable
3. Credential or sensitive response is extracted
4. Downstream service accepts the recovered material
5. Resulting capability is reproduced

## Evidence To Keep Together

- Source side: endpoint, parameter, URL construction, normalization rules
- Pivot side: target host, metadata path, token fields, scope, expiry
- Acceptance side: target API or service, replay method, resulting privilege

## Common Pitfalls

- Treating metadata reachability as equivalent to privilege without acceptance proof
- Ignoring redirects, host normalization, or proxy-injected headers
- Skipping the final accepted-access step and reporting only response leakage
