//go:build darwin || linux || freebsd || openbsd

package evaluator

import (
	"bytes"
	"errors"
	"fmt"
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

type failingControlTermInputDevice struct {
	*fakeTermInputDevice
	control string
	err     error
}

type failingRawModeTermInputDevice struct {
	*fakeTermInputDevice
	err error
}

func (d *failingRawModeTermInputDevice) setTermios(syscall.Termios) error {
	return d.err
}

func (d *failingControlTermInputDevice) Write(p []byte) (int, error) {
	if string(p) == d.control {
		_, _ = d.fakeTermInputDevice.Write(p)
		return 0, d.err
	}
	return d.fakeTermInputDevice.Write(p)
}

const (
	kittySupportReply        = "\x1b[?0u\x1b[?1;2c"
	primaryDeviceAttrsReply  = "\x1b[?1;2c"
	unsupportedInputControls = bracketedPasteEnable + kittyKeyboardQuery + primaryDeviceAttrsQuery + bracketedPasteDisable
	supportedInputControls   = bracketedPasteEnable + kittyKeyboardQuery + primaryDeviceAttrsQuery + kittyKeyboardPush + kittyKeyboardPop + bracketedPasteDisable
)

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

func TestTermInputSessionDoesNotNegotiateKeyboardBeforeRawMode(t *testing.T) {
	base := newFakeTermInputDevice(syscall.Termios{Lflag: syscall.ECHO | syscall.ICANON})
	modeErr := errors.New("raw mode failed")
	device := &failingRawModeTermInputDevice{fakeTermInputDevice: base, err: modeErr}

	err := (&termInputSession{}).start(device)
	if !errors.Is(err, modeErr) {
		t.Fatalf("start error = %v, want raw-mode failure", err)
	}
	_, _, writes, closed := base.snapshot()
	if writes != "" {
		t.Fatalf("terminal controls were written before raw mode succeeded: %q", writes)
	}
	if !closed {
		t.Fatal("terminal device was not closed after raw-mode failure")
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
	device.queueRead([]byte(kittySupportReply))
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
	if writes != supportedInputControls {
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
			device.queueRead([]byte(kittySupportReply))
			openTermInputDevice = func() (termInputDevice, error) { return device, nil }

			result := builtinWithTerminalInput(New(), tc.callback)
			if isError(result) != tc.isError {
				t.Fatalf("result = %#v, want error=%v", result, tc.isError)
			}
			mode, _, writes, closed := device.snapshot()
			if mode != original || writes != supportedInputControls || !closed {
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
	for _, device := range devices {
		device.queueRead([]byte(kittySupportReply))
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
			writes != supportedInputControls || !closed {
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
	device.queueRead([]byte(kittySupportReply))
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
	if mode != original || writes != supportedInputControls || !closed {
		t.Fatalf("cleanup after input error incomplete: mode=%#v writes=%q closed=%v", mode, writes, closed)
	}
}

func TestTermInputSessionConsumesKittyProbeAndPreservesFirstInput(t *testing.T) {
	device := newFakeTermInputDevice(syscall.Termios{})
	device.queueRead([]byte(kittySupportReply + "\x1b[13;2u"))
	session := &termInputSession{}
	if err := session.start(device); err != nil {
		t.Fatalf("start: %v", err)
	}

	event, ok, err := session.wait(t.Context(), time.Second)
	if err != nil || !ok || event != keyInputEvent("shift+enter") {
		t.Fatalf("first event = %#v ok=%v err=%v", event, ok, err)
	}
	if event, ok, err := session.wait(t.Context(), 5*time.Millisecond); err != nil || ok {
		t.Fatalf("probe reply leaked as event: %#v ok=%v err=%v", event, ok, err)
	}
	if err := session.stopSession(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	_, _, writes, _ := device.snapshot()
	if writes != supportedInputControls {
		t.Fatalf("terminal controls = %q", writes)
	}
}

func TestKittyProbeReplyAcrossEveryReadBoundary(t *testing.T) {
	stream := []byte("x" + kittySupportReply)
	for split := 1; split < len(stream); split++ {
		t.Run(fmt.Sprintf("split-%d", split), func(t *testing.T) {
			device := newFakeTermInputDevice(syscall.Termios{})
			device.queueRead(stream[:split])
			device.queueRead(stream[split:])

			supported, input, err := probeKittyKeyboard(device)
			if err != nil {
				t.Fatalf("probe: %v", err)
			}
			if !supported {
				t.Fatal("split Kitty reply was not detected")
			}
			if string(input) != "x" {
				t.Fatalf("preserved input = %q, want %q", input, "x")
			}
		})
	}
}

func TestTermInputSessionUnsupportedKittyFallbackPreservesFirstInput(t *testing.T) {
	device := newFakeTermInputDevice(syscall.Termios{})
	device.queueRead([]byte(primaryDeviceAttrsReply + "x"))
	session := &termInputSession{}
	if err := session.start(device); err != nil {
		t.Fatalf("start: %v", err)
	}

	event, ok, err := session.wait(t.Context(), time.Second)
	if err != nil || !ok || event != keyInputEvent("x") {
		t.Fatalf("first event = %#v ok=%v err=%v", event, ok, err)
	}
	if err := session.stopSession(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	_, _, writes, _ := device.snapshot()
	if writes != unsupportedInputControls {
		t.Fatalf("unsupported terminal controls = %q", writes)
	}
}

func TestTermInputSessionRestoresAfterKeyboardProbeError(t *testing.T) {
	original := syscall.Termios{Iflag: syscall.ICRNL, Lflag: syscall.ECHO | syscall.ICANON}
	device := newFakeTermInputDevice(original)
	probeErr := syscall.EIO
	device.waitErr <- probeErr

	err := (&termInputSession{}).start(device)
	if !errors.Is(err, probeErr) || !strings.Contains(err.Error(), "detect enhanced keyboard protocol") {
		t.Fatalf("start error = %v, want wrapped probe failure", err)
	}
	mode, modes, writes, closed := device.snapshot()
	if mode != original || len(modes) != 2 || !closed {
		t.Fatalf("probe failure was not restored: mode=%#v modes=%d closed=%v", mode, len(modes), closed)
	}
	if writes != unsupportedInputControls {
		t.Fatalf("probe failure controls = %q, want %q", writes, unsupportedInputControls)
	}
}

func TestTermInputSessionRestoresAfterKeyboardEnableFailure(t *testing.T) {
	original := syscall.Termios{Iflag: syscall.ICRNL, Lflag: syscall.ECHO | syscall.ICANON}
	base := newFakeTermInputDevice(original)
	base.queueRead([]byte(kittySupportReply))
	writeErr := errors.New("keyboard control failed")
	device := &failingControlTermInputDevice{
		fakeTermInputDevice: base,
		control:             kittyKeyboardPush,
		err:                 writeErr,
	}

	err := (&termInputSession{}).start(device)
	if !errors.Is(err, writeErr) || !strings.Contains(err.Error(), "enable enhanced keyboard reporting") {
		t.Fatalf("start error = %v, want wrapped keyboard write failure", err)
	}
	mode, modes, writes, closed := base.snapshot()
	if mode != original || len(modes) != 2 || !closed {
		t.Fatalf("partial setup was not restored: mode=%#v modes=%d closed=%v", mode, len(modes), closed)
	}
	wantWrites := bracketedPasteEnable + kittyKeyboardQuery + primaryDeviceAttrsQuery + kittyKeyboardPush + kittyKeyboardPop + bracketedPasteDisable
	if writes != wantWrites {
		t.Fatalf("partial setup controls = %q, want %q", writes, wantWrites)
	}
}

func TestTermInputSessionContinuesCleanupAfterKeyboardPopFailure(t *testing.T) {
	original := syscall.Termios{Iflag: syscall.ICRNL, Lflag: syscall.ECHO | syscall.ICANON}
	base := newFakeTermInputDevice(original)
	base.queueRead([]byte(kittySupportReply))
	popErr := errors.New("keyboard pop failed")
	device := &failingControlTermInputDevice{
		fakeTermInputDevice: base,
		control:             kittyKeyboardPop,
		err:                 popErr,
	}
	session := &termInputSession{}
	if err := session.start(device); err != nil {
		t.Fatalf("start: %v", err)
	}

	err := session.stopSession()
	if !errors.Is(err, popErr) || !strings.Contains(err.Error(), "restore enhanced keyboard mode") {
		t.Fatalf("stop error = %v, want wrapped keyboard pop failure", err)
	}
	mode, modes, writes, closed := base.snapshot()
	if mode != original || len(modes) != 2 || !closed {
		t.Fatalf("cleanup stopped after pop failure: mode=%#v modes=%d closed=%v", mode, len(modes), closed)
	}
	if writes != supportedInputControls {
		t.Fatalf("pop failure controls = %q, want %q", writes, supportedInputControls)
	}
}

func TestTermInputSessionSignalTerminationSupersedesStreamClosure(t *testing.T) {
	device := newFakeTermInputDevice(syscall.Termios{})
	device.queueRead([]byte(kittySupportReply))
	session := &termInputSession{}
	if err := session.start(device); err != nil {
		t.Fatalf("start: %v", err)
	}

	session.beginSignalTermination()
	if err := session.stopSession(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	_, ok, err := session.wait(t.Context(), time.Second)
	if ok || !errors.Is(err, errTermInputSignalTermination) {
		t.Fatalf("wait after signal cleanup returned ok=%v err=%v", ok, err)
	}
}
