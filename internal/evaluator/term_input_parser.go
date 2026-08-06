package evaluator

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	bracketedPasteStart = []byte("\x1b[200~")
	bracketedPasteEnd   = []byte("\x1b[201~")
)

const (
	maxTermInputPasteBytes  = 16 << 20
	maxTermInputEscapeBytes = 4 << 10
)

// termInputParser parses a byte stream rather than individual Read results.
// pending holds an incomplete key/escape sequence between reads. paste holds
// bracketed-paste payload until its end marker is complete.
type termInputParser struct {
	pending       []byte
	paste         []byte
	inPaste       bool
	maxPasteSize  int // Tests may lower the limit; zero selects the public limit.
	maxEscapeSize int // Tests may lower the limit; zero selects the public limit.
}

func (p *termInputParser) feed(data []byte) []termInputEvent {
	if len(data) > 0 {
		p.pending = append(p.pending, data...)
	}
	events := p.parse(false)
	limit := p.maxEscapeSize
	if limit <= 0 {
		limit = maxTermInputEscapeBytes
	}
	if !p.inPaste && len(p.pending) > limit {
		p.reset()
		events = append(events, errorInputEvent(fmt.Sprintf("terminal escape sequence exceeds the %d-byte limit", limit)))
	}
	return events
}

// flushEscape is called after an inter-byte timeout. It resolves an incomplete
// escape sequence as a standalone Escape key without discarding following data.
func (p *termInputParser) flushEscape() []termInputEvent {
	if p.inPaste || len(p.pending) == 0 || p.pending[0] != 0x1b {
		return nil
	}
	events := []termInputEvent{keyInputEvent("escape")}
	p.pending = p.pending[1:]
	return append(events, p.parse(false)...)
}

func (p *termInputParser) parse(final bool) []termInputEvent {
	var events []termInputEvent
	for len(p.pending) > 0 {
		if p.inPaste {
			if idx := bytes.Index(p.pending, bracketedPasteEnd); idx >= 0 {
				if event, exceeded := p.appendPaste(p.pending[:idx]); exceeded {
					return append(events, event)
				}
				if !utf8.Valid(p.paste) {
					offset := firstInvalidUTF8Byte(p.paste)
					p.reset()
					return append(events, errorInputEvent(fmt.Sprintf("invalid UTF-8 in bracketed paste at byte %d", offset)))
				}
				events = append(events, textInputEvent(string(p.paste)))
				p.pending = p.pending[idx+len(bracketedPasteEnd):]
				p.paste = p.paste[:0]
				p.inPaste = false
				continue
			}

			keep := longestSuffixPrefix(p.pending, bracketedPasteEnd)
			if final {
				keep = 0
			}
			if event, exceeded := p.appendPaste(p.pending[:len(p.pending)-keep]); exceeded {
				return append(events, event)
			}
			p.pending = p.pending[len(p.pending)-keep:]
			break
		}

		if bytes.HasPrefix(p.pending, bracketedPasteStart) {
			p.pending = p.pending[len(bracketedPasteStart):]
			p.inPaste = true
			continue
		}
		if !final && len(p.pending) < len(bracketedPasteStart) && bytes.HasPrefix(bracketedPasteStart, p.pending) {
			break
		}

		event, consumed, needMore := parseTermInputKey(p.pending, final)
		if needMore {
			break
		}
		if consumed <= 0 {
			break
		}
		p.pending = p.pending[consumed:]
		events = append(events, event)
		if event.kind == termInputError {
			p.reset()
			break
		}
	}
	return events
}

func (p *termInputParser) appendPaste(data []byte) (termInputEvent, bool) {
	limit := p.maxPasteSize
	if limit <= 0 {
		limit = maxTermInputPasteBytes
	}
	if len(data) > limit-len(p.paste) {
		p.reset()
		return errorInputEvent(fmt.Sprintf("bracketed paste exceeds the %d-byte limit", limit)), true
	}
	p.paste = append(p.paste, data...)
	return termInputEvent{}, false
}

func (p *termInputParser) reset() {
	p.pending = nil
	p.paste = nil
	p.inPaste = false
}

func firstInvalidUTF8Byte(data []byte) int {
	for offset := 0; offset < len(data); {
		_, size := utf8.DecodeRune(data[offset:])
		if size == 1 && data[offset] >= utf8.RuneSelf {
			return offset
		}
		offset += size
	}
	return -1
}

