package evaluator

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/funvibe/funxy/internal/typesystem"

	"github.com/mattn/go-isatty"
)

// =============================================================================
// Output buffering (double-buffering for flicker-free rendering)
// =============================================================================

var (
	termBufMu     sync.Mutex
	termBuf       bytes.Buffer
	termRealOut   io.Writer
	termBuffering bool
)

func builtinTermBufferStart(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("termBufferStart expects 0 arguments, got %d", len(args))
	}
	termBufMu.Lock()
	defer termBufMu.Unlock()
	if !termBuffering {
		termRealOut = e.Out
		termBuffering = true
	}
	termBuf.Reset()
	e.Out = &termBuf
	return &Nil{}
}

func builtinTermBufferFlush(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("termBufferFlush expects 0 arguments, got %d", len(args))
	}
	termBufMu.Lock()
	defer termBufMu.Unlock()
	if termBuffering && termRealOut != nil {
		_, _ = termRealOut.Write(termBuf.Bytes())
		termBuf.Reset()
		e.Out = termRealOut
		termBuffering = false
	}
	return &Nil{}
}

// =============================================================================
// Color support detection
// =============================================================================

// colorLevel caches the detected color support: 0=none, 1=basic(16), 256=256colors, 16777216=truecolor
var (
	colorLevelOnce sync.Once
	colorLevelVal  int
)

func detectColorLevel() int {
	// NO_COLOR convention: https://no-color.org/
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return 0
	}

	// Not a terminal
	if !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		return 0
	}

	term := os.Getenv("TERM")
	if term == "dumb" {
		return 0
	}

	// Truecolor detection
	colorTerm := os.Getenv("COLORTERM")
	if colorTerm == "truecolor" || colorTerm == "24bit" {
		return 16777216
	}

	// 256-color detection
	if strings.Contains(term, "256color") {
		return 256
	}

	// Basic color support
	return 1
}

func getColorLevel() int {
	colorLevelOnce.Do(func() {
		colorLevelVal = detectColorLevel()
	})
	return colorLevelVal
}

// =============================================================================
// ANSI escape code helpers
// =============================================================================

func ansiWrap(code, resetCode, s string) string {
	if getColorLevel() == 0 {
		return s
	}
	return code + s + resetCode
}

func ansiFg(colorCode int, s string) string {
	return ansiWrap(fmt.Sprintf("\033[%dm", colorCode), "\033[39m", s)
}

func ansiBg(colorCode int, s string) string {
	return ansiWrap(fmt.Sprintf("\033[%dm", colorCode), "\033[49m", s)
}

func ansiStyle(styleCode int, resetCode int, s string) string {
	return ansiWrap(fmt.Sprintf("\033[%dm", styleCode), fmt.Sprintf("\033[%dm", resetCode), s)
}

// =============================================================================
// Phase 1: Styles & Colors
// =============================================================================

// --- Text styles ---

func builtinBold(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bold expects 1 argument, got %d", len(args))
	}
	s := objectToDisplayString(args[0])
	return stringToList(ansiStyle(1, 22, s))
}

func builtinDim(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dim expects 1 argument, got %d", len(args))
	}
	s := objectToDisplayString(args[0])
	return stringToList(ansiStyle(2, 22, s))
}

func builtinItalic(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("italic expects 1 argument, got %d", len(args))
	}
	s := objectToDisplayString(args[0])
	return stringToList(ansiStyle(3, 23, s))
}

func builtinUnderline(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("underline expects 1 argument, got %d", len(args))
	}
	s := objectToDisplayString(args[0])
	return stringToList(ansiStyle(4, 24, s))
}

func builtinStrikethrough(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("strikethrough expects 1 argument, got %d", len(args))
	}
	s := objectToDisplayString(args[0])
	return stringToList(ansiStyle(9, 29, s))
}

// --- Foreground colors ---

func builtinTermRed(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("red expects 1 argument, got %d", len(args))
	}
	return stringToList(ansiFg(31, objectToDisplayString(args[0])))
}

func builtinTermGreen(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("green expects 1 argument, got %d", len(args))
	}
	return stringToList(ansiFg(32, objectToDisplayString(args[0])))
}

func builtinTermYellow(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("yellow expects 1 argument, got %d", len(args))
	}
	return stringToList(ansiFg(33, objectToDisplayString(args[0])))
}

func builtinTermBlue(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("blue expects 1 argument, got %d", len(args))
	}
	return stringToList(ansiFg(34, objectToDisplayString(args[0])))
}

func builtinTermMagenta(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("magenta expects 1 argument, got %d", len(args))
	}
	return stringToList(ansiFg(35, objectToDisplayString(args[0])))
}

func builtinTermCyan(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("cyan expects 1 argument, got %d", len(args))
	}
	return stringToList(ansiFg(36, objectToDisplayString(args[0])))
}

func builtinTermWhite(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("white expects 1 argument, got %d", len(args))
	}
	return stringToList(ansiFg(37, objectToDisplayString(args[0])))
}

func builtinTermGray(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("gray expects 1 argument, got %d", len(args))
	}
	return stringToList(ansiFg(90, objectToDisplayString(args[0])))
}

