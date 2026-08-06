//go:build darwin || linux || freebsd || openbsd

package evaluator

import (
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
	bracketedPasteEnable  = "\x1b[?2004h"
	bracketedPasteDisable = "\x1b[?2004l"
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

	device  termInputDevice
	oldMode syscall.Termios
	events  chan termInputEvent
	stop    chan struct{}
	done    chan struct{}
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
		_ = device.Close()
		return errors.Join(fmt.Errorf("enable bracketed paste: %w", err), disableErr, restoreErr)
	}

	s.active = true
	s.stopErr = nil
	s.readErr = nil
	s.device = device
	s.oldMode = oldMode
	s.events = make(chan termInputEvent, 64)
	s.stop = make(chan struct{})
	s.done = make(chan struct{})

	go s.readLoop(device, s.events, s.stop, s.done)
	return nil
}

func (s *termInputSession) readLoop(device termInputDevice, events chan<- termInputEvent, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	defer close(events)

	parser := &termInputParser{}
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
	stop := s.stop
	done := s.done
	close(stop)
	s.mu.Unlock()

	<-done
	disableErr := writeTermInputControl(device, bracketedPasteDisable)
	restoreErr := device.setTermios(oldMode)
	closeErr := device.Close()
	cleanupErr := errors.Join(disableErr, restoreErr, closeErr)

	s.mu.Lock()
	s.device = nil
	s.events = nil
	s.stop = nil
	s.done = nil
	s.stopErr = cleanupErr
	close(stopping)
	s.stopping = nil
	s.mu.Unlock()

	return cleanupErr
}

func (s *termInputSession) wait(ctx context.Context, timeout time.Duration) (termInputEvent, bool, error) {
	s.mu.Lock()
	if !s.active {
		s.mu.Unlock()
		return termInputEvent{}, false, fmt.Errorf("no active terminal input session; call inside withTerminalInput")
	}
	events := s.events
	s.mu.Unlock()

	receive := func(event termInputEvent, ok bool) (termInputEvent, bool, error) {
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
	case event, ok := <-events:
		return receive(event, ok)
	case <-timer.C:
		return termInputEvent{}, false, nil
	case <-ctx.Done():
		return termInputEvent{}, false, ctx.Err()
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
