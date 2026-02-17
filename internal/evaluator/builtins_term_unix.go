//go:build !windows
// +build !windows

package evaluator

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// readLineFallback reads a full line from stdin using the shared bufio reader (handles spaces in input).
func readLineFallback() ([]byte, error) {
	line, err := getStdinReader().ReadString('\n')
	line = strings.TrimRight(line, "\r\n")
	// If we got data but hit EOF (no trailing newline), treat as success
	if err != nil && line != "" {
		return []byte(line), nil
	}
	return []byte(line), err
}

func getTerminalSize() (int, int) {
	type winsize struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}

	ws := &winsize{}
	_, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		os.Stdout.Fd(),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)),
	)
	if err != 0 || ws.Col == 0 {
		return 80, 24 // fallback
	}
	return int(ws.Col), int(ws.Row)
}

func readPassword() ([]byte, error) {
	// Set terminal to raw mode to hide input
	fd := int(os.Stdin.Fd())

	// Get current terminal settings
	var oldState syscall.Termios
	if _, _, err := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(getTermiosGet()),
		uintptr(unsafe.Pointer(&oldState)),
		0, 0, 0,
	); err != 0 {
		// Fallback: just read a full line
		return readLineFallback()
	}

	// Disable echo
	newState := oldState
	newState.Lflag &^= syscall.ECHO

	if _, _, err := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(getTermiosSet()),
		uintptr(unsafe.Pointer(&newState)),
		0, 0, 0,
	); err != 0 {
		return readLineFallback()
	}

	defer func() {
		// Restore old settings
		_, _, _ = syscall.Syscall6(
			syscall.SYS_IOCTL,
			uintptr(fd),
			uintptr(getTermiosSet()),
			uintptr(unsafe.Pointer(&oldState)),
			0, 0, 0,
		)
	}()

	// Read input
	var pass []byte
	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			break
		}
		if buf[0] == '\n' || buf[0] == '\r' {
			break
		}
		if buf[0] == 127 || buf[0] == 8 { // backspace
			if len(pass) > 0 {
				pass = pass[:len(pass)-1]
			}
			continue
		}
		pass = append(pass, buf[0])
	}

	return pass, nil
}

