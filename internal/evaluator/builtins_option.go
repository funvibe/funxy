package evaluator

// OptionBuiltins returns built-in functions for Option type
func OptionBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		"isSome": {
			Fn:   builtinIsSome,
			Name: "isSome",
		},
		"isNone": {
			Fn:   builtinIsNone,
			Name: "isNone",
		},
		"unwrap": {
			Fn:   builtinUnwrap,
			Name: "unwrap",
		},
		"unwrapOr": {
			Fn:   builtinUnwrapOr,
			Name: "unwrapOr",
		},
		"unwrapOrElse": {
			Fn:   builtinUnwrapOrElse,
			Name: "unwrapOrElse",
		},
	}
}

// isSome: Option<T> -> Bool
func builtinIsSome(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("isSome expects 1 argument, got %d", len(args))
	}
	if _, ok := UnwrapOption(args[0]); ok {
		return TRUE
	}
	return FALSE
}

// isNone: Option<T> -> Bool
func builtinIsNone(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("isNone expects 1 argument, got %d", len(args))
	}
	if IsOptionNone(args[0]) {
		return TRUE
	}
	return FALSE
}

// unwrap: Option<T> -> T (panics on None)
func builtinUnwrap(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("unwrap expects 1 argument, got %d", len(args))
	}
	if inner, ok := UnwrapOption(args[0]); ok {
		return inner
	}
	return newError("unwrap: expected Some, got None")
}

// unwrapOr: (Option<T>, T) -> T
func builtinUnwrapOr(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("unwrapOr expects 2 arguments, got %d", len(args))
	}
	if inner, ok := UnwrapOption(args[0]); ok {
		return inner
	}
	return args[1]
}

// unwrapOrElse: (Option<T>, () -> T) -> T
func builtinUnwrapOrElse(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("unwrapOrElse expects 2 arguments, got %d", len(args))
	}
	if inner, ok := UnwrapOption(args[0]); ok {
		return inner
	}
	return e.ApplyFunction(args[1], []Object{})
}

// SetOptionBuiltinTypes sets type signatures for Option builtins
