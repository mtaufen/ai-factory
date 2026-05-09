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

### The Solution: Init Container
We will use an `initContainer` specifically for creating the pipes.
- **Volume:** A single `emptyDir` mounted at `/var/run/mcp` across all relevant containers.
- **Init Container:** `pipe-setup` runs a simple script:
  ```sh
  mkfifo /var/run/mcp/git-mcp
  mkfifo /var/run/mcp/dev-mcp
  chmod 0666 /var/run/mcp/*
  ```
- **Permissions:** `chmod 0666` allows any container running within the Pod (regardless of their UID) to read and write to the FIFOs. Since this is an isolated `emptyDir` within the Pod, this is secure against external interference.

### Sidecar Integration
The MCP servers (running as K8s native sidecars via `initContainers` with `restartPolicy: Always`) will mount the `/var/run/mcp` volume and listen on their respective pipes. The `factory runtime loop` main container will connect to these pipes.

## Tests

- Must verify that the `pipe-setup` init container successfully creates the pipes.
- Must verify that a non-root sidecar container can read/write to the created pipes.
