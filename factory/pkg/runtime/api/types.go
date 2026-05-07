package api

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Run struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RunSpec `json:"spec"`
}

type RunSpec struct {
	GlobalMaxSteps int        `json:"globalMaxSteps"`
	Start          string     `json:"start"`
	Args           []Argument `json:"args,omitempty"`
}

type Argument struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Loop struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              LoopSpec `json:"spec"`
}

type LoopSpec struct {
	Start    string `json:"start"`
	MaxSteps int    `json:"maxSteps,omitempty"`
	Steps    []Step `json:"steps"`
}

type Step struct {
	Name  string       `json:"name"`
	MCP   *MCPAction   `json:"mcp,omitempty"`
	Loop  *LoopAction  `json:"loop,omitempty"`
	Agent *AgentAction `json:"agent,omitempty"`
	Pass  NextAction   `json:"pass"`
	Fail  NextAction   `json:"fail"`
}

type MCPAction struct {
	Name string     `json:"name"`
	Tool string     `json:"tool"`
	Args []Argument `json:"args,omitempty"`
}

type LoopAction struct {
	Name string     `json:"name"`
	Args []Argument `json:"args,omitempty"`
}

type AgentAction struct {
	Name   string     `json:"name"`
	Prompt string     `json:"prompt,omitempty"`
	Args   []Argument `json:"args,omitempty"`
}

type HistoryMode string

const (
	HistoryNone    HistoryMode = "none"
	HistoryFull    HistoryMode = "full"
	HistorySummary HistoryMode = "summary"
)

type NextAction struct {
	Next    string      `json:"next"`
	Message string      `json:"message,omitempty"`
	History HistoryMode `json:"history,omitempty"`
}

// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Agent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              AgentSpec `json:"spec"`
}

type AgentSpec struct {
	Path  string         `json:"path"`
	Tools []ToolProvider `json:"tools,omitempty"`
}

type ToolProvider struct {
	MCP *MCPToolSet `json:"mcp,omitempty"`
}

type MCPToolSet struct {
	Name  string       `json:"name"`
	Tools []ToolConfig `json:"tools"`
}

type ToolConfig struct {
	Name string `json:"name"`
}

// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type LocalMCPServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              LocalMCPServerSpec `json:"spec"`
}

type LocalMCPServerSpec struct {
	Pipe string `json:"pipe"`
}
