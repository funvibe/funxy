//go:build darwin || linux || freebsd || openbsd

package evaluator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	bracketedPasteEnable      = "\x1b[?2004h"
	bracketedPasteDisable     = "\x1b[?2004l"
	kittyKeyboardQuery        = "\x1b[?u"
	primaryDeviceAttrsQuery   = "\x1b[c"
	kittyKeyboardPush         = "\x1b[>24u" // report all keys (8) + associated text (16)
	kittyKeyboardPop          = "\x1b[<u"
	kittyKeyboardProbeTimeout = 100 * time.Millisecond
)

type termInputDevice interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
	waitReadable(time.Duration) (bool, error)
	getTermios() (syscall.Termios, error)
	setTermios(syscall.Termios) error
}

type unixTermInputDevice struct {
	file *os.File
}

func (d *unixTermInputDevice) Read(p []byte) (int, error)  { return d.file.Read(p) }
func (d *unixTermInputDevice) Write(p []byte) (int, error) { return d.file.Write(p) }
func (d *unixTermInputDevice) Close() error                { return d.file.Close() }

func (d *unixTermInputDevice) waitReadable(timeout time.Duration) (bool, error) {
	fd := int(d.file.Fd())
	if fd < 0 || fd >= unix.FD_SETSIZE {
		return false, fmt.Errorf("terminal descriptor %d is outside select range", fd)
	}
	for {
		var reads unix.FdSet
		reads.Set(fd)
		timeval := unix.NsecToTimeval(timeout.Nanoseconds())
		ready, err := unix.Select(fd+1, &reads, nil, nil, &timeval)
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return false, err
		}
		if ready == 0 {
			return false, nil
		}
		if reads.IsSet(fd) {
			return true, nil
		}
		return false, fmt.Errorf("select reported readiness without terminal descriptor")
	}
}

func (d *unixTermInputDevice) getTermios() (syscall.Termios, error) {
	var state syscall.Termios
	if _, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		d.file.Fd(),
		uintptr(getTermiosGet()),
		uintptr(unsafe.Pointer(&state)),
		0, 0, 0,
	); errno != 0 {
		return syscall.Termios{}, errno
	}
	return state, nil
}

func (d *unixTermInputDevice) setTermios(state syscall.Termios) error {
	if _, _, errno := syscall.Syscall6(
		syscall.SYS_IOCTL,
		d.file.Fd(),
		uintptr(getTermiosSet()),
		uintptr(unsafe.Pointer(&state)),
		0, 0, 0,
	); errno != 0 {
		return errno
	}
	return nil
}

type termInputSession struct {
	mu sync.Mutex

	active   bool
	stopping chan struct{}
	stopErr  error
	readErr  error

	signalTerminating bool
	signalExit        chan struct{}

	device              termInputDevice
	oldMode             syscall.Termios
	kittyKeyboardActive bool
	events              chan termInputEvent
	stop                chan struct{}
	done                chan struct{}
}

var defaultTermInputSession termInputSession

var openTermInputDevice = func() (termInputDevice, error) {
	file, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	return &unixTermInputDevice{file: file}, nil
}

func makeRawTermios(old syscall.Termios) syscall.Termios {
	raw := old
	raw.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.INPCK | syscall.ISTRIP |
		syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON
	raw.Oflag &^= syscall.OPOST
	raw.Cflag &^= syscall.CSIZE | syscall.PARENB
	raw.Cflag |= syscall.CS8
	raw.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.IEXTEN | syscall.ISIG
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	return raw
}

