package evaluator

import (
	"unicode"
)

// CharBuiltins returns built-in functions for lib/char virtual package
func CharBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		// Conversion
		"charToCode":   {Fn: builtinCharCode, Name: "charToCode"},
		"charFromCode": {Fn: builtinChar, Name: "charFromCode"},

		// Classification
		"charIsUpper": {Fn: builtinCharIsUpper, Name: "charIsUpper"},
		"charIsLower": {Fn: builtinCharIsLower, Name: "charIsLower"},

		// Case conversion
		"charToUpper": {Fn: builtinCharToUpper, Name: "charToUpper"},
		"charToLower": {Fn: builtinCharToLower, Name: "charToLower"},
	}
}

// SetCharBuiltinTypes sets type info for char builtins

// =============================================================================
// Conversion
// =============================================================================

// charCode: (Char) -> Int
func builtinCharCode(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("charCode expects 1 argument, got %d", len(args))
	}

	c, ok := args[0].(*Char)
	if !ok {
		return newError("charCode expects a Char argument, got %s", args[0].Type())
	}

	return &Integer{Value: c.Value}
}

// char: (Int) -> Char
func builtinChar(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("char expects 1 argument, got %d", len(args))
	}

	i, ok := args[0].(*Integer)
	if !ok {
		return newError("char expects an Int argument, got %s", args[0].Type())
	}

	return &Char{Value: i.Value}
}

// =============================================================================
// Classification
// =============================================================================

// charIsUpper: (Char) -> Bool
func builtinCharIsUpper(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("charIsUpper expects 1 argument, got %d", len(args))
	}

	c, ok := args[0].(*Char)
	if !ok {
		return newError("charIsUpper expects a Char argument, got %s", args[0].Type())
	}

	return &Boolean{Value: unicode.IsUpper(rune(c.Value))}
}

// charIsLower: (Char) -> Bool
func builtinCharIsLower(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("charIsLower expects 1 argument, got %d", len(args))
	}

	c, ok := args[0].(*Char)
	if !ok {
		return newError("charIsLower expects a Char argument, got %s", args[0].Type())
	}

	return &Boolean{Value: unicode.IsLower(rune(c.Value))}
}

// =============================================================================
// Case conversion
// =============================================================================

// charToUpper: (Char) -> Char
func builtinCharToUpper(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("charToUpper expects 1 argument, got %d", len(args))
	}

	c, ok := args[0].(*Char)
	if !ok {
		return newError("charToUpper expects a Char argument, got %s", args[0].Type())
	}

	return &Char{Value: int64(unicode.ToUpper(rune(c.Value)))}
}

// charToLower: (Char) -> Char
func builtinCharToLower(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("charToLower expects 1 argument, got %d", len(args))
	}

	c, ok := args[0].(*Char)
	if !ok {
		return newError("charToLower expects a Char argument, got %s", args[0].Type())
	}

	return &Char{Value: int64(unicode.ToLower(rune(c.Value)))}
}