// --- Background colors ---

func builtinBgRed(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bgRed expects 1 argument, got %d", len(args))
	}
	return stringToList(ansiBg(41, objectToDisplayString(args[0])))
}

func builtinBgGreen(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bgGreen expects 1 argument, got %d", len(args))
	}
	return stringToList(ansiBg(42, objectToDisplayString(args[0])))
}

func builtinBgYellow(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bgYellow expects 1 argument, got %d", len(args))
	}
	return stringToList(ansiBg(43, objectToDisplayString(args[0])))
}

func builtinBgBlue(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bgBlue expects 1 argument, got %d", len(args))
	}
	return stringToList(ansiBg(44, objectToDisplayString(args[0])))
}

func builtinBgCyan(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bgCyan expects 1 argument, got %d", len(args))
	}
	return stringToList(ansiBg(46, objectToDisplayString(args[0])))
}

func builtinBgMagenta(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("bgMagenta expects 1 argument, got %d", len(args))
	}
	return stringToList(ansiBg(45, objectToDisplayString(args[0])))
}

// --- RGB / Hex colors ---

func builtinRgb(e *Evaluator, args ...Object) Object {
	if len(args) != 4 {
		return newError("rgb expects 4 arguments (r, g, b, text), got %d", len(args))
	}
	r, ok1 := args[0].(*Integer)
	g, ok2 := args[1].(*Integer)
	b, ok3 := args[2].(*Integer)
	if !ok1 || !ok2 || !ok3 {
		return newError("rgb: first 3 arguments must be Int")
	}
	s := objectToDisplayString(args[3])
	if getColorLevel() < 16777216 {
		return stringToList(s)
	}
	return stringToList(fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[39m", r.Value, g.Value, b.Value, s))
}

func builtinBgRgb(e *Evaluator, args ...Object) Object {
	if len(args) != 4 {
		return newError("bgRgb expects 4 arguments (r, g, b, text), got %d", len(args))
	}
	r, ok1 := args[0].(*Integer)
	g, ok2 := args[1].(*Integer)
	b, ok3 := args[2].(*Integer)
	if !ok1 || !ok2 || !ok3 {
		return newError("bgRgb: first 3 arguments must be Int")
	}
	s := objectToDisplayString(args[3])
	if getColorLevel() < 16777216 {
		return stringToList(s)
	}
	return stringToList(fmt.Sprintf("\033[48;2;%d;%d;%dm%s\033[49m", r.Value, g.Value, b.Value, s))
}

func parseHexColor(hex string) (int, int, int, error) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 3 {
		hex = string(hex[0]) + string(hex[0]) + string(hex[1]) + string(hex[1]) + string(hex[2]) + string(hex[2])
	}
	if len(hex) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid hex color: %s", hex)
	}
	r, err := strconv.ParseInt(hex[0:2], 16, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	g, err := strconv.ParseInt(hex[2:4], 16, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	b, err := strconv.ParseInt(hex[4:6], 16, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	return int(r), int(g), int(b), nil
}

func builtinHex(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("hex expects 2 arguments (color, text), got %d", len(args))
	}
	colorStr := objectToDisplayString(args[0])
	s := objectToDisplayString(args[1])
	if getColorLevel() < 16777216 {
		return stringToList(s)
	}
	r, g, b, err := parseHexColor(colorStr)
	if err != nil {
		return newError("hex: %s", err)
	}
	return stringToList(fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[39m", r, g, b, s))
}

func builtinBgHex(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("bgHex expects 2 arguments (color, text), got %d", len(args))
	}
	colorStr := objectToDisplayString(args[0])
	s := objectToDisplayString(args[1])
	if getColorLevel() < 16777216 {
		return stringToList(s)
	}
	r, g, b, err := parseHexColor(colorStr)
	if err != nil {
		return newError("bgHex: %s", err)
	}
	return stringToList(fmt.Sprintf("\033[48;2;%d;%d;%dm%s\033[49m", r, g, b, s))
}

// --- Utility ---

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func builtinStripAnsi(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("stripAnsi expects 1 argument, got %d", len(args))
	}
	s := objectToDisplayString(args[0])
	return stringToList(ansiRegex.ReplaceAllString(s, ""))
}

func builtinTermColors(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("termColors expects 0 arguments, got %d", len(args))
	}
	return &Integer{Value: int64(getColorLevel())}
}

// --- cprint: print with style ---

func builtinCprint(e *Evaluator, args ...Object) Object {
	if len(args) < 2 {
		return newError("cprint expects at least 2 arguments (styleFn, ...values), got %d", len(args))
	}

	styleFn := args[0]

	for i := 1; i < len(args); i++ {
		if i > 1 {
			_, _ = fmt.Fprint(e.Out, " ")
		}

		// Convert arg to a Funxy string, apply style function, print result
		text := objectToDisplayString(args[i])
		styled := e.ApplyFunction(styleFn, []Object{stringToList(text)})
		if isError(styled) {
			return styled
		}
		_, _ = fmt.Fprint(e.Out, objectToDisplayString(styled))
	}
	_, _ = fmt.Fprintln(e.Out)
	return &Nil{}
}

// =============================================================================
// Phase 2: Terminal control
// =============================================================================

func builtinTermSize(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("termSize expects 0 arguments, got %d", len(args))
	}

	cols, rows := getTerminalSize()
	return &Tuple{Elements: []Object{
		&Integer{Value: int64(cols)},
		&Integer{Value: int64(rows)},
	}}
}

