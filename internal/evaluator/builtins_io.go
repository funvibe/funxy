package evaluator

import (
	"bufio"
	"os"
	"github.com/funvibe/funxy/internal/typesystem"
	"sync"
)

// stdinReader is a shared buffered reader for stdin to avoid buffering issues
// when readLine is called multiple times
var (
	stdinReader     *bufio.Reader
	stdinReaderOnce sync.Once
)

func getStdinReader() *bufio.Reader {
	stdinReaderOnce.Do(func() {
		stdinReader = bufio.NewReader(os.Stdin)
	})
	return stdinReader
}

// IOBuiltins returns built-in functions for lib/io virtual package
func IOBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		// File operations
		"readLine":   {Fn: builtinReadLine, Name: "readLine"},
		"fileRead":   {Fn: builtinReadFile, Name: "fileRead"},
		"fileReadAt": {Fn: builtinReadFileAt, Name: "fileReadAt"},
		"fileWrite":  {Fn: builtinWriteFile, Name: "fileWrite"},
		"fileAppend": {Fn: builtinAppendFile, Name: "fileAppend"},
		"fileExists": {Fn: builtinFileExists, Name: "fileExists"},
		"fileSize":   {Fn: builtinFileSize, Name: "fileSize"},
		"fileDelete": {Fn: builtinDeleteFile, Name: "fileDelete"},

		// Directory operations
		"dirCreate":    {Fn: builtinDirCreate, Name: "dirCreate"},
		"dirCreateAll": {Fn: builtinDirCreateAll, Name: "dirCreateAll"},
		"dirRemove":    {Fn: builtinDirRemove, Name: "dirRemove"},
		"dirRemoveAll": {Fn: builtinDirRemoveAll, Name: "dirRemoveAll"},
		"dirList":      {Fn: builtinDirList, Name: "dirList"},
		"dirExists":    {Fn: builtinDirExists, Name: "dirExists"},

		// Path type checks
		"isDir":  {Fn: builtinIsDir, Name: "isDir"},
		"isFile": {Fn: builtinIsFile, Name: "isFile"},
	}
}

// stringToCharList converts a Go string to List<Char>
func stringToCharList(s string) []Object {
	chars := make([]Object, 0, len(s))
	for _, r := range s {
		chars = append(chars, &Char{Value: int64(r)})
	}
	return chars
}

// charListToString converts List<Char> to Go string
func charListToString(list *List) string {
	runes := make([]rune, list.len())
	for i, elem := range list.ToSlice() {
		if ch, ok := elem.(*Char); ok {
			runes[i] = rune(ch.Value)
		}
	}
	return string(runes)
}

// readLine: () -> Option<String>
func builtinReadLine(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("readLine expects 0 arguments, got %d", len(args))
	}

	reader := getStdinReader()
	line, err := reader.ReadString('\n')
	if err != nil {
		// EOF or error
		return makeZero()
	}

	// Remove trailing newline
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	// Remove trailing carriage return (Windows)
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}

	return makeSome(newList(stringToCharList(line)))
}

// readFile: (String) -> Result<String, String>
func builtinReadFile(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("readFile expects 1 argument, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("readFile expects a string path, got %s", args[0].Type())
	}
	path := charListToString(pathList)

	content, err := os.ReadFile(path)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(newList(stringToCharList(string(content))))
}

// readFileAt: (String, Int, Int) -> Result<String, String>
func builtinReadFileAt(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("readFileAt expects 3 arguments, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("readFileAt expects a string path, got %s", args[0].Type())
	}
	path := charListToString(pathList)

	offsetArg, ok := args[1].(*Integer)
	if !ok {
		return newError("readFileAt expects an integer offset, got %s", args[1].Type())
	}
	offset := offsetArg.Value

	lengthArg, ok := args[2].(*Integer)
	if !ok {
		return newError("readFileAt expects an integer length, got %s", args[2].Type())
	}
	length := lengthArg.Value

	if offset < 0 {
		return makeFailStr("offset cannot be negative")
	}
	if length < 0 {
		return makeFailStr("length cannot be negative")
	}

	file, err := os.Open(path)
	if err != nil {
		return makeFailStr(err.Error())
	}
	defer func() { _ = file.Close() }()

	_, err = file.Seek(offset, 0)
	if err != nil {
		return makeFailStr(err.Error())
	}

	buffer := make([]byte, length)
	n, err := file.Read(buffer)
	if err != nil && n == 0 {
		return makeFailStr(err.Error())
	}

	return makeOk(newList(stringToCharList(string(buffer[:n]))))
}

// writeFile: (String, String) -> Result<Int, String>
func builtinWriteFile(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("writeFile expects 2 arguments, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("writeFile expects a string path, got %s", args[0].Type())
	}
	path := charListToString(pathList)

	contentList, ok := args[1].(*List)
	if !ok {
		return newError("writeFile expects a string content, got %s", args[1].Type())
	}
	content := charListToString(contentList)

	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Integer{Value: int64(len(content))})
}

