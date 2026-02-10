package targets

import (
	"github.com/funvibe/funxy/internal/evaluator"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var lineInfoRegex = regexp.MustCompile(`at \d+:\d+:`)

// errorLocationRegex matches "ERROR at N:N: " or "at N:N " prefixes in error messages.
var errorLocationRegex = regexp.MustCompile(`(?:ERROR )?at \d+:\d+:?\s*`)

// runtimeErrorPrefix matches "runtime error: " that VM prepends.
var runtimeErrorPrefix = "runtime error: "

// isResourceExhaustionError returns true if the error is caused by resource limits
// (timeout, recursion depth, etc.) rather than a semantic bug.
func isResourceExhaustionError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "context canceled") ||
		strings.Contains(msg, "execution cancelled") ||
		strings.Contains(msg, "maximum recursion depth exceeded")
}

// isTimeoutError is an alias kept for backward compatibility.
func isTimeoutError(err error) bool {
	return isResourceExhaustionError(err)
}

// extractCoreError strips wrapper prefixes (runtime error:, ERROR at N:N:),
// stack traces, and source labels (<script>, <stdin>, <main>) to get the
// canonical error message for comparison between backends.
func extractCoreError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()

	// Take only the first line (before stack trace)
	if idx := strings.Index(msg, "\n"); idx >= 0 {
		msg = msg[:idx]
	}

	// Strip "runtime error: " prefix (VM)
	msg = strings.TrimPrefix(msg, runtimeErrorPrefix)

	// Strip "ERROR at N:N: " location prefix
	msg = errorLocationRegex.ReplaceAllString(msg, "")

	return strings.TrimSpace(msg)
}

// areResultsEqual checks if two evaluator objects are equal.
func areResultsEqual(a, b evaluator.Object) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Type() != b.Type() {
		return false
	}
	// Simple string comparison for now
	return a.Inspect() == b.Inspect()
}

// inspect returns a string representation of an evaluator object.
func inspect(o evaluator.Object) string {
	if o == nil {
		return "nil"
	}
	return o.Inspect()
}

// getErrorType categorizes an error based on its message.
func getErrorType(err error) string {
	if err == nil {
		return "nil"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "index out of bounds"):
		return "IndexOutOfBounds"
	case strings.Contains(msg, "division by zero"):
		return "DivisionByZero"
	case strings.Contains(msg, "wrong number of arguments"):
		return "ArgumentError"
	case strings.Contains(msg, "type error"), strings.Contains(msg, "type mismatch"),
		strings.Contains(msg, "operator") && strings.Contains(msg, "not supported"),
		strings.Contains(msg, "unknown operator"),
		strings.Contains(msg, "argument to") && strings.Contains(msg, "must be") && strings.Contains(msg, "got"),
		strings.Contains(msg, "expects") && strings.Contains(msg, "got"):
		return "TypeError"
	case strings.Contains(msg, "name not found"), strings.Contains(msg, "not defined"),
		strings.Contains(msg, "identifier not found"), strings.Contains(msg, "undefined variable"):
		return "NameError"
	case strings.Contains(msg, "stack overflow"):
		return "StackOverflow"
	case strings.Contains(msg, "runtime error"):
		return "RuntimeError"
	case strings.Contains(msg, "panic"):
		return "Panic"
	case strings.Contains(msg, "invalid hex string"), strings.Contains(msg, "invalid syntax"),
		strings.Contains(msg, "unexpected token"), strings.Contains(msg, "expected"):
		return "SyntaxError"
	default:
		// Strip line info for fallback comparison
		cleanMsg := lineInfoRegex.ReplaceAllString(msg, "at ?:?:")

		// Fallback to a generic category or the message itself if it's unique enough
		if len(cleanMsg) > 50 {
			return "Other"
		}
		return strings.ReplaceAll(cleanMsg, " ", "_")
	}
}

// LoadCorpus loads all .lang files from the given directories and adds them to the fuzz corpus.
func LoadCorpus(f *testing.F, dirs ...string) {
	for _, dir := range dirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(path, ".lang") {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				f.Add(data)
			}
			return nil
		})
		if err != nil {
			// It's okay if we can't load examples, just log it
			f.Logf("Failed to load corpus from %s: %v", dir, err)
		}
	}
}