func builtinTermIsTTY(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("termIsTTY expects 0 arguments, got %d", len(args))
	}
	isTTY := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	if isTTY {
		return TRUE
	}
	return FALSE
}

func builtinTermClear(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("termClear expects 0 arguments, got %d", len(args))
	}
	_, _ = fmt.Fprint(e.Out, "\033[2J\033[H")
	return &Nil{}
}

func builtinTermClearLine(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("termClearLine expects 0 arguments, got %d", len(args))
	}
	_, _ = fmt.Fprint(e.Out, "\033[2K\r")
	return &Nil{}
}

func builtinCursorUp(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("cursorUp expects 1 argument, got %d", len(args))
	}
	n, ok := args[0].(*Integer)
	if !ok {
		return newError("cursorUp: argument must be Int")
	}
	_, _ = fmt.Fprintf(e.Out, "\033[%dA", n.Value)
	return &Nil{}
}

func builtinCursorDown(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("cursorDown expects 1 argument, got %d", len(args))
	}
	n, ok := args[0].(*Integer)
	if !ok {
		return newError("cursorDown: argument must be Int")
	}
	_, _ = fmt.Fprintf(e.Out, "\033[%dB", n.Value)
	return &Nil{}
}

func builtinCursorLeft(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("cursorLeft expects 1 argument, got %d", len(args))
	}
	n, ok := args[0].(*Integer)
	if !ok {
		return newError("cursorLeft: argument must be Int")
	}
	_, _ = fmt.Fprintf(e.Out, "\033[%dD", n.Value)
	return &Nil{}
}

func builtinCursorRight(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("cursorRight expects 1 argument, got %d", len(args))
	}
	n, ok := args[0].(*Integer)
	if !ok {
		return newError("cursorRight: argument must be Int")
	}
	_, _ = fmt.Fprintf(e.Out, "\033[%dC", n.Value)
	return &Nil{}
}

func builtinCursorTo(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("cursorTo expects 2 arguments (col, row), got %d", len(args))
	}
	col, ok1 := args[0].(*Integer)
	row, ok2 := args[1].(*Integer)
	if !ok1 || !ok2 {
		return newError("cursorTo: arguments must be Int")
	}
	// ANSI uses 1-based, user provides 0-based
	_, _ = fmt.Fprintf(e.Out, "\033[%d;%dH", row.Value+1, col.Value+1)
	return &Nil{}
}

func builtinCursorHide(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("cursorHide expects 0 arguments, got %d", len(args))
	}
	_, _ = fmt.Fprint(e.Out, "\033[?25l")
	return &Nil{}
}

func builtinCursorShow(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("cursorShow expects 0 arguments, got %d", len(args))
	}
	_, _ = fmt.Fprint(e.Out, "\033[?25h")
	return &Nil{}
}

// =============================================================================
// Phase 3: Interactive prompts
// =============================================================================

func builtinPrompt(e *Evaluator, args ...Object) Object {
	if len(args) < 1 || len(args) > 2 {
		return newError("prompt expects 1-2 arguments (question, default?), got %d", len(args))
	}

	question := objectToDisplayString(args[0])
	defaultVal := ""
	hasDefault := false
	if len(args) == 2 {
		defaultVal = objectToDisplayString(args[1])
		hasDefault = true
	}

	if hasDefault {
		_, _ = fmt.Fprintf(e.Out, "%s [%s]: ", question, defaultVal)
	} else {
		_, _ = fmt.Fprintf(e.Out, "%s: ", question)
	}

	input, _ := getStdinReader().ReadString('\n')
	input = strings.TrimRight(input, "\r\n")
	input = strings.TrimSpace(input)

	if input == "" && hasDefault {
		return stringToList(defaultVal)
	}
	return stringToList(input)
}

func builtinConfirm(e *Evaluator, args ...Object) Object {
	if len(args) < 1 || len(args) > 2 {
		return newError("confirm expects 1-2 arguments (question, default?), got %d", len(args))
	}

	question := objectToDisplayString(args[0])
	defaultYes := true
	if len(args) == 2 {
		b, ok := args[1].(*Boolean)
		if !ok {
			return newError("confirm: default must be Bool")
		}
		defaultYes = b.Value
	}

	hint := "Y/n"
	if !defaultYes {
		hint = "y/N"
	}
	_, _ = fmt.Fprintf(e.Out, "%s [%s]: ", question, hint)

	input, _ := getStdinReader().ReadString('\n')
	input = strings.TrimRight(input, "\r\n")
	input = strings.TrimSpace(strings.ToLower(input))

	result := defaultYes
	if input != "" {
		result = input == "y" || input == "yes"
	}
	if result {
		return TRUE
	}
	return FALSE
}

