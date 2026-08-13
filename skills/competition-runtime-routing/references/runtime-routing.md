# Runtime Routing Checklist

## Inputs That Commonly Matter

- `Host`, `X-Forwarded-Host`, `X-Forwarded-Proto`, `X-Forwarded-Port`
- Path prefixes, base path rewrites, tenant prefixes, cookie domain or path scope
- `Origin`, websocket upgrade headers, absolute URL generation, callback base URLs

## Route Map To Build

1. Client-visible host, scheme, port, and path
2. Reverse proxy or ingress rule that matches it
3. Target service, container, pod, or local process
4. Downstream store, worker, or peer node if the request fans out

## Verification Pattern

- Change one routing input at a time
- Compare both request traces and backend behavior
- Keep checked-in config separate from live config when they differ

## Common Pitfalls

- Treating a cloud-looking or public-looking hostname as external without checking sandbox routing
- Mixing auth failures with route resolution failures
- Explaining the path from code only without capturing one real request that proves the route
