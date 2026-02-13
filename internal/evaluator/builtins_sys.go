package evaluator

import (
	"bytes"
	"os"
	"os/exec"
	"github.com/funvibe/funxy/internal/typesystem"
	"path/filepath"
)

// SysBuiltins returns built-in functions for lib/sys virtual package
func SysBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		"sysArgs":      {Fn: builtinArgs, Name: "sysArgs"},
		"sysEnv":       {Fn: builtinEnv, Name: "sysEnv"},
		"sysExit":      {Fn: builtinExit, Name: "sysExit"},
		"sysExec":      {Fn: builtinExec, Name: "sysExec"},
		"sysExePath":   {Fn: builtinSysExePath, Name: "sysExePath"},
		"sysScriptDir": {Fn: builtinSysScriptDir, Name: "sysScriptDir"},
	}
}

// args: () -> List<String>
// Returns command line arguments (excluding program name)
func builtinArgs(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("args expects 0 arguments, got %d", len(args))
	}

	// os.Args[0] is the program name, skip it
	osArgs := os.Args
	if len(osArgs) > 1 {
		osArgs = osArgs[1:]
	} else {
		osArgs = []string{}
	}

	elements := make([]Object, len(osArgs))
	for i, arg := range osArgs {
		elements[i] = stringToList(arg)
	}

	return newList(elements)
}

// env: (String) -> Option<String>
// Returns environment variable value or None if not set
func builtinEnv(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("env expects 1 argument, got %d", len(args))
	}

	nameList, ok := args[0].(*List)
	if !ok {
		return newError("env expects a string argument, got %s", args[0].Type())
	}

	name := listToString(nameList)
	value, exists := os.LookupEnv(name)
	if !exists {
		return makeNone()
	}

	return makeSome(stringToList(value))
}

// exit: (Int) -> Nil
// Exits the program with the given status code
func builtinExit(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("exit expects 1 argument, got %d", len(args))
	}

	code, ok := args[0].(*Integer)
	if !ok {
		return newError("exit expects an integer argument, got %s", args[0].Type())
	}

	os.Exit(int(code.Value))
	return &Nil{} // unreachable
}

// exec: (String, List<String>) -> { code: Int, stdout: String, stderr: String }
// Executes a command with arguments and returns the result
func builtinExec(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("exec expects 2 arguments, got %d", len(args))
	}

	// Get command name
	cmdList, ok := args[0].(*List)
	if !ok {
		return newError("exec expects a string as first argument, got %s", args[0].Type())
	}
	cmdName := listToString(cmdList)

	// Get command arguments
	argsList, ok := args[1].(*List)
	if !ok {
		return newError("exec expects a list of strings as second argument, got %s", args[1].Type())
	}

	cmdArgs := make([]string, argsList.len())
	for i, arg := range argsList.ToSlice() {
		argList, ok := arg.(*List)
		if !ok {
			return newError("exec argument %d is not a string", i)
		}
		cmdArgs[i] = listToString(argList)
	}

	// Execute command
	cmd := exec.Command(cmdName, cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			// Command failed to start
			return NewRecord(map[string]Object{
				"code":   &Integer{Value: -1},
				"stdout": stringToList(""),
				"stderr": stringToList(err.Error()),
			})
		}
	}

	return NewRecord(map[string]Object{
		"code":   &Integer{Value: int64(exitCode)},
		"stdout": stringToList(stdout.String()),
		"stderr": stringToList(stderr.String()),
	})
}

// sysExePath: () -> String
// Returns the absolute path of the current executable.
// Useful for self-contained binaries that need to invoke themselves as interpreters.
func builtinSysExePath(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("sysExePath expects 0 arguments, got %d", len(args))
	}

	exePath, err := os.Executable()
	if err != nil {
		return newError("sysExePath: %s", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return newError("sysExePath: %s", err)
	}

	return stringToList(exePath)
}

// sysScriptDir: () -> String
// Returns the directory of the currently running script.
// In interpreted mode: directory of the .lang file being executed.
// In built binary mode: directory of the executable.
func builtinSysScriptDir(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("sysScriptDir expects 0 arguments, got %d", len(args))
	}

	// Try to get the script path from sysArgs (first arg is the script path)
	osArgs := os.Args
	if len(osArgs) > 1 {
		scriptPath := osArgs[1]
		absPath, err := filepath.Abs(scriptPath)
		if err == nil {
			return stringToList(filepath.Dir(absPath))
		}
	}

	// Fallback: directory of the executable itself
	exePath, err := os.Executable()
	if err != nil {
		return newError("sysScriptDir: %s", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return newError("sysScriptDir: %s", err)
	}

	return stringToList(filepath.Dir(exePath))
}

// SetSysBuiltinTypes sets type info for sys builtins
func SetSysBuiltinTypes(builtins map[string]*Builtin) {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}
	// List<String>
	listString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{stringType},
	}
	// Option<String>
	optionString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{stringType},
	}
	// ExecResult = { code: Int, stdout: String, stderr: String }
	execResultType := typesystem.TRecord{
		Fields: map[string]typesystem.Type{
			"code":   typesystem.Int,
			"stdout": stringType,
			"stderr": stringType,
		},
	}

	types := map[string]typesystem.Type{
		"sysArgs":      typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: listString},
		"sysEnv":       typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: optionString},
		"sysExit":      typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: typesystem.Nil},
		"sysExec":      typesystem.TFunc{Params: []typesystem.Type{stringType, listString}, ReturnType: execResultType},
		"sysExePath":   typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: stringType},
		"sysScriptDir": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: stringType},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}
