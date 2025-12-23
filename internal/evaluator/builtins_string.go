package evaluator

import (
	"strings"
	"unicode"

	"github.com/funvibe/funxy/internal/typesystem"
)

// StringBuiltins returns built-in functions for lib/string virtual package
func StringBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		// Split/Join
		"stringSplit": {Fn: builtinSplit, Name: "stringSplit"},
		"stringJoin":  {Fn: builtinJoin, Name: "stringJoin"},
		"stringLines": {Fn: builtinLines, Name: "stringLines"},
		"stringWords": {Fn: builtinWords, Name: "stringWords"},

		// Trimming
		"stringTrim":      {Fn: builtinTrim, Name: "stringTrim"},
		"stringTrimStart": {Fn: builtinTrimStart, Name: "stringTrimStart"},
		"stringTrimEnd":   {Fn: builtinTrimEnd, Name: "stringTrimEnd"},

		// Case conversion
		"stringToUpper":    {Fn: builtinToUpper, Name: "stringToUpper"},
		"stringToLower":    {Fn: builtinToLower, Name: "stringToLower"},
		"stringCapitalize": {Fn: builtinCapitalize, Name: "stringCapitalize"},

		// Search/Replace
		"stringReplace":    {Fn: builtinReplace, Name: "stringReplace"},
		"stringReplaceAll": {Fn: builtinReplaceAll, Name: "stringReplaceAll"},
		"stringStartsWith": {Fn: builtinStartsWith, Name: "stringStartsWith"},
		"stringEndsWith":   {Fn: builtinEndsWith, Name: "stringEndsWith"},
		"stringIndexOf":  {Fn: builtinStringIndexOf, Name: "stringIndexOf"},

		// Other
		"stringRepeat":   {Fn: builtinRepeat, Name: "stringRepeat"},
		"stringPadLeft":  {Fn: builtinPadLeft, Name: "stringPadLeft"},
		"stringPadRight": {Fn: builtinPadRight, Name: "stringPadRight"},
	}
}

// SetStringBuiltinTypes sets type info for string builtins
func SetStringBuiltinTypes(builtins map[string]*Builtin) {
	// String = List<Char>
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}
	listString := typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{stringType}}
	optionInt := typesystem.TApp{Constructor: typesystem.TCon{Name: "Option"}, Args: []typesystem.Type{typesystem.Int}}

	types := map[string]typesystem.Type{
		"stringSplit":      typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: listString},
		"stringJoin":       typesystem.TFunc{Params: []typesystem.Type{listString, stringType}, ReturnType: stringType},
		"stringLines":      typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: listString},
		"stringWords":      typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: listString},
		"stringTrim":       typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
		"stringTrimStart":  typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
		"stringTrimEnd":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
		"stringToUpper":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
		"stringToLower":    typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
		"stringCapitalize": typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: stringType},
		"stringReplace":    typesystem.TFunc{Params: []typesystem.Type{stringType, stringType, stringType}, ReturnType: stringType},
		"stringReplaceAll": typesystem.TFunc{Params: []typesystem.Type{stringType, stringType, stringType}, ReturnType: stringType},
		"stringStartsWith": typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: typesystem.Bool},
		"stringEndsWith":   typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: typesystem.Bool},
		"stringIndexOf":  typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: optionInt},
		"stringRepeat":     typesystem.TFunc{Params: []typesystem.Type{stringType, typesystem.Int}, ReturnType: stringType},
		"stringPadLeft":    typesystem.TFunc{Params: []typesystem.Type{stringType, typesystem.Int, typesystem.Char}, ReturnType: stringType},
		"stringPadRight":   typesystem.TFunc{Params: []typesystem.Type{stringType, typesystem.Int, typesystem.Char}, ReturnType: stringType},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}

// =============================================================================
// Helper: convert List<Char> (our string representation) to Go string
// =============================================================================

func listToGoString(obj Object) (string, bool) {
	list, ok := obj.(*List)
	if !ok {
		return "", false
	}

	var sb strings.Builder
	for _, elem := range list.ToSlice() {
		if ch, ok := elem.(*Char); ok {
			sb.WriteRune(rune(ch.Value))
		} else {
			return "", false
		}
	}
	return sb.String(), true
}

// StringToList converts Go string to List<Char> (exported for VM)
func StringToList(s string) *List {
	return goStringToList(s)
}

// IsStringList checks if a list contains only Char elements (is a string)
func IsStringList(l *List) bool {
	if l.ElementType == "Char" {
		return true
	}
	if l.Len() == 0 {
		return false // Empty list is not considered a string
	}
	for i := 0; i < l.Len(); i++ {
		if _, ok := l.Get(i).(*Char); !ok {
			return false
		}
	}
	return true
}

