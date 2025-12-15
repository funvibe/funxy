package evaluator

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/funvibe/funxy/internal/ast"
	"github.com/funvibe/funxy/internal/config"
	"github.com/funvibe/funxy/internal/typesystem"
)

func init() {
	// Verify all builtins have TypeInfo defined
	for name, builtin := range Builtins {
		if builtin.TypeInfo == nil {
			panic(fmt.Sprintf("builtin %q is missing TypeInfo", name))
		}
	}
}

// astTypeToTypesystem converts ast.Type to typesystem.Type for runtime type display
func astTypeToTypesystem(t ast.Type) typesystem.Type {
	if t == nil {
		return typesystem.TCon{Name: "?"}
	}
	switch tt := t.(type) {
	case *ast.NamedType:
		if len(tt.Args) == 0 {
			return typesystem.TCon{Name: tt.Name.Value}
		}
		args := []typesystem.Type{}
		for _, arg := range tt.Args {
			args = append(args, astTypeToTypesystem(arg))
		}
		return typesystem.TApp{
			Constructor: typesystem.TCon{Name: tt.Name.Value},
			Args:        args,
		}
	case *ast.TupleType:
		elems := []typesystem.Type{}
		for _, el := range tt.Types {
			elems = append(elems, astTypeToTypesystem(el))
		}
		return typesystem.TTuple{Elements: elems}
	case *ast.FunctionType:
		params := []typesystem.Type{}
		for _, p := range tt.Parameters {
			params = append(params, astTypeToTypesystem(p))
		}
		return typesystem.TFunc{
			Params:     params,
			ReturnType: astTypeToTypesystem(tt.ReturnType),
		}
	case *ast.RecordType:
		fields := make(map[string]typesystem.Type)
		for k, v := range tt.Fields {
			fields[k] = astTypeToTypesystem(v)
		}
		return typesystem.TRecord{Fields: fields}
	default:
		return typesystem.TCon{Name: "?"}
	}
}

