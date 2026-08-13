# OAuth OIDC Chain Checklist

## First Pass

- Entry route, issuer, client ID, redirect URI, callback route, token endpoint, refresh path
- Authorize parameters: `response_type`, `scope`, `state`, `nonce`, PKCE challenge, audience, prompt
- Browser redirects, backend token exchanges, stored session state, final app acceptance

## Chain To Reconstruct

1. Entry request or login trigger
2. Authorize redirect built
3. Callback parameters received
4. Code or token exchange performed
5. Claims or token accepted by app or backend

## Evidence To Keep Together

- Redirect side: route, parameters, callback values, issuer
- Token side: type, claims, scopes, audience, expiration, subject
- Acceptance side: session created, claims mapped, tenant selected, route unlocked, or privilege granted

## Common Pitfalls

- Stopping at the callback without proving token exchange or claim acceptance
- Treating possession of a token as authorization without showing where it is accepted
- Mixing browser-only redirect evidence and backend-only auth evidence without linking them
