package evaluator

import (
	"bufio"
	"io"
	"os"
	"github.com/funvibe/funxy/internal/typesystem"
	"path/filepath"
	"sync"
)

// lookupEmbed checks embedded resources with path normalization.
// Normalizes "./file" → "file", "dir/../file" → "file", etc.
// Also converts backslashes to forward slashes for cross-platform consistency.
func lookupEmbed(resources map[string][]byte, path string) ([]byte, bool) {
	if resources == nil {
		return nil, false
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	data, found := resources[clean]
	return data, found
}

// hasEmbed checks if a path exists in embedded resources (with normalization).
func hasEmbed(resources map[string][]byte, path string) bool {
	_, found := lookupEmbed(resources, path)
	return found
}

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

// resetStdinReader resets the shared stdin reader. Used in tests when os.Stdin is swapped.
func resetStdinReader() {
	stdinReaderOnce = sync.Once{}
	stdinReader = nil
}

// IOBuiltins returns built-in functions for lib/io virtual package
func IOBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		// Stdin operations
		"readLine":     {Fn: builtinReadLine, Name: "readLine"},
		"readAll":      {Fn: builtinReadAll, Name: "readAll"},
		"readAllBytes": {Fn: builtinReadAllBytes, Name: "readAllBytes"},

		// File operations
		"fileRead":        {Fn: builtinReadFile, Name: "fileRead"},
		"fileReadBytes":   {Fn: builtinReadFileBytes, Name: "fileReadBytes"},
		"fileReadBytesAt": {Fn: builtinReadFileBytesAt, Name: "fileReadBytesAt"},
		"fileReadAt":      {Fn: builtinReadFileAt, Name: "fileReadAt"},
		"fileWrite":       {Fn: builtinWriteFile, Name: "fileWrite"},
		"fileAppend":      {Fn: builtinAppendFile, Name: "fileAppend"},
		"fileExists":      {Fn: builtinFileExists, Name: "fileExists"},
		"fileSize":        {Fn: builtinFileSize, Name: "fileSize"},
		"fileDelete":      {Fn: builtinDeleteFile, Name: "fileDelete"},

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

// readLine: () -> Option<String>
func builtinReadLine(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("readLine expects 0 arguments, got %d", len(args))
	}

	reader := getStdinReader()
	line, err := reader.ReadString('\n')
	if err != nil {
		// EOF or error
		return makeNone()
	}

	// Remove trailing newline
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	// Remove trailing carriage return (Windows)
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}

	return makeSome(stringToList(line))
}

// readAll: () -> String
// Reads all remaining data from stdin as a UTF-8 string
func builtinReadAll(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("readAll expects 0 arguments, got %d", len(args))
	}

	reader := getStdinReader()
	data, err := io.ReadAll(reader)
	if err != nil {
		return stringToList("")
	}

	return stringToList(string(data))
}

// readAllBytes: () -> Bytes
// Reads all remaining data from stdin as raw bytes
func builtinReadAllBytes(e *Evaluator, args ...Object) Object {
	if len(args) != 0 {
		return newError("readAllBytes expects 0 arguments, got %d", len(args))
	}

	reader := getStdinReader()
	data, err := io.ReadAll(reader)
	if err != nil {
		return BytesFromSlice([]byte{})
	}

	return BytesFromSlice(data)
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
	path := ListToString(pathList)

	// Check embedded resources first (for self-contained binaries built with --embed)
	if data, found := lookupEmbed(e.EmbeddedResources, path); found {
		return makeOk(stringToList(string(data)))
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(stringToList(string(content)))
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
	path := ListToString(pathList)

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

	// Check embedded resources first
	if data, found := lookupEmbed(e.EmbeddedResources, path); found {
		end := offset + length
		if offset >= int64(len(data)) {
			return makeOk(stringToList(""))
		}
		if end > int64(len(data)) {
			end = int64(len(data))
		}
		return makeOk(stringToList(string(data[offset:end])))
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

	return makeOk(stringToList(string(buffer[:n])))
}

// readFileBytes: (String) -> Result<String, Bytes>
func builtinReadFileBytes(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("readFileBytes expects 1 argument, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("readFileBytes expects a string path, got %s", args[0].Type())
	}
	path := ListToString(pathList)

	// Check embedded resources first (for self-contained binaries built with --embed)
	if data, found := lookupEmbed(e.EmbeddedResources, path); found {
		return makeOk(BytesFromSlice(data))
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(BytesFromSlice(content))
}

// readFileBytesAt: (String, Int, Int) -> Result<String, Bytes>
func builtinReadFileBytesAt(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("readFileBytesAt expects 3 arguments, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("readFileBytesAt expects a string path, got %s", args[0].Type())
	}
	path := ListToString(pathList)

	offsetArg, ok := args[1].(*Integer)
	if !ok {
		return newError("readFileBytesAt expects an integer offset, got %s", args[1].Type())
	}
	offset := offsetArg.Value

	lengthArg, ok := args[2].(*Integer)
	if !ok {
		return newError("readFileBytesAt expects an integer length, got %s", args[2].Type())
	}
	length := lengthArg.Value

	if offset < 0 {
		return makeFailStr("offset cannot be negative")
	}
	if length < 0 {
		return makeFailStr("length cannot be negative")
	}

	// Check embedded resources first
	if data, found := lookupEmbed(e.EmbeddedResources, path); found {
		end := offset + length
		if offset >= int64(len(data)) {
			return makeOk(BytesFromSlice([]byte{}))
		}
		if end > int64(len(data)) {
			end = int64(len(data))
		}
		return makeOk(BytesFromSlice(data[offset:end]))
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

	return makeOk(BytesFromSlice(buffer[:n]))
}

// writeFile: (String, String | Bytes) -> Result<String, Int>
func builtinWriteFile(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("writeFile expects 2 arguments, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("writeFile expects a string path, got %s", args[0].Type())
	}
	path := ListToString(pathList)

	content, err := GetContentBytes(args[1])
	if err != nil {
		return makeFailStr(err.Error())
	}

	err = os.WriteFile(path, content, 0644)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Integer{Value: int64(len(content))})
}

// appendFile: (String, String | Bytes) -> Result<String, Int>
func builtinAppendFile(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("appendFile expects 2 arguments, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("appendFile expects a string path, got %s", args[0].Type())
	}
	path := ListToString(pathList)

	content, err := GetContentBytes(args[1])
	if err != nil {
		return makeFailStr(err.Error())
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return makeFailStr(err.Error())
	}
	defer func() { _ = file.Close() }()

	n, err := file.Write(content)
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
	path := ListToString(pathList)

	// Check embedded resources first
	if hasEmbed(e.EmbeddedResources, path) {
		return TRUE
	}

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return FALSE
	}
	return TRUE
}

// fileSize: (String) -> Result<String, Int>
func builtinFileSize(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("fileSize expects 1 argument, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("fileSize expects a string path, got %s", args[0].Type())
	}
	path := ListToString(pathList)

	// Check embedded resources first
	if data, found := lookupEmbed(e.EmbeddedResources, path); found {
		return makeOk(&Integer{Value: int64(len(data))})
	}

	info, err := os.Stat(path)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Integer{Value: info.Size()})
}

// fileDelete: (String) -> Result<String, Nil>
func builtinDeleteFile(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("deleteFile expects 1 argument, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("deleteFile expects a string path, got %s", args[0].Type())
	}
	path := ListToString(pathList)

	err := os.Remove(path)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Nil{})
}