var Builtins = map[string]*Builtin{
	config.PrintFuncName: {
		Name: config.PrintFuncName,
		TypeInfo: typesystem.TFunc{
			Params:     []typesystem.Type{typesystem.TVar{Name: "a"}},
			ReturnType: typesystem.Nil,
			IsVariadic: true,
		},
		Fn: func(e *Evaluator, args ...Object) Object {
			for i, arg := range args {
				if i > 0 {
					_, _ = fmt.Fprint(e.Out, " ")
				}

				// Unquote strings: if arg is a List of Chars, print it as a string directly
				// Empty generic list [] should print as [], but empty string "" should print nothing
				if list, ok := arg.(*List); ok {
					// If it's explicitly marked as a Char list (string), print as string
					if list.ElementType == "Char" {
						var s string
						for _, el := range list.ToSlice() {
							if c, ok := el.(*Char); ok {
								s += string(rune(c.Value))
							}
						}
						_, _ = fmt.Fprint(e.Out, s)
						continue
					}

					// For non-empty lists, check if all elements are chars
					if list.len() > 0 {
						isString := true
						var s string
						for _, el := range list.ToSlice() {
							if c, ok := el.(*Char); ok {
								s += string(rune(c.Value))
							} else {
								isString = false
								break
							}
						}
						if isString {
							_, _ = fmt.Fprint(e.Out, s)
							continue
						}
					}
				}

				_, _ = fmt.Fprint(e.Out, arg.Inspect())
			}
			_, _ = fmt.Fprintln(e.Out)
			return &Nil{}
		},
	},
	config.WriteFuncName: {
		Name: config.WriteFuncName,
		TypeInfo: typesystem.TFunc{
			Params:     []typesystem.Type{typesystem.TVar{Name: "a"}},
			ReturnType: typesystem.Nil,
			IsVariadic: true,
		},
		Fn: func(e *Evaluator, args ...Object) Object {
			for i, arg := range args {
				if i > 0 {
					_, _ = fmt.Fprint(e.Out, " ")
				}

				// Unquote strings: if arg is a List of Chars, print it as a string directly
				if list, ok := arg.(*List); ok {
					if list.ElementType == "Char" {
						var s string
						for _, el := range list.ToSlice() {
							if c, ok := el.(*Char); ok {
								s += string(rune(c.Value))
							}
						}
						_, _ = fmt.Fprint(e.Out, s)
						continue
					}

					if list.len() > 0 {
						isString := true
						var s string
						for _, el := range list.ToSlice() {
							if c, ok := el.(*Char); ok {
								s += string(rune(c.Value))
							} else {
								isString = false
								break
							}
						}
						if isString {
							_, _ = fmt.Fprint(e.Out, s)
							continue
						}
					}
				}

				_, _ = fmt.Fprint(e.Out, arg.Inspect())
			}
			// No newline for write()
			return &Nil{}
		},
	},
	config.TypeOfFuncName: {
		Name: config.TypeOfFuncName,
		TypeInfo: typesystem.TFunc{
			Params:     []typesystem.Type{typesystem.TVar{Name: "a"}, typesystem.TCon{Name: "Type"}},
			ReturnType: typesystem.Bool,
		},
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 2 {
				return newError("wrong number of arguments. got=%d, want=2", len(args))
			}
			val := args[0]
			expectedTypeObj, ok := args[1].(*TypeObject)
			if !ok {
				return newError("argument 2 must be a Type, got=%s", args[1].Type())
			}

			return e.nativeBoolToBooleanObject(checkType(val, expectedTypeObj.TypeVal))
		},
	},
	config.ShowFuncName: {
		Name: config.ShowFuncName,
		TypeInfo: typesystem.TFunc{
			Params:     []typesystem.Type{typesystem.TVar{Name: "a"}},
			ReturnType: typesystem.TApp{Constructor: typesystem.TCon{Name: config.ListTypeName}, Args: []typesystem.Type{typesystem.Char}},
		},
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to show. got=%d, want=1", len(args))
			}
			return stringToList(objectToString(args[0]))
		},
	},
	config.IdFuncName: {
		Name: config.IdFuncName,
		TypeInfo: typesystem.TFunc{
			Params:     []typesystem.Type{typesystem.TVar{Name: "a"}},
			ReturnType: typesystem.TVar{Name: "a"},
		},
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to id. got=%d, want=1", len(args))
			}
			return args[0]
		},
	},
	config.ConstFuncName: {
		Name: config.ConstFuncName,
		TypeInfo: typesystem.TFunc{
			Params:     []typesystem.Type{typesystem.TVar{Name: "a"}, typesystem.TVar{Name: "b"}},
			ReturnType: typesystem.TVar{Name: "a"},
		},
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 2 {
				return newError("wrong number of arguments to const. got=%d, want=2", len(args))
			}
			return args[0]
		},
	},
	config.ReadFuncName: {
		Name: config.ReadFuncName,
		TypeInfo: typesystem.TFunc{
			Params: []typesystem.Type{
				typesystem.TApp{Constructor: typesystem.TCon{Name: config.ListTypeName}, Args: []typesystem.Type{typesystem.Char}},
				typesystem.TType{Type: typesystem.TVar{Name: "t"}},
			},
			ReturnType: typesystem.TApp{Constructor: typesystem.TCon{Name: config.OptionTypeName}, Args: []typesystem.Type{typesystem.TVar{Name: "t"}}},
		},
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 2 {
				return newError("wrong number of arguments to read. got=%d, want=2", len(args))
			}
			// Extract string from first argument
			str := listToString(args[0])
			if str == "" && args[0] != nil {
				if list, ok := args[0].(*List); ok && list.len() > 0 {
					// Non-empty list that's not a string
					return makeZero()
				}
			}

			// Get target type from second argument
			typeObj, ok := args[1].(*TypeObject)
			if !ok {
				return newError("second argument to read must be a Type")
			}

			// Parse based on type
			return parseStringToType(str, typeObj.TypeVal)
		},
	},
	"intToFloat": {
		Name: "intToFloat",
		TypeInfo: typesystem.TFunc{
			Params:     []typesystem.Type{typesystem.Int},
			ReturnType: typesystem.Float,
		},
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 1 {
				return newError("intToFloat expects 1 argument, got %d", len(args))
			}
			i, ok := args[0].(*Integer)
			if !ok {
				return newError("intToFloat expects an Int, got %s", args[0].Type())
			}
			return &Float{Value: float64(i.Value)}
		},
	},
	"floatToInt": {
		Name: "floatToInt",
		TypeInfo: typesystem.TFunc{
			Params:     []typesystem.Type{typesystem.Float},
			ReturnType: typesystem.Int,
		},
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 1 {
				return newError("floatToInt expects 1 argument, got %d", len(args))
			}
			f, ok := args[0].(*Float)
			if !ok {
				return newError("floatToInt expects a Float, got %s", args[0].Type())
			}
			return &Integer{Value: int64(f.Value)}
		},
	},
	"sprintf": {
		Name: "sprintf",
		TypeInfo: typesystem.TFunc{
			Params: []typesystem.Type{
				typesystem.TApp{Constructor: typesystem.TCon{Name: config.ListTypeName}, Args: []typesystem.Type{typesystem.TCon{Name: "Char"}}},
				typesystem.TVar{Name: "a"},
			},
			ReturnType: typesystem.TApp{Constructor: typesystem.TCon{Name: config.ListTypeName}, Args: []typesystem.Type{typesystem.TCon{Name: "Char"}}},
			IsVariadic: true,
		},
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) < 1 {
				return newError("sprintf expects at least 1 argument (format string)")
			}

			// 1. Get format string
			fmtStr := listToString(args[0])

			// 2. Unwrap values
			var goArgs []interface{}
			for _, arg := range args[1:] {
				var goVal interface{}
				switch v := arg.(type) {
				case *Integer:
					goVal = v.Value
				case *Float:
					goVal = v.Value
				case *Boolean:
					goVal = v.Value
				case *Char:
					goVal = v.Value
				case *BigInt:
					goVal = v.Value
				case *Rational:
					goVal = v.Value
				case *List:
					// If string, convert to string
					if s := listToString(v); s != "" || v.len() == 0 {
						goVal = s
					} else {
						goVal = v.Inspect()
					}
				default:
					goVal = v.Inspect()
				}
				goArgs = append(goArgs, goVal)
			}

			// 3. Sprintf
			res := fmt.Sprintf(fmtStr, goArgs...)
			return stringToList(res)
		},
	},
	config.PanicFuncName: {
		Name: config.PanicFuncName,
		TypeInfo: typesystem.TFunc{
			Params:     []typesystem.Type{typesystem.TCon{Name: "String"}},
			ReturnType: typesystem.TVar{Name: "a"},
		},
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}

			var msg string
			// Try to extract string from List Char
			if list, ok := args[0].(*List); ok {
				// Check if elements are chars
				isString := true
				var s string
				for _, el := range list.ToSlice() {
					if c, ok := el.(*Char); ok {
						s += string(rune(c.Value))
					} else {
						isString = false
						break
					}
				}
				if isString {
					msg = s
				} else {
					msg = list.Inspect()
				}
			} else {
				msg = args[0].Inspect()
			}

			return newError("%s", msg)
		},
	},
	config.DebugFuncName: {
		Name: config.DebugFuncName,
		TypeInfo: typesystem.TFunc{
			Params:     []typesystem.Type{typesystem.TVar{Name: "a"}},
			ReturnType: typesystem.Nil,
		},
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 1 {
				return newError("debug: wrong number of arguments. got=%d, want=1", len(args))
			}
			val := args[0]
			typeName := getTypeName(val)
			// Get location from call stack
			location := "?"
			if len(e.CallStack) > 0 {
				frame := e.CallStack[len(e.CallStack)-1]
				location = fmt.Sprintf("%s:%d", frame.File, frame.Line)
			}
			_, _ = fmt.Fprintf(e.Out, "[DEBUG %s] %s : %s\n", location, val.Inspect(), typeName)
			return &Nil{}
		},
	},
	config.TraceFuncName: {
		Name: config.TraceFuncName,
		TypeInfo: typesystem.TFunc{
			Params:     []typesystem.Type{typesystem.TVar{Name: "a"}},
			ReturnType: typesystem.TVar{Name: "a"},
		},
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 1 {
				return newError("trace: wrong number of arguments. got=%d, want=1", len(args))
			}
			val := args[0]
			typeName := getTypeName(val)
			// Get location from call stack
			location := "?"
			if len(e.CallStack) > 0 {
				frame := e.CallStack[len(e.CallStack)-1]
				location = fmt.Sprintf("%s:%d", frame.File, frame.Line)
			}
			_, _ = fmt.Fprintf(e.Out, "[TRACE %s] %s : %s\n", location, val.Inspect(), typeName)
			return val // Return the value for pipe chains
		},
	},
	config.LenFuncName: {
		Name: config.LenFuncName,
		TypeInfo: typesystem.TFunc{
			Params:     []typesystem.Type{typesystem.TVar{Name: "a"}},
			ReturnType: typesystem.Int,
		},
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}

			switch obj := args[0].(type) {
			case *List:
				return &Integer{Value: int64(obj.Len())}
			case *Tuple:
				return &Integer{Value: int64(len(obj.Elements))}
			case *Map:
				return &Integer{Value: int64(obj.Len())}
			case *Bytes:
				return &Integer{Value: int64(obj.Len())}
			case *Bits:
				return &Integer{Value: int64(obj.Len())}
			default:
				return newError("argument to `len` must be List, Tuple, Map, Bytes or Bits, got %s", args[0].Type())
			}
		},
	},
	config.LenBytesFuncName: {
		Name: config.LenBytesFuncName,
		TypeInfo: typesystem.TFunc{
			Params:     []typesystem.Type{typesystem.TApp{Constructor: typesystem.TCon{Name: config.ListTypeName}, Args: []typesystem.Type{typesystem.Char}}},
			ReturnType: typesystem.Int,
		},
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments to lenBytes. got=%d, want=1", len(args))
			}
			str := listToString(args[0])
			return &Integer{Value: int64(len(str))}
		},
	},
	config.GetTypeFuncName: {
		Name: config.GetTypeFuncName,
		TypeInfo: typesystem.TFunc{
			Params: []typesystem.Type{typesystem.TVar{Name: "a"}},
			ReturnType: typesystem.TType{
				Type: typesystem.TVar{Name: "a"},
			},
		},
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			t := getTypeFromObject(args[0])
			return &TypeObject{TypeVal: t}
		},
	},
	// Note: default() is handled specially in evalCallExpression to avoid init cycle
}

