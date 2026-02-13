//go:build windows
// +build windows

package evaluator

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
	procSetConsoleMode             = kernel32.NewProc("SetConsoleMode")
	procGetConsoleMode             = kernel32.NewProc("GetConsoleMode")
)

type coord struct {
	X int16
	Y int16
}

type smallRect struct {
	Left   int16
	Top    int16
	Right  int16
	Bottom int16
}

type consoleScreenBufferInfo struct {
	Size              coord
	CursorPosition    coord
	Attributes        uint16
	Window            smallRect
	MaximumWindowSize coord
}

func getTerminalSize() (int, int) {
	handle, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	if err != nil {
		return 80, 24
	}

	var info consoleScreenBufferInfo
	r, _, _ := procGetConsoleScreenBufferInfo.Call(uintptr(handle), uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		return 80, 24
	}

	cols := int(info.Window.Right-info.Window.Left) + 1
	rows := int(info.Window.Bottom-info.Window.Top) + 1
	return cols, rows
}

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

func readPassword() ([]byte, error) {
	handle, err := syscall.GetStdHandle(syscall.STD_INPUT_HANDLE)
	if err != nil {
		return readLineFallback()
	}

	var mode uint32
	r, _, _ := procGetConsoleMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&mode)))
	if r == 0 {
		return readLineFallback()
	}

	// Disable echo
	const enableEchoInput = 0x0004
	newMode := mode &^ enableEchoInput
	procSetConsoleMode.Call(uintptr(handle), uintptr(newMode))

	defer func() {
		procSetConsoleMode.Call(uintptr(handle), uintptr(mode))
	}()

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
		if buf[0] == 8 { // backspace
			if len(pass) > 0 {
				pass = pass[:len(pass)-1]
			}
			continue
		}
		pass = append(pass, buf[0])
	}

	return pass, nil
}

// runSelect on Windows uses fallback numbered selection
func runSelect(e *Evaluator, question string, options []string, multi bool) ([]int, error) {
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
		for _, part := range splitStringWin(input, ',') {
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

func splitStringWin(s string, sep byte) []string {
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