// ============================================================================
// Directory operations
// ============================================================================

// dirCreate: (String) -> Result<String, Nil>
func builtinDirCreate(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dirCreate expects 1 argument, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("dirCreate expects a string path, got %s", args[0].Type())
	}
	path := ListToString(pathList)

	err := os.Mkdir(path, 0755)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Nil{})
}

// dirCreateAll: (String) -> Result<String, Nil>
func builtinDirCreateAll(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dirCreateAll expects 1 argument, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("dirCreateAll expects a string path, got %s", args[0].Type())
	}
	path := ListToString(pathList)

	err := os.MkdirAll(path, 0755)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Nil{})
}

// dirRemove: (String) -> Result<String, Nil>
func builtinDirRemove(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dirRemove expects 1 argument, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("dirRemove expects a string path, got %s", args[0].Type())
	}
	path := ListToString(pathList)

	err := os.Remove(path)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Nil{})
}

// dirRemoveAll: (String) -> Result<String, Nil>
func builtinDirRemoveAll(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dirRemoveAll expects 1 argument, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("dirRemoveAll expects a string path, got %s", args[0].Type())
	}
	path := ListToString(pathList)

	err := os.RemoveAll(path)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(&Nil{})
}

// dirList: (String) -> Result<String, List<String>>
func builtinDirList(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("dirList expects 1 argument, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("dirList expects a string path, got %s", args[0].Type())
	}
	path := ListToString(pathList)

	entries, err := os.ReadDir(path)
	if err != nil {
		return makeFailStr(err.Error())
	}

	names := make([]Object, len(entries))
	for i, entry := range entries {
		names[i] = stringToList(entry.Name())
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
	path := ListToString(pathList)

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
	path := ListToString(pathList)

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
	path := ListToString(pathList)

	// Embedded resources are always "files"
	if hasEmbed(e.EmbeddedResources, path) {
		return TRUE
	}

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
	stringType := typesystem.String
	// String | Bytes
	stringOrBytes := typesystem.StringOrBytes

	// Result<String, Bytes>
	resultBytes := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, typesystem.Bytes},
	}

	resultString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, stringType},
	}
	// Result<String, Int>
	resultInt := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, typesystem.Int},
	}
	// Result<String, Nil>
	resultNil := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, typesystem.Nil},
	}
	optionString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{stringType},
	}

	listString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{stringType},
	}
	// Result<String, List<String>>
	resultListString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{stringType, listString},
	}

	types := map[string]typesystem.Type{
		// Stdin operations
		"readLine":     typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: optionString},
		"readAll":      typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: stringType},
		"readAllBytes": typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: typesystem.Bytes},

		// File operations
		"fileRead":        typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultString},
		"fileReadBytes":   typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultBytes},
		"fileReadBytesAt": typesystem.TFunc{Params: []typesystem.Type{stringType, typesystem.Int, typesystem.Int}, ReturnType: resultBytes},
		"fileReadAt":      typesystem.TFunc{Params: []typesystem.Type{stringType, typesystem.Int, typesystem.Int}, ReturnType: resultString},
		"fileWrite":       typesystem.TFunc{Params: []typesystem.Type{stringType, stringOrBytes}, ReturnType: resultInt},
		"fileAppend":      typesystem.TFunc{Params: []typesystem.Type{stringType, stringOrBytes}, ReturnType: resultInt},
		"fileExists":      typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: typesystem.Bool},
		"fileSize":        typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultInt},
		"fileDelete":      typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultNil},

		// Directory operations
		"dirCreate":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultNil},
		"dirCreateAll": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultNil},
		"dirRemove":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultNil},
		"dirRemoveAll": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultNil},
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
