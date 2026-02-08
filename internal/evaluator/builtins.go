package evaluator

import (
	"fmt"
	"math/big"
	"path/filepath"
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

// RegisterExtensionMethods registers built-in extension methods on the evaluator
func (e *Evaluator) RegisterExtensionMethods() {
	// Option
	optionMethods := OptionBuiltins()
	if _, ok := e.ExtensionMethods["Option"]; !ok {
		e.ExtensionMethods["Option"] = make(map[string]Object)
	}
	for name, builtin := range optionMethods {
		e.ExtensionMethods["Option"][name] = builtin
	}

	// Result
	resultMethods := ResultBuiltins()
	if _, ok := e.ExtensionMethods["Result"]; !ok {
		e.ExtensionMethods["Result"] = make(map[string]Object)
	}
	for name, builtin := range resultMethods {
		e.ExtensionMethods["Result"][name] = builtin
	}
}

// ASTTypeToTypesystem converts ast.Type to typesystem.Type for runtime type display
func ASTTypeToTypesystem(t ast.Type) typesystem.Type {
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
			args = append(args, ASTTypeToTypesystem(arg))
		}
		return typesystem.TApp{
			Constructor: typesystem.TCon{Name: tt.Name.Value},
			Args:        args,
		}
	case *ast.TupleType:
		elems := []typesystem.Type{}
		for _, el := range tt.Types {
			elems = append(elems, ASTTypeToTypesystem(el))
		}
		return typesystem.TTuple{Elements: elems}
	case *ast.FunctionType:
		params := []typesystem.Type{}
		for _, p := range tt.Parameters {
			params = append(params, ASTTypeToTypesystem(p))
		}
		return typesystem.TFunc{
			Params:     params,
			ReturnType: ASTTypeToTypesystem(tt.ReturnType),
		}
	case *ast.RecordType:
		fields := make(map[string]typesystem.Type)
		for k, v := range tt.Fields {
			fields[k] = ASTTypeToTypesystem(v)
		}
		return typesystem.TRecord{Fields: fields}
	default:
		return typesystem.TCon{Name: "?"}
	}
}

// astTypeToTypesystem is a deprecated alias for ASTTypeToTypesystem
func astTypeToTypesystem(t ast.Type) typesystem.Type {
	return ASTTypeToTypesystem(t)
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
	// show is now a trait method (Show trait), registered in RegisterFPTraits
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
				return newError("wrong number of arguments to constant. got=%d, want=2", len(args))
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
					return makeNone()
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
	"format": {
		Name: "format",
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
				return newError("format expects at least 1 argument (format string)")
			}

			// 1. Get format string
			fmtStr := listToString(args[0])

			// Validate format string and argument count
			expectedArgs, err := CountFormatVerbs(fmtStr)
			if err != nil {
				return newError("invalid format string: %s", err.Error())
			}
			// args[0] is fmtStr, args[1:] are arguments
			if len(args)-1 != expectedArgs {
				return newError("format string expects %d arguments, got %d", expectedArgs, len(args)-1)
			}

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
				file := frame.File
				// Extract just the filename without directory path
				file = filepath.Base(file)
				location = fmt.Sprintf("%s:%d", file, frame.Line)
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
				file := frame.File
				// Extract just the filename without directory path
				file = filepath.Base(file)
				location = fmt.Sprintf("%s:%d", file, frame.Line)
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
	"optionT":    {Name: "optionT", Fn: builtinOptionT, TypeInfo: getOptionTConstructorType()},
	"resultT":    {Name: "resultT", Fn: builtinResultT, TypeInfo: getResultTConstructorType()},
	"runOptionT": {Name: "runOptionT", Fn: builtinRunOptionT, TypeInfo: getRunOptionTType()},
	"runResultT": {Name: "runResultT", Fn: builtinRunResultT, TypeInfo: getRunResultTType()},

	// Reflection
	"kindOf":    {Name: "kindOf", Fn: builtinKindOf, TypeInfo: typesystem.TFunc{Params: []typesystem.Type{typesystem.TVar{Name: "a"}}, ReturnType: typesystem.TCon{Name: "String"}}},
	"debugType": {Name: "debugType", Fn: builtinDebugType, TypeInfo: typesystem.TFunc{Params: []typesystem.Type{typesystem.TVar{Name: "a"}}, ReturnType: typesystem.TCon{Name: "String"}}},
	"debugRepr": {Name: "debugRepr", Fn: builtinDebugRepr, TypeInfo: typesystem.TFunc{Params: []typesystem.Type{typesystem.TVar{Name: "a"}}, ReturnType: typesystem.TCon{Name: "String"}}},
}

