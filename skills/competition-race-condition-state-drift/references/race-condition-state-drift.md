# Race Condition State Drift Checklist

## First Pass

- Mutable rows, cache keys, counters, files, queue payloads
- Read-check-write boundaries, lock scope, retry behavior, idempotency handling
- Worker delays, commit timing, stale reads, invalidation timing

## Chain To Reconstruct

1. Baseline ordering established
2. Concurrent or reordered flow injected
3. Check or lock boundary is bypassed
4. Conflicting state mutation lands
5. Decisive drift is reproduced

## Evidence To Keep Together

- State side: key or row, initial value, final value, commit boundaries
- Timing side: request order, delays, retries, queue timing
- Effect side: duplicated action, privilege drift, balance drift, stale authorization

## Common Pitfalls

- Running noisy stress tests without a minimal deterministic sequence
- Mixing several mutable keys without proving which key drives the effect
- Reporting timing sensitivity without final-state parity proof