// objectToString converts any object to its string representation
func objectToString(obj Object) string {
	// For strings (List<Char>), extract the string directly
	// But empty list should be shown as [] not ""
	if list, ok := obj.(*List); ok && list.len() > 0 {
		isString := true
		var s string
		for _, el := range list.ToSlice() {
			if c, ok := el.(*Char); ok {
				s += string(rune(c.Value))
			} else {
				isString = false
				break
			}
		}
		if isString {
			return s
		}
	}
	return obj.Inspect()
}

// stringToList converts a Go string to List<Char>
func stringToList(s string) *List {
	chars := make([]Object, 0, len(s))
	for _, r := range s {
		chars = append(chars, &Char{Value: int64(r)})
	}
	return newListWithType(chars, "Char")
}

// getDefaultValue returns the default value for a type
func getDefaultValue(t typesystem.Type) Object {
	switch typ := t.(type) {
	case typesystem.TCon:
		switch typ.Name {
		case "Int":
			return &Integer{Value: 0}
		case "Float":
			return &Float{Value: 0.0}
		case "Bool":
			return FALSE
		case "Char":
			return &Char{Value: 0} // null char
		case "BigInt":
			return &BigInt{Value: big.NewInt(0)}
		case "Rational":
			return &Rational{Value: big.NewRat(0, 1)}
		case "Nil":
			return &Nil{}
		case config.ListTypeName:
			return newList([]Object{})
		}
	case typesystem.TApp:
		if con, ok := typ.Constructor.(typesystem.TCon); ok {
			switch con.Name {
			case config.ListTypeName:
				return newList([]Object{})
			case config.OptionTypeName:
				return &DataInstance{
					TypeName: config.OptionTypeName,
					Name:     config.ZeroCtorName,
					Fields:   []Object{},
				}
			}
		}
	case typesystem.TRecord:
		// Create record with default values for all fields
		fields := make(map[string]Object)
		for name, fieldType := range typ.Fields {
			fieldDefault := getDefaultValue(fieldType)
			if err, ok := fieldDefault.(*Error); ok {
				return err
			}
			fields[name] = fieldDefault
		}
		return NewRecord(fields)
	}
	return newError("no default value for type %s", t)
}