func builtinPassword(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("password expects 1 argument, got %d", len(args))
	}

	question := objectToDisplayString(args[0])
	_, _ = fmt.Fprintf(e.Out, "%s: ", question)

	pass, err := readPassword()
	if err != nil {
		return newError("password: %s", err)
	}

	_, _ = fmt.Fprintln(e.Out) // newline after hidden input
	return stringToList(string(pass))
}

// =============================================================================
// Phase 4: Select / MultiSelect
// =============================================================================

func builtinSelect(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("select expects 2 arguments (question, options), got %d", len(args))
	}

	question := objectToDisplayString(args[0])
	optionsList, ok := args[1].(*List)
	if !ok {
		return newError("select: second argument must be a List")
	}

	options := make([]string, 0, optionsList.len())
	for _, item := range optionsList.ToSlice() {
		options = append(options, objectToDisplayString(item))
	}

	if len(options) == 0 {
		return newError("select: options list is empty")
	}

	selected, err := runSelect(e, question, options, false)
	if err != nil {
		return newError("select: %s", err)
	}
	if len(selected) == 0 {
		return stringToList(options[0])
	}
	return stringToList(options[selected[0]])
}

func builtinMultiSelect(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("multiSelect expects 2 arguments (question, options), got %d", len(args))
	}

	question := objectToDisplayString(args[0])
	optionsList, ok := args[1].(*List)
	if !ok {
		return newError("multiSelect: second argument must be a List")
	}

	options := make([]string, 0, optionsList.len())
	for _, item := range optionsList.ToSlice() {
		options = append(options, objectToDisplayString(item))
	}

	if len(options) == 0 {
		return newError("multiSelect: options list is empty")
	}

	selected, err := runSelect(e, question, options, true)
	if err != nil {
		return newError("multiSelect: %s", err)
	}

	elements := make([]Object, len(selected))
	for i, idx := range selected {
		elements[i] = stringToList(options[idx])
	}
	return newList(elements)
}

// =============================================================================
// Phase 5: Spinner & Progress
// =============================================================================

// Handle is an opaque reference to a terminal widget
type TermHandle struct {
	id   int64
	kind string // "spinner" or "progress"
}

func (h *TermHandle) Type() ObjectType             { return "TERM_HANDLE" }
func (h *TermHandle) TypeName() string             { return "Handle" }
func (h *TermHandle) Inspect() string              { return fmt.Sprintf("Handle(%s#%d)", h.kind, h.id) }
func (h *TermHandle) RuntimeType() typesystem.Type { return typesystem.TCon{Name: "Handle"} }
func (h *TermHandle) Hash() uint32                 { return uint32(h.id) }

// --- Spinner ---

type spinnerState struct {
	mu       sync.Mutex
	msg      string
	done     chan struct{} // signal goroutine to stop
	finished chan struct{} // closed when goroutine has exited
	stopped  bool
}

var (
	spinnerRegistry   = make(map[int64]*spinnerState)
	spinnerRegistryMu sync.Mutex
	handleSeq         int64
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func builtinSpinnerStart(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("spinnerStart expects 1 argument, got %d", len(args))
	}

	msg := objectToDisplayString(args[0])
	id := atomic.AddInt64(&handleSeq, 1)
	state := &spinnerState{
		msg:      msg,
		done:     make(chan struct{}),
		finished: make(chan struct{}),
	}

	spinnerRegistryMu.Lock()
	spinnerRegistry[id] = state
	spinnerRegistryMu.Unlock()

	out := e.Out

	// Run spinner animation in background
	go func() {
		defer close(state.finished)
		i := 0
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()

		// Hide cursor
		_, _ = fmt.Fprint(out, "\033[?25l")

		for {
			select {
			case <-state.done:
				return
			case <-ticker.C:
				state.mu.Lock()
				m := state.msg
				state.mu.Unlock()

				frame := spinnerFrames[i%len(spinnerFrames)]
				if getColorLevel() > 0 {
					_, _ = fmt.Fprintf(out, "\r\033[2K\033[36m%s\033[39m %s", frame, m)
				} else {
					_, _ = fmt.Fprintf(out, "\r\033[2K%s %s", frame, m)
				}
				i++
			}
		}
	}()

	return &TermHandle{id: id, kind: "spinner"}
}

func builtinSpinnerUpdate(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("spinnerUpdate expects 2 arguments (handle, msg), got %d", len(args))
	}

	handle, ok := args[0].(*TermHandle)
	if !ok || handle.kind != "spinner" {
		return newError("spinnerUpdate: first argument must be a spinner Handle")
	}

	msg := objectToDisplayString(args[1])

	spinnerRegistryMu.Lock()
	state, exists := spinnerRegistry[handle.id]
	spinnerRegistryMu.Unlock()

	if !exists {
		return newError("spinnerUpdate: invalid handle")
	}

	state.mu.Lock()
	state.msg = msg
	state.mu.Unlock()

	return &Nil{}
}

