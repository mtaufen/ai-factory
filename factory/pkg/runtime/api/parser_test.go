package api

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	run := objects[0]
	if run.GetKind() != "Run" {
		t.Fatalf("expected Run kind, got %v", run.GetKind())
	}
	if run.GetName() != "spec-developer-12345" {
		t.Errorf("run.GetName() = %v, want %v", run.GetName(), "spec-developer-12345")
	}
	steps, _, _ := unstructured.NestedInt64(run.Object, "spec", "globalMaxSteps")
	if steps != 1000 {
		t.Errorf("globalMaxSteps = %v, want %v", steps, 1000)
	}

	// 2. Loop
	loop := objects[1]
	if loop.GetKind() != "Loop" {
		t.Fatalf("expected Loop kind, got %v", loop.GetKind())
	}
	if loop.GetName() != "spec-review-main" {
		t.Errorf("loop.GetName() = %v, want %v", loop.GetName(), "spec-review-main")
	}
	loopSteps, _, _ := unstructured.NestedSlice(loop.Object, "spec", "steps")
	if len(loopSteps) != 1 {
		t.Fatalf("loop steps len = %v, want 1", len(loopSteps))
	}
	stepMap, ok := loopSteps[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected step to be map[string]interface{}, got %T", loopSteps[0])
	}
	if stepMap["name"] != "clone-step" {
		t.Errorf("step name = %v, want clone-step", stepMap["name"])
	}

	// 3. Agent
	agent := objects[2]
	if agent.GetKind() != "Agent" {
		t.Fatalf("expected Agent kind, got %v", agent.GetKind())
	}
	if agent.GetName() != "speccer-agent" {
		t.Errorf("agent.GetName() = %v, want %v", agent.GetName(), "speccer-agent")
	}
	path, _, _ := unstructured.NestedString(agent.Object, "spec", "path")
	if path != "/etc/agents/speccer/agent.md" {
		t.Errorf("agent path = %v, want /etc/agents/speccer/agent.md", path)
	}
	tools, _, _ := unstructured.NestedSlice(agent.Object, "spec", "tools")
	if len(tools) != 1 {
		t.Fatalf("agent tools len = %v, want 1", len(tools))
	}

	// 4. LocalMCPServer
	mcp := objects[3]
	if mcp.GetKind() != "LocalMCPServer" {
		t.Fatalf("expected LocalMCPServer kind, got %v", mcp.GetKind())
	}
	if mcp.GetName() != "git-mcp" {
		t.Errorf("mcp.GetName() = %v, want %v", mcp.GetName(), "git-mcp")
	}
	pipe, _, _ := unstructured.NestedString(mcp.Object, "spec", "pipe")
	if pipe != "/var/run/mcp/git-mcp" {
		t.Errorf("mcp pipe = %v, want /var/run/mcp/git-mcp", pipe)
	}
}

func TestParseGeneric(t *testing.T) {
	yamlData := `
kind: SomeCustomKind
apiVersion: factory.ai.gke.io/v1alpha1
metadata:
  name: dynamic-test
spec:
  unknownField: "this is perfectly fine"
`
	objects, err := Parse(strings.NewReader(yamlData))
	if err != nil {
		t.Fatalf("expected Parse() to succeed on generic resources, but it failed: %v", err)
	}
	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}
	obj := objects[0]
	if obj.GetKind() != "SomeCustomKind" {
		t.Errorf("expected SomeCustomKind, got %v", obj.GetKind())
	}
	val, _, _ := unstructured.NestedString(obj.Object, "spec", "unknownField")
	if val != "this is perfectly fine" {
		t.Errorf("expected unknownField to be preserved, got %v", val)
	}
}

func TestParse_DecodeError(t *testing.T) {
	_, err := Parse(strings.NewReader("kind: [invalid yaml"))
	if err == nil {
		t.Fatal("expected Parse() to fail on invalid YAML")
	}
}

func TestParse_NullDoc(t *testing.T) {
	objs, err := Parse(strings.NewReader("null\n---\nkind: Valid\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objs))
	}
}

func TestParse_UnmarshalError(t *testing.T) {
	// Valid YAML/JSON literal, but not an object
	_, err := Parse(strings.NewReader("\"just a string literal\""))
	if err == nil {
		t.Fatal("expected Parse() to fail on non-object JSON")
	}
}