// Helpers for type info construction
func getOptionTConstructorType() typesystem.Type {
	return typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TApp{
				Constructor: typesystem.TVar{Name: "M"},
				Args: []typesystem.Type{
					typesystem.TApp{
						Constructor: typesystem.TCon{Name: config.OptionTypeName},
						Args:        []typesystem.Type{typesystem.TVar{Name: "A"}},
					},
				},
			},
		},
		ReturnType: typesystem.TApp{
			Constructor: typesystem.TCon{Name: "OptionT"},
			Args:        []typesystem.Type{typesystem.TVar{Name: "M"}, typesystem.TVar{Name: "A"}},
		},
	}
}

func getResultTConstructorType() typesystem.Type {
	return typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TApp{
				Constructor: typesystem.TVar{Name: "M"},
				Args: []typesystem.Type{
					typesystem.TApp{
						Constructor: typesystem.TCon{Name: config.ResultTypeName},
						Args:        []typesystem.Type{typesystem.TVar{Name: "E"}, typesystem.TVar{Name: "A"}},
					},
				},
			},
		},
		ReturnType: typesystem.TApp{
			Constructor: typesystem.TCon{Name: "ResultT"},
			Args:        []typesystem.Type{typesystem.TVar{Name: "M"}, typesystem.TVar{Name: "E"}, typesystem.TVar{Name: "A"}},
		},
	}
}

func getRunOptionTType() typesystem.Type {
	return typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TApp{
				Constructor: typesystem.TCon{Name: "OptionT"},
				Args:        []typesystem.Type{typesystem.TVar{Name: "M"}, typesystem.TVar{Name: "A"}},
			},
		},
		ReturnType: typesystem.TApp{
			Constructor: typesystem.TVar{Name: "M"},
			Args: []typesystem.Type{
				typesystem.TApp{
					Constructor: typesystem.TCon{Name: config.OptionTypeName},
					Args:        []typesystem.Type{typesystem.TVar{Name: "A"}},
				},
			},
		},
	}
}