// getTypeFromObject returns the runtime type of an object.
// Now delegates to the RuntimeType() method on each Object.
func getTypeFromObject(val Object) typesystem.Type {
	return val.RuntimeType()
}

func checkType(val Object, expected typesystem.Type) bool {
	switch t := expected.(type) {
	case typesystem.TCon:
		switch t.Name {
		case "Int":
			_, ok := val.(*Integer)
			return ok
		case "Float":
			_, ok := val.(*Float)
			return ok
		case "Bool":
			_, ok := val.(*Boolean)
			return ok
		case "Nil":
			_, ok := val.(*Nil)
			return ok
		case config.ListTypeName:
			_, ok := val.(*List)
			return ok
		case "Char":
			_, ok := val.(*Char)
			return ok
		case "BigInt":
			_, ok := val.(*BigInt)
			return ok
		case "Rational":
			_, ok := val.(*Rational)
			return ok
		case "Bytes":
			_, ok := val.(*Bytes)
			return ok
		case "Bits":
			_, ok := val.(*Bits)
			return ok
		case "Map":
			_, ok := val.(*Map)
			return ok
		default:
			// ADT check by TypeName
			if di, ok := val.(*DataInstance); ok {
				return di.TypeName == t.Name
			}
			// Record with TypeName check
			if ri, ok := val.(*RecordInstance); ok {
				return ri.TypeName == t.Name
			}
			return false
		}
	case typesystem.TTuple:
		// Check if val is Tuple and elements match
		valTuple, ok := val.(*Tuple)
		if !ok {
			return false
		}
		if len(valTuple.Elements) != len(t.Elements) {
			return false
		}
		for i, elType := range t.Elements {
			if !checkType(valTuple.Elements[i], elType) {
				return false
			}
		}
		return true

	case typesystem.TApp:
		// For generic types, we only check the base constructor for now.
		// e.g. Option(Int) checks if it is Option.
		return checkType(val, t.Constructor)

	case typesystem.TType:
		// If we are checking against Type<T>, it means val should be a TypeObject wrapping T.
		valTypeObj, ok := val.(*TypeObject)
		if !ok {
			return false
		}
		// Unify or Equal check?
		// Runtime types should match exactly or unify?
		// Let's compare string representation for simplicity or DeepEqual logic?
		// Reuse CheckType recursively?
		// expected.Type vs valTypeObj.TypeVal
		// We don't have deep equality helper here easily.
		// Using string.
		return valTypeObj.TypeVal.String() == t.Type.String()

	default:
		return false
	}
}

