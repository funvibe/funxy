package vm

import (
	"fmt"
	"github.com/funvibe/funxy/internal/evaluator"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DebuggerMode represents the current debugging mode
type DebuggerMode int

const (
	// ModeRun - normal execution (no debugging)
	ModeRun DebuggerMode = iota
	// ModeStep - step through instructions one at a time
	ModeStep
	// ModeStepOver - step over function calls
	ModeStepOver
	// ModeStepOut - step out of current function
	ModeStepOut
	// ModeContinue - continue until next breakpoint
	ModeContinue
)

// Breakpoint represents a breakpoint location
type Breakpoint struct {
	File   string
	Line   int
	Column int // Optional, 0 means any column
}

// Debugger provides debugging capabilities for the VM
type Debugger struct {
	// Enabled flag
	Enabled bool

	// Current mode
	mode DebuggerMode

	// Breakpoints map: file -> line -> Breakpoint
	breakpoints map[string]map[int]*Breakpoint

	// Step over: track frame depth when step over started
	stepOverFrameDepth int
	stepOverFile       string
	stepOverLine       int

	// Step out: track frame depth when step out started
	stepOutFrameDepth int

	// Input/Output for debugger commands
	Input  io.Reader
	Output io.Writer

	// Callback for when debugger stops
	OnStop func(*Debugger, *VM)

	// Last stopped location (for step commands)
	lastFile string
	lastLine int

	// Last breakpoint hit (to avoid stopping on the same breakpoint twice)
	lastBreakpointFile string
	lastBreakpointLine int
}

// NewDebugger creates a new debugger instance
func NewDebugger() *Debugger {
	return &Debugger{
		Enabled:     false,
		mode:        ModeRun,
		breakpoints: make(map[string]map[int]*Breakpoint),
		Input:       nil,
		Output:      nil,
	}
}

// SetBreakpoint sets a breakpoint at the given file and line
func (d *Debugger) SetBreakpoint(file string, line int) *Breakpoint {
	if d.breakpoints[file] == nil {
		d.breakpoints[file] = make(map[int]*Breakpoint)
	}

	bp := &Breakpoint{
		File: file,
		Line: line,
	}
	d.breakpoints[file][line] = bp
	return bp
}

// RemoveBreakpoint removes a breakpoint at the given file and line
func (d *Debugger) RemoveBreakpoint(file string, line int) {
	if d.breakpoints[file] != nil {
		delete(d.breakpoints[file], line)
		if len(d.breakpoints[file]) == 0 {
			delete(d.breakpoints, file)
		}
	}
}

// ClearBreakpoints removes all breakpoints
func (d *Debugger) ClearBreakpoints() {
	d.breakpoints = make(map[string]map[int]*Breakpoint)
}

// GetBreakpoints returns all breakpoints
func (d *Debugger) GetBreakpoints() []*Breakpoint {
	var result []*Breakpoint
	for _, lineMap := range d.breakpoints {
		for _, bp := range lineMap {
			result = append(result, bp)
		}
	}
	return result
}

// normalizePath normalizes file paths for comparison (handles both absolute and relative)
// Always converts to absolute path for consistent comparison
func normalizePath(path string) string {
	// Convert to absolute path for consistent comparison
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

// ShouldBreak checks if execution should break at the current location
func (d *Debugger) ShouldBreak(vm *VM) bool {
	if !d.Enabled {
		return false
	}

	if vm.frame == nil || vm.frame.chunk == nil {
		return false
	}

	// Get current location
	file := vm.frame.chunk.File
	if file == "" {
		file = vm.currentFile
	}
	if file == "" {
		return false
	}

	// Normalize file path for comparison
	normalizedFile := normalizePath(file)

	// Get current line
	line := 0
	if vm.frame.ip < len(vm.frame.chunk.Lines) {
		line = vm.frame.chunk.Lines[vm.frame.ip]
	}
	if line == 0 {
		return false
	}

	// Check if we moved from last stop (Step mode)
	if d.lastFile != "" && (d.lastFile != normalizedFile || d.lastLine != line) {
		d.lastFile = ""
		d.lastLine = 0
	}

	// Check step modes
	switch d.mode {
	case ModeStep:
		// Always break in step mode
		// But skip if we're on the same line (multiple instructions per line)
		if d.lastFile == normalizedFile && d.lastLine == line {
			return false
		}
		d.lastFile = normalizedFile
		d.lastLine = line
		return true

	case ModeStepOver:
		// Break if we've returned from the function (frame depth decreased)
		if vm.frameCount < d.stepOverFrameDepth {
			d.mode = ModeRun
			return true
		}
		// Or if we are at the same depth but changed line/file
		if vm.frameCount == d.stepOverFrameDepth {
			// Check if we moved to a new line relative to start of StepOver
			if d.stepOverFile != "" && (d.stepOverFile != normalizedFile || d.stepOverLine != line) {
				d.mode = ModeRun
				// Update last position as we are stopping
				d.lastFile = normalizedFile
				d.lastLine = line
				return true
			}
		}
		// Otherwise (deeper stack OR same line), continue
		return false

	case ModeStepOut:
		// Break if we've returned from the function (frame depth decreased)
		if vm.frameCount < d.stepOutFrameDepth {
			d.mode = ModeRun
			// Update last position
			d.lastFile = normalizedFile
			d.lastLine = line
			return true
		}
		return false

	case ModeContinue:
		// Check breakpoints - normalize breakpoint file paths too
		for bpFile, lineMap := range d.breakpoints {
			normalizedBpFile := normalizePath(bpFile)
			if normalizedBpFile == normalizedFile {
				if bp := lineMap[line]; bp != nil {
					// Skip if we just stopped at this breakpoint
					if d.lastBreakpointFile == normalizedFile && d.lastBreakpointLine == line {
						// Clear the last breakpoint once we've moved past it
						return false
					}
					// Also skip if we just stepped to this line (from Step mode)
					if d.lastFile == normalizedFile && d.lastLine == line {
						return false
					}
					d.lastBreakpointFile = normalizedFile
					d.lastBreakpointLine = line
					// Update last position so Step works correctly from here
					d.lastFile = normalizedFile
					d.lastLine = line
					return true
				}
			}
		}
		// Clear last breakpoint if we're on a different line/file
		if d.lastBreakpointFile != "" && (d.lastBreakpointFile != normalizedFile || d.lastBreakpointLine != line) {
			d.lastBreakpointFile = ""
			d.lastBreakpointLine = 0
		}
		return false

	case ModeRun:
		// Check breakpoints only - normalize breakpoint file paths too
		for bpFile, lineMap := range d.breakpoints {
			normalizedBpFile := normalizePath(bpFile)
			if normalizedBpFile == normalizedFile {
				if bp := lineMap[line]; bp != nil {
					d.lastFile = normalizedFile
					d.lastLine = line
					return true
				}
			}
		}
		return false
	}

	return false
}

// Step sets debugger to step mode
func (d *Debugger) Step() {
	d.mode = ModeStep
	d.stepOverFrameDepth = 0
	d.stepOutFrameDepth = 0
}

// StepOver sets debugger to step over mode
func (d *Debugger) StepOver(vm *VM) {
	d.mode = ModeStepOver
	if vm != nil {
		d.stepOverFrameDepth = vm.frameCount
		// Initialize start position for step over
		file, line, _ := d.GetCurrentLocation(vm)
		d.stepOverFile = normalizePath(file)
		d.stepOverLine = line
	}
	d.stepOutFrameDepth = 0
}

// StepOut sets debugger to step out mode
func (d *Debugger) StepOut(vm *VM) {
	d.mode = ModeStepOut
	if vm != nil {
		d.stepOutFrameDepth = vm.frameCount
	}
	d.stepOverFrameDepth = 0
}

// Continue sets debugger to continue mode (run until breakpoint)
func (d *Debugger) Continue() {
	d.mode = ModeContinue
	d.stepOverFrameDepth = 0
	d.stepOutFrameDepth = 0
	// Don't clear lastBreakpoint here - we need it to skip the current breakpoint
}

// Run sets debugger to run mode (no debugging)
func (d *Debugger) Run() {
	d.mode = ModeRun
	d.stepOverFrameDepth = 0
	d.stepOutFrameDepth = 0
}

// GetCurrentLocation returns the current file and line
func (d *Debugger) GetCurrentLocation(vm *VM) (file string, line int, column int) {
	if vm.frame == nil || vm.frame.chunk == nil {
		return "", 0, 0
	}

	file = vm.frame.chunk.File
	if file == "" {
		file = vm.currentFile
	}
	if file == "" {
		file = "<script>"
	}

	// Don't normalize here - return original path
	// FormatLocation will handle normalization for display

	if vm.frame.ip < len(vm.frame.chunk.Lines) {
		line = vm.frame.chunk.Lines[vm.frame.ip]
	}
	if vm.frame.ip < len(vm.frame.chunk.Columns) {
		column = vm.frame.chunk.Columns[vm.frame.ip]
	}

	return file, line, column
}

// GetCallStack returns the current call stack
func (d *Debugger) GetCallStack(vm *VM) []CallFrameInfo {
	var stack []CallFrameInfo

	for i := vm.frameCount - 1; i >= 0; i-- {
		frame := &vm.frames[i]
		info := CallFrameInfo{
			Index: i,
		}

		// Get function name
		if frame.closure != nil && frame.closure.Function != nil {
			info.FunctionName = frame.closure.Function.Name
			if info.FunctionName == "" {
				info.FunctionName = "<anonymous>"
			}
		} else {
			info.FunctionName = "<script>"
		}

		// Get file and line
		if frame.chunk != nil {
			info.File = frame.chunk.File
			if info.File == "" {
				info.File = vm.currentFile
			}
			if info.File == "" {
				info.File = "<script>"
			}
			// Don't normalize here - FormatLocation will handle it for display

			// Determine IP to use for line number
			// For top frame, use current IP (next instruction)
			// For parent frames, use previous IP (call site) because they are waiting at return address
			targetIP := frame.ip
			if i < vm.frameCount-1 && targetIP > 0 {
				targetIP--
			}

			if targetIP < len(frame.chunk.Lines) {
				info.Line = frame.chunk.Lines[targetIP]
			}
			if targetIP < len(frame.chunk.Columns) {
				info.Column = frame.chunk.Columns[targetIP]
			}
		}

		stack = append(stack, info)
	}

	return stack
}

// CallFrameInfo represents information about a call frame
type CallFrameInfo struct {
	Index        int
	FunctionName string
	File         string
	Line         int
	Column       int
}

// GetLocals returns local variables for the current frame
func (d *Debugger) GetLocals(vm *VM) map[string]evaluator.Object {
	locals := make(map[string]evaluator.Object)

	if vm.frame == nil || vm.frame.closure == nil {
		return locals
	}

	fn := vm.frame.closure.Function
	if fn == nil {
		return locals
	}

	// Get local variable names from compiler metadata
	base := vm.frame.base
	localCount := fn.LocalCount
	if localCount == 0 {
		localCount = vm.sp - base
	}

	// Use LocalNames if available, otherwise use slot indices
	for i := 0; i < localCount && i < vm.sp-base; i++ {
		slot := base + i
		if slot >= len(vm.stack) {
			break
		}

		var name string
		if fn.LocalNames != nil && i < len(fn.LocalNames) && fn.LocalNames[i] != "" {
			name = fn.LocalNames[i]
		} else {
			name = fmt.Sprintf("slot%d", i)
		}

		locals[name] = vm.stack[slot].AsObject()
	}

	return locals
}

// GetGlobals returns global variables (filtering out built-in functions)
func (d *Debugger) GetGlobals(vm *VM) map[string]evaluator.Object {
	globals := make(map[string]evaluator.Object)

	if vm.globals == nil || vm.globals.Globals == nil {
		return globals
	}

	// Get globals that are not built-ins
	vm.globals.Globals.Range(func(name string, val evaluator.Object) bool {
		// Filter out built-in functions and class methods
		switch val.(type) {
		case *evaluator.Builtin:
			// Skip all built-in functions
			return true
		case *evaluator.ClassMethod:
			// Skip all class methods
			return true
		case *evaluator.TypeObject:
			// Skip built-in types
			if isBuiltinType(name) {
				return true
			}
			globals[name] = val
		case *evaluator.Constructor:
			// Skip built-in constructors
			if isBuiltinConstructor(name) {
				return true
			}
			globals[name] = val
		case *evaluator.DataInstance:
			// Skip built-in nullary constructors (like None, JNull)
			if isBuiltinConstructor(name) {
				return true
			}
			// User-defined ADT instances
			globals[name] = val
		case *ObjClosure:
			// User-defined function
			globals[name] = val
		case *evaluator.Integer, *evaluator.Float, *evaluator.Boolean,
			*evaluator.List, *evaluator.Tuple, *evaluator.RecordInstance,
			*evaluator.Map, *evaluator.Bytes, *evaluator.Bits,
			*evaluator.BigInt, *evaluator.Rational, *evaluator.Char:
			// User-defined values
			globals[name] = val
		case *evaluator.Nil:
			// Skip Nil constant
			if name == "Nil" {
				return true
			}
			globals[name] = val
		default:
			// Include other user-defined values
			// But skip if it looks like a built-in
			if !isLikelyBuiltin(name) {
				globals[name] = val
			}
		}
		return true
	})

	return globals
}

// isBuiltinType checks if a type name is a built-in type
func isBuiltinType(name string) bool {
	builtinTypes := map[string]bool{
		"Int": true, "Float": true, "Bool": true, "String": true,
		"Char": true, "List": true, "Map": true, "Option": true,
		"Result": true, "Nil": true, "BigInt": true, "Rational": true,
		"Json": true, "Email": true, "Bytes": true, "Bits": true,
	}
	return builtinTypes[name]
}

// isBuiltinConstructor checks if a constructor name is built-in
func isBuiltinConstructor(name string) bool {
	builtinConstructors := map[string]bool{
		"Some": true, "None": true, "Ok": true, "Fail": true,
		"JNull": true, "JBool": true, "JNum": true, "JStr": true,
		"JArr": true, "JObj": true, "Nil": true,
		// Note: MkEmail is not a built-in, it's user-defined in the test file
	}
	return builtinConstructors[name]
}

// isLikelyBuiltin checks if a name is likely a built-in based on naming patterns
func isLikelyBuiltin(name string) bool {
	// Check for operator-like names
	if strings.HasPrefix(name, "(") && strings.HasSuffix(name, ")") {
		return true
	}
	// Check for known built-in prefixes
	builtinPrefixes := []string{"string", "flag", "task", "ws", "http", "json", "sql", "regex", "time", "io", "file"}
	lowerName := strings.ToLower(name)
	for _, prefix := range builtinPrefixes {
		if strings.HasPrefix(lowerName, prefix) {
			return true
		}
	}
	// Check for known built-in function names
	builtinFuncs := map[string]bool{
		"print": true, "panic": true, "len": true, "head": true, "tail": true,
		"map": true, "filter": true, "fold": true, "foldl": true, "foldr": true,
		"zip": true, "unzip": true, "take": true, "drop": true, "reverse": true,
		"sort": true, "sortBy": true, "concat": true, "flatten": true, "unique": true,
		"min": true, "max": true, "abs": true, "sqrt": true, "pow": true,
		"sin": true, "cos": true, "tan": true, "asin": true, "acos": true, "atan": true,
		"log": true, "log2": true, "log10": true, "exp": true, "floor": true, "ceil": true,
		"round": true, "trunc": true, "sign": true, "pi": true, "e": true,
		"intToFloat": true, "floatToInt": true, "format": true, "read": true, "write": true,
		"getType": true, "typeOf": true, "id": true, "constant": true, "flip": true,
		"pure": true, "fmap": true, "mempty": true, "show": true,
	}
	return builtinFuncs[name]
}

// GetStack returns the current stack contents
func (d *Debugger) GetStack(vm *VM) []evaluator.Object {
	var stack []evaluator.Object

	for i := 0; i < vm.sp && i < len(vm.stack); i++ {
		stack = append(stack, vm.stack[i].AsObject())
	}

	return stack
}

// FormatLocation formats a file:line location string
// Prefers relative paths for display (for readability)
func (d *Debugger) FormatLocation(file string, line int) string {
	// Try to show relative path if possible (for display)
	displayFile := file
	if wd, err := os.Getwd(); err == nil {
		// Normalize to absolute first
		if abs, err := filepath.Abs(file); err == nil {
			// Try to make relative for display
			if rel, err := filepath.Rel(wd, abs); err == nil && !strings.HasPrefix(rel, "..") {
				displayFile = rel
			} else {
				displayFile = abs
			}
		}
	}

	if line > 0 {
		return fmt.Sprintf("%s:%d", displayFile, line)
	}
	return displayFile
}

// PrintLocation prints the current location
func (d *Debugger) PrintLocation(vm *VM) {
	file, line, col := d.GetCurrentLocation(vm)

	// Check if this is the initial stop
	isInitialStop := false
	if vm.frameCount == 1 && vm.frame != nil && vm.frame.ip <= 1 {
		isInitialStop = true
	}

	if isInitialStop {
		loc := d.FormatLocation(file, line)
		fmt.Fprintf(d.Output, "Breakpoint at %s (program start)\n", loc)
	} else if col > 0 {
		// Use FormatLocation with 0 to get just the file path
		displayFile := d.FormatLocation(file, 0)
		fmt.Fprintf(d.Output, "Breakpoint at %s:%d:%d\n", displayFile, line, col)
	} else {
		loc := d.FormatLocation(file, line)
		fmt.Fprintf(d.Output, "Breakpoint at %s\n", loc)
	}
}

// PrintCallStack prints the call stack
func (d *Debugger) PrintCallStack(vm *VM) {
	stack := d.GetCallStack(vm)
	fmt.Fprintf(d.Output, "Call stack:\n")
	for i, frame := range stack {
		indent := strings.Repeat("  ", i)
		loc := d.FormatLocation(frame.File, frame.Line)
		fmt.Fprintf(d.Output, "%s%d. %s at %s\n", indent, i+1, frame.FunctionName, loc)
	}
}

// PrintLocals prints local variables
func (d *Debugger) PrintLocals(vm *VM) {
	// Show locals for functions (including top-level script which is frame 0)
	locals := d.GetLocals(vm)
	if len(locals) == 0 {
		fmt.Fprintf(d.Output, "No local variables in current scope.\n")
	} else {
		fmt.Fprintf(d.Output, "Local variables:\n")
		// Sort keys for consistent output
		var names []string
		for name := range locals {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			val := locals[name]
			fmt.Fprintf(d.Output, "  %s = %s\n", name, val.Inspect())
		}
	}
}

// PrintGlobals prints global variables
func (d *Debugger) PrintGlobals(vm *VM) {
	globals := d.GetGlobals(vm)
	if len(globals) == 0 {
		fmt.Fprintf(d.Output, "No user-defined global variables.\n")
		return
	}
	fmt.Fprintf(d.Output, "Global variables:\n")
	// Sort keys for consistent output
	var names []string
	for name := range globals {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		val := globals[name]
		fmt.Fprintf(d.Output, "  %s = %s\n", name, val.Inspect())
	}
}

// PrintStack prints the stack
func (d *Debugger) PrintStack(vm *VM) {
	stack := d.GetStack(vm)
	fmt.Fprintf(d.Output, "Stack (top to bottom):\n")
	for i := len(stack) - 1; i >= 0; i-- {
		fmt.Fprintf(d.Output, "  [%d] %s\n", i, stack[i].Inspect())
	}
}
