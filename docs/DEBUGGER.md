# Debugger

Funxy includes a built-in debugger for the VM backend that allows you to step through code, set breakpoints, and inspect variables.

## Usage

To start debugging, use the `-debug` flag:

```bash
./funxy -debug program.lang
```

This will start the debugger and pause execution at the first line of code.

## Commands

### Execution Control

- `continue`, `c` - Continue execution until next breakpoint
- `step`, `s` - Step into next instruction (step into function calls)
- `stepover`, `so`, `next`, `n` - Step over function call (don't enter function)
- `stepout`, `out`, `finish`, `fin` - Step out of current function
- `quit`, `q`, `exit` - Exit debugger and program

### Breakpoints

- `break`, `b <file>:<line>` - Set breakpoint at file:line
  - Example: `break main.lang:10` or `break /absolute/path/main.lang:10`
  - **File paths**: Can be relative (to current working directory) or absolute
  - The debugger automatically normalizes paths (converts to absolute) for matching breakpoints
  - When displaying locations, relative paths are shown when possible for readability
- `delete`, `d <file>:<line>` - Delete breakpoint at file:line
  - Uses the same path normalization as `break`
- `list`, `l` - List all breakpoints
  - Shows breakpoints with relative paths when possible

### Inspection

- `locals`, `vars` - Show local variables in current frame
- `globals` - Show global variables
- `stack` - Show stack contents
- `backtrace`, `bt` - Show call stack
- `print`, `p <expr>` - Evaluate and print expression value

### Help

- `help`, `h` - Show help message

## Example Session

```bash
$ ./funxy -debug test.lang
Debugger started. Type 'help' for commands.
Breakpoint at test.lang:1

(funxy) break test.lang:10
Breakpoint set at test.lang:10

(funxy) continue
Breakpoint at test.lang:10

(funxy) locals
Local variables:
  x = 42
  y = 10

(funxy) print x + y
52

(funxy) step
Breakpoint at test.lang:11

(funxy) backtrace
Call stack:
  1. main at test.lang:11
  2. <script> at test.lang:1

(funxy) continue
Program completed.
```

## Implementation Details

The debugger is integrated into the VM's `step()` function. When a breakpoint is hit or step mode is active, execution pauses and the debugger CLI takes over.

### Breakpoints

Breakpoints are stored by file and line number. When execution reaches a line with a breakpoint, the debugger stops.

### Step Modes

- **Step**: Executes one instruction at a time, entering function calls
- **Step Over**: Executes until returning to the same or lower frame depth (skips function internals)
- **Step Out**: Executes until returning from the current function

### Variable Inspection

Local variables are shown with their names (if available from compilation metadata) or as `slot0`, `slot1`, etc. Global variables show all module-level definitions.

### Expression Evaluation

The `print` command can evaluate:
- Simple variables: `print x`
- Arithmetic expressions: `print x + y * 2`
- Function calls: `print myFunc(10)`
- List/array access: `print myList[0]`
- Record field access: `print myRecord.field`

The debugger first checks if the expression is a simple variable name and returns its value directly for performance. For complex expressions, it compiles and evaluates them in the current context with access to both local and global variables.

## Limitations

- Breakpoints only work at line granularity (not column-level)
- Debugging is only available for VM backend (not tree-walk interpreter)

## Future Improvements

- Conditional breakpoints
- Watch expressions
- Better variable formatting
- Integration with LSP for IDE debugging
- Enhanced expression evaluation with complex expressions

