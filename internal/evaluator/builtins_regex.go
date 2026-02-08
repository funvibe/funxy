package evaluator

import (
	"github.com/funvibe/funxy/internal/typesystem"
	"regexp"
)

// RegexBuiltins returns built-in functions for lib/regex virtual package
func RegexBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		"regexMatch":      {Fn: builtinRegexMatch, Name: "regexMatch"},
		"regexFind":       {Fn: builtinRegexFind, Name: "regexFind"},
		"regexFindAll":    {Fn: builtinRegexFindAll, Name: "regexFindAll"},
		"regexCapture":    {Fn: builtinRegexCapture, Name: "regexCapture"},
		"regexReplace":    {Fn: builtinRegexReplace, Name: "regexReplace"},
		"regexReplaceAll": {Fn: builtinRegexReplaceAll, Name: "regexReplaceAll"},
		"regexSplit":      {Fn: builtinRegexSplit, Name: "regexSplit"},
		"regexValidate":   {Fn: builtinRegexValidate, Name: "regexValidate"},
	}
}

// matchRe: (String, String) -> Bool
// Tests if pattern matches anywhere in string
func builtinRegexMatch(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("matchRe expects 2 arguments, got %d", len(args))
	}

	patternList, ok := args[0].(*List)
	if !ok {
		return newError("matchRe expects a string pattern, got %s", args[0].Type())
	}

	strList, ok := args[1].(*List)
	if !ok {
		return newError("matchRe expects a string to match, got %s", args[1].Type())
	}

	pattern := listToString(patternList)
	str := listToString(strList)

	re, err := regexp.Compile(pattern)
	if err != nil {
		return &Boolean{Value: false}
	}

	return &Boolean{Value: re.MatchString(str)}
}

// find: (String, String) -> Option<String>
// Finds first match of pattern in string
func builtinRegexFind(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("find expects 2 arguments, got %d", len(args))
	}

	patternList, ok := args[0].(*List)
	if !ok {
		return newError("find expects a string pattern, got %s", args[0].Type())
	}

	strList, ok := args[1].(*List)
	if !ok {
		return newError("find expects a string to search, got %s", args[1].Type())
	}

	pattern := listToString(patternList)
	str := listToString(strList)

	re, err := regexp.Compile(pattern)
	if err != nil {
		return makeNone()
	}

	match := re.FindString(str)
	if match == "" && !re.MatchString(str) {
		return makeNone()
	}

	return makeSome(stringToList(match))
}

// findAll: (String, String) -> List<String>
// Finds all matches of pattern in string
func builtinRegexFindAll(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("findAll expects 2 arguments, got %d", len(args))
	}

	patternList, ok := args[0].(*List)
	if !ok {
		return newError("findAll expects a string pattern, got %s", args[0].Type())
	}

	strList, ok := args[1].(*List)
	if !ok {
		return newError("findAll expects a string to search, got %s", args[1].Type())
	}

	pattern := listToString(patternList)
	str := listToString(strList)

	re, err := regexp.Compile(pattern)
	if err != nil {
		return newList([]Object{})
	}

	matches := re.FindAllString(str, -1)
	elements := make([]Object, len(matches))
	for i, m := range matches {
		elements[i] = stringToList(m)
	}

	return newList(elements)
}

// capture: (String, String) -> Option<List<String>>
// Captures groups from first match (index 0 is full match, 1+ are capture groups)
func builtinRegexCapture(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("capture expects 2 arguments, got %d", len(args))
	}

	patternList, ok := args[0].(*List)
	if !ok {
		return newError("capture expects a string pattern, got %s", args[0].Type())
	}

	strList, ok := args[1].(*List)
	if !ok {
		return newError("capture expects a string to search, got %s", args[1].Type())
	}

	pattern := listToString(patternList)
	str := listToString(strList)

	re, err := regexp.Compile(pattern)
	if err != nil {
		return makeNone()
	}

	matches := re.FindStringSubmatch(str)
	if matches == nil {
		return makeNone()
	}

	elements := make([]Object, len(matches))
	for i, m := range matches {
		elements[i] = stringToList(m)
	}

	return makeSome(newList(elements))
}

