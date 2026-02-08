package config

const SourceFileExt = ".lang"

// SourceFileExtensions are all recognized source file extensions
var SourceFileExtensions = []string{".lang", ".funxy", ".fx"}

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