// GetBuiltinsList returns a map of all builtin names to their objects
// This is used by the VM to register builtins
func GetBuiltinsList() map[string]Object {
	env := NewEnvironment()
	RegisterBuiltins(env)

	// Add list builtins (head, tail, map, filter, etc.)
	for name, builtin := range ListBuiltins() {
		env.Set(name, builtin)
	}

	// Add math builtins (abs, sqrt, sin, cos, etc.)
	for name, builtin := range MathBuiltins() {
		env.Set(name, builtin)
	}

	// Add string builtins
	for name, builtin := range StringBuiltins() {
		env.Set(name, builtin)
	}

	// Add option builtins (isSome, isZero, unwrap, unwrapOr, etc.)
	for name, builtin := range OptionBuiltins() {
		env.Set(name, builtin)
	}

	// Add result builtins (isOk, isFail, unwrapResult, etc.)
	for name, builtin := range ResultBuiltins() {
		env.Set(name, builtin)
	}

	// Add flag builtins
	for name, builtin := range FlagBuiltins() {
		env.Set(name, builtin)
	}

	// Return the internal store
	return env.GetStore()
}

// RegisterBuiltins registers built-in functions and types into the environment.
func RegisterBuiltins(env *Environment) {
	// Built-in types
	env.Set("Int", &TypeObject{TypeVal: typesystem.TCon{Name: "Int"}})
	env.Set("Float", &TypeObject{TypeVal: typesystem.TCon{Name: "Float"}})
	env.Set("Bool", &TypeObject{TypeVal: typesystem.TCon{Name: "Bool"}})
	env.Set("Char", &TypeObject{TypeVal: typesystem.TCon{Name: "Char"}})
	// Nil is both a type and a value (singleton pattern)
	// When used as a value, it's the only inhabitant of type Nil
	env.Set("Nil", &Nil{})
	env.Set("BigInt", &TypeObject{TypeVal: typesystem.TCon{Name: "BigInt"}})
	env.Set("Rational", &TypeObject{TypeVal: typesystem.TCon{Name: "Rational"}})
	env.Set(config.ListTypeName, &TypeObject{TypeVal: typesystem.TCon{Name: config.ListTypeName}})
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: config.ListTypeName},
		Args:        []typesystem.Type{typesystem.TCon{Name: "Char"}},
	}
	env.Set("String", &TypeObject{TypeVal: stringType})

	// Built-in ADTs
	// Result
	env.Set(config.ResultTypeName, &TypeObject{TypeVal: typesystem.TCon{Name: config.ResultTypeName}})
	env.Set(config.OkCtorName, &Constructor{Name: config.OkCtorName, TypeName: config.ResultTypeName, Arity: 1})
	env.Set(config.FailCtorName, &Constructor{Name: config.FailCtorName, TypeName: config.ResultTypeName, Arity: 1})

	// Option
	env.Set(config.OptionTypeName, &TypeObject{TypeVal: typesystem.TCon{Name: config.OptionTypeName}})
	env.Set(config.SomeCtorName, &Constructor{Name: config.SomeCtorName, TypeName: config.OptionTypeName, Arity: 1})
	env.Set(config.ZeroCtorName, &DataInstance{Name: config.ZeroCtorName, Fields: []Object{}, TypeName: config.OptionTypeName})

	// Json ADT
	env.Set("Json", &TypeObject{TypeVal: typesystem.TCon{Name: "Json"}})
	env.Set("JNull", &DataInstance{Name: "JNull", Fields: []Object{}, TypeName: "Json"})
	env.Set("JBool", &Constructor{Name: "JBool", TypeName: "Json", Arity: 1})
	env.Set("JNum", &Constructor{Name: "JNum", TypeName: "Json", Arity: 1})
	env.Set("JStr", &Constructor{Name: "JStr", TypeName: "Json", Arity: 1})
	env.Set("JArr", &Constructor{Name: "JArr", TypeName: "Json", Arity: 1})
	env.Set("JObj", &Constructor{Name: "JObj", TypeName: "Json", Arity: 1})

	// SqlValue ADT
	env.Set("SqlValue", &TypeObject{TypeVal: typesystem.TCon{Name: "SqlValue"}})
	env.Set("SqlNull", &DataInstance{Name: "SqlNull", Fields: []Object{}, TypeName: "SqlValue"})
	env.Set("SqlInt", &Constructor{Name: "SqlInt", TypeName: "SqlValue", Arity: 1})
	env.Set("SqlFloat", &Constructor{Name: "SqlFloat", TypeName: "SqlValue", Arity: 1})
	env.Set("SqlString", &Constructor{Name: "SqlString", TypeName: "SqlValue", Arity: 1})
	env.Set("SqlBool", &Constructor{Name: "SqlBool", TypeName: "SqlValue", Arity: 1})
	env.Set("SqlBytes", &Constructor{Name: "SqlBytes", TypeName: "SqlValue", Arity: 1})
	env.Set("SqlTime", &Constructor{Name: "SqlTime", TypeName: "SqlValue", Arity: 1})
	env.Set("SqlBigInt", &Constructor{Name: "SqlBigInt", TypeName: "SqlValue", Arity: 1})

	// Bytes and Bits types
	env.Set("Bytes", &TypeObject{TypeVal: typesystem.TCon{Name: "Bytes"}})
	env.Set("Bits", &TypeObject{TypeVal: typesystem.TCon{Name: "Bits"}})

	// Map type
	env.Set("Map", &TypeObject{TypeVal: typesystem.TCon{Name: "Map"}})

	// Register all builtin functions from the Builtins map
	for name, builtin := range Builtins {
		env.Set(name, builtin)
	}
}

