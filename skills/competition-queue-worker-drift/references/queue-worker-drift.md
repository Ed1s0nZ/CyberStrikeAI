# Queue Worker Drift Checklist

## First Pass

- Enqueue endpoint, queue or topic name, payload schema, worker process, schedule or delay, downstream store
- Worker env, mounts, credentials, feature flags, config files, retry policy, dedupe behavior
- Final effects: file write, cache mutation, email, report, artifact generation, privilege-bearing action

## Async Chain To Reconstruct

1. Request or cron enqueue occurs
2. Payload stored or scheduled
3. Worker picks it up under its own runtime state
4. Retry, backoff, or failure path taken if relevant
5. Side effect lands in file, DB, cache, email, or service

## Evidence To Keep Together

- Enqueue side: route, payload, queue name, task ID
- Worker side: process, env, config, retry metadata, dedupe or lease state
- Effect side: resulting artifact, downstream mutation, timestamps, and replay prerequisites

## Common Pitfalls

- Explaining only the request path and never proving the worker branch
- Ignoring worker-only env or mount differences
- Treating eventual side effects as synchronous behavior without isolating the async boundary
