package main

import (
	"log"
	"os"

	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/modules"
)

func main() {
	config.IsLSPMode = true // Enable LSP mode behaviors (e.g. type variable normalization)

	// Initialize virtual packages (includes documentation)
	modules.InitVirtualPackages()

	log.SetFlags(0)          // Disable timestamp in logs
	log.SetOutput(os.Stderr) // Log to stderr, not stdout (stdout is for LSP protocol)

	server := NewLanguageServer(os.Stdout)
	server.Start()
}

// findMatchingOpener finds the position of the matching opening bracket for a closing bracket at closePos.
// Returns -1 if not found.
func findMatchingOpener(content string, closePos int) int {
	// Simple state machine to skip strings/comments and track brackets
	const (
		NORMAL = iota
		STRING
		RAW_STRING
		TRIPLE_RAW_STRING
		CHAR
		LINE_COMMENT
		BLOCK_COMMENT
	)

	state := NORMAL
	stack := []int{} // stack of opener positions

	// We need to handle escapes in strings/chars.
	escaped := false

	// Iterate bytes to match closePos index
	for i := 0; i < len(content); i++ {
		b := content[i]

		if i == closePos {
			// If we are at the target closing bracket
			// Verify it is actually a bracket in NORMAL state
			if state == NORMAL && (b == ')' || b == ']' || b == '}') {
				if len(stack) > 0 {
					return stack[len(stack)-1]
				}
			}
			return -1
		}

		switch state {
		case NORMAL:
			switch b {
			case '/':
				if i+1 < len(content) {
					if content[i+1] == '/' {
						state = LINE_COMMENT
						i++ // consume /
					} else if content[i+1] == '*' {
						state = BLOCK_COMMENT
						i++ // consume *
					}
				}
			case '"':
				state = STRING
			case '`':
				if i+2 < len(content) && content[i+1] == '`' && content[i+2] == '`' {
					state = TRIPLE_RAW_STRING
					i += 2
				} else {
					state = RAW_STRING
				}
			case '\'':
				state = CHAR
			case '(', '[', '{':
				stack = append(stack, i)
			case ')', ']', '}':
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
			}
		case LINE_COMMENT:
			if b == '\n' {
				state = NORMAL
			}
		case BLOCK_COMMENT:
			if b == '*' && i+1 < len(content) && content[i+1] == '/' {
				state = NORMAL
				i++
			}
		case STRING:
			if escaped {
				escaped = false
			} else if b == '\\' {
				escaped = true
			} else if b == '"' {
				state = NORMAL
			}
		case CHAR:
			if escaped {
				escaped = false
			} else if b == '\\' {
				escaped = true
			} else if b == '\'' {
				state = NORMAL
			}
		case RAW_STRING:
			if b == '`' {
				state = NORMAL
			}
		case TRIPLE_RAW_STRING:
			if b == '`' && i+2 < len(content) && content[i+1] == '`' && content[i+2] == '`' {
				state = NORMAL
				i += 2
			}
		}
	}
	return -1
}
