//go:build darwin || linux || freebsd || openbsd

package evaluator

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

type fakeTermInputDevice struct {
	mu        sync.Mutex
	mode      syscall.Termios
	setModes  []syscall.Termios
	writes    bytes.Buffer
	readQueue chan fakeTermInputRead
	waitErr   chan error
	nextRead  *fakeTermInputRead
	closed    bool
}

type fakeTermInputRead struct {
	data []byte
	err  error
}

type failingWriteTermInputDevice struct {
	*fakeTermInputDevice
	err error
}

func (d *failingWriteTermInputDevice) Write([]byte) (int, error) {
	return 0, d.err
}

func newFakeTermInputDevice(mode syscall.Termios) *fakeTermInputDevice {
	return &fakeTermInputDevice{
		mode:      mode,
		readQueue: make(chan fakeTermInputRead, 8),
		waitErr:   make(chan error, 1),
	}
}

func (d *fakeTermInputDevice) Read(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.nextRead == nil {
		return 0, errors.New("fake read called without readiness")
	}
	result := d.nextRead
	d.nextRead = nil
	n := copy(p, result.data)
	if n < len(result.data) {
		d.nextRead = &fakeTermInputRead{data: append([]byte(nil), result.data[n:]...), err: result.err}
		return n, nil
	}
	return n, result.err
}

func (d *fakeTermInputDevice) waitReadable(timeout time.Duration) (bool, error) {
	d.mu.Lock()
	if d.nextRead != nil {
		d.mu.Unlock()
		return true, nil
	}
	d.mu.Unlock()

	if timeout > time.Millisecond {
		timeout = time.Millisecond
	}
	select {
	case result := <-d.readQueue:
		d.mu.Lock()
		d.nextRead = &result
		d.mu.Unlock()
		return true, nil
	case err := <-d.waitErr:
		return false, err
	case <-time.After(timeout):
		return false, nil
	}
}

func (d *fakeTermInputDevice) queueRead(data []byte) {
	d.readQueue <- fakeTermInputRead{data: append([]byte(nil), data...)}
}

func (d *fakeTermInputDevice) queueReadError(err error) {
	d.readQueue <- fakeTermInputRead{err: err}
}

func (d *fakeTermInputDevice) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.writes.Write(p)
}

func (d *fakeTermInputDevice) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	return nil
}

func (d *fakeTermInputDevice) getTermios() (syscall.Termios, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.mode, nil
}

func (d *fakeTermInputDevice) setTermios(mode syscall.Termios) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.mode = mode
	d.setModes = append(d.setModes, mode)
	return nil
}

func (d *fakeTermInputDevice) snapshot() (syscall.Termios, []syscall.Termios, string, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.mode, append([]syscall.Termios(nil), d.setModes...), d.writes.String(), d.closed
}

func TestMakeRawTermiosDisablesInputTransformations(t *testing.T) {
	original := syscall.Termios{
		Iflag: syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.INPCK | syscall.ISTRIP |
			syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON | syscall.IXOFF,
		Oflag: syscall.OPOST,
		Cflag: syscall.CSIZE | syscall.PARENB,
		Lflag: syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.IEXTEN | syscall.ISIG,
	}
	raw := makeRawTermios(original)

	inputTransforms := uint64(syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.INPCK | syscall.ISTRIP |
		syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON)
	if uint64(raw.Iflag)&inputTransforms != 0 {
		t.Errorf("raw input flags still transform bytes: %#x", raw.Iflag)
	}
	if raw.Iflag&syscall.IXOFF == 0 {
		t.Error("raw mode changed an unrelated input flag")
	}
	if raw.Oflag&syscall.OPOST != 0 {
		t.Error("raw mode left output post-processing enabled")
	}
	if raw.Cflag&syscall.CSIZE != syscall.CS8 || raw.Cflag&syscall.PARENB != 0 {
		t.Errorf("raw character size/parity = %#x", raw.Cflag)
	}
	if raw.Lflag&(syscall.ECHO|syscall.ECHONL|syscall.ICANON|syscall.IEXTEN|syscall.ISIG) != 0 {
		t.Errorf("raw local flags still process input: %#x", raw.Lflag)
	}
	if raw.Cc[syscall.VMIN] != 1 || raw.Cc[syscall.VTIME] != 0 {
		t.Errorf("raw read timing = VMIN %d VTIME %d", raw.Cc[syscall.VMIN], raw.Cc[syscall.VTIME])
	}
}