// listToString extracts a Go string from a List<Char> object
func listToString(obj Object) string {
	list, ok := obj.(*List)
	if !ok {
		return ""
	}
	return ListToString(list)
}

// ListToString converts List<Char> to Go string (exported for VM)
func ListToString(list *List) string {
	var result string
	for _, el := range list.ToSlice() {
		if c, ok := el.(*Char); ok {
			result += string(rune(c.Value))
		} else {
			return ""
		}
	}
	return result
}

// makeZero creates an Option Zero (None) value
func makeZero() Object {
	return &DataInstance{
		Name:     config.ZeroCtorName,
		Fields:   []Object{},
		TypeName: config.OptionTypeName,
	}
}

// makeSome creates an Option Some(value)
func makeSome(val Object) Object {
	return &DataInstance{
		Name:     config.SomeCtorName,
		Fields:   []Object{val},
		TypeName: config.OptionTypeName,
	}
}

// makeOk creates a Result Ok(value)
func makeOk(val Object) Object {
	return &DataInstance{
		Name:     config.OkCtorName,
		Fields:   []Object{val},
		TypeName: config.ResultTypeName,
	}
}

// makeFail creates a Result Fail(error)
func makeFail(err Object) Object {
	return &DataInstance{
		Name:     config.FailCtorName,
		Fields:   []Object{err},
		TypeName: config.ResultTypeName,
	}
}

