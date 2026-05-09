---
name: sandbox-kind-cluster
deps: []
---

# Sandbox Kind Cluster

## Overview

Defines the local `kind` cluster setup using gVisor (`runsc`) and network sandboxing to protect the host machine and local network from potentially malicious code executed during local development and testing of the AI Factory Loop.

## Goals

- Provide a repeatable setup for creating a local `kind` cluster running `runsc` as the container runtime.
- Prevent Pods within the `kind` cluster from accessing the host machine's `localhost` and local LAN resources.
- Allow limited external internet access for necessary development tasks (e.g., pulling Go modules, git clone).

## Non-Goals

- Full production-grade multi-tenant isolation (this is for local testing).
- Implementation of the Kubernetes Operator for managing resources (this is for the local sandbox environment).

## Key Requirements

- `kind` node image must be customized to install `runsc` and configure `containerd` to use it as a runtime class.
- The default `RuntimeClass` for the cluster (or at least the `gvisor` RuntimeClass) must point to `runsc`.
- Network restrictions must be applied at the host level (e.g., via Docker bridge network rules or kind custom networking) to drop traffic destined for RFC1918 addresses not belonging to the kind cluster's own CIDR.

## Design

### gVisor in Kind
We will use a custom `Dockerfile` to build a `kind` node image that includes `runsc`.
- Base image: Standard `kindest/node` image.
- Steps:
  1. Download and install `runsc` binaries to `/usr/local/bin`.
  2. Modify `/etc/containerd/config.toml` to configure `runsc` as the **default** runtime for all containers, ensuring no Pods can accidentally bypass the sandbox by omitting a `runtimeClassName`.
- Create a `RuntimeClass` resource named `gvisor` in the cluster that maps to the `runsc` handler (for backwards compatibility if requested explicitly).

### Network Isolation
The local `kind` cluster runs within a Docker network. To isolate it from the host and adjacent local networks:
- Create a custom Docker network with `com.docker.network.bridge.enable_icc=false`. Ensure IPv6 is disabled on this network to prevent IPv6-based bypasses.
- Note: Disabling ICC (`enable_icc=false`) restricts this `kind` cluster to a single node. Multi-node clusters would require ICC to be enabled and rely solely on `iptables` rules for isolation.
- Configure the Docker daemon/network to inject public DNS servers (e.g., `8.8.8.8`, `1.1.1.1`) into the containers, avoiding the need to allow traffic to local LAN DNS resolvers.
- Apply `iptables` rules in the `FORWARD` and/or `DOCKER-USER` chains to isolate the network.
- Rule definitions (applied in order):
  - `DROP` traffic destined for the Docker bridge gateway IP (to protect host services bound to `0.0.0.0`).
  - `DROP` traffic destined for `169.254.0.0/16` (Cloud Metadata Services).
  - `DROP` traffic destined for `100.64.0.0/10` (CGNAT / Tailscale / VPNs).
  - `DROP` traffic destined for `224.0.0.0/4` and `255.255.255.255/32` (Multicast/Broadcast).
  - `DROP` traffic destined for `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16` (excluding the specific subnet assigned to the `kind` Docker network).
  - `ACCEPT` traffic destined for `0.0.0.0/0` (External Internet).

## Tests

- Must verify that `runsc` is available and the node uses it as the default runtime.
- **Independent Network Isolation Test**: 
  - Create a dedicated "Network Test Pod" containing common networking tools (e.g., `curl`, `nc`, `ping`).
  - **Verify Egress**: The Pod must successfully reach an external FQDN and IP (e.g., `curl -I https://github.com`, `ping 8.8.8.8`).
  - **Verify Blacklist Enforcement**: The Pod must execute a script that attempts to connect (using short timeouts) to the dropped CIDRs:
    - Docker bridge gateway IP (e.g. `curl -v --connect-timeout 2 http://<bridge_ip>`).
    - Cloud Metadata (`169.254.169.254`).
    - CGNAT/Tailscale (`100.64.0.1`).
    - Private IP ranges (`10.0.0.1`, `172.16.0.1`, `192.168.0.1`).
  - The script must assert that *all* blacklist connection attempts fail (e.g., via timeout or connection refused, not successful HTTP response) before the test is considered passing.
