# Browser Persistence Checklist

## Surfaces To Check

- Cookies: domain, path, expiry, `HttpOnly`, `Secure`, `SameSite`
- `localStorage`, `sessionStorage`, IndexedDB object stores, Cache Storage entries
- Service worker registrations, cached responses, fetch handlers, offline fallbacks
- Boot-time globals, hydration payloads, feature flags, role or tenant selectors

## Correlation Pattern

1. Storage item or cookie identified
2. Origin and scope confirmed
3. Request, render, or cache behavior linked to that item
4. Clean-state vs mutated-state run compared
5. Decisive branch reproduced

## Evidence To Keep Together

- State identity: origin, key, value shape, scope, expiry, DB or cache name
- Runtime effect: header, route, feature flag, cached response, or rendered branch
- Replay prerequisites: initial route, prior request, login state, or service worker registration

## Common Pitfalls

- Listing storage contents without showing which item changes behavior
- Mixing several origins or tenants in one storage explanation
- Treating cached UI data as backend authorization without proving a server-side effect