func builtinSpinnerStop(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("spinnerStop expects 2 arguments (handle, finalMsg), got %d", len(args))
	}

	handle, ok := args[0].(*TermHandle)
	if !ok || handle.kind != "spinner" {
		return newError("spinnerStop: first argument must be a spinner Handle")
	}

	finalMsg := objectToDisplayString(args[1])

	spinnerRegistryMu.Lock()
	state, exists := spinnerRegistry[handle.id]
	if exists {
		delete(spinnerRegistry, handle.id)
	}
	spinnerRegistryMu.Unlock()

	if !exists || state.stopped {
		return &Nil{}
	}

	state.stopped = true
	close(state.done)

	// Wait for the goroutine to finish writing before we write
	<-state.finished

	// Clear line, print final message, show cursor
	_, _ = fmt.Fprintf(e.Out, "\r\033[2K%s\n\033[?25h", finalMsg)

	return &Nil{}
}

// --- Progress bar ---

type progressState struct {
	mu      sync.Mutex
	total   int64
	current int64
	label   string
}

var (
	progressRegistry   = make(map[int64]*progressState)
	progressRegistryMu sync.Mutex
)

func builtinProgressNew(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("progressNew expects 2 arguments (total, label), got %d", len(args))
	}

	total, ok1 := args[0].(*Integer)
	if !ok1 {
		return newError("progressNew: first argument must be Int")
	}
	label := objectToDisplayString(args[1])

	id := atomic.AddInt64(&handleSeq, 1)
	state := &progressState{
		total: total.Value,
		label: label,
	}

	progressRegistryMu.Lock()
	progressRegistry[id] = state
	progressRegistryMu.Unlock()

	// Print initial bar
	renderProgressBar(e, state)

	return &TermHandle{id: id, kind: "progress"}
}

func builtinProgressTick(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("progressTick expects 1 argument, got %d", len(args))
	}

	handle, ok := args[0].(*TermHandle)
	if !ok || handle.kind != "progress" {
		return newError("progressTick: argument must be a progress Handle")
	}

	progressRegistryMu.Lock()
	state, exists := progressRegistry[handle.id]
	progressRegistryMu.Unlock()

	if !exists {
		return newError("progressTick: invalid handle")
	}

	state.mu.Lock()
	state.current++
	state.mu.Unlock()

	renderProgressBar(e, state)
	return &Nil{}
}

func builtinProgressSet(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("progressSet expects 2 arguments (handle, value), got %d", len(args))
	}

	handle, ok := args[0].(*TermHandle)
	if !ok || handle.kind != "progress" {
		return newError("progressSet: first argument must be a progress Handle")
	}

	val, ok2 := args[1].(*Integer)
	if !ok2 {
		return newError("progressSet: second argument must be Int")
	}

	progressRegistryMu.Lock()
	state, exists := progressRegistry[handle.id]
	progressRegistryMu.Unlock()

	if !exists {
		return newError("progressSet: invalid handle")
	}

	state.mu.Lock()
	state.current = val.Value
	state.mu.Unlock()

	renderProgressBar(e, state)
	return &Nil{}
}

func builtinProgressDone(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("progressDone expects 1 argument, got %d", len(args))
	}

	handle, ok := args[0].(*TermHandle)
	if !ok || handle.kind != "progress" {
		return newError("progressDone: argument must be a progress Handle")
	}

	progressRegistryMu.Lock()
	state, exists := progressRegistry[handle.id]
	if exists {
		delete(progressRegistry, handle.id)
	}
	progressRegistryMu.Unlock()

	if exists {
		state.mu.Lock()
		state.current = state.total
		state.mu.Unlock()
		renderProgressBar(e, state)
	}

	_, _ = fmt.Fprintln(e.Out)
	return &Nil{}
}