// runSelect implements interactive select/multiSelect using raw terminal mode
func runSelect(e *Evaluator, question string, options []string, multi bool) ([]int, error) {
	fd := int(os.Stdin.Fd())

	// Get current terminal settings
	var oldState syscall.Termios
	if _, _, err := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(getTermiosGet()),
		uintptr(unsafe.Pointer(&oldState)),
		0, 0, 0,
	); err != 0 {
		// Fallback: simple numbered selection
		return fallbackSelect(e, question, options, multi)
	}

	// Set raw mode: no echo, no canonical mode, read one byte at a time
	newState := oldState
	newState.Lflag &^= syscall.ECHO | syscall.ICANON
	newState.Cc[syscall.VMIN] = 1
	newState.Cc[syscall.VTIME] = 0

	if _, _, err := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(getTermiosSet()),
		uintptr(unsafe.Pointer(&newState)),
		0, 0, 0,
	); err != 0 {
		return fallbackSelect(e, question, options, multi)
	}

	defer func() {
		// Restore
		_, _, _ = syscall.Syscall6(
			syscall.SYS_IOCTL,
			uintptr(fd),
			uintptr(getTermiosSet()),
			uintptr(unsafe.Pointer(&oldState)),
			0, 0, 0,
		)
		_, _ = fmt.Fprint(e.Out, "\033[?25h") // show cursor
	}()

	_, _ = fmt.Fprint(e.Out, "\033[?25l") // hide cursor

	cursor := 0
	selected := make(map[int]bool)

	render := func() {
		// Move back up to the question line and clear everything
		// We printed 1 question line + len(options) option lines = len(options)+1 total
		totalLines := len(options) + 1
		for i := 0; i < totalLines; i++ {
			_, _ = fmt.Fprint(e.Out, "\033[1A") // move up
			_, _ = fmt.Fprint(e.Out, "\033[2K") // clear line
		}
		_, _ = fmt.Fprint(e.Out, "\r")

		// Question
		if getColorLevel() > 0 {
			_, _ = fmt.Fprintf(e.Out, "\033[1m%s\033[22m\n", question)
		} else {
			_, _ = fmt.Fprintf(e.Out, "%s\n", question)
		}

		// Options
		for i, opt := range options {
			if multi {
				check := "  "
				if selected[i] {
					if getColorLevel() > 0 {
						check = "\033[32m✓\033[39m "
					} else {
						check = "x "
					}
				}
				if i == cursor {
					if getColorLevel() > 0 {
						_, _ = fmt.Fprintf(e.Out, "  \033[36m❯\033[39m %s%s\n", check, opt)
					} else {
						_, _ = fmt.Fprintf(e.Out, "  > %s%s\n", check, opt)
					}
				} else {
					_, _ = fmt.Fprintf(e.Out, "    %s%s\n", check, opt)
				}
			} else {
				if i == cursor {
					if getColorLevel() > 0 {
						_, _ = fmt.Fprintf(e.Out, "  \033[36m❯\033[39m %s\n", opt)
					} else {
						_, _ = fmt.Fprintf(e.Out, "  > %s\n", opt)
					}
				} else {
					_, _ = fmt.Fprintf(e.Out, "    %s\n", opt)
				}
			}
		}
	}

	// Initial render — reserve space, then let render() fill it in
	for i := 0; i < len(options)+1; i++ {
		_, _ = fmt.Fprintln(e.Out)
	}
	render()

	buf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			break
		}

		switch {
		case buf[0] == '\n' || buf[0] == '\r': // Enter
			if multi {
				result := make([]int, 0)
				for i := 0; i < len(options); i++ {
					if selected[i] {
						result = append(result, i)
					}
				}
				// Clear selection UI
				clearSelectUI(e, len(options)+1)
				return result, nil
			}
			clearSelectUI(e, len(options)+1)
			return []int{cursor}, nil

		case buf[0] == ' ' && multi: // Space toggles in multi mode
			selected[cursor] = !selected[cursor]
			render()

		case buf[0] == 'q' || buf[0] == 3: // q or Ctrl+C
			clearSelectUI(e, len(options)+1)
			if multi {
				return []int{}, nil
			}
			return []int{cursor}, nil

		case buf[0] == 'k' || (n >= 3 && buf[0] == 27 && buf[1] == '[' && buf[2] == 'A'): // Up
			cursor--
			if cursor < 0 {
				cursor = len(options) - 1
			}
			render()

		case buf[0] == 'j' || (n >= 3 && buf[0] == 27 && buf[1] == '[' && buf[2] == 'B'): // Down
			cursor++
			if cursor >= len(options) {
				cursor = 0
			}
			render()
		}
	}

	return []int{cursor}, nil
}

func clearSelectUI(e *Evaluator, lines int) {
	for i := 0; i < lines; i++ {
		_, _ = fmt.Fprint(e.Out, "\033[1A") // move up
		_, _ = fmt.Fprint(e.Out, "\033[2K") // clear line
	}
	_, _ = fmt.Fprint(e.Out, "\r")
}

func fallbackSelect(e *Evaluator, question string, options []string, multi bool) ([]int, error) {
	_, _ = fmt.Fprintln(e.Out, question)
	for i, opt := range options {
		_, _ = fmt.Fprintf(e.Out, "  %d) %s\n", i+1, opt)
	}

	reader := getStdinReader()

	if multi {
		_, _ = fmt.Fprint(e.Out, "Enter numbers (comma-separated): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimRight(input, "\r\n")
		result := []int{}
		for _, part := range splitString(input, ',') {
			part = strings.TrimSpace(part)
			var n int
			if _, err := fmt.Sscanf(part, "%d", &n); err == nil && n >= 1 && n <= len(options) {
				result = append(result, n-1)
			}
		}
		return result, nil
	}

	_, _ = fmt.Fprint(e.Out, "Enter number: ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimRight(input, "\r\n")
	input = strings.TrimSpace(input)
	var n int
	if _, err := fmt.Sscanf(input, "%d", &n); err == nil && n >= 1 && n <= len(options) {
		return []int{n - 1}, nil
	}
	return []int{0}, nil
}

func splitString(s string, sep byte) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// =============================================================================
// Raw mode & readKey
// =============================================================================

var (
	rawMu          sync.Mutex
	rawModeActive  bool
	rawOldTermios  syscall.Termios
	rawKeyChannel  chan string
	rawStopChannel chan struct{}
)

// enterRawMode puts the terminal into raw mode and starts a background key reader.
func enterRawMode() error {
	rawMu.Lock()
	defer rawMu.Unlock()

	if rawModeActive {
		return nil
	}

	fd := int(os.Stdin.Fd())

	// Save current terminal settings
	if _, _, err := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(getTermiosGet()),
		uintptr(unsafe.Pointer(&rawOldTermios)),
		0, 0, 0,
	); err != 0 {
		return fmt.Errorf("failed to get terminal settings")
	}

	// Set raw mode:
	// - No echo, no canonical mode, no signal generation
	// - No input processing (IXON, ICRNL)
	// - VMIN=0, VTIME=1: read returns after 100ms even with no input
	newState := rawOldTermios
	newState.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.ISIG
	newState.Iflag &^= syscall.IXON | syscall.ICRNL
	newState.Cc[syscall.VMIN] = 0
	newState.Cc[syscall.VTIME] = 1 // 100ms timeout

	if _, _, err := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(getTermiosSet()),
		uintptr(unsafe.Pointer(&newState)),
		0, 0, 0,
	); err != 0 {
		return fmt.Errorf("failed to set raw mode")
	}

	rawKeyChannel = make(chan string, 32)
	rawStopChannel = make(chan struct{})
	rawModeActive = true

	go keyReaderLoop(rawKeyChannel, rawStopChannel)

	return nil
}

