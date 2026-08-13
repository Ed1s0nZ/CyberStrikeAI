# Template Render Path Checklist

## First Pass

- Host, path, route params, route match, loader name, template or layout, response mode
- SSR HTML, inline data, hydration payload, client boot chunk, preview or feature flags
- Hidden fields, alternate partials, meta tags, host-based switches, tenant context

## Chain To Reconstruct

1. Request reaches route match
2. Loader or server data fetch runs
3. Template or layout context is built
4. HTML or hydration payload is emitted
5. Client takeover or handler follow-up turns it into the decisive effect

## Evidence To Keep Together

- Route side: host, path, matched route, params, loader or handler
- Render side: template or layout, server-only variables, hydration keys, hidden fields
- Effect side: rendered content, leaked data, privileged action, or bypassed branch

## Common Pitfalls

- Trusting visible UI gating without checking loader or handler enforcement
- Looking only at HTML and ignoring hydration JSON or inline data blobs
- Mixing route selection bugs with client-only rendering differences without proving the server boundary
