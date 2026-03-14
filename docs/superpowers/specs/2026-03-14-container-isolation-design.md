# Container Isolation Design Spec

## Overview

Add Docker-based container isolation to CyberStrikeAI's tool execution layer. Tools currently run as raw subprocesses on the host via `security/executor.go`. This design wraps tool execution in per-scan Docker containers with graceful fallback to subprocess mode.

## Goals

- Isolate offensive tool execution from the host system
- Prevent tools from leaving orphan processes, temp files, or leaked credentials
- Enable reproducible scan environments
- Maintain backward compatibility when Docker is unavailable

## Architecture

```
┌─────────────────────┐
│   security/executor  │
│   ExecuteTool()      │
└──────────┬──────────┘
           │
           ▼
    ┌──────────────┐     container.enabled: false
    │ Container    │────────────────────────────► exec.CommandContext() (current path)
    │ Manager      │
    └──────┬───────┘     container.enabled: true
           │
           ▼
    ┌──────────────┐
    │ Docker Engine│
    │ API          │
    └──────┬───────┘
           │
           ▼
    ┌──────────────────────────────┐
    │ Per-scan container           │
    │ - Volume: tmp/scans/<id>/    │
    │ - Image: cyberstrike/base    │
    │ - Network: bridge (default)  │
    │   or host (per tool YAML)    │
    └──────────────────────────────┘
```

**Key principle:** The executor's interface doesn't change. `ExecuteTool()` checks `container.enabled` — if true, routes through `ContainerManager`; if false or Docker unavailable, falls back to subprocess with a warning.

## Container Manager

**Package:** `internal/container/`

**Core struct:**

```go
type ContainerManager struct {
    client       *docker.Client       // Docker Engine API
    enabled      bool                 // from config
    baseImage    string               // "cyberstrike/base"
    imageMap     map[string]string    // tool category → image override
    containers   map[string]string    // conversationID → containerID
    workspaceDir string               // host path for volume mounts
    mu           sync.RWMutex
    logger       *zap.Logger
}
```

**Lifecycle per scan:**

1. `GetOrCreateContainer(conversationID, toolName)` — checks if a container exists for this scan. If not, picks the right image (base or specialized) and creates one
2. `ExecInContainer(containerID, command, args)` — runs the tool command inside the running container via Docker exec API, streams stdout/stderr
3. `CleanupContainer(conversationID)` — called when conversation ends or times out. Stops and removes the container

**Fallback logic in executor:**

```go
if containerManager.IsEnabled() {
    containerID, err := containerManager.GetOrCreateContainer(convID, toolName)
    if err != nil {
        logger.Warn("Docker unavailable, falling back to subprocess", zap.Error(err))
        return e.executeViaSubprocess(ctx, toolConfig, cmdArgs)  // current path
    }
    return containerManager.ExecInContainer(ctx, containerID, toolConfig.Command, cmdArgs)
}
return e.executeViaSubprocess(ctx, toolConfig, cmdArgs)
```

## Isolation Granularity

**Per-scan container** — one container per conversation/scan session. All tool calls within that scan share the container. This allows tools to chain results (nmap output → nuclei input, recon files → exploit tools).

## Image Strategy: Hybrid

**Base image** (`cyberstrike/base`) with Python 3, common libs, and the ~20 most-used tools (nmap, nuclei, sqlmap, dirsearch, httpx, etc.). Covers ~80% of tool calls. Size: ~1-2GB.

**Specialized overlay images** for tools that need additional heavy dependencies:

| Image | Tools | Use Case |
|-------|-------|----------|
| `cyberstrike/binary` | angr, binwalk, checksec, pwntools | Binary analysis |
| `cyberstrike/exploit` | metasploit, impacket, responder | Exploitation |
| `cyberstrike/cloud` | checkov, cloudmapper, docker-bench | Cloud security |

Overlay images inherit from the base image. Pre-built via `docker/build.sh`. No runtime image building.

## Dockerfiles

**`docker/base.Dockerfile`:**

```dockerfile
FROM python:3.11-slim
RUN apt-get update && apt-get install -y \
    nmap nuclei httpx subfinder sqlmap dirsearch \
    dirb nikto whatweb wafw00f curl wget jq dnsutils \
    && pip install uro qsreplace arjun dalfox
WORKDIR /workspace
```

**`docker/binary.Dockerfile`:**

```dockerfile
FROM cyberstrike/base:latest
RUN pip install angr pwntools
RUN apt-get install -y binwalk checksec
```

**`docker/exploit.Dockerfile`:**