func (s *termInputSession) start(device termInputDevice) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.active || s.stopping != nil {
		_ = device.Close()
		return fmt.Errorf("terminal input session is already active")
	}

	oldMode, err := device.getTermios()
	if err != nil {
		_ = device.Close()
		return fmt.Errorf("get terminal mode: %w", err)
	}
	if err := device.setTermios(makeRawTermios(oldMode)); err != nil {
		_ = device.Close()
		return fmt.Errorf("set raw terminal mode: %w", err)
	}
	if err := writeTermInputControl(device, bracketedPasteEnable); err != nil {
		disableErr := writeTermInputControl(device, bracketedPasteDisable)
		restoreErr := device.setTermios(oldMode)
		closeErr := device.Close()
		return errors.Join(
			fmt.Errorf("enable bracketed paste: %w", err),
			wrapTermInputError("disable bracketed paste", disableErr),
			wrapTermInputError("restore terminal mode", restoreErr),
			wrapTermInputError("close terminal", closeErr),
		)
	}

	kittySupported, initialInput, err := probeKittyKeyboard(device)
	if err != nil {
		disableErr := writeTermInputControl(device, bracketedPasteDisable)
		restoreErr := device.setTermios(oldMode)
		closeErr := device.Close()
		return errors.Join(
			fmt.Errorf("detect enhanced keyboard protocol: %w", err),
			wrapTermInputError("disable bracketed paste", disableErr),
			wrapTermInputError("restore terminal mode", restoreErr),
			wrapTermInputError("close terminal", closeErr),
		)
	}
	if kittySupported {
		if err := writeTermInputControl(device, kittyKeyboardPush); err != nil {
			// A failed write can still have delivered a complete push before
			// returning its error, so always attempt a balancing pop.
			popErr := writeTermInputControl(device, kittyKeyboardPop)
			disableErr := writeTermInputControl(device, bracketedPasteDisable)
			restoreErr := device.setTermios(oldMode)
			closeErr := device.Close()
			return errors.Join(
				fmt.Errorf("enable enhanced keyboard reporting: %w", err),
				wrapTermInputError("restore enhanced keyboard mode", popErr),
				wrapTermInputError("disable bracketed paste", disableErr),
				wrapTermInputError("restore terminal mode", restoreErr),
				wrapTermInputError("close terminal", closeErr),
			)
		}
	}

	s.active = true
	s.stopErr = nil
	s.readErr = nil
	s.signalTerminating = false
	s.signalExit = make(chan struct{})
	s.device = device
	s.oldMode = oldMode
	s.kittyKeyboardActive = kittySupported
	s.events = make(chan termInputEvent, 64)
	s.stop = make(chan struct{})
	s.done = make(chan struct{})

	go s.readLoop(device, initialInput, s.events, s.stop, s.done)
	return nil
}

func (s *termInputSession) readLoop(device termInputDevice, initialInput []byte, events chan<- termInputEvent, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	defer close(events)

	parser := &termInputParser{}
	if !sendTermInputEvents(events, stop, parser.feed(initialInput)) {
		return
	}
	buf := make([]byte, 4096)
	for {
		select {
		case <-stop:
			return
		default:
		}

		ready, err := device.waitReadable(100 * time.Millisecond)
		if err != nil {
			select {
			case <-stop:
				return
			default:
			}
			s.setReadError(fmt.Errorf("wait for terminal input: %w", err))
			return
		}
		if !ready {
			if !sendTermInputEvents(events, stop, parser.flushEscape()) {
				return
			}
			continue
		}

		n, err := device.Read(buf)
		if n > 0 {
			if !sendTermInputEvents(events, stop, parser.feed(buf[:n])) {
				return
			}
		}
		if err != nil {
			select {
			case <-stop:
				return
			default:
			}
			s.setReadError(fmt.Errorf("read terminal input: %w", err))
			return
		}
		if n == 0 {
			s.setReadError(fmt.Errorf("read terminal input: %w", io.ErrNoProgress))
			return
		}
	}
}

func (s *termInputSession) setReadError(err error) {
	s.mu.Lock()
	if s.readErr == nil {
		s.readErr = err
	}
	s.mu.Unlock()
}

func sendTermInputEvents(ch chan<- termInputEvent, stop <-chan struct{}, events []termInputEvent) bool {
	for _, event := range events {
		select {
		case ch <- event:
		case <-stop:
			return false
		}
		if event.kind == termInputError {
			return false
		}
	}
	return true
}

func (s *termInputSession) stopSession() error {
	s.mu.Lock()
	if !s.active {
		stopping := s.stopping
		err := s.stopErr
		s.mu.Unlock()
		if stopping != nil {
			<-stopping
			s.mu.Lock()
			err = s.stopErr
			s.mu.Unlock()
		}
		return err
	}

	s.active = false
	s.stopping = make(chan struct{})
	stopping := s.stopping
	device := s.device
	oldMode := s.oldMode
	kittyKeyboardActive := s.kittyKeyboardActive
	stop := s.stop
	done := s.done
	close(stop)
	s.mu.Unlock()

	<-done
	var keyboardErr error
	if kittyKeyboardActive {
		keyboardErr = writeTermInputControl(device, kittyKeyboardPop)
	}
	disableErr := writeTermInputControl(device, bracketedPasteDisable)
	restoreErr := device.setTermios(oldMode)
	closeErr := device.Close()
	cleanupErr := errors.Join(
		wrapTermInputError("restore enhanced keyboard mode", keyboardErr),
		wrapTermInputError("disable bracketed paste", disableErr),
		wrapTermInputError("restore terminal mode", restoreErr),
		wrapTermInputError("close terminal", closeErr),
	)

	s.mu.Lock()
	s.device = nil
	s.kittyKeyboardActive = false
	s.signalExit = nil
	s.events = nil
	s.stop = nil
	s.done = nil
	s.stopErr = cleanupErr
	close(stopping)
	s.stopping = nil
	s.mu.Unlock()

	return cleanupErr
}