func renderProgressBar(e *Evaluator, state *progressState) {
	state.mu.Lock()
	current := state.current
	total := state.total
	label := state.label
	state.mu.Unlock()

	if total <= 0 {
		total = 1
	}

	barWidth := 30
	cols, _ := getTerminalSize()
	if cols > 60 {
		barWidth = 40
	}
	if cols > 100 {
		barWidth = 50
	}

	pct := float64(current) / float64(total)
	if pct > 1.0 {
		pct = 1.0
	}
	filled := int(pct * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	pctStr := fmt.Sprintf("%3d%%", int(pct*100))

	if getColorLevel() > 0 {
		_, _ = fmt.Fprintf(e.Out, "\r\033[2K%s \033[36m[%s]\033[39m %s", label, bar, pctStr)
	} else {
		_, _ = fmt.Fprintf(e.Out, "\r\033[2K%s [%s] %s", label, bar, pctStr)
	}
}

// =============================================================================
// Phase 6: Table
// =============================================================================

func builtinTable(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("table expects 2 arguments (headers, rows), got %d", len(args))
	}

	headersList, ok := args[0].(*List)
	if !ok {
		return newError("table: first argument must be a List of strings")
	}
	rowsList, ok := args[1].(*List)
	if !ok {
		return newError("table: second argument must be a List of rows")
	}

	// Extract headers
	headers := make([]string, 0, headersList.len())
	for _, h := range headersList.ToSlice() {
		headers = append(headers, objectToDisplayString(h))
	}

	// Extract rows
	rows := make([][]string, 0, rowsList.len())
	for _, row := range rowsList.ToSlice() {
		rowList, ok := row.(*List)
		if !ok {
			return newError("table: each row must be a List")
		}
		cells := make([]string, 0, rowList.len())
		for _, cell := range rowList.ToSlice() {
			cells = append(cells, objectToDisplayString(cell))
		}
		rows = append(rows, cells)
	}

	// Calculate column widths (based on visible text, not ANSI codes)
	numCols := len(headers)
	widths := make([]int, numCols)
	for i, h := range headers {
		w := visibleLen(h)
		if w > widths[i] {
			widths[i] = w
		}
	}
	for _, row := range rows {
		for i := 0; i < numCols && i < len(row); i++ {
			w := visibleLen(row[i])
			if w > widths[i] {
				widths[i] = w
			}
		}
	}

	// Box-drawing characters
	const (
		topLeft     = "┌"
		topRight    = "┐"
		bottomLeft  = "└"
		bottomRight = "┘"
		horizontal  = "─"
		vertical    = "│"
		teeDown     = "┬"
		teeUp       = "┴"
		teeRight    = "├"
		teeLeft     = "┤"
		cross       = "┼"
	)

	// Build separator lines
	buildSep := func(left, mid, right string) string {
		parts := make([]string, numCols)
		for i, w := range widths {
			parts[i] = strings.Repeat(horizontal, w+2)
		}
		return left + strings.Join(parts, mid) + right
	}

	// Build row
	buildRow := func(cells []string) string {
		parts := make([]string, numCols)
		for i := 0; i < numCols; i++ {
			cell := ""
			if i < len(cells) {
				cell = cells[i]
			}
			padding := widths[i] - visibleLen(cell)
			if padding < 0 {
				padding = 0
			}
			parts[i] = " " + cell + strings.Repeat(" ", padding) + " "
		}
		return vertical + strings.Join(parts, vertical) + vertical
	}

	// Render
	var sb strings.Builder
	sb.WriteString(buildSep(topLeft, teeDown, topRight))
	sb.WriteByte('\n')

	// Header
	if getColorLevel() > 0 {
		boldHeaders := make([]string, len(headers))
		for i, h := range headers {
			boldHeaders[i] = "\033[1m" + h + "\033[22m"
		}
		sb.WriteString(buildRow(boldHeaders))
	} else {
		sb.WriteString(buildRow(headers))
	}
	sb.WriteByte('\n')

	sb.WriteString(buildSep(teeRight, cross, teeLeft))
	sb.WriteByte('\n')

	// Data rows
	for _, row := range rows {
		sb.WriteString(buildRow(row))
		sb.WriteByte('\n')
	}

	sb.WriteString(buildSep(bottomLeft, teeUp, bottomRight))

	_, _ = fmt.Fprintln(e.Out, sb.String())
	return &Nil{}
}

// visibleLen returns the display width of a string in terminal columns,
// ignoring ANSI escape codes and accounting for fullwidth characters (CJK).
func visibleLen(s string) int {
	stripped := ansiRegex.ReplaceAllString(s, "")
	w := 0
	for _, r := range stripped {
		w += runeWidth(r)
	}
	return w
}

// runeWidth returns the display width of a rune in terminal columns.
// Fullwidth characters (CJK ideographs, fullwidth forms, etc.) take 2 columns.
func runeWidth(r rune) int {
	if r < 0x20 {
		return 0 // control chars
	}
	// CJK Radicals Supplement .. Enclosed CJK Letters
	if r >= 0x2E80 && r <= 0x33FF {
		return 2
	}
	// CJK Unified Ideographs Extension A
	if r >= 0x3400 && r <= 0x4DBF {
		return 2
	}
	// CJK Unified Ideographs
	if r >= 0x4E00 && r <= 0x9FFF {
		return 2
	}
	// Hangul Jamo
	if r >= 0x1100 && r <= 0x115F {
		return 2
	}
	// Hangul Compatibility Jamo
	if r >= 0x3130 && r <= 0x318F {
		return 2
	}
	// Hangul Syllables
	if r >= 0xAC00 && r <= 0xD7AF {
		return 2
	}
	// Hiragana, Katakana
	if r >= 0x3040 && r <= 0x30FF {
		return 2
	}
	// CJK Compatibility Ideographs
	if r >= 0xF900 && r <= 0xFAFF {
		return 2
	}
	// Fullwidth Forms
	if r >= 0xFF01 && r <= 0xFF60 {
		return 2
	}
	if r >= 0xFFE0 && r <= 0xFFE6 {
		return 2
	}
	// CJK Unified Ideographs Extension B and beyond
	if r >= 0x20000 && r <= 0x2FA1F {
		return 2
	}
	return 1
}

// =============================================================================
// Helper: convert object to display string
// =============================================================================

