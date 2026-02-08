package main

func getLine(content string, lineIndex int) string {
	start := 0
	currentLine := 0
	n := len(content)

	for i := 0; i < n; i++ {
		if content[i] == '\n' {
			if currentLine == lineIndex {
				return content[start:i]
			}
			start = i + 1
			currentLine++
		}
	}

	if currentLine == lineIndex {
		return content[start:]
	}

	return ""
}

func getWordAtPosition(content string, line, char int) string {
	lineStr := getLine(content, line)
	if char < 0 || char >= len(lineStr) {
		// If cursor is at the end of line (after last char), check previous char
		if char == len(lineStr) && char > 0 {
			char--
		} else {
			return ""
		}
	}

	// Find start of word
	start := char
	for start > 0 && isIdentifierChar(lineStr[start-1]) {
		start--
	}

	// Find end of word
	end := char
	for end < len(lineStr) && isIdentifierChar(lineStr[end]) {
		end++
	}

	if start > end {
		return ""
	}
	return lineStr[start:end]
}

func isIdentifierChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}
