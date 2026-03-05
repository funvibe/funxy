package evaluator

import (
	"os"
	"path/filepath"
	"strings"
)

// PathBuiltins returns all path-related built-in functions
func PathBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		// Parsing
		"pathJoin":  {Fn: builtinPathJoin, Name: "pathJoin"},
		"pathSplit": {Fn: builtinPathSplit, Name: "pathSplit"},
		"pathDir":   {Fn: builtinPathDir, Name: "pathDir"},
		"pathBase":  {Fn: builtinPathBase, Name: "pathBase"},
		"pathExt":   {Fn: builtinPathExt, Name: "pathExt"},
		"pathStem":  {Fn: builtinPathStem, Name: "pathStem"},

		// Manipulation
		"pathWithExt":  {Fn: builtinPathWithExt, Name: "pathWithExt"},
		"pathWithBase": {Fn: builtinPathWithBase, Name: "pathWithBase"},

		// Query
		"pathIsAbs": {Fn: builtinPathIsAbs, Name: "pathIsAbs"},
		"pathIsRel": {Fn: builtinPathIsRel, Name: "pathIsRel"},

		// Normalization
		"pathClean": {Fn: builtinPathClean, Name: "pathClean"},
		"pathAbs":   {Fn: builtinPathAbs, Name: "pathAbs"},
		"pathRel":   {Fn: builtinPathRel, Name: "pathRel"},

		// Matching
		"pathMatch": {Fn: builtinPathMatch, Name: "pathMatch"},

		// Separator
		"pathSep": {Fn: builtinPathSep, Name: "pathSep"},

		// Temp directory
		"pathTemp": {Fn: builtinPathTemp, Name: "pathTemp"},

		// POSIX-style (handles dotfiles correctly)
		"pathExtPosix":  {Fn: builtinPathExtPosix, Name: "pathExtPosix"},
		"pathStemPosix": {Fn: builtinPathStemPosix, Name: "pathStemPosix"},
		"pathIsHidden":  {Fn: builtinPathIsHidden, Name: "pathIsHidden"},
	}
}

// pathJoin: List<String> -> String
func builtinPathJoin(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("pathJoin expects 1 argument, got %d", len(args))
	}

	list, ok := args[0].(*List)
	if !ok {
		return newError("pathJoin expects a list, got %s", args[0].Type())
	}

	parts := make([]string, list.len())
	for i := 0; i < list.len(); i++ {
		part, ok := list.get(i).(*List)
		if !ok {
			return newError("pathJoin expects list of strings")
		}
		parts[i] = listToString(part)
	}

	return stringToList(filepath.Join(parts...))
}

// pathSplit: String -> List<String>
func builtinPathSplit(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("pathSplit expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("pathSplit expects a string, got %s", args[0].Type())
	}

	path := listToString(str)
	// Clean path first
	path = filepath.Clean(path)

	// Split by separator
	var parts []string
	if filepath.IsAbs(path) {
		// Handle absolute path - keep root
		vol := filepath.VolumeName(path)
		if vol != "" {
			parts = append(parts, vol)
			path = path[len(vol):]
		}
		if len(path) > 0 && path[0] == filepath.Separator {
			parts = append(parts, string(filepath.Separator))
			path = path[1:]
		}
	}

	// Split remaining path
	if path != "" && path != "." {
		for _, p := range strings.Split(path, string(filepath.Separator)) {
			if p != "" {
				parts = append(parts, p)
			}
		}
	}

	result := make([]Object, len(parts))
	for i, p := range parts {
		result[i] = stringToList(p)
	}

	return newList(result)
}

// pathDir: String -> String
func builtinPathDir(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("pathDir expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("pathDir expects a string, got %s", args[0].Type())
	}

	return stringToList(filepath.Dir(listToString(str)))
}

// pathBase: String -> String
func builtinPathBase(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("pathBase expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("pathBase expects a string, got %s", args[0].Type())
	}

	return stringToList(filepath.Base(listToString(str)))
}

// pathExt: String -> String
func builtinPathExt(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("pathExt expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("pathExt expects a string, got %s", args[0].Type())
	}

	return stringToList(filepath.Ext(listToString(str)))
}

// pathStem: String -> String (filename without extension)
func builtinPathStem(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("pathStem expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("pathStem expects a string, got %s", args[0].Type())
	}

	path := listToString(str)
	base := filepath.Base(path)
	ext := filepath.Ext(base)

	if ext != "" {
		return stringToList(base[:len(base)-len(ext)])
	}
	return stringToList(base)
}

// pathWithExt: String, String -> String
func builtinPathWithExt(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("pathWithExt expects 2 arguments, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("pathWithExt expects a string path, got %s", args[0].Type())
	}

	extList, ok := args[1].(*List)
	if !ok {
		return newError("pathWithExt expects a string extension, got %s", args[1].Type())
	}

	path := listToString(pathList)
	newExt := listToString(extList)

	// Remove old extension
	oldExt := filepath.Ext(path)
	if oldExt != "" {
		path = path[:len(path)-len(oldExt)]
	}

	// Add new extension (ensure it starts with .)
	if newExt != "" && !strings.HasPrefix(newExt, ".") {
		newExt = "." + newExt
	}

	return stringToList(path + newExt)
}

