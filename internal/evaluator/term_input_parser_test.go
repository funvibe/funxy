package evaluator

import (
	"strings"
	"testing"
)

func assertTermInputEvents(t *testing.T, got, want []termInputEvent) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d events %#v, want %d %#v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestTermInputParserMultiplePrintableKeysPerRead(t *testing.T) {
	p := &termInputParser{}
	assertTermInputEvents(t, p.feed([]byte("abc XYZ")), []termInputEvent{
		keyInputEvent("a"), keyInputEvent("b"), keyInputEvent("c"),
		keyInputEvent("space"), keyInputEvent("X"), keyInputEvent("Y"), keyInputEvent("Z"),
	})
}

func TestTermInputParserEscapeSequenceAcrossEveryReadBoundary(t *testing.T) {
	sequence := []byte("\x1b[3~")
	for split := 1; split < len(sequence); split++ {
		t.Run(string(rune('0'+split)), func(t *testing.T) {
			p := &termInputParser{}
			if got := p.feed(sequence[:split]); len(got) != 0 {
				t.Fatalf("first chunk emitted incomplete sequence: %#v", got)
			}
			assertTermInputEvents(t, p.feed(sequence[split:]), []termInputEvent{keyInputEvent("delete")})
		})
	}
}

func TestTermInputParserSequenceAndPayloadInOneRead(t *testing.T) {
	p := &termInputParser{}
	assertTermInputEvents(t, p.feed([]byte("\x1b[Axy")), []termInputEvent{
		keyInputEvent("up"), keyInputEvent("x"), keyInputEvent("y"),
	})
}

func TestTermInputParserBracketedPasteByteByByte(t *testing.T) {
	payload := "first line\r\nsecond\tж🙂\rtrailing\n"
	stream := []byte(string(bracketedPasteStart) + payload + string(bracketedPasteEnd))
	p := &termInputParser{}
	var events []termInputEvent
	for _, b := range stream {
		events = append(events, p.feed([]byte{b})...)
	}
	assertTermInputEvents(t, events, []termInputEvent{textInputEvent(payload)})
}

func TestTermInputParserRejectsInvalidUTF8(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stream []byte
	}{
		{name: "key", stream: []byte{0xff}},
		{name: "paste", stream: append(append(append([]byte{}, bracketedPasteStart...), 0xff), bracketedPasteEnd...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &termInputParser{}
			got := p.feed(tc.stream)
			if len(got) != 1 || got[0].kind != termInputError || !strings.Contains(got[0].value, "invalid UTF-8") {
				t.Fatalf("events = %#v, want one invalid UTF-8 error", got)
			}
		})
	}
}

func TestTermInputParserLimitsUnterminatedPaste(t *testing.T) {
	p := &termInputParser{maxPasteSize: 8}
	stream := append(append([]byte{}, bracketedPasteStart...), []byte("123456789")...)
	got := p.feed(stream)
	if len(got) != 1 || got[0].kind != termInputError || !strings.Contains(got[0].value, "8-byte limit") {
		t.Fatalf("events = %#v, want one paste-limit error", got)
	}
	if p.inPaste || len(p.pending) != 0 || len(p.paste) != 0 {
		t.Fatalf("parser retained data after fatal error: %#v", p)
	}
}

func TestTermInputParserLimitsUnterminatedEscapeSequence(t *testing.T) {
	p := &termInputParser{maxEscapeSize: 8}
	got := p.feed([]byte("\x1b[1234567"))
	if len(got) != 1 || got[0].kind != termInputError || !strings.Contains(got[0].value, "8-byte limit") {
		t.Fatalf("events = %#v, want one escape-limit error", got)
	}
}

func TestTermInputParserUnknownSequencesRemainVerbatim(t *testing.T) {
	p := &termInputParser{}
	assertTermInputEvents(t, p.feed([]byte("\x1b[1;5A\x1bOx")), []termInputEvent{
		keyInputEvent("\x1b[1;5A"), keyInputEvent("\x1bOx"),
	})
}

func TestTermInputParserLongPasteAndFollowingKey(t *testing.T) {
	payload := strings.Repeat("abcdefghijklmnopqrstuvwxyz", 20)
	p := &termInputParser{}
	stream := []byte(string(bracketedPasteStart) + payload + string(bracketedPasteEnd) + "q")
	var events []termInputEvent
	for len(stream) > 0 {
		n := min(13, len(stream))
		events = append(events, p.feed(stream[:n])...)
		stream = stream[n:]
	}
	assertTermInputEvents(t, events, []termInputEvent{textInputEvent(payload), keyInputEvent("q")})
}

func TestTermInputParserPasteSpecialKeyNamesRemainText(t *testing.T) {
	p := &termInputParser{}
	var stream strings.Builder
	for _, payload := range []string{"space", "enter", "tab"} {
		stream.Write(bracketedPasteStart)
		stream.WriteString(payload)
		stream.Write(bracketedPasteEnd)
	}
	assertTermInputEvents(t, p.feed([]byte(stream.String())), []termInputEvent{
		textInputEvent("space"), textInputEvent("enter"), textInputEvent("tab"),
	})
}

func TestTermInputParserUTF8KeyAcrossReads(t *testing.T) {
	encoded := []byte("ж")
	p := &termInputParser{}
	if got := p.feed(encoded[:1]); len(got) != 0 {
		t.Fatalf("incomplete UTF-8 rune emitted: %#v", got)
	}
	assertTermInputEvents(t, p.feed(encoded[1:]), []termInputEvent{keyInputEvent("ж")})
}

func TestTermInputParserStandaloneEscapeAfterTimeout(t *testing.T) {
	p := &termInputParser{}
	if got := p.feed([]byte{0x1b}); len(got) != 0 {
		t.Fatalf("Escape emitted before inter-byte timeout: %#v", got)
	}
	assertTermInputEvents(t, p.flushEscape(), []termInputEvent{keyInputEvent("escape")})
}

func TestTermInputParserControlKeysOutsidePaste(t *testing.T) {
	p := &termInputParser{}
	assertTermInputEvents(t, p.feed([]byte{'\t', '\n', 8, 3}), []termInputEvent{
		keyInputEvent("tab"), keyInputEvent("enter"), keyInputEvent("backspace"), keyInputEvent("ctrl+c"),
	})
}

func TestTermInputParserAltSpaceHasStableName(t *testing.T) {
	p := &termInputParser{}
	assertTermInputEvents(t, p.feed([]byte{'\x1b', ' '}), []termInputEvent{keyInputEvent("alt+space")})
}
