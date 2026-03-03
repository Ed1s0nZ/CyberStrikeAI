---
name: container-security-testing
description: Professional skills and methodology for container security testing
version: 1.0.0
---

# Container Security Testing

## Overview

Container security testing is an essential part of ensuring the security of containerized applications. This skill provides methods, tools, and best practices for container security testing, covering container technologies such as Docker and Kubernetes.

## Testing Scope

### 1. Image Security

**Checklist:**
- Base image vulnerabilities
- Dependency package vulnerabilities
- Image configuration
- Sensitive information

### 2. Runtime Security

**Checklist:**
- Container privileges
- Resource limits
- Network isolation
- File system

### 3. Orchestration Security

**Checklist:**
- Kubernetes configuration
- Service accounts
- RBAC
- Network policies

## Docker Security Testing

### Image Scanning

**Using Trivy:**
```bash
# Scan image
trivy image nginx:latest

# Scan local image
trivy image --input nginx.tar

# Show only high-severity vulnerabilities
trivy image --severity HIGH,CRITICAL nginx:latest
```

**Using Clair:**
```bash
# Start Clair
docker run -d --name clair clair:latest

# Scan image
clair-scanner --ip 192.168.1.100 nginx:latest
```

**Using Docker Bench:**
```bash
# Run Docker security benchmark test
docker run --rm --net host --pid host --userns host --cap-add audit_control \
  -e DOCKER_CONTENT_TRUST=$DOCKER_CONTENT_TRUST \
  -v /etc:/etc:ro \
  -v /usr/bin/containerd:/usr/bin/containerd:ro \
  -v /usr/bin/runc:/usr/bin/runc:ro \
  -v /usr/lib/systemd:/usr/lib/systemd:ro \
  -v /var/lib:/var/lib:ro \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  --label docker_bench_security \
  docker/docker-bench-security
```

### Container Configuration Check

**Check Dockerfile:**
```dockerfile
# Security issue examples
FROM ubuntu:latest  # Using latest tag
RUN apt-get update && apt-get install -y curl  # Version not specified
COPY . /app  # May include sensitive files
ENV PASSWORD=secret  # Hardcoded password
USER root  # Using root user
```

**Security best practices:**
```dockerfile
# Use specific version
FROM ubuntu:20.04

# Specify package version
RUN apt-get update && apt-get install -y curl=7.68.0-1ubuntu2.7

# Use non-root user
RUN useradd -m appuser
USER appuser

# Minimal image
FROM alpine:3.15

# Multi-stage build
FROM golang:1.18 AS builder
WORKDIR /app
COPY . .
RUN go build -o app

FROM alpine:3.15
COPY --from=builder /app/app /app
```

### Runtime Checks

**Check container privileges:**
```bash
# Check privileged containers
docker ps --filter "label=privileged=true"

# Check mounted host directories
docker inspect container_name | grep -A 10 Mounts

# Check container network
docker network inspect network_name
```

**Check resource limits:**
```bash
# Check memory limits
docker stats container_name

# Check CPU limits
docker inspect container_name | grep -i cpu
```

## Kubernetes Security Testing

### Configuration Check

**Using kube-bench:**
```bash
# Run kube-bench
kube-bench run

# Check specific benchmark
kube-bench run --targets master,node,etcd
```

**Using kube-hunter:**
```bash
# Run kube-hunter
kube-hunter --remote target-ip

# Active mode
kube-hunter --active
```

### Pod Security

**Check Pod security policies:**
```yaml
# Insecure Pod configuration
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: app
    image: nginx
    securityContext:
      privileged: true  # Privileged mode
      runAsUser: 0  # root user
```

**Secure configuration:**
```yaml
apiVersion: v1
kind: Pod
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    fsGroup: 2000
  containers:
  - name: app
    image: nginx
    securityContext:
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      capabilities:
        drop:
        - ALL
        add:
        - NET_BIND_SERVICE
```

### RBAC Check

**Check role permissions:**
```bash
# List all roles
kubectl get roles --all-namespaces

# Check role bindings
kubectl get rolebindings --all-namespaces

# Check cluster roles
kubectl get clusterroles

# Check user permissions
kubectl auth can-i --list --as=system:serviceaccount:default:sa-name
```

**Common issues:**
- Excessive permissions
- Unused roles
- Unused service accounts

### Network Policies

**Check network policies:**
```bash
# List all network policies
kubectl get networkpolicies --all-namespaces

# Check network policy configuration
kubectl describe networkpolicy policy-name -n namespace
```

**Network policy example:**
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
```

## Tool Usage

### Falco

**Runtime security monitoring:**
```bash
# Install Falco
helm repo add falcosecurity https://falcosecurity.github.io/charts
helm install falco falcosecurity/falco

# Check rules
falco -r /etc/falco/rules.d/
```

### Aqua Security

```bash
# Scan image
aqua image scan nginx:latest

# Scan Kubernetes cluster
aqua k8s scan
```

### Snyk

```bash
# Scan Dockerfile
snyk test --docker nginx:latest

# Scan Kubernetes configuration
snyk iac test k8s/
```

## Testing Checklist

### Image Security
- [ ] Scan base image vulnerabilities
- [ ] Scan dependency package vulnerabilities
- [ ] Check Dockerfile configuration
- [ ] Check for sensitive information leakage

### Runtime Security
- [ ] Check container privileges
- [ ] Check resource limits
- [ ] Check network isolation
- [ ] Check file system mounts

### Orchestration Security
- [ ] Check Kubernetes configuration
- [ ] Check RBAC configuration
- [ ] Check network policies
- [ ] Check Pod security policies

## Common Security Issues

### 1. Image Vulnerabilities

**Issue:**
- Base image contains vulnerabilities
- Dependency packages contain vulnerabilities
- Not updated in a timely manner

**Remediation:**
- Regularly scan images
- Update base images promptly
- Use minimal images

### 2. Excessive Privileges

**Issue:**
- Container runs as root
- Privileged mode
- Sensitive directories mounted

**Remediation:**
- Use non-root user
- Disable privileged mode
- Restrict file system access

### 3. Misconfiguration

**Issue:**
- Default configuration is insecure
- Network policies missing
- RBAC misconfigured

**Remediation:**
- Follow security best practices
- Implement network policies
- Configure RBAC correctly

### 4. Sensitive Information Leakage

**Issue:**
- Image contains keys
- Environment variables exposed
- Configuration files leaked

**Remediation:**
- Use key management
- Avoid hardcoding
- Use Secret objects

## Best Practices

### 1. Image Security

- Use official base images
- Update images regularly
- Scan images for vulnerabilities
- Minimize image size

### 2. Runtime Security

- Use non-root user
- Restrict container privileges
- Implement resource limits
- Enable security context

### 3. Orchestration Security

- Configure network policies
- Implement RBAC
- Use Pod security policies
- Enable audit logs

## Notes

- Only perform testing in authorized environments
- Avoid impacting production environments
- Note differences across container platforms
- Conduct security scans regularly
