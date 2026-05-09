---
name: pod-network-isolation
deps: []
---

# Pod Network Isolation Init Container

## Overview

A spec for an init container that configures UID-based iptables rules to restrict network egress for sidecar containers within a Pod, taking a KRM-style declarative file as input.

## Goals

- Isolate individual containers within a Pod from making unauthorized network requests based on their UID.
- Configure isolation dynamically via a KRM-style YAML file.
- Provide zero-egress capabilities for sensitive MCP servers (e.g., Dev MCP).

## Non-Goals

- Cluster-wide network policies (this is strictly intra-pod/node-level egress restriction using iptables).
- Ingress filtering (this focuses on egress).

## Key Requirements

- Must run as an `initContainer` with `NET_ADMIN` capability.
- Must execute quickly and fail visibly if the configuration is invalid.
- Containers inside the pod must be run with specific known `securityContext.runAsUser` values that match the rules.
- All non-init containers in the Pod MUST enforce a `securityContext` that drops `ALL` capabilities, sets `privileged: false`, and sets `allowPrivilegeEscalation: false`. This prevents sandbox breakout via `iptables -F`.

## Design

### KRM Configuration
The init container will accept a configuration file (e.g., mounted via `ConfigMap` to `/etc/network-isolation/config.yaml`).
```yaml
kind: PodNetworkIsolation
apiVersion: factory.ai.gke.io/v1alpha1
spec:
  rules:
  - uid: 1001
    allowEgress: false # Block all external network access
  - uid: 1002
    allowEgress: true  # Allow external access
```

### Implementation Details
The init container will be a simple Go binary or bash script that parses the config and enforces a **Default-Deny** architecture.
- Set the default policy of the `OUTPUT` chain to `DROP` for both IPv4 (`iptables`) and IPv6 (`ip6tables`).
- Explicitly `ACCEPT` traffic only for the UIDs that have `allowEgress: true` configured.
- **Strict Localhost Blocking**: Ensure that *all* loopback network traffic (TCP/UDP over `127.0.0.1` and `::1`) is `DROP`ped for UIDs where `allowEgress: false`. Because communication between the loop and MCP servers is handled entirely via named pipes (filesystem IPC), there is no legitimate reason for sandboxed containers to access the Pod's network loopback interface. This prevents local proxy/SSRF bypasses.
- Traffic matching restricted UIDs will use `-j DROP` instead of `REJECT` to silently blackhole packets, providing zero feedback to malicious code.

## Tests

- Must verify that a container running as UID 1001 (Dev, `allowEgress: false`) cannot reach the internet.
- Must verify that a container running as UID 1002 (Git, `allowEgress: true`) can reach the internet.
- Must verify that a container running as an unconfigured UID (e.g., UID 1003) cannot reach the internet (Default-Deny validation).
- Must verify that UID 1001 cannot reach localhost (`127.0.0.1` or `::1`) on any port.
- Must verify that UID 1001 cannot reach external resources via IPv6.
- Must verify that the init container fails to start the Pod if given an invalid config.
