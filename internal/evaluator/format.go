package evaluator

import (
	"fmt"
	"strings"
)

const formatFlags = "#+- 0"

func isAllowedFormatVerb(verb rune) bool {
	switch verb {
	case 'b', 'c', 'd', 'e', 'E', 'f', 'F', 'g', 'G', 'o', 'O', 'p', 'q', 's', 't', 'U', 'v', 'x', 'X':
		return true
	default:
		return false
	}
}

// CountFormatVerbs counts format verbs in fmtStr, ignoring escaped "%%".
// Returns an error for invalid or unterminated verbs.
func CountFormatVerbs(fmtStr string) (int, error) {
	count := 0
	for i := 0; i < len(fmtStr); i++ {
		if fmtStr[i] != '%' {
			continue
		}
		if i+1 >= len(fmtStr) {
			return 0, fmt.Errorf("unterminated format verb")
		}
		if fmtStr[i+1] == '%' {
			i++
			continue
		}
		j := i + 1
		for j < len(fmtStr) && strings.ContainsRune(formatFlags, rune(fmtStr[j])) {
			j++
		}
		for j < len(fmtStr) && fmtStr[j] >= '0' && fmtStr[j] <= '9' {
			j++
		}
		if j < len(fmtStr) && fmtStr[j] == '.' {
			j++
			for j < len(fmtStr) && fmtStr[j] >= '0' && fmtStr[j] <= '9' {
				j++
			}
		}
		if j >= len(fmtStr) {
			return 0, fmt.Errorf("unterminated format verb")
		}
		verb := rune(fmtStr[j])
		if !isAllowedFormatVerb(verb) {
			return 0, fmt.Errorf("invalid format verb %%%c", verb)
		}
		count++
		i = j
	}
	return count, nil
}
