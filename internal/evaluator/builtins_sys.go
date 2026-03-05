package evaluator

import (
	"bytes"
	"os"
	"os/exec"
	"github.com/funvibe/funxy/internal/config"
	"path/filepath"
	"strings"
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
// In compiled binary (bundle) mode: returns "" (empty string).
//
//	Resources are embedded — there is no script directory on disk.
//	pathJoin(["", "file"]) gives "file", which matches embed keys.
func builtinSysScriptDir(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("sysScriptDir expects 0 arguments, got %d", len(args))
	}

	// In bundle mode, there is no script on disk — return empty string.
	// pathJoin(["", "file.html"]) → "file.html" → matches embed key.
	if e.IsBundleMode {
		return stringToList("")
	}

	// Prefer runtime evaluation context when available.
	// This is reliable for modes like `funxy vmm ...` where os.Args[1] is a subcommand.
	currentFile := strings.TrimSpace(e.CurrentFile)
	baseDir := strings.TrimSpace(e.BaseDir)
	if currentFile != "" && currentFile != "<stdin>" && currentFile != "<eval>" && currentFile != "<script>" {
		if filepath.IsAbs(currentFile) {
			return stringToList(filepath.Dir(currentFile))
		}
		if baseDir != "" {
			if absBase, err := filepath.Abs(baseDir); err == nil {
				return stringToList(absBase)
			}
			return stringToList(filepath.Clean(baseDir))
		}
		if absFile, err := filepath.Abs(currentFile); err == nil {
			return stringToList(filepath.Dir(absFile))
		}
	}
	if baseDir != "" && baseDir != "." {
		if absBase, err := filepath.Abs(baseDir); err == nil {
			return stringToList(absBase)
		}
		return stringToList(filepath.Clean(baseDir))
	}

	// Fallback to argv parsing for contexts where evaluator metadata is unavailable.
	// Pick the first source/bytecode script-like argument.
	osArgs := os.Args
	for _, arg := range osArgs[1:] {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(strings.TrimSpace(arg)))
		if isKnownScriptExt(ext) {
			if absPath, err := filepath.Abs(arg); err == nil {
				return stringToList(filepath.Dir(absPath))
			}
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

func isKnownScriptExt(ext string) bool {
	if ext == ".fbc" {
		return true
	}
	for _, srcExt := range config.SourceFileExtensions {
		if ext == strings.ToLower(srcExt) {
			return true
		}
	}
	return false
}

// SetSysBuiltinTypes sets type info for sys builtins
