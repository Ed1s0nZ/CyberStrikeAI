# WebSocket Runtime Checklist

## Handshake First

- Path, query, cookies, auth headers, `Origin`, upgrade headers, negotiated protocol, upgrade response
- SSE setup path, auth carrier, retry directives, event names, last-event IDs when applicable
- Connection lifecycle: open, subscribe, heartbeat, reconnect, close

## Message Flow To Reconstruct

1. Handshake or stream setup
2. Auth or subscribe message
3. Ack or server acceptance
4. Push or command frames
5. Persisted, rendered, or backend-visible side effect

## Evidence To Keep Together

- Setup side: request shape, tokens, cookies, channel or room identity
- Frame side: type, topic, payload schema, order, reconnect behavior
- Effect side: UI update, storage mutation, route unlock, server action, or worker effect

## Common Pitfalls

- Treating keepalive traffic as business logic
- Listing frames without showing which one changes state
- Ignoring reconnect or resubscribe behavior that is necessary to reproduce the issue