func objectToDisplayString(obj Object) string {
	if obj == nil {
		return ""
	}
	// If it's a string (List<Char>), convert directly
	if list, ok := obj.(*List); ok {
		if list.ElementType == "Char" {
			return listToString(list)
		}
	}
	return obj.Inspect()
}

// =============================================================================
// Raw mode & readKey builtins
// =============================================================================

func builtinTermRaw(e *Evaluator, args ...Object) Object {
	if err := enterRawMode(); err != nil {
		return newError("termRaw: %s", err)
	}
	return &Nil{}
}

func builtinTermRestore(e *Evaluator, args ...Object) Object {
	exitRawMode()
	return &Nil{}
}

func builtinReadKey(e *Evaluator, args ...Object) Object {
	timeoutMs := int64(0)
	if len(args) >= 1 {
		switch v := args[0].(type) {
		case *Integer:
			timeoutMs = v.Value
		default:
			return newError("readKey: expected Int for timeout, got %s", args[0].Type())
		}
	}

	result := readKeyImpl(int(timeoutMs))
	return stringToList(result)
}

// =============================================================================
// Registration
// =============================================================================

// TermBuiltins returns built-in functions for lib/term virtual package
func TermBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		// Phase 1: Styles & Colors
		"bold":          {Fn: builtinBold, Name: "bold"},
		"dim":           {Fn: builtinDim, Name: "dim"},
		"italic":        {Fn: builtinItalic, Name: "italic"},
		"underline":     {Fn: builtinUnderline, Name: "underline"},
		"strikethrough": {Fn: builtinStrikethrough, Name: "strikethrough"},

		"red":     {Fn: builtinTermRed, Name: "red"},
		"green":   {Fn: builtinTermGreen, Name: "green"},
		"yellow":  {Fn: builtinTermYellow, Name: "yellow"},
		"blue":    {Fn: builtinTermBlue, Name: "blue"},
		"magenta": {Fn: builtinTermMagenta, Name: "magenta"},
		"cyan":    {Fn: builtinTermCyan, Name: "cyan"},
		"white":   {Fn: builtinTermWhite, Name: "white"},
		"gray":    {Fn: builtinTermGray, Name: "gray"},

		"bgRed":     {Fn: builtinBgRed, Name: "bgRed"},
		"bgGreen":   {Fn: builtinBgGreen, Name: "bgGreen"},
		"bgYellow":  {Fn: builtinBgYellow, Name: "bgYellow"},
		"bgBlue":    {Fn: builtinBgBlue, Name: "bgBlue"},
		"bgCyan":    {Fn: builtinBgCyan, Name: "bgCyan"},
		"bgMagenta": {Fn: builtinBgMagenta, Name: "bgMagenta"},

		"rgb":   {Fn: builtinRgb, Name: "rgb"},
		"bgRgb": {Fn: builtinBgRgb, Name: "bgRgb"},
		"hex":   {Fn: builtinHex, Name: "hex"},
		"bgHex": {Fn: builtinBgHex, Name: "bgHex"},

		"stripAnsi":  {Fn: builtinStripAnsi, Name: "stripAnsi"},
		"termColors": {Fn: builtinTermColors, Name: "termColors"},
		"cprint":     {Fn: builtinCprint, Name: "cprint"},

		// Phase 2: Terminal control
		"termSize":      {Fn: builtinTermSize, Name: "termSize"},
		"termIsTTY":     {Fn: builtinTermIsTTY, Name: "termIsTTY"},
		"termClear":     {Fn: builtinTermClear, Name: "termClear"},
		"termClearLine": {Fn: builtinTermClearLine, Name: "termClearLine"},
		"cursorUp":      {Fn: builtinCursorUp, Name: "cursorUp"},
		"cursorDown":    {Fn: builtinCursorDown, Name: "cursorDown"},
		"cursorLeft":    {Fn: builtinCursorLeft, Name: "cursorLeft"},
		"cursorRight":   {Fn: builtinCursorRight, Name: "cursorRight"},
		"cursorTo":      {Fn: builtinCursorTo, Name: "cursorTo"},
		"cursorHide":    {Fn: builtinCursorHide, Name: "cursorHide"},
		"cursorShow":    {Fn: builtinCursorShow, Name: "cursorShow"},

		// Phase 3: Interactive prompts
		"prompt":   {Fn: builtinPrompt, Name: "prompt"},
		"confirm":  {Fn: builtinConfirm, Name: "confirm"},
		"password": {Fn: builtinPassword, Name: "password"},

		// Phase 4: Select / MultiSelect
		"select":      {Fn: builtinSelect, Name: "select"},
		"multiSelect": {Fn: builtinMultiSelect, Name: "multiSelect"},

		// Phase 5: Spinner & Progress
		"spinnerStart":  {Fn: builtinSpinnerStart, Name: "spinnerStart"},
		"spinnerUpdate": {Fn: builtinSpinnerUpdate, Name: "spinnerUpdate"},
		"spinnerStop":   {Fn: builtinSpinnerStop, Name: "spinnerStop"},
		"progressNew":   {Fn: builtinProgressNew, Name: "progressNew"},
		"progressTick":  {Fn: builtinProgressTick, Name: "progressTick"},
		"progressSet":   {Fn: builtinProgressSet, Name: "progressSet"},
		"progressDone":  {Fn: builtinProgressDone, Name: "progressDone"},

		// Phase 6: Table
		"table": {Fn: builtinTable, Name: "table"},

		// Phase 7: Raw mode & key reading
		"termRaw":     {Fn: builtinTermRaw, Name: "termRaw"},
		"termRestore": {Fn: builtinTermRestore, Name: "termRestore"},
		"readKey":     {Fn: builtinReadKey, Name: "readKey"},

		// Phase 8: Output buffering (double-buffering for flicker-free rendering)
		"termBufferStart": {Fn: builtinTermBufferStart, Name: "termBufferStart"},
		"termBufferFlush": {Fn: builtinTermBufferFlush, Name: "termBufferFlush"},
	}
}

