# K8s Control Plane Checklist

## First Pass

- Namespaces, service accounts, kubeconfigs, tokens, Roles, ClusterRoles, bindings
- Admission webhooks, mutating or validating policy, controllers, operators, CRDs
- Secrets, ConfigMaps, projected volumes, generated Jobs, and owner references

## Trust Chain To Reconstruct

1. Principal or token identified
2. RBAC or admission edge established
3. Object create, patch, or read action performed
4. Controller or scheduler turns object into workload state
5. Secret, route, workload, or artifact effect observed

## Evidence To Keep Together

- Principal side: service account, token source, namespace, binding, verb, resource
- Mutation side: object manifest, admission change, controller output, owner refs
- Effect side: mounted secret, env var, spawned pod, reachable route, or recovered artifact

## Common Pitfalls

- Stopping at a RoleBinding without proving the resulting API action
- Explaining pod behavior without showing which cluster object created it
- Mixing static YAML and live cluster objects without accounting for admission or controller drift