func goStringToList(s string) *List {
	elements := make([]Object, 0, len(s))
	for _, r := range s {
		elements = append(elements, &Char{Value: int64(r)})
	}
	return newListWithType(elements, "Char")
}

func goStringsToList(strs []string) *List {
	elements := make([]Object, len(strs))
	for i, s := range strs {
		elements[i] = goStringToList(s)
	}
	return newList(elements)
}

func listToGoStrings(obj Object) ([]string, bool) {
	list, ok := obj.(*List)
	if !ok {
		return nil, false
	}

	result := make([]string, list.len())
	for i, elem := range list.ToSlice() {
		s, ok := listToGoString(elem)
		if !ok {
			return nil, false
		}
		result[i] = s
	}
	return result, true
}

// =============================================================================
// Split/Join
// =============================================================================

// split: (String, String) -> List<String>
func builtinSplit(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("split expects 2 arguments, got %d", len(args))
	}

	str, ok := listToGoString(args[0])
	if !ok {
		return newError("split: first argument must be a string")
	}

	delim, ok := listToGoString(args[1])
	if !ok {
		return newError("split: second argument must be a string")
	}

	parts := strings.Split(str, delim)
	return goStringsToList(parts)
}

// join: (List<String>, String) -> String
func builtinJoin(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("join expects 2 arguments, got %d", len(args))
	}

	strs, ok := listToGoStrings(args[0])
	if !ok {
		return newError("join: first argument must be a list of strings")
	}

	delim, ok := listToGoString(args[1])
	if !ok {
		return newError("join: second argument must be a string")
	}

	result := strings.Join(strs, delim)
	return goStringToList(result)
}

// lines: (String) -> List<String>
func builtinLines(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("lines expects 1 argument, got %d", len(args))
	}

	str, ok := listToGoString(args[0])
	if !ok {
		return newError("lines: argument must be a string")
	}

	// Split by newlines, handling both \n and \r\n
	str = strings.ReplaceAll(str, "\r\n", "\n")
	parts := strings.Split(str, "\n")
	return goStringsToList(parts)
}

// words: (String) -> List<String>
func builtinWords(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("words expects 1 argument, got %d", len(args))
	}

	str, ok := listToGoString(args[0])
	if !ok {
		return newError("words: argument must be a string")
	}

	parts := strings.Fields(str)
	return goStringsToList(parts)
}

// =============================================================================
// Trimming
// =============================================================================

// trim: (String) -> String
func builtinTrim(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("trim expects 1 argument, got %d", len(args))
	}

	str, ok := listToGoString(args[0])
	if !ok {
		return newError("trim: argument must be a string")
	}

	return goStringToList(strings.TrimSpace(str))
}

// trimStart: (String) -> String
func builtinTrimStart(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("trimStart expects 1 argument, got %d", len(args))
	}

	str, ok := listToGoString(args[0])
	if !ok {
		return newError("trimStart: argument must be a string")
	}

	return goStringToList(strings.TrimLeftFunc(str, unicode.IsSpace))
}

// trimEnd: (String) -> String
func builtinTrimEnd(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("trimEnd expects 1 argument, got %d", len(args))
	}

	str, ok := listToGoString(args[0])
	if !ok {
		return newError("trimEnd: argument must be a string")
	}

	return goStringToList(strings.TrimRightFunc(str, unicode.IsSpace))
}

// =============================================================================
// Case conversion
// =============================================================================

// toUpper: (String) -> String
func builtinToUpper(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("toUpper expects 1 argument, got %d", len(args))
	}

	str, ok := listToGoString(args[0])
	if !ok {
		return newError("toUpper: argument must be a string")
	}

	return goStringToList(strings.ToUpper(str))
}

// toLower: (String) -> String
func builtinToLower(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("toLower expects 1 argument, got %d", len(args))
	}

	str, ok := listToGoString(args[0])
	if !ok {
		return newError("toLower: argument must be a string")
	}

	return goStringToList(strings.ToLower(str))
}

// capitalize: (String) -> String
func builtinCapitalize(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("capitalize expects 1 argument, got %d", len(args))
	}

	str, ok := listToGoString(args[0])
	if !ok {
		return newError("capitalize: argument must be a string")
	}

	if len(str) == 0 {
		return goStringToList("")
	}

	runes := []rune(str)
	runes[0] = unicode.ToUpper(runes[0])
	return goStringToList(string(runes))
}

// =============================================================================
// Search/Replace
// =============================================================================