func TestTermInputSessionRestoresIfBracketedPasteEnableFails(t *testing.T) {
	original := syscall.Termios{Iflag: syscall.ICRNL, Lflag: syscall.ECHO | syscall.ICANON}
	base := newFakeTermInputDevice(original)
	writeErr := errors.New("write control failed")
	device := &failingWriteTermInputDevice{fakeTermInputDevice: base, err: writeErr}

	err := (&termInputSession{}).start(device)
	if !errors.Is(err, writeErr) {
		t.Fatalf("start error = %v, want wrapped write failure", err)
	}
	mode, modes, _, closed := base.snapshot()
	if mode != original {
		t.Errorf("terminal mode was not restored: got %#v want %#v", mode, original)
	}
	if len(modes) != 2 {
		t.Fatalf("setTermios called %d times, want raw + restore", len(modes))
	}
	if !closed {
		t.Error("terminal device was not closed")
	}
}

func TestTermInputSessionParsesBurstAndRestores(t *testing.T) {
	original := syscall.Termios{
		Iflag: syscall.BRKINT | syscall.ICRNL | syscall.IXON,
		Oflag: syscall.OPOST,
		Cflag: syscall.CS8,
		Lflag: syscall.ECHO | syscall.ICANON | syscall.IEXTEN | syscall.ISIG,
	}
	device := newFakeTermInputDevice(original)
	session := &termInputSession{}
	if err := session.start(device); err != nil {
		t.Fatalf("start: %v", err)
	}

	device.queueRead([]byte("abc"))
	for _, want := range []string{"a", "b", "c"} {
		event, ok, err := session.wait(t.Context(), time.Second)
		if err != nil || !ok {
			t.Fatalf("wait: event=%#v ok=%v err=%v", event, ok, err)
		}
		if event != keyInputEvent(want) {
			t.Errorf("event = %#v, want %q", event, want)
		}
	}

	if err := session.stopSession(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	mode, modes, writes, closed := device.snapshot()
	if mode != original {
		t.Errorf("terminal mode was not restored: got %#v want %#v", mode, original)
	}
	if len(modes) != 2 {
		t.Fatalf("setTermios called %d times, want raw + restore", len(modes))
	}
	if modes[0] == original {
		t.Error("first setTermios did not change terminal mode")
	}
	if writes != bracketedPasteEnable+bracketedPasteDisable {
		t.Errorf("terminal controls = %q", writes)
	}
	if !closed {
		t.Error("terminal device was not closed")
	}
}

func TestWithTerminalInputRestoresAfterReturnAndEvaluatorError(t *testing.T) {
	oldOpen := openTermInputDevice
	defer func() { openTermInputDevice = oldOpen }()

	for _, tc := range []struct {
		name     string
		callback Object
		isError  bool
	}{
		{
			name: "normal return",
			callback: &Builtin{Name: "normal", Fn: func(*Evaluator, ...Object) Object {
				return &Nil{}
			}},
		},
		{
			name: "evaluator error",
			callback: &Builtin{Name: "error", Fn: func(*Evaluator, ...Object) Object {
				return newError("boom")
			}},
			isError: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := syscall.Termios{Lflag: syscall.ECHO | syscall.ICANON}
			device := newFakeTermInputDevice(original)
			openTermInputDevice = func() (termInputDevice, error) { return device, nil }

			result := builtinWithTerminalInput(New(), tc.callback)
			if isError(result) != tc.isError {
				t.Fatalf("result = %#v, want error=%v", result, tc.isError)
			}
			mode, _, writes, closed := device.snapshot()
			if mode != original || writes != bracketedPasteEnable+bracketedPasteDisable || !closed {
				t.Errorf("cleanup incomplete: mode=%#v writes=%q closed=%v", mode, writes, closed)
			}
		})
	}
}

func TestTermInputSessionReportsReadError(t *testing.T) {
	device := newFakeTermInputDevice(syscall.Termios{})
	session := &termInputSession{}
	if err := session.start(device); err != nil {
		t.Fatalf("start: %v", err)
	}
	device.queueReadError(io.ErrUnexpectedEOF)

	_, ok, err := session.wait(t.Context(), time.Second)
	if ok || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("wait returned ok=%v err=%v", ok, err)
	}
	if stopErr := session.stopSession(); stopErr != nil {
		t.Fatalf("stop after read error: %v", stopErr)
	}
}

func TestTermInputSessionReportsEOF(t *testing.T) {
	device := newFakeTermInputDevice(syscall.Termios{})
	session := &termInputSession{}
	if err := session.start(device); err != nil {
		t.Fatalf("start: %v", err)
	}
	device.queueReadError(io.EOF)

	_, ok, err := session.wait(t.Context(), time.Second)
	if ok || !errors.Is(err, io.EOF) {
		t.Fatalf("wait returned ok=%v err=%v, want EOF error", ok, err)
	}
	if stopErr := session.stopSession(); stopErr != nil {
		t.Fatalf("stop after EOF: %v", stopErr)
	}
}