```dockerfile
FROM cyberstrike/base:latest
RUN apt-get install -y metasploit-framework
RUN pip install impacket responder
```

**`docker/cloud.Dockerfile`:**

```dockerfile
FROM cyberstrike/base:latest
RUN pip install checkov cloudmapper
RUN apt-get install -y docker-bench-security
```

## Configuration

**`config.yaml` — new `container` section:**

```yaml
container:
  enabled: true                          # master switch
  base_image: "cyberstrike/base:latest"  # default image
  workspace_dir: "tmp/scans"             # host volume mount root
  cleanup_timeout_minutes: 60            # auto-cleanup idle containers
  image_map:                             # category → specialized image
    binary: "cyberstrike/binary:latest"
    exploit: "cyberstrike/exploit:latest"
    cloud: "cyberstrike/cloud:latest"
```

**`internal/config/config.go` — new struct:**

```go
type ContainerConfig struct {
    Enabled               bool              `yaml:"enabled"`
    BaseImage             string            `yaml:"base_image"`
    WorkspaceDir          string            `yaml:"workspace_dir"`
    CleanupTimeoutMinutes int               `yaml:"cleanup_timeout_minutes"`
    ImageMap              map[string]string  `yaml:"image_map,omitempty"`
}
```

## Tool YAML Changes

Two new optional fields added to `ToolConfig`:

```yaml
network_mode: "bridge"       # "bridge" (default) or "host"
container_category: ""       # maps to image_map key, empty = base image
```

**Examples:**

```yaml
# tools/responder.yaml — needs host networking
name: "responder"
command: "responder"
network_mode: "host"
container_category: "exploit"

# tools/angr.yaml — specialized image, bridge network
name: "angr"
command: "python3"
network_mode: "bridge"
container_category: "binary"
```

**Internal tools skip containers entirely** — tools with `command: "internal:*"` (create-file, modify-file, cat) continue running on the host.

## Network Access

- **Default:** Bridge networking — containers get their own IP, can reach external targets but are isolated from host services
- **Per-tool override:** Tools that need raw network access (responder, arp-scan, nmap SYN scans) declare `network_mode: host` in their YAML
- Configured declaratively per-tool, not globally

## File Exchange

**Volume mount strategy:** Host directory `tmp/scans/<conversationID>/` is mounted into the container at `/workspace`. Tools read/write there, results persist after container dies.

- Zero copy overhead
- Tools can chain results via shared filesystem
- Results survive container crashes
- Workspace dirs cleaned up by existing storage cleanup logic

## Container Cleanup

**Three triggers:**

1. **Conversation ends** — handler calls `containerManager.CleanupContainer(conversationID)`
2. **Idle timeout** — background goroutine checks every 5 minutes, removes containers idle longer than `cleanup_timeout_minutes` (default 60)
3. **Graceful shutdown** — on `App.Shutdown()`, stop and remove all active containers

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Docker daemon not running | Log warning, fall back to subprocess for all tools |
| Image not found locally | Return error to agent: "Image not found. Run docker/build.sh" |
| Container OOM / crash mid-tool | Detect via Docker API, return error to agent, auto-recreate container on next tool call |
| Tool timeout (`tool_timeout_minutes`) | Existing timeout kills the exec, container stays alive for next tool call |
| Volume mount permission error | Log error, fall back to subprocess for that tool call |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/container/manager.go` | ContainerManager — lifecycle, exec, cleanup, fallback |
| `docker/base.Dockerfile` | Base image with common tools |
| `docker/binary.Dockerfile` | Binary analysis tools |
| `docker/exploit.Dockerfile` | Exploitation tools |
| `docker/cloud.Dockerfile` | Cloud security tools |
| `docker/build.sh` | Image build script |

## Files to Modify

| File | Change |
|------|--------|
| `internal/security/executor.go` | Route through ContainerManager when enabled |
| `internal/app/app.go` | Initialize ContainerManager, wire cleanup on shutdown |
| `internal/config/config.go` | Add `ContainerConfig` struct, add `NetworkMode`/`ContainerCategory` to `ToolConfig` |
| `config.yaml` | Add `container:` section |
| Tool YAMLs (as needed) | Add `network_mode` and `container_category` for tools needing host networking or specialized images |

## Dependencies

- `github.com/docker/docker` — Docker Engine API client for Go
- `github.com/docker/go-connections` — Docker connection helpers

## Out of Scope

- Runtime image building (images are pre-built)
- Container orchestration (Kubernetes, Swarm)
- Per-tool-call containers (we use per-scan)
- Custom user-uploaded tool images (future consideration)