func parseTermInputKey(data []byte, final bool) (termInputEvent, int, bool) {
	if len(data) == 0 {
		return termInputEvent{}, 0, true
	}

	if data[0] == 0x1b {
		if len(data) == 1 {
			if final {
				return keyInputEvent("escape"), 1, false
			}
			return termInputEvent{}, 0, true
		}

		if data[1] == '[' {
			end := csiSequenceEnd(data)
			if end < 0 {
				if final {
					return keyInputEvent("escape"), 1, false
				}
				return termInputEvent{}, 0, true
			}
			seq := data[:end]
			return keyInputEvent(mapCSISequence(seq)), end, false
		}

		if data[1] == 'O' {
			if len(data) < 3 {
				if final {
					return keyInputEvent("escape"), 1, false
				}
				return termInputEvent{}, 0, true
			}
			return keyInputEvent(mapSS3Sequence(data[:3])), 3, false
		}

		// Preserve both bytes of common Alt+key input as one key event.
		if data[1] >= 32 && data[1] < 127 {
			if data[1] == ' ' {
				return keyInputEvent("alt+space"), 2, false
			}
			return keyInputEvent("alt+" + string(data[1])), 2, false
		}
		return keyInputEvent("escape"), 1, false
	}

	if name, ok := controlKeyName(data[0]); ok {
		return keyInputEvent(name), 1, false
	}
	if data[0] < utf8.RuneSelf {
		return keyInputEvent(string(data[0])), 1, false
	}
	if !utf8.FullRune(data) {
		if !final {
			return termInputEvent{}, 0, true
		}
		return errorInputEvent("incomplete UTF-8 in terminal input"), 1, false
	}
	r, size := utf8.DecodeRune(data)
	if r == utf8.RuneError && size == 1 {
		return errorInputEvent(fmt.Sprintf("invalid UTF-8 in terminal input at byte 0x%02x", data[0])), 1, false
	}
	return keyInputEvent(string(r)), size, false
}

func csiSequenceEnd(data []byte) int {
	for i := 2; i < len(data); i++ {
		if data[i] >= 0x40 && data[i] <= 0x7e {
			return i + 1
		}
	}
	return -1
}

func mapCSISequence(seq []byte) string {
	if len(seq) < 3 {
		return string(seq)
	}
	if len(seq) == 3 {
		switch seq[2] {
		case 'A':
			return "up"
		case 'B':
			return "down"
		case 'C':
			return "right"
		case 'D':
			return "left"
		case 'H':
			return "home"
		case 'F':
			return "end"
		case 'Z':
			return "shift+tab"
		}
	}

	if seq[len(seq)-1] == '~' {
		params := string(seq[2 : len(seq)-1])
		primary := strings.SplitN(params, ";", 2)[0]
		switch primary {
		case "1", "7":
			return "home"
		case "2":
			return "insert"
		case "3":
			return "delete"
		case "4", "8":
			return "end"
		case "5":
			return "pageup"
		case "6":
			return "pagedown"
		case "11", "12", "13", "14", "15", "17", "18", "19", "20", "21", "23", "24":
			functionKeys := map[string]int{
				"11": 1, "12": 2, "13": 3, "14": 4, "15": 5, "17": 6,
				"18": 7, "19": 8, "20": 9, "21": 10, "23": 11, "24": 12,
			}
			return "f" + strconv.Itoa(functionKeys[primary])
		}
	}

	// Unknown terminal sequences remain lossless in the KeyEvent payload.
	return string(seq)
}

func mapSS3Sequence(seq []byte) string {
	if len(seq) != 3 {
		return string(seq)
	}
	switch seq[2] {
	case 'A':
		return "up"
	case 'B':
		return "down"
	case 'C':
		return "right"
	case 'D':
		return "left"
	case 'H':
		return "home"
	case 'F':
		return "end"
	case 'P', 'Q', 'R', 'S':
		return fmt.Sprintf("f%d", int(seq[2]-'P')+1)
	default:
		return string(seq)
	}
}

func controlKeyName(b byte) (string, bool) {
	switch b {
	case 0:
		return "ctrl+space", true
	case '\t':
		return "tab", true
	case '\n', '\r':
		return "enter", true
	case 8, 127:
		return "backspace", true
	case ' ':
		return "space", true
	}
	if b >= 1 && b <= 26 {
		return "ctrl+" + string(rune('a'+b-1)), true
	}
	if b >= 28 && b <= 31 {
		return []string{"ctrl+\\", "ctrl+]", "ctrl+^", "ctrl+_"}[b-28], true
	}
	return "", false
}

func longestSuffixPrefix(data, marker []byte) int {
	maxLen := min(len(data), len(marker)-1)
	for n := maxLen; n > 0; n-- {
		if bytes.Equal(data[len(data)-n:], marker[:n]) {
			return n
		}
	}
	return 0
}