// exitRawMode restores the terminal to its original state.
func exitRawMode() {
	rawMu.Lock()
	defer rawMu.Unlock()

	if !rawModeActive {
		return
	}

	rawModeActive = false
	close(rawStopChannel)

	fd := int(os.Stdin.Fd())
	_, _, _ = syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(getTermiosSet()),
		uintptr(unsafe.Pointer(&rawOldTermios)),
		0, 0, 0,
	)
}

// keyReaderLoop runs in a goroutine, reading raw bytes from stdin and sending
// parsed key names to the channel. Exits when stop is closed.
func keyReaderLoop(ch chan<- string, stop <-chan struct{}) {
	buf := make([]byte, 16)
	for {
		select {
		case <-stop:
			return
		default:
		}

		n, err := os.Stdin.Read(buf)
		if n == 0 {
			if err == io.EOF {
				// VMIN=0 timeout — no data, check stop and continue
				continue
			}
			if err != nil {
				return
			}
			continue
		}

		key := mapKeyBytes(buf[:n])
		if key != "" {
			select {
			case ch <- key:
			case <-stop:
				return
			}
		}
	}
}

// readKeyImpl reads a single key with a timeout in milliseconds.
// Returns "" if no key is pressed within the timeout.
// If timeoutMs <= 0, returns immediately (non-blocking).
func readKeyImpl(timeoutMs int) string {
	rawMu.Lock()
	active := rawModeActive
	rawMu.Unlock()

	if !active {
		return ""
	}

	if timeoutMs <= 0 {
		select {
		case key := <-rawKeyChannel:
			return key
		default:
			return ""
		}
	}

	select {
	case key := <-rawKeyChannel:
		return key
	case <-time.After(time.Duration(timeoutMs) * time.Millisecond):
		return ""
	}
}

// mapKeyBytes converts raw terminal bytes into a human-readable key name.
func mapKeyBytes(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	// Escape sequences
	if data[0] == 27 {
		if len(data) == 1 {
			return "escape"
		}
		if len(data) >= 3 && data[1] == '[' {
			switch data[2] {
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
			}
			// Extended sequences: \x1b[N~ (delete, pgup, pgdown, etc.)
			if len(data) >= 4 && data[3] == '~' {
				switch data[2] {
				case '2':
					return "insert"
				case '3':
					return "delete"
				case '5':
					return "pageup"
				case '6':
					return "pagedown"
				}
			}
		}
		// SS3 sequences: \x1bOA etc (some terminals send these for arrows)
		if len(data) >= 3 && data[1] == 'O' {
			switch data[2] {
			case 'A':
				return "up"
			case 'B':
				return "down"
			case 'C':
				return "right"
			case 'D':
				return "left"
			}
		}
		return "escape"
	}

	// Control characters
	switch data[0] {
	case '\n', '\r':
		return "enter"
	case '\t':
		return "tab"
	case 127, 8:
		return "backspace"
	case ' ':
		return "space"
	case 3:
		return "ctrl+c"
	case 4:
		return "ctrl+d"
	case 26:
		return "ctrl+z"
	}

	// Printable ASCII
	if data[0] >= 32 && data[0] < 127 {
		return string(data[0])
	}

	// Multi-byte UTF-8
	return string(data)
}