// appendFile: (String, String) -> Result<Int, String>
func builtinAppendFile(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("appendFile expects 2 arguments, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("appendFile expects a string path, got %s", args[0].Type())
	}
	path := charListToString(pathList)

	contentList, ok := args[1].(*List)
	if !ok {
		return newError("appendFile expects a string content, got %s", args[1].Type())
	}
	content := charListToString(contentList)

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return makeFailStr(err.Error())
	}
	defer func() { _ = file.Close() }()

	n, err := file.WriteString(content)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Integer{Value: int64(n)})
}

// fileExists: (String) -> Bool
func builtinFileExists(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("fileExists expects 1 argument, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("fileExists expects a string path, got %s", args[0].Type())
	}
	path := charListToString(pathList)

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return FALSE
	}
	return TRUE
}

// fileSize: (String) -> Result<Int, String>
func builtinFileSize(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("fileSize expects 1 argument, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("fileSize expects a string path, got %s", args[0].Type())
	}
	path := charListToString(pathList)

	info, err := os.Stat(path)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Integer{Value: info.Size()})
}

// deleteFile: (String) -> Result<Nil, String>
func builtinDeleteFile(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("deleteFile expects 1 argument, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("deleteFile expects a string path, got %s", args[0].Type())
	}
	path := charListToString(pathList)

	err := os.Remove(path)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Nil{})
}

// ============================================================================
// Directory operations
// ============================================================================

// dirCreate: (String) -> Result<Nil, String>
func builtinDirCreate(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dirCreate expects 1 argument, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("dirCreate expects a string path, got %s", args[0].Type())
	}
	path := charListToString(pathList)

	err := os.Mkdir(path, 0755)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Nil{})
}

// dirCreateAll: (String) -> Result<Nil, String>
func builtinDirCreateAll(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dirCreateAll expects 1 argument, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("dirCreateAll expects a string path, got %s", args[0].Type())
	}
	path := charListToString(pathList)

	err := os.MkdirAll(path, 0755)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Nil{})
}

// dirRemove: (String) -> Result<Nil, String>
func builtinDirRemove(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dirRemove expects 1 argument, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("dirRemove expects a string path, got %s", args[0].Type())
	}
	path := charListToString(pathList)

	err := os.Remove(path)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Nil{})
}

// dirRemoveAll: (String) -> Result<Nil, String>
func builtinDirRemoveAll(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dirRemoveAll expects 1 argument, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("dirRemoveAll expects a string path, got %s", args[0].Type())
	}
	path := charListToString(pathList)

	err := os.RemoveAll(path)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Nil{})
}

// dirList: (String) -> Result<List<String>, String>
func builtinDirList(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dirList expects 1 argument, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("dirList expects a string path, got %s", args[0].Type())
	}
	path := charListToString(pathList)

	entries, err := os.ReadDir(path)
	if err != nil {
		return makeFailStr(err.Error())
	}

	names := make([]Object, len(entries))
	for i, entry := range entries {
		names[i] = newList(stringToCharList(entry.Name()))
	}

	return makeOk(newList(names))
}

// dirExists: (String) -> Bool
func builtinDirExists(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dirExists expects 1 argument, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("dirExists expects a string path, got %s", args[0].Type())
	}
	path := charListToString(pathList)

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return FALSE
	}
	if err != nil {
		return FALSE
	}

	if info.IsDir() {
		return TRUE
	}
	return FALSE
}

// isDir: (String) -> Bool
func builtinIsDir(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("isDir expects 1 argument, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("isDir expects a string path, got %s", args[0].Type())
	}
	path := charListToString(pathList)

	info, err := os.Stat(path)
	if err != nil {
		return FALSE
	}

	if info.IsDir() {
		return TRUE
	}
	return FALSE
}

// isFile: (String) -> Bool
func builtinIsFile(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("isFile expects 1 argument, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("isFile expects a string path, got %s", args[0].Type())
	}
	path := charListToString(pathList)

	info, err := os.Stat(path)
	if err != nil {
		return FALSE
	}

	if info.Mode().IsRegular() {
		return TRUE
	}
	return FALSE
}

// SetIOBuiltinTypes sets type info for io builtins
func SetIOBuiltinTypes(builtins map[string]*Builtin) {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}
	resultStringString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, stringType},
	}
	resultIntString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{typesystem.Int, stringType},
	}
	resultNilString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{typesystem.Nil, stringType},
	}
	optionString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{stringType},
	}

	listString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{stringType},
	}
	resultListString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, listString},
	}

	types := map[string]typesystem.Type{
		// File operations
		"readLine":   typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: optionString},
		"fileRead":   typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultStringString},
		"fileReadAt": typesystem.TFunc{Params: []typesystem.Type{stringType, typesystem.Int, typesystem.Int}, ReturnType: resultStringString},
		"fileWrite":  typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: resultIntString},
		"fileAppend": typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: resultIntString},
		"fileExists": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Bool},
		"fileSize":   typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultIntString},
		"fileDelete": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultNilString},

		// Directory operations
		"dirCreate":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultNilString},
		"dirCreateAll": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultNilString},
		"dirRemove":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultNilString},
		"dirRemoveAll": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultNilString},
		"dirList":      typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultListString},
		"dirExists":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Bool},

		// Path type checks
		"isDir":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Bool},
		"isFile": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Bool},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}