// replace: (String, String, String) -> String
// Replaces first match of pattern with replacement
func builtinRegexReplace(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("replace expects 3 arguments, got %d", len(args))
	}

	patternList, ok := args[0].(*List)
	if !ok {
		return newError("replace expects a string pattern, got %s", args[0].Type())
	}

	replacementList, ok := args[1].(*List)
	if !ok {
		return newError("replace expects a string replacement, got %s", args[1].Type())
	}

	strList, ok := args[2].(*List)
	if !ok {
		return newError("replace expects a string to modify, got %s", args[2].Type())
	}

	pattern := listToString(patternList)
	replacement := listToString(replacementList)
	str := listToString(strList)

	re, err := regexp.Compile(pattern)
	if err != nil {
		return stringToList(str)
	}

	// Replace only first match
	loc := re.FindStringIndex(str)
	if loc == nil {
		return stringToList(str)
	}

	expanded := re.Expand(nil, []byte(replacement), []byte(str), loc)
	result := str[:loc[0]] + string(expanded) + str[loc[1]:]
	return stringToList(result)
}

// replaceAll: (String, String, String) -> String
// Replaces all matches of pattern with replacement
func builtinRegexReplaceAll(e *Evaluator, args ...Object) Object {
	if len(args) != 3 {
		return newError("replaceAll expects 3 arguments, got %d", len(args))
	}

	patternList, ok := args[0].(*List)
	if !ok {
		return newError("replaceAll expects a string pattern, got %s", args[0].Type())
	}

	replacementList, ok := args[1].(*List)
	if !ok {
		return newError("replaceAll expects a string replacement, got %s", args[1].Type())
	}

	strList, ok := args[2].(*List)
	if !ok {
		return newError("replaceAll expects a string to modify, got %s", args[2].Type())
	}

	pattern := listToString(patternList)
	replacement := listToString(replacementList)
	str := listToString(strList)

	re, err := regexp.Compile(pattern)
	if err != nil {
		return stringToList(str)
	}

	result := re.ReplaceAllString(str, replacement)
	return stringToList(result)
}

// split: (String, String) -> List<String>
// Splits string by pattern
func builtinRegexSplit(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("split expects 2 arguments, got %d", len(args))
	}

	patternList, ok := args[0].(*List)
	if !ok {
		return newError("split expects a string pattern, got %s", args[0].Type())
	}

	strList, ok := args[1].(*List)
	if !ok {
		return newError("split expects a string to split, got %s", args[1].Type())
	}

	pattern := listToString(patternList)
	str := listToString(strList)

	re, err := regexp.Compile(pattern)
	if err != nil {
		return newList([]Object{stringToList(str)})
	}

	parts := re.Split(str, -1)
	elements := make([]Object, len(parts))
	for i, p := range parts {
		elements[i] = stringToList(p)
	}

	return newList(elements)
}

// validate: (String) -> Result<String, Nil>
// Validates regex pattern syntax
func builtinRegexValidate(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("validate expects 1 argument, got %d", len(args))
	}

	patternList, ok := args[0].(*List)
	if !ok {
		return newError("validate expects a string pattern, got %s", args[0].Type())
	}

	pattern := listToString(patternList)

	_, err := regexp.Compile(pattern)
	if err != nil {
		return makeFail(stringToList(err.Error()))
	}

	return makeOk(&Nil{})
}

// SetRegexBuiltinTypes sets type info for regex builtins
func SetRegexBuiltinTypes(builtins map[string]*Builtin) {
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
	// Option<List<String>>
	optionListString := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{listString},
	}
	// Result<String, Nil>
	resultNil := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{typesystem.Nil, stringType},
	}

	types := map[string]typesystem.Type{
		"regexMatch":      typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: typesystem.Bool},
		"regexFind":       typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: optionString},
		"regexFindAll":    typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: listString},
		"regexCapture":    typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: optionListString},
		"regexReplace":    typesystem.TFunc{Params: []typesystem.Type{stringType, stringType, stringType}, ReturnType: stringType},
		"regexReplaceAll": typesystem.TFunc{Params: []typesystem.Type{stringType, stringType, stringType}, ReturnType: stringType},
		"regexSplit":      typesystem.TFunc{Params: []typesystem.Type{stringType, stringType}, ReturnType: listString},
		"regexValidate":   typesystem.TFunc{Params: []typesystem.Type{stringType}, ReturnType: resultNil},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}