func wrapTermInputError(action string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", action, err)
}

// probeKittyKeyboard follows the protocol's capability query with a primary
// device-attributes query. A keyboard reply received before the DA reply means
// the protocol is supported. Protocol replies are consumed here; unrelated
// bytes are returned to the normal stream parser so a racing key press cannot
// be lost or exposed late as a fake reply event.
func probeKittyKeyboard(device termInputDevice) (bool, []byte, error) {
	if err := writeTermInputControl(device, kittyKeyboardQuery+primaryDeviceAttrsQuery); err != nil {
		return false, nil, err
	}

	deadline := time.Now().Add(kittyKeyboardProbeTimeout)
	var pending []byte
	var input []byte
	keyboardReply := false
	buf := make([]byte, 256)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			input = append(input, pending...)
			return keyboardReply, input, nil
		}
		ready, err := device.waitReadable(remaining)
		if err != nil {
			return false, nil, err
		}
		if !ready {
			input = append(input, pending...)
			return keyboardReply, input, nil
		}

		n, readErr := device.Read(buf)
		if n > 0 {
			pending = append(pending, buf[:n]...)
			var preserved []byte
			var gotKeyboard, gotDeviceAttrs bool
			pending, preserved, gotKeyboard, gotDeviceAttrs = consumeKittyProbeResponses(pending)
			input = append(input, preserved...)
			keyboardReply = keyboardReply || gotKeyboard
			if readErr != nil {
				return false, nil, readErr
			}
			if gotDeviceAttrs {
				input = append(input, pending...)
				return keyboardReply, input, nil
			}
		}
		if readErr != nil {
			return false, nil, readErr
		}
		if n == 0 {
			return false, nil, io.ErrNoProgress
		}
	}
}

func consumeKittyProbeResponses(data []byte) (pending, preserved []byte, keyboardReply, deviceAttrs bool) {
	csiPrefix := []byte("\x1b[")
	for len(data) > 0 {
		start := bytes.Index(data, csiPrefix)
		if start < 0 {
			keep := longestSuffixPrefix(data, csiPrefix)
			preserved = append(preserved, data[:len(data)-keep]...)
			return data[len(data)-keep:], preserved, keyboardReply, deviceAttrs
		}
		preserved = append(preserved, data[:start]...)
		data = data[start:]
		end := csiSequenceEnd(data)
		if end < 0 {
			return data, preserved, keyboardReply, deviceAttrs
		}

		sequence := data[:end]
		switch {
		case isKittyKeyboardReply(sequence):
			keyboardReply = true
		case isPrimaryDeviceAttrsReply(sequence):
			deviceAttrs = true
		default:
			preserved = append(preserved, sequence...)
		}
		data = data[end:]
	}
	return nil, preserved, keyboardReply, deviceAttrs
}

func isKittyKeyboardReply(sequence []byte) bool {
	if len(sequence) < 5 || !bytes.HasPrefix(sequence, []byte("\x1b[?")) || sequence[len(sequence)-1] != 'u' {
		return false
	}
	return decimalCSIParams(sequence[3 : len(sequence)-1])
}

func isPrimaryDeviceAttrsReply(sequence []byte) bool {
	if len(sequence) < 5 || !bytes.HasPrefix(sequence, []byte("\x1b[?")) || sequence[len(sequence)-1] != 'c' {
		return false
	}
	return decimalCSIParams(sequence[3 : len(sequence)-1])
}

func decimalCSIParams(params []byte) bool {
	if len(params) == 0 {
		return false
	}
	for _, field := range bytes.Split(params, []byte{';'}) {
		if len(field) == 0 {
			return false
		}
		for _, b := range field {
			if b < '0' || b > '9' {
				return false
			}
		}
	}
	return true
}

