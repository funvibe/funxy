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
	if seq[len(seq)-1] == 'u' {
		if key, ok := mapCSIUSequence(seq); ok {
			return key
		}
		return string(seq)
	}
	if key, ok := mapCSI1FunctionalSequence(seq); ok {
		return key
	}
	if len(seq) == 3 && seq[2] == 'Z' {
		return "shift+tab"
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

// mapCSI1FunctionalSequence handles the canonical Kitty form used for arrows,
// Home/End, and F1/F2/F4. Unmodified legacy sequences omit "1;1"; report-all
// mode may include it. Lock modifiers do not change the stable key name, while
// other modifiers remain lossless because termio has no names for them.
func mapCSI1FunctionalSequence(seq []byte) (string, bool) {
	var key string
	switch seq[len(seq)-1] {
	case 'A':
		key = "up"
	case 'B':
		key = "down"
	case 'C':
		key = "right"
	case 'D':
		key = "left"
	case 'H':
		key = "home"
	case 'F':
		key = "end"
	case 'P':
		key = "f1"
	case 'Q':
		key = "f2"
	case 'S':
		key = "f4"
	default:
		return "", false
	}

	params := string(seq[2 : len(seq)-1])
	if params == "" {
		return key, true
	}
	fields := strings.Split(params, ";")
	if len(fields) != 2 || fields[0] != "1" {
		return "", false
	}
	modifierBits, eventType, ok := parseKittyModifier(fields[1])
	if !ok || modifierBits != 0 || eventType != 1 {
		return "", false
	}
	return key, true
}

// mapCSIUSequence maps the subset of the Kitty keyboard protocol represented
// by termio's stable key vocabulary. Complete but malformed/unsupported CSI-u
// sequences deliberately return ok=false so the caller preserves them
// verbatim instead of guessing.
func mapCSIUSequence(seq []byte) (string, bool) {
	if len(seq) < 4 || !bytes.HasPrefix(seq, []byte("\x1b[")) || seq[len(seq)-1] != 'u' {
		return "", false
	}

	fields := strings.Split(string(seq[2:len(seq)-1]), ";")
	if len(fields) == 0 || len(fields) > 3 || fields[0] == "" || strings.Contains(fields[0], ":") {
		return "", false
	}
	keyCode, err := strconv.Atoi(fields[0])
	if err != nil || keyCode < 0 || keyCode > utf8.MaxRune || keyCode >= 0xd800 && keyCode <= 0xdfff {
		return "", false
	}

	modifierBits, eventType, ok := parseKittyModifier("")
	if len(fields) >= 2 {
		modifierBits, eventType, ok = parseKittyModifier(fields[1])
	}
	if !ok {
		return "", false
	}
	// termio currently exposes key presses only. Repeat/release sequences are
	// losslessly preserved if a surrounding terminal mode requests them.
	if eventType != 1 {
		return "", false
	}

	if len(fields) == 3 && fields[2] != "" {
		if text, ok := decodeCSIUText(fields[2]); ok {
			switch modifierBits {
			case 0, 1: // Plain or Shift: associated text is the produced character(s).
				return text, true
			case 2, 3: // Preserve legacy Alt+text naming, including Shift+Alt.
				if text == " " {
					return "alt+space", true
				}
				return "alt+" + text, true
			default:
				return "", false
			}
		}
		return "", false
	}

	switch keyCode {
	case 13:
		switch modifierBits {
		case 0:
			return "enter", true
		case 1:
			return "shift+enter", true
		}
	case 9:
		switch modifierBits {
		case 0:
			return "tab", true
		case 1:
			return "shift+tab", true
		}
	case 8, 127:
		if modifierBits == 0 {
			return "backspace", true
		}
	case 27:
		if modifierBits == 0 {
			return "escape", true
		}
	case 32:
		switch modifierBits {
		case 0:
			return "space", true
		case 2:
			return "alt+space", true
		case 4:
			return "ctrl+space", true
		}
	}

	if keyCode >= 32 && keyCode < 57344 && keyCode != 127 {
		r := rune(keyCode)
		switch modifierBits {
		case 0:
			return string(r), true
		case 2:
			return "alt+" + string(r), true
		case 4:
			if r >= 'a' && r <= 'z' {
				return "ctrl+" + string(r), true
			}
		}
	}

	return "", false
}

func parseKittyModifier(field string) (modifierBits, eventType int, ok bool) {
	modifierValue := 1
	eventType = 1
	if field != "" {
		parts := strings.Split(field, ":")
		if len(parts) > 2 || parts[0] == "" {
			return 0, 0, false
		}
		var err error
		modifierValue, err = strconv.Atoi(parts[0])
		if err != nil || modifierValue < 1 {
			return 0, 0, false
		}
		if len(parts) == 2 {
			if parts[1] == "" {
				return 0, 0, false
			}
			eventType, err = strconv.Atoi(parts[1])
			if err != nil {
				return 0, 0, false
			}
		}
	}

	// Lock modifiers do not change stable key names. For text-producing keys,
	// their effect is already reflected in the associated-text field.
	modifierBits = modifierValue - 1
	modifierBits &^= (1 << 6) | (1 << 7) // caps_lock, num_lock
	return modifierBits, eventType, true
}

func decodeCSIUText(encoded string) (string, bool) {
	parts := strings.Split(encoded, ":")
	var text strings.Builder
	for _, part := range parts {
		if part == "" {
			return "", false
		}
		codePoint, err := strconv.Atoi(part)
		if err != nil || codePoint < 0x20 || codePoint >= 0x7f && codePoint <= 0x9f ||
			codePoint > utf8.MaxRune || codePoint >= 0xd800 && codePoint <= 0xdfff {
			return "", false
		}
		text.WriteRune(rune(codePoint))
	}
	return text.String(), text.Len() > 0
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
