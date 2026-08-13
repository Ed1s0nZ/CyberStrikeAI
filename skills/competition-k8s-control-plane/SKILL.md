---
name: competition-k8s-control-plane
description: Internal downstream skill for ctf-sandbox-orchestrator. CTF-sandbox workflow
  for Kubernetes API analysis, service-account trust, RBAC edges, admission and controller
  behavior, cluster secrets, workload mutation, and namespace-scoped drift. Use when
  the user asks to inspect kube API permissions, service-account tokens, RoleBinding
  or ClusterRoleBinding edges, admission webhooks, controller-created pods, secret
  exposure, or why live workloads differ from manifests. Use only after `$ctf-sandbox-orchestrator`
  has already established sandbox assumptions and routed here.
---

# Competition K8s Control Plane

Use this skill only as a downstream specialization after `$ctf-sandbox-orchestrator` is already active and has established sandbox assumptions, node ownership, and evidence priorities. If that has not happened yet, return to `$ctf-sandbox-orchestrator` first.

Use this skill when the decisive path runs through Kubernetes control-plane state, API permissions, or controller behavior rather than a single container's runtime alone.

Reply in Simplified Chinese unless the user explicitly requests English.

## Quick Start

1. Separate manifest intent from live cluster state: API objects, mutations, controllers, secrets, and resulting workloads.
2. Identify the active principal first: service account, kubeconfig identity, node credential, webhook, or controller.
3. Map the smallest control-plane edge to its workload effect.
4. Keep RBAC, service accounts, owner references, namespace boundaries, and secret consumers in compact evidence blocks.
5. Reproduce the smallest cluster action that yields the decisive workload or secret effect.

## Workflow

### 1. Map The API Trust Path

- Record namespaces, service accounts, Roles, ClusterRoles, bindings, admission hooks, controllers, and the resources they can mutate.
- Distinguish read access, create access, patch access, exec access, and secret access.
- Keep principal, verb, resource, namespace, and resulting object in one chain.

### 2. Trace Mutation To Workload State

- Show how an API action becomes a pod, volume mount, secret exposure, env injection, job run, or controller-created artifact.
- Compare checked-in YAML against live objects after defaulting, admission mutation, or controller reconciliation.
- Distinguish pod-runtime behavior from cluster-level mutation logic.

### 3. Reduce To The Decisive Cluster Path

- Compress the result to the smallest chain: principal -> API permission -> mutated object -> resulting workload, secret, or route effect.
- Keep kube objects, live describes, and consumed secret or config paths tied to the same namespace and controller.
- If the problem narrows down to one container's mount or runtime deviation, switch back to the tighter container-runtime skill.

## Read This Reference

- Load `references/k8s-control-plane.md` for the RBAC checklist, controller checklist, and evidence packaging.
- If the hard part is metadata-service reachability, workload identity, instance credentials, or metadata-derived privilege, prefer `$competition-cloud-metadata-path`.

## What To Preserve

- Namespace, service account, verb, resource kind, RoleBinding or ClusterRoleBinding, and owner reference chains
- Admission mutations, generated workloads, mounted secrets, and controller-produced drift
- The exact API action or object diff that creates the decisive effect
