Break this down into a set of specs using .agents/speccer.
All specs must require following pristine Go and Kubernetes style.
All specs must require test quality on par with the Go standard library.

## Implementation Frameworks

- The loop itself will be implemented using [adk-go](https://github.com/google/adk-go).
- When we eventually get to the operator (note we are not building it yet), we will use [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime).

## Tools

All tools must be provided via local MCP servers that communicate
with `factory runtime loop` over named pipes. While an MCP
server could provide the ability to run arbitrary commands,
for example in a sandbox with the working directory mounted,
the loop itself is not allowed to execute commands directly.

We'll worry about how the MCP servers get set up later.
Most likely we'll do it in a Pod (see far below).

## Loop configuration

Since we don't know exactly which series of steps or loops
is best, we should be able to adjust them flexibly. To do
this, we need control flow and the ability to nest loops.
Here is an example of a loop config that could support this.

### Run

Run defines an instance of a loop to be executed.
This represents an actual execution. This will later be
reconciled to a real Pod or Job by an operator. 

```yaml
kind: Run
apiVersion: factory.ai.gke.io/v1alpha1
metadata:
  name: spec-developer-12345
spec:
  globalMaxSteps: 1000 # global limit on max steps, accounted recursively for all loops and sub-loops. This is different from Loop maxSteps which is only 1 level deep, for that specific loop.
  start: spec-review-main # name of the entrypoint Loop
  args: # args passed to start. Semantics same as K8s env vars and supports downward API semantics too.
  - name: IDEA
    value: "I want to build an agent to do X"
  - name: REPO_URL
    value: "https://www.github.com/user/repo"
# convention: passing runs exit with code 0, failing runs exit with nonzero code
```

### Loop

A Loop is a composable primitive that supports basic
looping, control flow, and nesting of loops based on pass/fail
behavior. The `factory runtime loop` exposes basic `pass`
and `fail` tools to agents so they can indicate when they
are done (pass) or unable to make progress (fail).

Here is a very high level view of a loop with details omitted:
```yaml
kind: Loop
metadata:
  name: spec-review-main
spec:
  start: clone-step
  steps:
  - name: clone-step
    mcp: # calls an MCP tool directly
    pass: # what to do on pass
    fail: # what to do on fail
  - name: spec-review-step
    loop: # runs a nested loop
    pass:
    fail:
  - name: push-step
    mcp:
    pass:
    fail:
```

Here is a more detailed expansion of that loop:
```yaml
kind: Loop
metadata:
  name: spec-review-main
spec:
  start: clone-step
  steps:
  - name: clone-step
    mcp: # calls an MCP tool directly
      name: git-mcp # name of the mcp server
      tool: clone_repository # name of the tool
      args: # mcp tool arguments
      - name: remote # argument name
        value: "$(REPO_URL)" # value interpolated from args, in this case configured by Run
    pass: # what to do on pass
      next: spec-review-step # step to move on to on pass
      message: "cloned $(REPO_URL) successfully" # message passed to the next step
      history: none # whether to forward context, default is "none" to start next step with fresh context
    fail: # what to do on fail
      next: return # return is a reserved keyword that returns message and history to the parent
      message: "failed to clone $(REPO_URL)" # failure message to pass to next step
      history: full # passes the full history from this step to the next step
  - name: spec-review-step
    loop: # runs a nested loop
      name: spec-review-loop # name of the loop
      args: # passed to the nested loop
      - name: IDEA
        value: "$(IDEA)"
    pass:
      # history defaults to none, and no message necessary for next step
      next: push
    fail:
      next: return
      message: "failed to write a spec"
      history: summary # passes an LLM summary of history to the next step
  - name: push
    mcp:
      name: git-mcp
      tool: push_branch
      args:
      - name: remote
        value: "$(REPO_URL)"
      - name: branch
        value: main
    pass:
      next: return # last step, so return to the parent on pass
    fail:
      next: return
      message: "failed to push changes to $(REPO_URL)"
      history: full
```

Drilling deeper, this is the definition of the spec-review-loop.
Note, it could be in the same multidoc yaml file as the above.

```yaml
kind: Loop
metadata:
  name: spec-review-loop
spec:
  start: speccer-step
  maxSteps: 25 # limit on maximum number of step executions. Each step execution counts as one, even if the step is a loop. That sub-loop would have its own internal maxSteps limit.
  steps:
  - name: speccer-step
    agent: # runs an agent
      name: speccer-agent # name of the Agent
      prompt: "Generate a spec for this idea (check args for idea)." # optional additional prompt, beyond agent definition
      args: # agents can be passed args exactly the same way as tools, this is "agents as tools" pattern
      # as an example, the idea could be passed as an arg.
      - name: idea
        value: "$(IDEA)" # interpolated from Loop env, just like other places above
    pass:
      next: spec-format-step
      history: none # explicit to clarify that the review agent should start fresh
    fail:
      next: retry # retry is a keyword that just keeps repeating the step until it succeeds
  - name: spec-format-step
    agent:
      name: spec-format-agent
      prompt: "Validate the format of new spec files."
    pass:
      next: spec-review-step
    fail:
      next: speccer-step # loop back for rework on failure
      message: "spec failed validation"
      history: full # full history so speccer can see what failed
  - name: spec-review-step
    agent:
      name: spec-review-agent
      prompt: "Review the spec for consistency with the idea, and consistency with adjacent specs."
      args:
      - name: idea
        value: "$(IDEA)"
    pass:
      next: return
    fail:
      next: speccer-step # loop back for rework on failure
      message: "spec failed review"
      history: summary # passes an LLM summary of history to the next step
```

Agents and MCP servers are also defined in KRM style resources:

```yaml
kind: Agent
apiVersion: factory.ai.gke.io/v1alpha1
metadata:
  name: speccer-agent
spec:
  path: /etc/agents/speccer/agent.md # path to agent.md definition
  tools:
  - mcp: 
      name: dev-mcp # name of MCP server
      tools: # allowlist of tools
      - name: ReadFile
      - name: WriteFile
---
kind: Agent
apiVersion: factory.ai.gke.io/v1alpha1
metadata:
  name: spec-format-agent
spec:
  path: /etc/agents/spec-format/agent.md 
  tools:
  - mcp: 
      name: dev-mcp 
      tools:
      - name: ReadFile
      - name: RunCommand
        allowedArgs: # TODO: just an example, we need something compatible with MCP, not sure if this is expressing the right way to filter args or not
        - name: command
          # TODO: this may be better off as a spec-mcp server, rather than a command filter...
          values: ["go run tool/cmd/tool/tool.go spec validate"] # or a prefix or something, TODO
---
kind: Agent
apiVersion: factory.ai.gke.io/v1alpha1
metadata:
  name: spec-review-agent
spec:
  path: /etc/agents/spec-review/agent.md # path to agent.md definition
  tools:
  - mcp: 
      name: dev-mcp # name of MCP server
      tools: # allowlist of tools
      - name: ReadFile
---
kind: LocalMCPServer # identifies where to find a local MCP server; the server itself is configured externally
apiVersion: factory.ai.gke.io/v1alpha1
metadata:
  name: git-mcp
spec:
  pipe: /var/run/mcp/git-mcp
---
kind: LocalMCPServer
apiVersion: factory.ai.gke.io/v1alpha1
metadata:
  name: dev-mcp
spec:
  pipe: /var/run/mcp/dev-mcp
```

## Operator

We aren't building the operator yet, but to give a sense
of how it will work, consider the following:

- Loop, Run, and Agent become namespaced KRM resources that can be created in a kube-apiserver as CRDs.
- We add a new MCPServer resource that has more specifics on how to set up the MCP containers, e.g. required mounts and such. TBD.
- We extend Run to take a list of these MCP servers and configure the secret volumes they'll need.

When a Run is created, the Operator deploys a Pod or Job like the below example.
Probably a Job, and `factory runtime loop` can use standard exit code semantics to signal pass/fail and K8s termination message in additon to logs.

### Pod approach rough idea

likely embedded in a K8s Job 

initContainers:
- future init container that sets up uid-based iptables rules to block egress where necessary, and to set up named pipes in emptydirs
- sidecar MCP for git
  - restartPolicy: Always (still an init container)
  - mounts: working directory so it can check out code, emptydir for git MCP, secret with github PAT if needed
  - network: egress allowed to internet e.g. for github access
- sidecar MCP for go module pulling and such? maybe gopls MCP can do it? https://go.dev/gopls/features/mcp says yes
  - restartPolicy: Always (still an init container)
  - mounts: working directory, secret if needed for private modules
  - network: egress allowed e.g. for pulling modules
- sidecar MCP for development
  - restartPolicy: Always (still an init container)
  - mounts: working directory, emptydir for dev MCP
  - network: no egress allowed, since this can run potentially dangerous commands
containers:
- `factory runtime loop` container, configured with the above loop yaml injected in the fs via a configmap or downward API
  - mounts: all the emptydirs for MCP servers, but NOT the working dir, and also all the agent definitions
  - network: egress allowed for LLM calls
volumes:
- emptydir for each MCP server 
- working directory for code