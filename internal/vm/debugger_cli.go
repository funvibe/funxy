package vm

import (
	"bufio"
	"fmt"
	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/evaluator"
	"github.com/funvibe/funxy/internal/lexer"
	"github.com/funvibe/funxy/internal/parser"
	"github.com/funvibe/funxy/internal/pipeline"
	"github.com/funvibe/funxy/internal/typesystem"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DebuggerCLI provides a command-line interface for the debugger
type DebuggerCLI struct {
	debugger *Debugger
	vm       *VM
	scanner  *bufio.Scanner
	input    io.Reader
	output   io.Writer
}

// NewDebuggerCLI creates a new CLI debugger
func NewDebuggerCLI(debugger *Debugger, vm *VM) *DebuggerCLI {
	return &DebuggerCLI{
		debugger: debugger,
		vm:       vm,
		input:    os.Stdin,
		output:   os.Stdout,
	}
}

// SetInput sets the input reader
func (cli *DebuggerCLI) SetInput(r io.Reader) {
	cli.input = r
	cli.scanner = bufio.NewScanner(r)
}

// SetOutput sets the output writer
func (cli *DebuggerCLI) SetOutput(w io.Writer) {
	cli.output = w
}

// Run starts the debugger CLI loop
func (cli *DebuggerCLI) Run() {
	if cli.scanner == nil {
		cli.scanner = bufio.NewScanner(cli.input)
	}

	// Set up debugger callbacks
	cli.debugger.Input = cli.input
	cli.debugger.Output = cli.output
	cli.debugger.OnStop = cli.onStop

	fmt.Fprintf(cli.output, "Debugger started. Type 'help' for commands.\n")
}

// onStop is called when the debugger stops
func (cli *DebuggerCLI) onStop(dbg *Debugger, vm *VM) {
	// Print current location
	dbg.PrintLocation(vm)
	fmt.Fprintf(cli.output, "\n")

	// Enter command loop
	for {
		fmt.Fprintf(cli.output, "(funxy) ")
		if !cli.scanner.Scan() {
			// EOF or error - exit debugger and program
			if err := cli.scanner.Err(); err != nil {
				fmt.Fprintf(cli.output, "\nDebugger error: %v\n", err)
			} else {
				fmt.Fprintf(cli.output, "\nExiting debugger (EOF).\n")
			}
			dbg.Run()
			os.Exit(0)
			return
		}

		line := strings.TrimSpace(cli.scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		cmd := parts[0]
		args := parts[1:]

		switch cmd {
		case "help", "h":
			printHelp(cli.output)
		case "continue", "c":
			dbg.Continue()
			return
		case "step", "s":
			dbg.Step()
			return
		case "stepover", "so", "next", "n":
			dbg.StepOver(vm)
			return
		case "stepout", "out", "finish", "fin":
			dbg.StepOut(vm)
			return
		case "break", "b":
			cli.handleBreakpoint(args)
		case "delete", "d":
			cli.handleDeleteBreakpoint(args)
		case "list", "l":
			cli.handleListBreakpoints()
		case "locals", "vars":
			dbg.PrintLocals(vm)
		case "globals":
			dbg.PrintGlobals(vm)
		case "stack":
			dbg.PrintStack(vm)
		case "backtrace", "bt":
			dbg.PrintCallStack(vm)
		case "print", "p":
			cli.handlePrint(args, vm)
		case "quit", "q", "exit":
			dbg.Run()
			os.Exit(0)
		default:
			fmt.Fprintf(cli.output, "Unknown command: %s. Type 'help' for help.\n", cmd)
		}
	}
}

// PrintHelp prints help information (exported for testing)
func (cli *DebuggerCLI) PrintHelp() {
	printHelp(cli.output)
}

// printHelp prints help information
func printHelp(output io.Writer) {
	help := `Debugger commands:
  help, h              - Show this help
  continue, c          - Continue execution until next breakpoint
  step, s              - Step into next instruction
  stepover, so, next, n - Step over function call
  stepout, out, finish - Step out of current function
  break, b <file>:<line> - Set breakpoint at file:line
  delete, d <file>:<line> - Delete breakpoint at file:line
  list, l              - List all breakpoints
  locals, vars         - Show local variables
  globals              - Show global variables
  stack                - Show stack contents
  backtrace, bt        - Show call stack
  print, p <expr>      - Print expression value (partial - globals only)
  quit, q, exit        - Exit debugger and program
`
	fmt.Fprint(output, help)
}

// handleBreakpoint handles breakpoint commands
func (cli *DebuggerCLI) handleBreakpoint(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(cli.output, "Usage: break <file>:<line>\n")
		return
	}

	arg := args[0]
	parts := strings.Split(arg, ":")
	if len(parts) != 2 {
		fmt.Fprintf(cli.output, "Invalid format. Use: break <file>:<line>\n")
		return
	}

	file := parts[0]
	// Normalize file path - try to make it absolute if relative
	if !filepath.IsAbs(file) {
		if wd, err := os.Getwd(); err == nil {
			if abs, err := filepath.Abs(filepath.Join(wd, file)); err == nil {
				file = abs
			}
		}
	} else {
		// Already absolute, normalize it
		if abs, err := filepath.Abs(file); err == nil {
			file = abs
		}
	}

	line, err := strconv.Atoi(parts[1])
	if err != nil {
		fmt.Fprintf(cli.output, "Invalid line number: %s\n", parts[1])
		return
	}

	bp := cli.debugger.SetBreakpoint(file, line)
	// Show relative path in output if possible
	displayFile := file
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, file); err == nil && !strings.HasPrefix(rel, "..") {
			displayFile = rel
		}
	}
	fmt.Fprintf(cli.output, "Breakpoint set at %s:%d\n", displayFile, bp.Line)
}

