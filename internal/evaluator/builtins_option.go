package evaluator

import "github.com/funvibe/funxy/internal/typesystem"

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
	if di, ok := args[0].(*DataInstance); ok {
		if di.Name == "Some" && di.TypeName == "Option" {
			return TRUE
		}
	}
	return FALSE
}

// isNone: Option<T> -> Bool
func builtinIsNone(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("isNone expects 1 argument, got %d", len(args))
	}
	if di, ok := args[0].(*DataInstance); ok {
		if di.Name == "None" && di.TypeName == "Option" {
			return TRUE
		}
	}
	return FALSE
}

// unwrap: Option<T> -> T (panics on None)
func builtinUnwrap(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("unwrap expects 1 argument, got %d", len(args))
	}
	if di, ok := args[0].(*DataInstance); ok && di.Name == "Some" && len(di.Fields) == 1 {
		return di.Fields[0]
	}
	return newError("unwrap: expected Some, got None")
}

// unwrapOr: (Option<T>, T) -> T
func builtinUnwrapOr(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("unwrapOr expects 2 arguments, got %d", len(args))
	}
	if di, ok := args[0].(*DataInstance); ok && di.Name == "Some" && len(di.Fields) == 1 {
		return di.Fields[0]
	}
	return args[1]
}

// unwrapOrElse: (Option<T>, () -> T) -> T
func builtinUnwrapOrElse(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("unwrapOrElse expects 2 arguments, got %d", len(args))
	}
	if di, ok := args[0].(*DataInstance); ok && di.Name == "Some" && len(di.Fields) == 1 {
		return di.Fields[0]
	}
	// Call the fallback function
	return e.ApplyFunction(args[1], []Object{})
}

// SetOptionBuiltinTypes sets type signatures for Option builtins
func SetOptionBuiltinTypes(builtins map[string]*Builtin) {
	optionT := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{typesystem.TVar{Name: "T"}},
	}
	T := typesystem.TVar{Name: "T"}

	if b, ok := builtins["isSome"]; ok {
		b.TypeInfo = typesystem.TFunc{Params: []typesystem.Type{optionT}, ReturnType: typesystem.Bool}
	}
	if b, ok := builtins["isNone"]; ok {
		b.TypeInfo = typesystem.TFunc{Params: []typesystem.Type{optionT}, ReturnType: typesystem.Bool}
	}
	if b, ok := builtins["unwrap"]; ok {
		b.TypeInfo = typesystem.TFunc{Params: []typesystem.Type{optionT}, ReturnType: T}
	}
	if b, ok := builtins["unwrapOr"]; ok {
		b.TypeInfo = typesystem.TFunc{Params: []typesystem.Type{optionT, T}, ReturnType: T}
	}
	if b, ok := builtins["unwrapOrElse"]; ok {
		fnType := typesystem.TFunc{Params: []typesystem.Type{}, ReturnType: T}
		b.TypeInfo = typesystem.TFunc{Params: []typesystem.Type{optionT, fnType}, ReturnType: T}
	}
}