// makeFailStr creates a Result Fail with string error message
func makeFailStr(errMsg string) Object {
	chars := make([]Object, 0, len(errMsg))
	for _, r := range errMsg {
		chars = append(chars, &Char{Value: int64(r)})
	}
	return makeFail(newList(chars))
}

// parseStringToType parses a string into a value of the specified type
// Returns Some(value) on success, Zero on failure
func parseStringToType(s string, t typesystem.Type) Object {
	switch ty := t.(type) {
	case typesystem.TCon:
		switch ty.Name {
		case "Int":
			// Strict parsing - must be a valid integer with no extra characters
			val, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
			if err != nil {
				return makeZero()
			}
			return makeSome(&Integer{Value: val})
		case "Float":
			// Strict parsing
			val, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
			if err != nil {
				return makeZero()
			}
			return makeSome(&Float{Value: val})
		case "Bool":
			switch strings.TrimSpace(s) {
			case "true":
				return makeSome(&Boolean{Value: true})
			case "false":
				return makeSome(&Boolean{Value: false})
			default:
				return makeZero()
			}
		case "BigInt":
			val := new(big.Int)
			_, ok := val.SetString(strings.TrimSpace(s), 10)
			if !ok {
				return makeZero()
			}
			return makeSome(&BigInt{Value: val})
		case "Rational":
			val := new(big.Rat)
			_, ok := val.SetString(strings.TrimSpace(s))
			if !ok {
				return makeZero()
			}
			return makeSome(&Rational{Value: val})
		case "Char":
			runes := []rune(s)
			if len(runes) != 1 {
				return makeZero()
			}
			return makeSome(&Char{Value: int64(runes[0])})
		default:
			return makeZero()
		}
	case typesystem.TApp:
		// Handle String = List<Char>
		if con, ok := ty.Constructor.(typesystem.TCon); ok {
			if con.Name == config.ListTypeName && len(ty.Args) == 1 {
				if argCon, ok := ty.Args[0].(typesystem.TCon); ok && argCon.Name == "Char" {
					// This is String (List<Char>)
					return makeSome(stringToList(s))
				}
			}
		}
		return makeZero()
	default:
		return makeZero()
	}
}
