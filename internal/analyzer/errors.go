package analyzer

import (
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/diagnostics"
	"strings"
)

// Error helper functions for the inference layer.
// These create structured errors with location information.

// combinedError holds multiple errors to report together
type combinedError struct {
	errors []error
}

func (e *combinedError) Error() string {
	msgs := make([]string, len(e.errors))
	for i, err := range e.errors {
		msgs[i] = err.Error()
	}
	return strings.Join(msgs, "\n")
}

// inferError creates a type error for inference failures
func inferError(node ast.Node, message string) error {
	tok := getNodeToken(node)
	return diagnostics.NewError(diagnostics.ErrA003, tok, message)
}

// inferErrorf creates a formatted type error
func inferErrorf(node ast.Node, format string, args ...interface{}) error {
	return inferError(node, fmt.Sprintf(format, args...))
}

// typeMismatch creates a type mismatch error
func typeMismatch(node ast.Node, expected, got string) error {
	return inferErrorf(node, "type mismatch: expected %s, got %s", expected, got)
}

// undefinedSymbol creates an undefined symbol error
func undefinedSymbol(node ast.Node, name string) error {
	tok := getNodeToken(node)
	return diagnostics.NewAnalyzerError(diagnostics.ErrA006, tok, name)
}

// undefinedWithHint creates an undefined symbol error with a hint
func undefinedWithHint(node ast.Node, name string, hint string) error {
	tok := getNodeToken(node)
	err := diagnostics.NewAnalyzerError(diagnostics.ErrA006, tok, name)
	err.Hint = hint
	return err
}

// findSimilarNames finds names similar to the given name using Levenshtein distance
func findSimilarNames(name string, table interface{ GetAllNames() []string }, maxDist int) []string {
	allNames := table.GetAllNames()
	var suggestions []string

	nameLower := strings.ToLower(name)
	isVar := len(name) > 0 && name[0] >= 'a' && name[0] <= 'z'

	for _, candidate := range allNames {
		// Skip type names when looking for variable
		if isVar && len(candidate) > 0 && candidate[0] >= 'A' && candidate[0] <= 'Z' {
			continue
		}

		dist := levenshtein(nameLower, strings.ToLower(candidate))
		if dist > 0 && dist <= maxDist {
			suggestions = append(suggestions, candidate)
		}
	}

	if len(suggestions) > 3 {
		return suggestions[:3]
	}
	return suggestions
}

// levenshtein calculates edit distance between two strings
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	matrix := make([][]int, len(a)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(b)+1)
		matrix[i][0] = i
	}
	for j := 0; j <= len(b); j++ {
		matrix[0][j] = j
	}

	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			del := matrix[i-1][j] + 1
			ins := matrix[i][j-1] + 1
			sub := matrix[i-1][j-1] + cost

			min := del
			if ins < min {
				min = ins
			}
			if sub < min {
				min = sub
			}
			matrix[i][j] = min
		}
	}
	return matrix[len(a)][len(b)]
}

// notExhaustive creates a non-exhaustive match error
func notExhaustive(node ast.Node, missing string) error {
	tok := getNodeToken(node)
	return diagnostics.NewAnalyzerError(diagnostics.ErrA007, tok, missing)
}

// wrapBuildTypeError wraps errors from BuildType
func wrapBuildTypeError(errs []*diagnostics.DiagnosticError) error {
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
