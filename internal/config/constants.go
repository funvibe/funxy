package config

// Version is the current Funxy version.
// Set at build time by prepare_release.sh via -ldflags or by writing to this file.
var Version = "0.6.5"

const SourceFileExt = ".lang"

// SourceFileExtensions are all recognized source file extensions
var SourceFileExtensions = []string{".lang", ".funxy", ".fx"}

// TrimSourceExt removes any recognized source extension from a filename.
// Returns the original string if no extension matches.
func TrimSourceExt(name string) string {
	for _, ext := range SourceFileExtensions {
		if len(name) >= len(ext) && name[len(name)-len(ext):] == ext {
			return name[:len(name)-len(ext)]
		}
	}
	return name
}

// HasSourceExt returns true if the path ends with any recognized source extension.
func HasSourceExt(path string) bool {
	for _, ext := range SourceFileExtensions {
		if len(path) >= len(ext) && path[len(path)-len(ext):] == ext {
			return true
		}
	}
	return false
}

// IsTestMode indicates if the program is running in test mode.
// This is set once at startup in main.go when handling test command.
var IsTestMode = false

// IsLSPMode indicates if the program is running in Language Server Protocol mode.
// This is set in cmd/lsp/main.go.
var IsLSPMode = false

// Built-in trait and method names
const (
	IterTraitName  = "Iter"
	IterMethodName = "iter"
)

// Built-in function names
const (
	PrintFuncName    = "print"
	WriteFuncName    = "write"
	PanicFuncName    = "panic"
	DebugFuncName    = "debug"
	TraceFuncName    = "trace"
	LenFuncName      = "len"
	LenBytesFuncName = "lenBytes"
	TypeOfFuncName   = "typeOf"
	GetTypeFuncName  = "getType"
	DefaultFuncName  = "default"
	ShowFuncName     = "show"
	ReadFuncName     = "read"
	IdFuncName       = "id"
	ConstFuncName    = "constant"
)

// Built-in type names
const (
	ListTypeName   = "List"
	MapTypeName    = "Map"
	BytesTypeName  = "Bytes"
	BitsTypeName   = "Bits"
	OptionTypeName = "Option"
	ResultTypeName = "Result"
	SomeCtorName   = "Some"
	NoneCtorName   = "None"
	OkCtorName     = "Ok"
	FailCtorName   = "Fail"
)
