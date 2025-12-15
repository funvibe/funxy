package evaluator

import "github.com/funvibe/funxy/internal/typesystem"

// ResultBuiltins returns built-in functions for Result type
func ResultBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		"isOk": {
			Fn:   builtinIsOk,
			Name: "isOk",
		},
		"isFail": {
			Fn:   builtinIsFail,
			Name: "isFail",
		},
		"unwrapResult": {
			Fn:   builtinUnwrapResult,
			Name: "unwrapResult",
		},
		"unwrapError": {
			Fn:   builtinUnwrapError,
			Name: "unwrapError",
		},
		"unwrapResultOr": {
			Fn:   builtinUnwrapResultOr,
			Name: "unwrapResultOr",
		},
	}
}

// isOk: Result<E, T> -> Bool
func builtinIsOk(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("isOk expects 1 argument, got %d", len(args))
	}
	if di, ok := args[0].(*DataInstance); ok {
		if di.Name == "Ok" && di.TypeName == "Result" {
			return TRUE
		}
	}
	return FALSE
}

// isFail: Result<E, T> -> Bool
func builtinIsFail(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("isFail expects 1 argument, got %d", len(args))
	}
	if di, ok := args[0].(*DataInstance); ok {
		if di.Name == "Fail" && di.TypeName == "Result" {
			return TRUE
		}
	}
	return FALSE
}

// unwrapResult: Result<E, T> -> T (panics on Fail)
func builtinUnwrapResult(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("unwrapResult expects 1 argument, got %d", len(args))
	}
	if di, ok := args[0].(*DataInstance); ok && di.Name == "Ok" && len(di.Fields) == 1 {
		return di.Fields[0]
	}
	if di, ok := args[0].(*DataInstance); ok && di.Name == "Fail" && len(di.Fields) == 1 {
		return newError("unwrapResult: got Fail(%s)", di.Fields[0].Inspect())
	}
	return newError("unwrapResult: expected Result, got %s", args[0].Type())
}

// unwrapError: Result<E, T> -> E (panics on Ok)
func builtinUnwrapError(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("unwrapError expects 1 argument, got %d", len(args))
	}
	if di, ok := args[0].(*DataInstance); ok && di.Name == "Fail" && len(di.Fields) == 1 {
		return di.Fields[0]
	}
	if di, ok := args[0].(*DataInstance); ok && di.Name == "Ok" {
		return newError("unwrapError: expected Fail, got Ok")
	}
	return newError("unwrapError: expected Result, got %s", args[0].Type())
}

// unwrapResultOr: (Result<E, T>, T) -> T
func builtinUnwrapResultOr(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("unwrapResultOr expects 2 arguments, got %d", len(args))
	}
	if di, ok := args[0].(*DataInstance); ok && di.Name == "Ok" && len(di.Fields) == 1 {
		return di.Fields[0]
	}
	return args[1]
}

// SetResultBuiltinTypes sets type signatures for Result builtins
func SetResultBuiltinTypes(builtins map[string]*Builtin) {
	resultET := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{typesystem.TVar{Name: "E"}, typesystem.TVar{Name: "T"}},
	}
	T := typesystem.TVar{Name: "T"}
	E := typesystem.TVar{Name: "E"}

	if b, ok := builtins["isOk"]; ok {
		b.TypeInfo = typesystem.TFunc{Params: []typesystem.Type{resultET}, ReturnType: typesystem.Bool}
	}
	if b, ok := builtins["isFail"]; ok {
		b.TypeInfo = typesystem.TFunc{Params: []typesystem.Type{resultET}, ReturnType: typesystem.Bool}
	}
	if b, ok := builtins["unwrapResult"]; ok {
		b.TypeInfo = typesystem.TFunc{Params: []typesystem.Type{resultET}, ReturnType: T}
	}
	if b, ok := builtins["unwrapError"]; ok {
		b.TypeInfo = typesystem.TFunc{Params: []typesystem.Type{resultET}, ReturnType: E}
	}
	if b, ok := builtins["unwrapResultOr"]; ok {
		b.TypeInfo = typesystem.TFunc{Params: []typesystem.Type{resultET, T}, ReturnType: T}
	}
}
