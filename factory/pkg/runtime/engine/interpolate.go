package engine

import (
	"github.com/ai-on-gke/ai-factory/factory/pkg/runtime/api"
)

// InterpolateArgs converts a slice of api.Argument to a map[string]string,
// resolving any $(VARIABLE) references in the argument values using the provided context.
func InterpolateArgs(args []api.Argument, context map[string]string) map[string]string {
	result := make(map[string]string, len(args))
	for _, arg := range args {
		result[arg.Name] = api.ExpandVariables(arg.Value, context)
	}
	return result
}
