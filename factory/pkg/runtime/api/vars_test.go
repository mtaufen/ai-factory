package api

import (
	"testing"
)

func TestExpandVariables(t *testing.T) {
	args := map[string]string{
		"REPO_URL": "https://github.com/example/repo",
		"IDEA":     "build an agent",
		"EMPTY":    "",
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no variables",
			input:    "just a normal string",
			expected: "just a normal string",
		},
		{
			name:     "single defined variable",
			input:    "clone $(REPO_URL) successfully",
			expected: "clone https://github.com/example/repo successfully",
		},
		{
			name:     "multiple defined variables",
			input:    "$(IDEA) at $(REPO_URL)",
			expected: "build an agent at https://github.com/example/repo",
		},
		{
			name:     "undefined variable",
			input:    "hello $(UNDEFINED) world",
			expected: "hello  world",
		},
		{
			name:     "defined empty variable",
			input:    "value is $(EMPTY)",
			expected: "value is ",
		},
		{
			name:     "variable with no name",
			input:    "value is $()",
			expected: "value is ",
		},
		{
			name:     "malformed - missing closing parenthesis",
			input:    "value is $(REPO_URL",
			expected: "value is $(REPO_URL",
		},
		{
			name:     "malformed - nested variables",
			input:    "value is $($(REPO_URL))",
			expected: "value is )", // Innermost $($(REPO_URL doesn't match since no ), it matches $(... up to first ) which is $(REPO_URL). Wait, my regex `\$\(([^)]*)\)` matches `$(REPO_URL)`. Wait, no. The string is `$($(REPO_URL))`. It encounters `$(`. Then `$(REPO_URL`. Then `)`. So the match is `$($(REPO_URL)`. The var name is `$(REPO_URL`. Look up yields `""`. Remaining string is `)`. Expected `)`.
		},
		{
			name:     "malformed - mismatched parentheses",
			input:    "value is $((REPO_URL)",
			expected: "value is ", // variable name is "(REPO_URL", yields ""
		},
		{
			name:     "variable adjacent to text",
			input:    "prefix$(REPO_URL)suffix",
			expected: "prefixhttps://github.com/example/reposuffix",
		},
		{
			name:     "only variable",
			input:    "$(REPO_URL)",
			expected: "https://github.com/example/repo",
		},
		{
			name:     "consecutive variables",
			input:    "$(IDEA)$(REPO_URL)",
			expected: "build an agenthttps://github.com/example/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ExpandVariables(tt.input, args)
			if actual != tt.expected {
				t.Errorf("ExpandVariables(%q) = %q; expected %q", tt.input, actual, tt.expected)
			}
		})
	}
}