// SetTermBuiltinTypes sets type info for term builtins
func SetTermBuiltinTypes(builtins map[string]*Builtin) {
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}
	listString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{stringType},
	}
	handleType := typesystem.TCon{Name: "Handle"}
	styleFn := typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType}
	// List<List<String>> for table rows
	listListString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{listString},
	}
	tupleIntInt := typesystem.TTuple{Elements: []typesystem.Type{typesystem.Int, typesystem.Int}}

	types := map[string]typesystem.Type{
		// Styles
		"bold": styleFn, "dim": styleFn, "italic": styleFn, "underline": styleFn, "strikethrough": styleFn,
		// Colors
		"red": styleFn, "green": styleFn, "yellow": styleFn, "blue": styleFn,
		"magenta": styleFn, "cyan": styleFn, "white": styleFn, "gray": styleFn,
		// Background colors
		"bgRed": styleFn, "bgGreen": styleFn, "bgYellow": styleFn, "bgBlue": styleFn,
		"bgCyan": styleFn, "bgMagenta": styleFn,
		// RGB/Hex
		"rgb":   typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, typesystem.Int, typesystem.Int, stringType}, ReturnType: stringType},
		"bgRgb": typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, typesystem.Int, typesystem.Int, stringType}, ReturnType: stringType},
		"hex":   typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: stringType},
		"bgHex": typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: stringType},
		// Utility
		"stripAnsi":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
		"termColors": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Int},
		"cprint":     typesystem.TFunc{Params: []typesystem.Type{styleFn, stringType}, ReturnType: typesystem.Nil, IsVariadic: true},
		// Terminal control
		"termSize":      typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: tupleIntInt},
		"termIsTTY":     typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Bool},
		"termClear":     typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Nil},
		"termClearLine": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Nil},
		"cursorUp":      typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: typesystem.Nil},
		"cursorDown":    typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: typesystem.Nil},
		"cursorLeft":    typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: typesystem.Nil},
		"cursorRight":   typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: typesystem.Nil},
		"cursorTo":      typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, typesystem.Int}, ReturnType: typesystem.Nil},
		"cursorHide":    typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Nil},
		"cursorShow":    typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Nil},
		// Interactive prompts
		"prompt":   typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: stringType, DefaultCount: 1},
		"confirm":  typesystem.TFunc{Params: []typesystem.Type{stringType, typesystem.Bool}, ReturnType: typesystem.Bool, DefaultCount: 1},
		"password": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
		// Select
		"select":      typesystem.TFunc{Params: []typesystem.Type{stringType, listString}, ReturnType: stringType},
		"multiSelect": typesystem.TFunc{Params: []typesystem.Type{stringType, listString}, ReturnType: listString},
		// Spinner & Progress
		"spinnerStart":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: handleType},
		"spinnerUpdate": typesystem.TFunc{Params: []typesystem.Type{handleType, stringType}, ReturnType: typesystem.Nil},
		"spinnerStop":   typesystem.TFunc{Params: []typesystem.Type{handleType, stringType}, ReturnType: typesystem.Nil},
		"progressNew":   typesystem.TFunc{Params: []typesystem.Type{typesystem.Int, stringType}, ReturnType: handleType},
		"progressTick":  typesystem.TFunc{Params: []typesystem.Type{handleType}, ReturnType: typesystem.Nil},
		"progressSet":   typesystem.TFunc{Params: []typesystem.Type{handleType, typesystem.Int}, ReturnType: typesystem.Nil},
		"progressDone":  typesystem.TFunc{Params: []typesystem.Type{handleType}, ReturnType: typesystem.Nil},
		// Table
		"table": typesystem.TFunc{Params: []typesystem.Type{listString, listListString}, ReturnType: typesystem.Nil},
		// Raw mode & key reading
		"termRaw":     typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Nil},
		"termRestore": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Nil},
		"readKey":     typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: stringType, DefaultCount: 1},
		// Output buffering
		"termBufferStart": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Nil},
		"termBufferFlush": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Nil},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}

	// Default args: readKey(timeoutMs=0) — non-blocking by default
	if b, ok := builtins["readKey"]; ok {
		b.DefaultArgs = []Object{&Integer{Value: 0}}
	}
}