func TestTermInputSessionUsesIdleWaitOnlyForTimeout(t *testing.T) {
	device := newFakeTermInputDevice(syscall.Termios{})
	session := &termInputSession{}
	if err := session.start(device); err != nil {
		t.Fatalf("start: %v", err)
	}

	if event, ok, err := session.wait(t.Context(), 5*time.Millisecond); err != nil || ok {
		t.Fatalf("idle wait returned event=%#v ok=%v err=%v", event, ok, err)
	}
	device.waitErr <- syscall.EIO
	_, ok, err := session.wait(t.Context(), time.Second)
	if ok || !errors.Is(err, syscall.EIO) || !strings.Contains(err.Error(), "wait for terminal input") {
		t.Fatalf("readiness-wait failure returned ok=%v err=%v", ok, err)
	}
	if stopErr := session.stopSession(); stopErr != nil {
		t.Fatalf("stop after readiness-wait error: %v", stopErr)
	}
}

func TestWithTerminalInputReportsOpenTTYError(t *testing.T) {
	oldOpen := openTermInputDevice
	defer func() { openTermInputDevice = oldOpen }()
	openErr := syscall.ENXIO
	openTermInputDevice = func() (termInputDevice, error) { return nil, openErr }

	callback := &Builtin{Name: "unused", Fn: func(*Evaluator, ...Object) Object {
		t.Fatal("callback ran after /dev/tty open failure")
		return &Nil{}
	}}
	result := builtinWithTerminalInput(New(), callback)
	evaluatorErr, ok := result.(*Error)
	if !ok || !strings.Contains(evaluatorErr.Message, "open /dev/tty") || !strings.Contains(evaluatorErr.Message, openErr.Error()) {
		t.Fatalf("result = %#v, want clear /dev/tty runtime error", result)
	}
}

func TestWithTerminalInputCanBeEnteredSequentially(t *testing.T) {
	oldOpen := openTermInputDevice
	defer func() { openTermInputDevice = oldOpen }()

	devices := []*fakeTermInputDevice{
		newFakeTermInputDevice(syscall.Termios{Lflag: syscall.ECHO | syscall.ICANON}),
		newFakeTermInputDevice(syscall.Termios{Lflag: syscall.ECHO | syscall.ICANON}),
	}
	opened := 0
	openTermInputDevice = func() (termInputDevice, error) {
		device := devices[opened]
		opened++
		return device, nil
	}
	callback := &Builtin{Name: "return", Fn: func(*Evaluator, ...Object) Object { return &Nil{} }}

	for i := range devices {
		if result := builtinWithTerminalInput(New(), callback); isError(result) {
			t.Fatalf("entry %d failed: %#v", i+1, result)
		}
		mode, modes, writes, closed := devices[i].snapshot()
		if mode.Lflag != syscall.ECHO|syscall.ICANON || len(modes) != 2 ||
			writes != bracketedPasteEnable+bracketedPasteDisable || !closed {
			t.Fatalf("entry %d cleanup incomplete: mode=%#v modes=%d writes=%q closed=%v", i+1, mode, len(modes), writes, closed)
		}
	}
	if opened != 2 || termInputSessionActive() {
		t.Fatalf("sequential lifecycle left stale state: opened=%d active=%v", opened, termInputSessionActive())
	}
	termInputSignals.Lock()
	signalsInstalled := termInputSignals.channel != nil || termInputSignals.done != nil
	termInputSignals.Unlock()
	if signalsInstalled {
		t.Fatal("sequential lifecycle left termio signal handlers installed")
	}
}

func TestWithTerminalInputTurnsInvalidUTF8IntoRuntimeErrorAndRestores(t *testing.T) {
	oldOpen := openTermInputDevice
	defer func() { openTermInputDevice = oldOpen }()

	original := syscall.Termios{Lflag: syscall.ECHO | syscall.ICANON}
	device := newFakeTermInputDevice(original)
	openTermInputDevice = func() (termInputDevice, error) { return device, nil }
	callback := &Builtin{Name: "invalid-utf8", Fn: func(e *Evaluator, _ ...Object) Object {
		device.queueRead([]byte{0xff})
		return builtinReadInputEvent(e, &Integer{Value: 1000})
	}}

	result := builtinWithTerminalInput(New(), callback)
	evaluatorErr, ok := result.(*Error)
	if !ok || !strings.Contains(evaluatorErr.Message, "invalid UTF-8") {
		t.Fatalf("result = %#v, want invalid UTF-8 runtime error", result)
	}
	mode, _, writes, closed := device.snapshot()
	if mode != original || writes != bracketedPasteEnable+bracketedPasteDisable || !closed {
		t.Fatalf("cleanup after input error incomplete: mode=%#v writes=%q closed=%v", mode, writes, closed)
	}
}
