package api

import (
	"regexp"
)

var varRegex = regexp.MustCompile(`\$\(([^)]*)\)`)

// ExpandVariables replaces instances of $(VAR) in strings using the provided map of arguments.
// If a variable is not defined, it evaluates to an empty string.
func ExpandVariables(input string, args map[string]string) string {
	return varRegex.ReplaceAllStringFunc(input, func(match string) string {
		varName := match[2 : len(match)-1]
		return args[varName]
	})
}
