package api

import (
	"strings"
	"testing"

)

func TestParse(t *testing.T) {
	yamlData := `
kind: Run
apiVersion: factory.ai.gke.io/v1alpha1
metadata:
  name: spec-developer-12345
spec:
  globalMaxSteps: 1000
  start: spec-review-main
  args:
  - name: IDEA
    value: "I want to build an agent to do X"
  - name: REPO_URL
    value: "https://www.github.com/user/repo"
---
kind: Loop
apiVersion: factory.ai.gke.io/v1alpha1
metadata:
  name: spec-review-main
spec:
  start: clone-step
  steps:
  - name: clone-step
    mcp:
      name: git-mcp
      tool: clone_repository
      args:
      - name: remote
        value: "$(REPO_URL)"
    pass:
      next: spec-review-step
      message: "cloned $(REPO_URL) successfully"
      history: none
    fail:
      next: return
      message: "failed to clone $(REPO_URL)"
      history: full
---
kind: Agent
apiVersion: factory.ai.gke.io/v1alpha1
metadata:
  name: speccer-agent
spec:
  path: /etc/agents/speccer/agent.md
  tools:
  - mcp:
      name: dev-mcp
      tools:
      - name: ReadFile
      - name: WriteFile
---
kind: LocalMCPServer
apiVersion: factory.ai.gke.io/v1alpha1
metadata:
  name: git-mcp
spec:
  pipe: /var/run/mcp/git-mcp
`
	objects, err := Parse(strings.NewReader(yamlData))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(objects) != 4 {
		t.Fatalf("expected 4 objects, got %d", len(objects))
	}

	// 1. Run
	run, ok := objects[0].(*Run)
	if !ok {
		t.Fatalf("expected first object to be *Run, got %T", objects[0])
	}
	if run.Name != "spec-developer-12345" {
		t.Errorf("run.Name = %v, want %v", run.Name, "spec-developer-12345")
	}
	if run.Spec.GlobalMaxSteps != 1000 {
		t.Errorf("run.Spec.GlobalMaxSteps = %v, want %v", run.Spec.GlobalMaxSteps, 1000)
	}

	// 2. Loop
	loop, ok := objects[1].(*Loop)
	if !ok {
		t.Fatalf("expected second object to be *Loop, got %T", objects[1])
	}
	if loop.Name != "spec-review-main" {
		t.Errorf("loop.Name = %v, want %v", loop.Name, "spec-review-main")
	}
	if len(loop.Spec.Steps) != 1 {
		t.Fatalf("loop.Spec.Steps len = %v, want 1", len(loop.Spec.Steps))
	}
	if loop.Spec.Steps[0].Name != "clone-step" {
		t.Errorf("loop.Spec.Steps[0].Name = %v, want %v", loop.Spec.Steps[0].Name, "clone-step")
	}

	// 3. Agent
	agent, ok := objects[2].(*Agent)
	if !ok {
		t.Fatalf("expected third object to be *Agent, got %T", objects[2])
	}
	if agent.Name != "speccer-agent" {
		t.Errorf("agent.Name = %v, want %v", agent.Name, "speccer-agent")
	}
	if agent.Spec.Path != "/etc/agents/speccer/agent.md" {
		t.Errorf("agent.Spec.Path = %v, want %v", agent.Spec.Path, "/etc/agents/speccer/agent.md")
	}
	if len(agent.Spec.Tools) != 1 {
		t.Fatalf("agent.Spec.Tools len = %v, want 1", len(agent.Spec.Tools))
	}

	// 4. LocalMCPServer
	mcp, ok := objects[3].(*LocalMCPServer)
	if !ok {
		t.Fatalf("expected fourth object to be *LocalMCPServer, got %T", objects[3])
	}
	if mcp.Name != "git-mcp" {
		t.Errorf("mcp.Name = %v, want %v", mcp.Name, "git-mcp")
	}
	if mcp.Spec.Pipe != "/var/run/mcp/git-mcp" {
		t.Errorf("mcp.Spec.Pipe = %v, want %v", mcp.Spec.Pipe, "/var/run/mcp/git-mcp")
	}
}

func TestParseStrict(t *testing.T) {
	yamlData := `
kind: Run
apiVersion: factory.ai.gke.io/v1alpha1
metadata:
  name: strict-test
spec:
  globalMaxSteps: 1000
  start: spec-review-main
  unknownField: "this should fail"
`
	_, err := Parse(strings.NewReader(yamlData))
	if err == nil {
		t.Fatalf("expected Parse() to fail on unknown field, but it succeeded")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("expected error to mention 'unknown field', got: %v", err)
	}
}
