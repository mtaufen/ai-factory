---
name: mcp-pod-test-deployment
deps:
  - sandbox-kind-cluster
  - pod-network-isolation
  - mcp-named-pipe-ipc
---

# MCP Pod Test Deployment

## Overview

A comprehensive test deployment manifesting the Pod concept outlined in `ideas/loop.md`. It brings together the `factory runtime loop`, containerized MCP servers, network sandboxing, and IPC via named pipes.

## Goals

- Deploy a standalone Pod (not a Job) validating the architecture on a local `kind` cluster.
- Include a basic containerized Git MCP server.
- Include a Dev MCP server packed with the Go toolchain.
- Integrate network isolation and named pipe IPC.

## Non-Goals

- Production operator deployment.
- High availability or scaling of the Pod.

## Key Requirements

- `runtimeClassName: gvisor` must be used.
- Git MCP sidecar needs egress to internet.
- Dev MCP sidecar must have zero egress.
- Loop container needs egress to LLM APIs (e.g., standard internet).
- Named pipes must be established before sidecars start.

## Design

### Image Management
To maintain a fast, local development loop without relying on an external container registry:
1. Container images for the MCP servers and the loop will be built locally (e.g., using `docker build`).
2. Images will be loaded directly into the `kind` cluster's nodes using `kind load docker-image <image-name>:<tag> --name <cluster-name>`.
3. The Pod manifest must set `imagePullPolicy: Never` for these containers to ensure the `kubelet` uses the locally loaded images and never attempts to pull from a remote registry.

### Pod Configuration
The test deployment will be a single Pod YAML manifest `mcp-test-pod.yaml`.

#### Volumes
- `git-mcp-pipe`: `emptyDir` for git MCP IPC.
- `dev-mcp-pipe`: `emptyDir` for dev MCP IPC.
- `workspace`: `emptyDir` for shared code workspace.
- `network-config`: `ConfigMap` containing the `PodNetworkIsolation` KRM definition.

#### Init Containers
1. **`network-setup`**:
   - Runs the network isolation tool (from `pod-network-isolation`).
   - `securityContext`: `capabilities: { add: ["NET_ADMIN"] }`, `runAsUser: 0`.
   - Mounts: `network-config`.
2. **`pipe-setup`**:
   - Runs `mkfifo` (from `mcp-named-pipe-ipc`).
   - Mounts: `git-mcp-pipe`, `dev-mcp-pipe`.
3. **`git-mcp`** (Native Sidecar - `restartPolicy: Always`):
   - Image: Basic Python or Go container with a Git MCP implementation.
   - `securityContext`: `runAsUser: 1002`, `capabilities: { drop: ["ALL"] }`, `privileged: false`, `allowPrivilegeEscalation: false`.
   - Mounts: `git-mcp-pipe`, `workspace`.
4. **`dev-mcp`** (Native Sidecar - `restartPolicy: Always`):
   - Image: `golang:1.22` (or similar) with Dev MCP implementation.
   - `securityContext`: `runAsUser: 1001`, `capabilities: { drop: ["ALL"] }`, `privileged: false`, `allowPrivilegeEscalation: false`.
   - Mounts: `dev-mcp-pipe`, `workspace`.

#### Main Container
- **`loop`**:
  - Image: `factory-runtime-loop`.
  - `securityContext`: `runAsUser: 1003`, `capabilities: { drop: ["ALL"] }`, `privileged: false`, `allowPrivilegeEscalation: false`.
  - Mounts: `git-mcp-pipe`, `dev-mcp-pipe`. (Intentionally *does not* mount `workspace` as the loop itself shouldn't directly touch the code, only through MCP).

## Examples

Draft of `mcp-test-pod.yaml`:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: loop-test-pod
spec:
  runtimeClassName: gvisor
  initContainers:
  - name: network-setup
    image: network-isolation-init:latest
    securityContext:
      capabilities:
        add: ["NET_ADMIN"]
  - name: pipe-setup
    image: busybox
    command: ["sh", "-c", "mkfifo /var/run/mcp/git-mcp/pipe && mkfifo /var/run/mcp/dev-mcp/pipe"]
    volumeMounts:
    - name: git-mcp-pipe
      mountPath: /var/run/mcp/git-mcp
    - name: dev-mcp-pipe
      mountPath: /var/run/mcp/dev-mcp
  - name: git-mcp
    image: git-mcp:latest
    imagePullPolicy: Never
    restartPolicy: Always
    securityContext:
      runAsUser: 1002
      allowPrivilegeEscalation: false
      privileged: false
      capabilities:
        drop: ["ALL"]
    volumeMounts:
    - name: git-mcp-pipe
      mountPath: /var/run/mcp/git-mcp
    - name: workspace
      mountPath: /workspace
  - name: dev-mcp
    image: dev-mcp:latest
    imagePullPolicy: Never
    restartPolicy: Always
    securityContext:
      runAsUser: 1001
      allowPrivilegeEscalation: false
      privileged: false
      capabilities:
        drop: ["ALL"]
    volumeMounts:
    - name: dev-mcp-pipe
      mountPath: /var/run/mcp/dev-mcp
    - name: workspace
      mountPath: /workspace
  containers:
  - name: loop
    image: factory-runtime-loop:latest
    imagePullPolicy: Never
    securityContext:
      runAsUser: 1003
      allowPrivilegeEscalation: false
      privileged: false
      capabilities:
        drop: ["ALL"]
    volumeMounts:
    - name: git-mcp-pipe
      mountPath: /var/run/mcp/git-mcp
    - name: dev-mcp-pipe
      mountPath: /var/run/mcp/dev-mcp
  volumes:
  - name: git-mcp-pipe
    emptyDir: {}
  - name: dev-mcp-pipe
    emptyDir: {}
  - name: workspace
    emptyDir: {}
```

## Tests

- Deploy the Pod to the sandboxed `kind` cluster.
- `kubectl exec` into the `loop` container and verify it can communicate with `dev-mcp` over `/var/run/mcp/dev-mcp/pipe` and `git-mcp` over `/var/run/mcp/git-mcp/pipe`.
- `kubectl exec` into `dev-mcp` and verify `curl https://google.com` fails (network isolation).
- `kubectl exec` into `git-mcp` and verify `curl https://github.com` succeeds.
- Verify that `dev-mcp` cannot access `/var/run/mcp/git-mcp/pipe` and vice versa, ensuring isolation between MCP servers.