func (s *termInputSession) wait(ctx context.Context, timeout time.Duration) (termInputEvent, bool, error) {
	s.mu.Lock()
	if s.signalTerminating {
		s.mu.Unlock()
		return termInputEvent{}, false, errTermInputSignalTermination
	}
	if !s.active {
		s.mu.Unlock()
		return termInputEvent{}, false, fmt.Errorf("no active terminal input session; call inside withTerminalInput")
	}
	events := s.events
	signalExit := s.signalExit
	s.mu.Unlock()

	receive := func(event termInputEvent, ok bool) (termInputEvent, bool, error) {
		s.mu.Lock()
		signalTerminating := s.signalTerminating
		s.mu.Unlock()
		if signalTerminating {
			return termInputEvent{}, false, errTermInputSignalTermination
		}
		if ok {
			if event.kind == termInputError {
				return termInputEvent{}, false, errors.New(event.value)
			}
			return event, true, nil
		}
		s.mu.Lock()
		err := s.readErr
		s.mu.Unlock()
		if err == nil {
			err = fmt.Errorf("terminal input stream closed")
		}
		return termInputEvent{}, false, err
	}

	if timeout <= 0 {
		select {
		case <-signalExit:
			return termInputEvent{}, false, errTermInputSignalTermination
		case event, ok := <-events:
			return receive(event, ok)
		case <-ctx.Done():
			return termInputEvent{}, false, ctx.Err()
		default:
			return termInputEvent{}, false, nil
		}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-signalExit:
		return termInputEvent{}, false, errTermInputSignalTermination
	case event, ok := <-events:
		return receive(event, ok)
	case <-timer.C:
		return termInputEvent{}, false, nil
	case <-ctx.Done():
		return termInputEvent{}, false, ctx.Err()
	}
}

func (s *termInputSession) beginSignalTermination() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.signalTerminating {
		return
	}
	s.signalTerminating = true
	if s.signalExit != nil {
		close(s.signalExit)
	}
}

func writeTermInputControl(device termInputDevice, control string) error {
	data := []byte(control)
	for len(data) > 0 {
		n, err := device.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func startTermInputSession() error {
	rawMu.Lock()
	defer rawMu.Unlock()
	if rawModeActive {
		return fmt.Errorf("lib/term raw input is already active")
	}

	// Install signal cleanup before changing terminal state. The handler waits
	// for rawMu, so a signal racing with startup can only clean up after start
	// has either completed or failed.
	installTermInputSignalCleanup()

	device, err := openTermInputDevice()
	if err != nil {
		uninstallTermInputSignalCleanup()
		return fmt.Errorf("open /dev/tty: %w", err)
	}
	if err := defaultTermInputSession.start(device); err != nil {
		uninstallTermInputSignalCleanup()
		return err
	}
	return nil
}

func stopTermInputSession() error {
	err := defaultTermInputSession.stopSession()
	uninstallTermInputSignalCleanup()
	return err
}

func waitTermInputEvent(ctx context.Context, timeout time.Duration) (termInputEvent, bool, error) {
	return defaultTermInputSession.wait(ctx, timeout)
}

func termInputSessionActive() bool {
	defaultTermInputSession.mu.Lock()
	defer defaultTermInputSession.mu.Unlock()
	return defaultTermInputSession.active || defaultTermInputSession.stopping != nil
}

var termInputSignals struct {
	sync.Mutex
	channel chan os.Signal
	done    chan struct{}
}

func installTermInputSignalCleanup() {
	termInputSignals.Lock()
	defer termInputSignals.Unlock()
	if termInputSignals.done != nil {
		return
	}

	termInputSignals.channel = make(chan os.Signal, 1)
	termInputSignals.done = make(chan struct{})
	signal.Notify(termInputSignals.channel, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	channel := termInputSignals.channel
	done := termInputSignals.done
	go func() {
		select {
		case sig := <-channel:
			// Startup holds rawMu across signal-handler installation and terminal
			// activation. Waiting here closes the only window where a signal could
			// otherwise exit after raw mode was set but before cleanup was ready.
			rawMu.Lock()
			rawMu.Unlock()
			defaultTermInputSession.beginSignalTermination()
			_ = stopTermInputSession()
			if unixSignal, ok := sig.(syscall.Signal); ok {
				// Restore the default disposition and re-deliver the same signal so
				// supervisors observe a signal termination, not a synthesized code.
				signal.Reset(unixSignal)
				if err := syscall.Kill(os.Getpid(), unixSignal); err == nil {
					select {}
				}
				os.Exit(128 + int(unixSignal))
			}
			os.Exit(1)
		case <-done:
		}
	}()
}

func uninstallTermInputSignalCleanup() {
	termInputSignals.Lock()
	defer termInputSignals.Unlock()
	if termInputSignals.done == nil {
		return
	}
	signal.Stop(termInputSignals.channel)
	close(termInputSignals.done)
	termInputSignals.channel = nil
	termInputSignals.done = nil
}
