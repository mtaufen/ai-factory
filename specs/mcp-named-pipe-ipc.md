---
name: mcp-named-pipe-ipc
deps: []
---

# MCP Named Pipe IPC Setup

## Overview

Specifies the mechanism to set up named pipes (FIFOs) for communication between the `factory runtime loop` and Local MCP servers within the same Pod.

## Goals

- Establish secure and reliable IPC without requiring host network or TCP/IP configuration.
- Ensure named pipes are created before any containers that depend on them start.
- Provide a clean permission model so required containers can read and write to the pipes.

## Non-Goals

- Defining the MCP protocol itself.
- Implementing the named pipe communication logic in the applications.

## Design

### The Problem
If containers create their own named pipes in a shared `emptyDir`, race conditions can occur if one container starts before the other, or if it lacks the necessary permissions to create the pipe in a specific way.

### The Solution: Init Container & Multi-Volume Isolation
We will use an `initContainer` specifically for creating the pipes, combined with dedicated `emptyDir` volumes for each pipe to enforce strict isolation between MCP servers.
- **Volumes:** Separate `emptyDir` volumes for each MCP server (e.g., `git-mcp-pipe`, `dev-mcp-pipe`).
- **Init Container:** `pipe-setup` runs a simple script:
  ```sh
  mkfifo /var/run/mcp/git-mcp/pipe
  chmod 0666 /var/run/mcp/git-mcp/pipe
  mkfifo /var/run/mcp/dev-mcp/pipe
  chmod 0666 /var/run/mcp/dev-mcp/pipe
  ```
- **Permissions:** `chmod 0666` allows containers running under different UIDs to read and write to the FIFOs. Because each pipe resides in its own isolated `emptyDir` volume, a container (like `dev-mcp`) will only mount its specific pipe volume, physically preventing it from accessing other pipes (like `git-mcp`'s) regardless of the file permissions.

### Sidecar Integration
The MCP servers (running as K8s native sidecars) will mount only their respective pipe volume (e.g., `git-mcp` mounts `git-mcp-pipe` to `/var/run/mcp/git-mcp`). The `factory runtime loop` main container will mount all required pipe volumes so it can communicate with all of them.

## Tests

- Must verify that the `pipe-setup` init container successfully creates the pipes.
- Must verify that a non-root sidecar container can read/write to the created pipes.
