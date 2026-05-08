package engine

import (
	"reflect"
	"testing"

	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/api"
)

func TestInterpolateArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []api.Argument
		context map[string]string
		want    map[string]string
	}{
		{
			name: "empty args",
			args: []api.Argument{},
			context: map[string]string{
				"FOO": "bar",
			},
			want: map[string]string{},
		},
		{
			name: "no interpolation needed",
			args: []api.Argument{
				{Name: "arg1", Value: "val1"},
				{Name: "arg2", Value: "val2"},
			},
			context: map[string]string{},
			want: map[string]string{
				"arg1": "val1",
				"arg2": "val2",
			},
		},
		{
			name: "basic interpolation",
			args: []api.Argument{
				{Name: "arg1", Value: "$(FOO)"},
				{Name: "arg2", Value: "prefix-$(BAR)-suffix"},
			},
			context: map[string]string{
				"FOO": "foo_val",
				"BAR": "bar_val",
			},
			want: map[string]string{
				"arg1": "foo_val",
				"arg2": "prefix-bar_val-suffix",
			},
		},
		{
			name: "missing context variables",
			args: []api.Argument{
				{Name: "arg1", Value: "$(MISSING)"},
				{Name: "arg2", Value: "$(FOO)-$(MISSING)"},
			},
			context: map[string]string{
				"FOO": "foo_val",
			},
			want: map[string]string{
				"arg1": "",
				"arg2": "foo_val-",
			},
		},
		{
			name: "malformed syntaxes",
			args: []api.Argument{
				{Name: "arg1", Value: "$(UNCLOSED"},
				{Name: "arg2", Value: "$NO_PARENS"},
				{Name: "arg3", Value: "$(FOO"},
			},
			context: map[string]string{
				"FOO": "foo_val",
			},
			want: map[string]string{
				"arg1": "$(UNCLOSED",
				"arg2": "$NO_PARENS",
				"arg3": "$(FOO",
			},
		},
		{
			name: "multiple interpolations in one value",
			args: []api.Argument{
				{Name: "arg1", Value: "$(A)/$(B)/$(C)"},
			},
			context: map[string]string{
				"A": "x",
				"B": "y",
				"C": "z",
			},
			want: map[string]string{
				"arg1": "x/y/z",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InterpolateArgs(tt.args, tt.context)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("InterpolateArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}