// replace: (String, String, String) -> String
func builtinReplace(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("replace expects 3 arguments, got %d", len(args))
	}

	str, ok := listToGoString(args[0])
	if !ok {
		return newError("replace: first argument must be a string")
	}

	old, ok := listToGoString(args[1])
	if !ok {
		return newError("replace: second argument must be a string")
	}

	new, ok := listToGoString(args[2])
	if !ok {
		return newError("replace: third argument must be a string")
	}

	return goStringToList(strings.Replace(str, old, new, 1))
}

// replaceAll: (String, String, String) -> String
func builtinReplaceAll(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("replaceAll expects 3 arguments, got %d", len(args))
	}

	str, ok := listToGoString(args[0])
	if !ok {
		return newError("replaceAll: first argument must be a string")
	}

	old, ok := listToGoString(args[1])
	if !ok {
		return newError("replaceAll: second argument must be a string")
	}

	new, ok := listToGoString(args[2])
	if !ok {
		return newError("replaceAll: third argument must be a string")
	}

	return goStringToList(strings.ReplaceAll(str, old, new))
}

// startsWith: (String, String) -> Bool
func builtinStartsWith(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("startsWith expects 2 arguments, got %d", len(args))
	}

	str, ok := listToGoString(args[0])
	if !ok {
		return newError("startsWith: first argument must be a string")
	}

	prefix, ok := listToGoString(args[1])
	if !ok {
		return newError("startsWith: second argument must be a string")
	}

	return &Boolean{Value: strings.HasPrefix(str, prefix)}
}

// endsWith: (String, String) -> Bool
func builtinEndsWith(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("endsWith expects 2 arguments, got %d", len(args))
	}

	str, ok := listToGoString(args[0])
	if !ok {
		return newError("endsWith: first argument must be a string")
	}

	suffix, ok := listToGoString(args[1])
	if !ok {
		return newError("endsWith: second argument must be a string")
	}

	return &Boolean{Value: strings.HasSuffix(str, suffix)}
}

// stringIndexOf: (String, String) -> Option<Int>
func builtinStringIndexOf(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("stringIndexOf expects 2 arguments, got %d", len(args))
	}

	str, ok := listToGoString(args[0])
	if !ok {
		return newError("stringIndexOf: first argument must be a string")
	}

	substr, ok := listToGoString(args[1])
	if !ok {
		return newError("stringIndexOf: second argument must be a string")
	}

	// Find byte index
	byteIdx := strings.Index(str, substr)
	if byteIdx == -1 {
		return makeZero()
	}

	// Convert byte index to rune index
	runeIdx := len([]rune(str[:byteIdx]))
	return makeSome(&Integer{Value: int64(runeIdx)})
}

// =============================================================================
// Other
// =============================================================================

// repeat: (String, Int) -> String
func builtinRepeat(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("repeat expects 2 arguments, got %d", len(args))
	}

	str, ok := listToGoString(args[0])
	if !ok {
		return newError("repeat: first argument must be a string")
	}

	count, ok := args[1].(*Integer)
	if !ok {
		return newError("repeat: second argument must be an integer")
	}

	if count.Value < 0 {
		return newError("repeat: count cannot be negative")
	}

	return goStringToList(strings.Repeat(str, int(count.Value)))
}

// padLeft: (String, Int, Char) -> String
func builtinPadLeft(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("padLeft expects 3 arguments, got %d", len(args))
	}

	str, ok := listToGoString(args[0])
	if !ok {
		return newError("padLeft: first argument must be a string")
	}

	length, ok := args[1].(*Integer)
	if !ok {
		return newError("padLeft: second argument must be an integer")
	}

	padChar, ok := args[2].(*Char)
	if !ok {
		return newError("padLeft: third argument must be a char")
	}

	targetLen := int(length.Value)
	currentLen := len([]rune(str))

	if currentLen >= targetLen {
		return goStringToList(str)
	}

	padding := strings.Repeat(string(rune(padChar.Value)), targetLen-currentLen)
	return goStringToList(padding + str)
}

// padRight: (String, Int, Char) -> String
func builtinPadRight(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("padRight expects 3 arguments, got %d", len(args))
	}

	str, ok := listToGoString(args[0])
	if !ok {
		return newError("padRight: first argument must be a string")
	}

	length, ok := args[1].(*Integer)
	if !ok {
		return newError("padRight: second argument must be an integer")
	}

	padChar, ok := args[2].(*Char)
	if !ok {
		return newError("padRight: third argument must be a char")
	}

	targetLen := int(length.Value)
	currentLen := len([]rune(str))

	if currentLen >= targetLen {
		return goStringToList(str)
	}

	padding := strings.Repeat(string(rune(padChar.Value)), targetLen-currentLen)
	return goStringToList(str + padding)
}