// pathWithBase: String, String -> String
func builtinPathWithBase(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("pathWithBase expects 2 arguments, got %d", len(args))
	}

	pathList, ok := args[0].(*List)
	if !ok {
		return newError("pathWithBase expects a string path, got %s", args[0].Type())
	}

	baseList, ok := args[1].(*List)
	if !ok {
		return newError("pathWithBase expects a string base, got %s", args[1].Type())
	}

	path := listToString(pathList)
	newBase := listToString(baseList)

	dir := filepath.Dir(path)
	return stringToList(filepath.Join(dir, newBase))
}

// pathIsAbs: String -> Bool
func builtinPathIsAbs(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("pathIsAbs expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("pathIsAbs expects a string, got %s", args[0].Type())
	}

	if filepath.IsAbs(listToString(str)) {
		return TRUE
	}
	return FALSE
}

// pathIsRel: String -> Bool
func builtinPathIsRel(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("pathIsRel expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("pathIsRel expects a string, got %s", args[0].Type())
	}

	if !filepath.IsAbs(listToString(str)) {
		return TRUE
	}
	return FALSE
}

// pathClean: String -> String
func builtinPathClean(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("pathClean expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("pathClean expects a string, got %s", args[0].Type())
	}

	return stringToList(filepath.Clean(listToString(str)))
}

// pathAbs: String -> Result<String, String>
func builtinPathAbs(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("pathAbs expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("pathAbs expects a string, got %s", args[0].Type())
	}

	abs, err := filepath.Abs(listToString(str))
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(stringToList(abs))
}

// pathRel: String, String -> Result<String, String>
func builtinPathRel(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("pathRel expects 2 arguments, got %d", len(args))
	}

	baseList, ok := args[0].(*List)
	if !ok {
		return newError("pathRel expects a string base, got %s", args[0].Type())
	}

	targetList, ok := args[1].(*List)
	if !ok {
		return newError("pathRel expects a string target, got %s", args[1].Type())
	}

	base := listToString(baseList)
	target := listToString(targetList)

	rel, err := filepath.Rel(base, target)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOk(stringToList(rel))
}

// pathMatch: String, String -> Result<String, Bool>
func builtinPathMatch(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("pathMatch expects 2 arguments, got %d", len(args))
	}

	patternList, ok := args[0].(*List)
	if !ok {
		return newError("pathMatch expects a string pattern, got %s", args[0].Type())
	}

	nameList, ok := args[1].(*List)
	if !ok {
		return newError("pathMatch expects a string name, got %s", args[1].Type())
	}

	pattern := listToString(patternList)
	name := listToString(nameList)

	matched, err := filepath.Match(pattern, name)
	if err != nil {
		return makeFailStr(err.Error())
	}

	if matched {
		return makeOk(TRUE)
	}
	return makeOk(FALSE)
}

// pathSep: () -> String
func builtinPathSep(e *Evaluator, args ...Object) Object {
	return stringToList(string(os.PathSeparator))
}

// pathTemp: () -> String
// Returns the default directory for temporary files (cross-platform)
func builtinPathTemp(e *Evaluator, args ...Object) Object {
	return stringToList(os.TempDir())
}

// pathExtPosix: String -> String (POSIX-style, handles dotfiles)
func builtinPathExtPosix(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("pathExtPosix expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("pathExtPosix expects a string, got %s", args[0].Type())
	}

	path := listToString(str)
	base := filepath.Base(path)

	// Handle dotfiles: .gitignore, .bashrc, etc. have no extension
	if strings.HasPrefix(base, ".") {
		// Find last dot after the first character
		rest := base[1:]
		idx := strings.LastIndex(rest, ".")
		if idx == -1 {
			return stringToList("") // No extension for dotfiles like .gitignore
		}
		return stringToList(rest[idx:])
	}

	return stringToList(filepath.Ext(path))
}

// pathStemPosix: String -> String (POSIX-style, handles dotfiles)
func builtinPathStemPosix(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("pathStemPosix expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("pathStemPosix expects a string, got %s", args[0].Type())
	}

	path := listToString(str)
	base := filepath.Base(path)

	// Handle dotfiles
	if strings.HasPrefix(base, ".") {
		// Find last dot after the first character
		rest := base[1:]
		idx := strings.LastIndex(rest, ".")
		if idx == -1 {
			return stringToList(base) // Full name for dotfiles like .gitignore
		}
		return stringToList(base[:idx+1])
	}

	// Regular file
	ext := filepath.Ext(base)
	if ext != "" {
		return stringToList(base[:len(base)-len(ext)])
	}
	return stringToList(base)
}

// pathIsHidden: String -> Bool (checks if file is hidden - starts with dot)
func builtinPathIsHidden(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("pathIsHidden expects 1 argument, got %d", len(args))
	}

	str, ok := args[0].(*List)
	if !ok {
		return newError("pathIsHidden expects a string, got %s", args[0].Type())
	}

	base := filepath.Base(listToString(str))

	// Hidden if starts with dot (but not . or ..)
	if strings.HasPrefix(base, ".") && base != "." && base != ".." {
		return TRUE
	}
	return FALSE
}

// SetPathBuiltinTypes sets up type information for path builtins