// handleDeleteBreakpoint handles delete breakpoint commands
func (cli *DebuggerCLI) handleDeleteBreakpoint(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(cli.output, "Usage: delete <file>:<line>\n")
		return
	}

	arg := args[0]
	parts := strings.Split(arg, ":")
	if len(parts) != 2 {
		fmt.Fprintf(cli.output, "Invalid format. Use: delete <file>:<line>\n")
		return
	}

	file := parts[0]
	// Normalize file path - try to make it absolute if relative
	if !filepath.IsAbs(file) {
		if wd, err := os.Getwd(); err == nil {
			if abs, err := filepath.Abs(filepath.Join(wd, file)); err == nil {
				file = abs
			}
		}
	} else {
		// Already absolute, normalize it
		if abs, err := filepath.Abs(file); err == nil {
			file = abs
		}
	}

	line, err := strconv.Atoi(parts[1])
	if err != nil {
		fmt.Fprintf(cli.output, "Invalid line number: %s\n", parts[1])
		return
	}

	cli.debugger.RemoveBreakpoint(file, line)
	// Show relative path in output if possible
	displayFile := file
	if wd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(wd, file); err == nil && !strings.HasPrefix(rel, "..") {
			displayFile = rel
		}
	}
	fmt.Fprintf(cli.output, "Breakpoint removed at %s:%d\n", displayFile, line)
}

// handleListBreakpoints lists all breakpoints
func (cli *DebuggerCLI) handleListBreakpoints() {
	bps := cli.debugger.GetBreakpoints()
	if len(bps) == 0 {
		fmt.Fprintf(cli.output, "No breakpoints set.\n")
		return
	}

	fmt.Fprintf(cli.output, "Breakpoints:\n")
	wd, _ := os.Getwd()
	for i, bp := range bps {
		displayFile := bp.File
		// Try to show relative path if possible
		if wd != "" {
			if abs, err := filepath.Abs(bp.File); err == nil {
				if rel, err := filepath.Rel(wd, abs); err == nil && !strings.HasPrefix(rel, "..") {
					displayFile = rel
				}
			}
		}
		fmt.Fprintf(cli.output, "  %d. %s:%d\n", i+1, displayFile, bp.Line)
	}
}

