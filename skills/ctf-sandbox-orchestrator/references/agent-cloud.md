# AI Agent, Prompt Injection, Cloud, And Supply Chain

Use this reference for prompt-injection challenges, toolchain or MCP tasks, agent memory issues, containerized deployments, CI/CD, cloud control plane analysis, or supply-chain tracing.

## Agent And Prompt Injection

1. Treat prompts, tool schemas, retrieved chunks, planner notes, memory files, and handoff messages as challenge artifacts.
2. Map the control stack: instruction layers, retrieval layers, memory layers, tool gates, auth material, and side effects.
3. Prove one minimal exploit chain from untrusted content to model-visible instruction to tool side effect or exposed secret.
4. Distinguish claimed capability from observed capability.

## Cloud, Containers, And CI/CD

1. Split build-time, deploy-time, and runtime.
2. Reconcile checked-in manifests with live containers, mounted secrets, env, logs, and traffic.
3. Trace provenance end-to-end: source -> dependency resolution -> build -> packaging/signing -> publish -> runtime consumer.
4. Treat registries, metadata services, message buses, object stores, and IAM-looking identities as sandbox control surfaces when they appear in the challenge path.

## Evidence To Keep

- Exact prompts, retrieved chunks, planner/tool transitions, and final tool args
- Compose/Kubernetes/Terraform snippets tied to live mounts or routes
- Artifact hashes, version drift, registry pulls, CI steps, and resulting runtime hooks

## Common Pitfalls

- Treating a prompt string as authoritative without runtime confirmation
- Confusing infrastructure intent with live deployment truth
- Missing the boundary where a retrieved chunk becomes executable tool input