func getRunResultTType() typesystem.Type {
	return typesystem.TFunc{
		Params: []typesystem.Type{
			typesystem.TApp{
				Constructor: typesystem.TCon{Name: "ResultT"},
				Args:        []typesystem.Type{typesystem.TVar{Name: "M"}, typesystem.TVar{Name: "E"}, typesystem.TVar{Name: "A"}},
			},
		},
		ReturnType: typesystem.TApp{
			Constructor: typesystem.TVar{Name: "M"},
			Args: []typesystem.Type{
				typesystem.TApp{
					Constructor: typesystem.TCon{Name: config.ResultTypeName},
					Args:        []typesystem.Type{typesystem.TVar{Name: "E"}, typesystem.TVar{Name: "A"}},
				},
			},
		},
	}
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
		case "String":
			return newList([]Object{}) // Return generic empty list to match VM behavior (prints as [])
		case "Bytes":
			return &Bytes{data: []byte{}}
		case "Bits":
			return &Bits{data: []byte{}, length: 0}
		case "Uuid":
			return &DataInstance{TypeName: "Uuid", Fields: []Object{}} // Or default UUID
		case "Map":
			return NewMap()
		}
	case typesystem.TApp:
		if con, ok := typ.Constructor.(typesystem.TCon); ok {
			switch con.Name {
			case config.ListTypeName:
				return newList([]Object{})
			case config.OptionTypeName:
				return &DataInstance{
					TypeName: config.OptionTypeName,
					Name:     config.NoneCtorName,
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
	// Special handling for checking against generic constructors (e.g. typeOf(val, Box))
	// If expected is a TCon "Box", but val is an instance of "Box", we should match.
	if tCon, ok := expected.(typesystem.TCon); ok {
		if ri, ok := val.(*RecordInstance); ok {
			// val is a record instance, check if its TypeName matches
			// This covers both non-generic "Point" and generic "Box" (where TypeName is "Box")
			if ri.TypeName == tCon.Name {
				return true
			}
		}
	}

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
		// For generic types, check against the base constructor
		// e.g. Box(Int) vs Box (constructor)
		if con, ok := t.Constructor.(typesystem.TCon); ok {
			// If expected type is a generic constructor (TCon), match against the Constructor of TApp
			if expectedCon, ok := expected.(typesystem.TCon); ok {
				return con.Name == expectedCon.Name
			}
		}

		// Recursively check against the constructor if expected is also TApp (not happening in typeOf(x, Box) case)
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

	// Add option builtins (isSome, isNone, unwrap, unwrapOr, etc.)
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

	// Add grpc builtins
	for name, builtin := range GrpcBuiltins() {
		env.Set(name, builtin)
	}

	// Add proto builtins
	for name, builtin := range ProtoBuiltins() {
		env.Set(name, builtin)
	}

	// Return the internal store
	return env.GetStore()
}

// RegisterBuiltins registers built-in functions and types into the environment.
func RegisterBuiltins(env *Environment) {
	// Internal builtin for dictionary creation (Analyzer usage)
	env.Set("__make_dictionary", &Builtin{
		Name: "__make_dictionary",
		TypeInfo: typesystem.TFunc{
			// We can be loose with types here as it's internal
			Params:     []typesystem.Type{typesystem.TCon{Name: "String"}, typesystem.TCon{Name: "List"}, typesystem.TCon{Name: "List"}},
			ReturnType: typesystem.TCon{Name: "Dictionary"},
		},
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 3 {
				return newError("__make_dictionary expects 3 arguments, got %d", len(args))
			}

			// 1. Name
			nameList, ok := args[0].(*List)
			if !ok {
				return newError("__make_dictionary arg 1 (name) must be String/List<Char>")
			}
			name := ListToString(nameList)

			// 2. Methods
			var methods []Object
			if tuple, ok := args[1].(*Tuple); ok {
				methods = tuple.Elements
			} else if list, ok := args[1].(*List); ok {
				// Fallback for homogeneous lists or if empty
				methods = list.ToSlice()
			} else {
				return newError("__make_dictionary arg 2 (methods) must be Tuple or List")
			}

			// 3. Supers
			supersList, ok := args[2].(*List)
			if !ok {
				return newError("__make_dictionary arg 3 (supers) must be List")
			}
			supersSlice := supersList.ToSlice()
			supers := make([]*Dictionary, len(supersSlice))
			for i, s := range supersSlice {
				dict, ok := s.(*Dictionary)
				if !ok {
					return newError("__make_dictionary arg 3 (supers) must contain Dictionaries")
				}
				supers[i] = dict
			}

			return &Dictionary{
				TraitName: name,
				Methods:   methods,
				Supers:    supers,
			}
		},
	})

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
	env.Set("String", &TypeObject{TypeVal: stringType, Alias: "String"})

	// Built-in ADTs
	// Result
	env.Set(config.ResultTypeName, &TypeObject{TypeVal: typesystem.TCon{Name: config.ResultTypeName}})
	env.Set(config.OkCtorName, &Constructor{Name: config.OkCtorName, TypeName: config.ResultTypeName, Arity: 1})
	env.Set(config.FailCtorName, &Constructor{Name: config.FailCtorName, TypeName: config.ResultTypeName, Arity: 1})

	// Option
	env.Set(config.OptionTypeName, &TypeObject{TypeVal: typesystem.TCon{Name: config.OptionTypeName}})
	env.Set(config.SomeCtorName, &Constructor{Name: config.SomeCtorName, TypeName: config.OptionTypeName, Arity: 1})
	// None is a nullary constructor (constant), not a function
	env.Set(config.NoneCtorName, &DataInstance{Name: config.NoneCtorName, Fields: []Object{}, TypeName: config.OptionTypeName})

	// Json ADT
	env.Set("Json", &TypeObject{TypeVal: typesystem.TCon{Name: "Json"}})
	// JNull is a nullary constructor (constant), not a function
	env.Set("JNull", &DataInstance{Name: "JNull", Fields: []Object{}, TypeName: "Json"})
	env.Set("JBool", &Constructor{Name: "JBool", TypeName: "Json", Arity: 1})
	env.Set("JNum", &Constructor{Name: "JNum", TypeName: "Json", Arity: 1})
	env.Set("JStr", &Constructor{Name: "JStr", TypeName: "Json", Arity: 1})
	env.Set("JArr", &Constructor{Name: "JArr", TypeName: "Json", Arity: 1})
	env.Set("JObj", &Constructor{Name: "JObj", TypeName: "Json", Arity: 1})

	// SqlValue ADT (moved to lib/sql)
	// env.Set("SqlValue", &TypeObject{TypeVal: typesystem.TCon{Name: "SqlValue"}})
	// env.Set("SqlNull", &DataInstance{Name: "SqlNull", Fields: []Object{}, TypeName: "SqlValue"})
	// env.Set("SqlInt", &Constructor{Name: "SqlInt", TypeName: "SqlValue", Arity: 1})
	// ... (others removed)

	// Bytes and Bits types (moved to lib/bytes and lib/bits)
	// env.Set("Bytes", &TypeObject{TypeVal: typesystem.TCon{Name: "Bytes"}})
	// env.Set("Bits", &TypeObject{TypeVal: typesystem.TCon{Name: "Bits"}})

	// Map type
	env.Set("Map", &TypeObject{TypeVal: typesystem.TCon{Name: "Map"}})

	// Reader<E, A>
	// Type: Reader
	env.Set("Reader", &TypeObject{TypeVal: typesystem.TCon{Name: "Reader"}})
	// Constructor: reader(fn)
	env.Set("reader", &Constructor{Name: "reader", TypeName: "Reader", Arity: 1})
	// runReader(r, e)
	env.Set("runReader", &Builtin{
		Name: "runReader",
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 2 {
				return newError("runReader expects 2 arguments, got %d", len(args))
			}
			r := args[0]
			envVal := args[1]
			// Check if r is Reader
			di, ok := r.(*DataInstance)
			if !ok || di.TypeName != "Reader" || len(di.Fields) != 1 {
				return newError("runReader expects a Reader")
			}
			fn := di.Fields[0]
			return e.ApplyFunction(fn, []Object{envVal})
		},
	})

	// Identity<T>
	// Type: Identity
	env.Set("Identity", &TypeObject{TypeVal: typesystem.TCon{Name: "Identity"}})
	// Constructor: identity(val)
	env.Set("identity", &Constructor{Name: "identity", TypeName: "Identity", Arity: 1})
	// runIdentity(i)
	env.Set("runIdentity", &Builtin{
		Name: "runIdentity",
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 1 {
				return newError("runIdentity expects 1 argument")
			}
			i := args[0]
			di, ok := i.(*DataInstance)
			if !ok || di.TypeName != "Identity" || len(di.Fields) != 1 {
				return newError("runIdentity expects an Identity")
			}
			return di.Fields[0]
		},
	})

	// State<S, A>
	// Type: State
	env.Set("State", &TypeObject{TypeVal: typesystem.TCon{Name: "State"}})
	// Constructor: state(fn)
	env.Set("state", &Constructor{Name: "state", TypeName: "State", Arity: 1})
	// runState(s, init)
	env.Set("runState", &Builtin{
		Name: "runState",
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 2 {
				return newError("runState expects 2 arguments")
			}
			s := args[0]
			init := args[1]
			di, ok := s.(*DataInstance)
			if !ok || di.TypeName != "State" || len(di.Fields) != 1 {
				return newError("runState expects a State, got %s (TypeName=%s)", s.Inspect(), getTypeName(s))
			}
			fn := di.Fields[0]
			return e.ApplyFunction(fn, []Object{init})
		},
	})
	// evalState(s, init) -> val
	env.Set("evalState", &Builtin{
		Name: "evalState",
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 2 {
				return newError("evalState expects 2 arguments")
			}
			s := args[0]
			init := args[1]
			di, ok := s.(*DataInstance)
			if !ok || di.TypeName != "State" || len(di.Fields) != 1 {
				return newError("evalState expects a State")
			}
			fn := di.Fields[0]
			res := e.ApplyFunction(fn, []Object{init})
			if isError(res) {
				return res
			}
			// Result is (val, state) tuple
			tuple, ok := res.(*Tuple)
			if !ok || len(tuple.Elements) != 2 {
				return newError("State function must return (value, state) tuple")
			}
			return tuple.Elements[0]
		},
	})
	// execState(s, init) -> state
	env.Set("execState", &Builtin{
		Name: "execState",
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 2 {
				return newError("execState expects 2 arguments")
			}
			s := args[0]
			init := args[1]
			di, ok := s.(*DataInstance)
			if !ok || di.TypeName != "State" || len(di.Fields) != 1 {
				return newError("execState expects a State")
			}
			fn := di.Fields[0]
			res := e.ApplyFunction(fn, []Object{init})
			if isError(res) {
				return res
			}
			// Result is (val, state) tuple
			tuple, ok := res.(*Tuple)
			if !ok || len(tuple.Elements) != 2 {
				return newError("State function must return (value, state) tuple")
			}
			return tuple.Elements[1]
		},
	})
	// sGet() -> State (\s -> (s, s))
	env.Set("sGet", &Builtin{
		Name: "sGet",
		Fn: func(e *Evaluator, args ...Object) Object {
			fn := &Builtin{
				Name: "get_state",
				Fn: func(ev *Evaluator, callArgs ...Object) Object {
					if len(callArgs) != 1 {
						return newError("State function expects 1 argument")
					}
					s := callArgs[0]
					return &Tuple{Elements: []Object{s, s}}
				},
			}
			return &DataInstance{Name: "state", TypeName: "State", Fields: []Object{fn}}
		},
	})
	// sPut(s) -> State (\_ -> ((), s))
	env.Set("sPut", &Builtin{
		Name: "sPut",
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 1 {
				return newError("sPut expects 1 argument")
			}
			newState := args[0]
			fn := &Builtin{
				Name: "put_state",
				Fn: func(ev *Evaluator, callArgs ...Object) Object {
					return &Tuple{Elements: []Object{&Nil{}, newState}}
				},
			}
			return &DataInstance{Name: "state", TypeName: "State", Fields: []Object{fn}}
		},
	})

	// Writer<W, A>
	// Type: Writer
	env.Set("Writer", &TypeObject{TypeVal: typesystem.TCon{Name: "Writer"}})
	// Constructor: writer(val, log)
	env.Set("writer", &Constructor{Name: "writer", TypeName: "Writer", Arity: 2})
	// runWriter(w) -> (val, log)
	env.Set("runWriter", &Builtin{
		Name: "runWriter",
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 1 {
				return newError("runWriter expects 1 argument")
			}
			w := args[0]
			di, ok := w.(*DataInstance)
			if !ok || di.TypeName != "Writer" || len(di.Fields) != 2 {
				return newError("runWriter expects a Writer")
			}
			val := di.Fields[0]
			log := di.Fields[1]
			return &Tuple{Elements: []Object{val, log}}
		},
	})
	// execWriter(w) -> log
	env.Set("execWriter", &Builtin{
		Name: "execWriter",
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 1 {
				return newError("execWriter expects 1 argument")
			}
			w := args[0]
			di, ok := w.(*DataInstance)
			if !ok || di.TypeName != "Writer" || len(di.Fields) != 2 {
				return newError("execWriter expects a Writer")
			}
			return di.Fields[1]
		},
	})
	// wTell(log) -> Writer((), log)
	env.Set("wTell", &Builtin{
		Name: "wTell",
		Fn: func(e *Evaluator, args ...Object) Object {
			if len(args) != 1 {
				return newError("wTell expects 1 argument")
			}
			log := args[0]
			return &DataInstance{
				Name:     "writer",
				TypeName: "Writer",
				Fields:   []Object{&Nil{}, log},
			}
		},
	})

	// Register all builtin functions from the Builtins map
	for name, builtin := range Builtins {
		env.Set(name, builtin)
	}

	// Register Option/Result helpers in prelude (tree-walk runtime)
	for name, builtin := range OptionBuiltins() {
		env.Set(name, builtin)
	}
	for name, builtin := range ResultBuiltins() {
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

// isZeroValue is a helper function to check if an object represents a "zero" or "empty" value
// This is used for Option and Result types, and also for general truthiness checks.
func isZeroValue(obj Object) bool {
	if obj == nil {
		return true
	}

	switch obj := obj.(type) {
	case *Boolean:
		return !obj.Value
	case *Integer:
		return obj.Value == 0
	case *Float:
		return obj.Value == 0.0
	case *List:
		return obj.Len() == 0
	case *Map:
		return obj.Len() == 0
	case *DataInstance:
		// For ADTs like Option and Result, check if it's the "empty" constructor
		return obj.Name == config.NoneCtorName || obj.Name == config.FailCtorName
	case *Nil:
		return true
	case *BigInt:
		return obj.Value.Cmp(big.NewInt(0)) == 0
	case *Rational:
		return obj.Value.Cmp(big.NewRat(0, 1)) == 0
	case *Bytes:
		return obj.Len() == 0
	case *Bits:
		return obj.Len() == 0
	case *Char:
		return obj.Value == 0
	default:
		return false
	}
}

// makeNone creates an Option None (None) value
func makeNone() Object {
	return &DataInstance{
		Name:     config.NoneCtorName,
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
// Returns Some(value) on success, None on failure
func parseStringToType(s string, t typesystem.Type) Object {
	switch ty := t.(type) {
	case typesystem.TCon:
		switch ty.Name {
		case "Int":
			// Strict parsing - must be a valid integer with no extra characters
			val, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
			if err != nil {
				return makeNone()
			}
			return makeSome(&Integer{Value: val})
		case "Float":
			// Strict parsing
			val, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
			if err != nil {
				return makeNone()
			}
			return makeSome(&Float{Value: val})
		case "Bool":
			switch strings.TrimSpace(s) {
			case "true":
				return makeSome(&Boolean{Value: true})
			case "false":
				return makeSome(&Boolean{Value: false})
			default:
				return makeNone()
			}
		case "BigInt":
			val := new(big.Int)
			_, ok := val.SetString(strings.TrimSpace(s), 10)
			if !ok {
				return makeNone()
			}
			return makeSome(&BigInt{Value: val})
		case "Rational":
			val := new(big.Rat)
			_, ok := val.SetString(strings.TrimSpace(s))
			if !ok {
				return makeNone()
			}
			return makeSome(&Rational{Value: val})
		case "Char":
			runes := []rune(s)
			if len(runes) != 1 {
				return makeNone()
			}
			return makeSome(&Char{Value: int64(runes[0])})
		default:
			return makeNone()
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
		return makeNone()
	default:
		return makeNone()
	}
}

// OptionT constructor
func builtinOptionT(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("optionT expects 1 argument")
	}
	return &DataInstance{Name: "OptionT", TypeName: "OptionT", Fields: []Object{args[0]}}
}

// ResultT constructor
func builtinResultT(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("resultT expects 1 argument")
	}
	return &DataInstance{Name: "ResultT", TypeName: "ResultT", Fields: []Object{args[0]}}
}

// runOptionT
func builtinRunOptionT(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("runOptionT expects 1 argument")
	}
	if di, ok := args[0].(*DataInstance); ok && di.TypeName == "OptionT" && len(di.Fields) == 1 {
		return di.Fields[0]
	}
	return newError("runOptionT expects OptionT")
}

// runResultT
func builtinRunResultT(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("runResultT expects 1 argument")
	}
	if di, ok := args[0].(*DataInstance); ok {
		if di.TypeName == "ResultT" && len(di.Fields) == 1 {
			return di.Fields[0]
		}
		// If check failed, inspect why
		return newError("runResultT expects ResultT. Got: Name=%s, TypeName=%s, Fields=%d", di.Name, di.TypeName, len(di.Fields))
	}
	return newError("runResultT expects ResultT. Got: %s (Type: %s)", args[0].Inspect(), args[0].Type())
}
