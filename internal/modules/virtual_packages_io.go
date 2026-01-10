package modules

import (
	"github.com/funvibe/funxy/internal/typesystem"
)

func initTimePackage() {
	pkg := &VirtualPackage{
		Name: "time",
		Symbols: map[string]typesystem.Type{
			// Unix timestamp
			"timeNow": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Int},
			// Monotonic clocks for benchmarking
			"clockNs": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Int},
			"clockMs": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Int},
			// Sleep functions
			"sleep":   typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: typesystem.Nil},
			"sleepMs": typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: typesystem.Nil},
		},
	}
	RegisterVirtualPackage("lib/time", pkg)
}

// initIOPackage registers the lib/io virtual package
func initIOPackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{typesystem.Char},
	}
	// List<String>
	listString := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{stringType},
	}
	// Result<E, A> - E is error type, A is success type (like Haskell Either)
	// Result<String, String>
	resultStringString := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, stringType},
	}
	// Result<String, Int> - error is String, success is Int
	resultStringInt := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, typesystem.Int},
	}
	// Result<String, Nil> - error is String, success is Nil
	resultStringNil := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, typesystem.Nil},
	}
	// Result<String, List<String>>
	resultStringListString := typesystem.TApp{
		Constructor: ResultCon,
		Args:        []typesystem.Type{stringType, listString},
	}
	// Option<String>
	optionString := typesystem.TApp{
		Constructor: OptionCon,
		Args:        []typesystem.Type{stringType},
	}
	pkg := &VirtualPackage{
		Name: "io",
		Symbols: map[string]typesystem.Type{
			// Console
			"readLine": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: optionString},
			// File reading
			"fileRead":   typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringString},
			"fileReadAt": typesystem.TFunc{Params: []typesystem.Type{stringType, typesystem.Int, typesystem.Int}, ReturnType: resultStringString},
			// File writing
			"fileWrite":  typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: resultStringInt},
			"fileAppend": typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: resultStringInt},
			// File info
			"fileExists": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Bool},
			"fileSize":   typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringInt},
			// File management
			"fileDelete": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringNil},
			// Directory operations
			"dirCreate":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringNil},
			"dirCreateAll": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringNil},
			"dirRemove":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringNil},
			"dirRemoveAll": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringNil},
			"dirList":      typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringListString},
			"dirExists":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Bool},
			// Path type checks
			"isDir":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Bool},
			"isFile": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Bool},
		},
	}
	RegisterVirtualPackage("lib/io", pkg)
}

// initSysPackage registers the lib/sys virtual package
func initSysPackage() {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{typesystem.Char},
	}
	// List<String>
	listString := typesystem.TApp{
		Constructor: ListCon,
		Args:        []typesystem.Type{stringType},
	}
	// Option<String>
	optionString := typesystem.TApp{
		Constructor: OptionCon,
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
	pkg := &VirtualPackage{
		Name: "sys",
		Symbols: map[string]typesystem.Type{
			// Command line arguments
			"sysArgs": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: listString},
			// Environment variable
			"sysEnv": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: optionString},
			// Exit with code
			"sysExit": typesystem.TFunc{Params: []typesystem.Type{typesystem.Int}, ReturnType: typesystem.Nil},
			// Execute command: exec(cmd: String, args: List<String>) -> { code: Int, stdout: String, stderr: String }
			"sysExec": typesystem.TFunc{Params: []typesystem.Type{stringType, listString}, ReturnType: execResultType},
		},
	}
	RegisterVirtualPackage("lib/sys", pkg)
}