// handlePrint handles print commands - evaluates and prints expression value
func (cli *DebuggerCLI) handlePrint(args []string, vm *VM) {
	if len(args) == 0 {
		fmt.Fprintf(cli.output, "Usage: print <expression>\n")
		return
	}

	// Join all args to form the expression
	exprStr := strings.Join(args, " ")

	// First, try to look up as a simple variable name (most common case)
	if len(args) == 1 && isValidIdentifier(args[0]) {
		name := args[0]

		// Check locals
		locals := cli.debugger.GetLocals(vm)
		if val, ok := locals[name]; ok {
			fmt.Fprintf(cli.output, "%s\n", val.Inspect())
			return
		}

		// Check globals
		globals := cli.debugger.GetGlobals(vm)
		if val, ok := globals[name]; ok {
			fmt.Fprintf(cli.output, "%s\n", val.Inspect())
			return
		}
	}

	// For complex expressions, parse and evaluate
	// Parse the expression
	ctx := pipeline.NewPipelineContext(exprStr)
	ctx.FilePath = "<debug>"

	lexerProc := &lexer.LexerProcessor{}
	ctx = lexerProc.Process(ctx)
	if len(ctx.Errors) > 0 {
		fmt.Fprintf(cli.output, "Parse error: %v\n", ctx.Errors[0])
		return
	}

	parserProc := &parser.ParserProcessor{}
	ctx = parserProc.Process(ctx)
	if len(ctx.Errors) > 0 {
		fmt.Fprintf(cli.output, "Parse error: %v\n", ctx.Errors[0])
		return
	}

	// Extract expression from AST
	var expr ast.Expression
	if prog, ok := ctx.AstRoot.(*ast.Program); ok && len(prog.Statements) > 0 {
		// If it's a statement, try to extract expression
		if exprStmt, ok := prog.Statements[0].(*ast.ExpressionStatement); ok {
			expr = exprStmt.Expression
		} else {
			fmt.Fprintf(cli.output, "Error: Expected expression, got statement\n")
			return
		}
	} else {
		fmt.Fprintf(cli.output, "Error: Could not parse expression\n")
		return
	}

	// For simple identifiers, we already handled them above
	// For complex expressions, create a minimal compiler

	// Create a base compiler with globals
	baseCompiler := &Compiler{
		function: &CompiledFunction{
			Chunk: NewChunk(),
			Name:  "<debug-base>",
		},
		funcType:    TYPE_SCRIPT,
		locals:      make([]Local, 256),
		upvalues:    make([]Upvalue, 256),
		globals:     make(map[string]bool),
		typeAliases: make(map[string]typesystem.Type),
	}

	// Register all known globals
	if vm.globals != nil && vm.globals.Globals != nil {
		vm.globals.Globals.Range(func(name string, _ evaluator.Object) bool {
			baseCompiler.globals[name] = true
			return true
		})
	}

	// Create function compiler with base as enclosing
	funcCompiler := newFunctionCompiler(baseCompiler, "<debug>", 0)

	// Add current frame's locals to compiler so expression can access them
	if vm.frame != nil && vm.frame.closure != nil && vm.frame.closure.Function != nil {
		fn := vm.frame.closure.Function
		// Copy locals to the compiler
		for i, name := range fn.LocalNames {
			if name != "" && i < fn.LocalCount {
				funcCompiler.addLocal(name, i)
			}
		}
		funcCompiler.slotCount = fn.LocalCount
		funcCompiler.localCount = fn.LocalCount
	}

	// Compile the expression
	if err := funcCompiler.compileExpression(expr); err != nil {
		fmt.Fprintf(cli.output, "Compilation error: %v\n", err)
		return
	}

	// Add return instruction
	line := 0
	if expr.GetToken().Line > 0 {
		line = expr.GetToken().Line
	}
	funcCompiler.emit(OP_RETURN, line)
	funcCompiler.function.LocalCount = funcCompiler.localCount

	chunk := funcCompiler.function.Chunk
	if chunk == nil {
		fmt.Fprintf(cli.output, "Error: Compiled chunk is nil\n")
		return
	}
	chunk.File = "<debug>"

	// Create a closure for the compiled function
	closure := &ObjClosure{
		Function: funcCompiler.function,
		Upvalues: make([]*ObjUpvalue, funcCompiler.upvalueCount),
	}

	// Save current VM state
	savedSp := vm.sp
	savedFrame := vm.frame
	savedFrameCount := vm.frameCount

	// Temporarily disable debugger
	debuggerWasEnabled := false
	if vm.debugger != nil {
		debuggerWasEnabled = vm.debugger.Enabled
		vm.debugger.Enabled = false
	}

	// Push a new frame for evaluation
	// First, copy current locals to stack if we have them
	if vm.frame != nil && vm.frame.closure != nil && vm.frame.closure.Function != nil {
		fn := vm.frame.closure.Function
		base := vm.frame.base
		// Ensure we have space
		for vm.sp < base+fn.LocalCount {
			vm.push(NilVal())
		}
		// Copy locals to new frame's expected positions
		for i := 0; i < fn.LocalCount && base+i < len(vm.stack); i++ {
			vm.stack[vm.sp-fn.LocalCount+i] = vm.stack[base+i]
		}
	}

	// Create frame for evaluation
	if vm.frameCount >= len(vm.frames) {
		fmt.Fprintf(cli.output, "Error: Frame stack overflow\n")
		vm.sp = savedSp
		if vm.debugger != nil {
			vm.debugger.Enabled = debuggerWasEnabled
		}
		return
	}

	frame := &vm.frames[vm.frameCount]
	frame.closure = closure
	frame.chunk = chunk
	frame.ip = 0
	frame.base = vm.sp - funcCompiler.localCount
	vm.frame = frame
	vm.frameCount++

	// Execute the expression
	var resultObj evaluator.Object
	var err error

	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic during evaluation: %v", r)
			}
		}()

		// Run the VM step by step until we hit RETURN
		for {
			if vm.frame.ip >= len(chunk.Code) {
				err = fmt.Errorf("unexpected end of bytecode")
				break
			}

			instruction := Opcode(chunk.Code[vm.frame.ip])
			vm.frame.ip++

			if instruction == OP_RETURN {
				// Get the result
				if vm.sp > 0 {
					resultObj = vm.stack[vm.sp-1].AsObject()
				}
				break
			}

			// Execute the instruction using executeOneOp
			if err = vm.executeOneOp(instruction); err != nil {
				break
			}
		}
	}()

	// Restore VM state
	vm.sp = savedSp
	vm.frame = savedFrame
	vm.frameCount = savedFrameCount

	// Restore debugger state
	if vm.debugger != nil {
		vm.debugger.Enabled = debuggerWasEnabled
	}

	if err != nil {
		fmt.Fprintf(cli.output, "Evaluation error: %v\n", err)
		return
	}

	// Print the result
	if resultObj != nil {
		fmt.Fprintf(cli.output, "%s\n", resultObj.Inspect())
	} else {
		fmt.Fprintf(cli.output, "nil\n")
	}
}

// isValidIdentifier checks if a string is a valid identifier
func isValidIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Check first character (letter or underscore)
	if !((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z') || s[0] == '_') {
		return false
	}
	// Check remaining characters (letters, digits, underscore)
	for i := 1; i < len(s); i++ {
		if !((s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z') ||
			(s[i] >= '0' && s[i] <= '9') || s[i] == '_') {
			return false
		}
	}
	return true
}
