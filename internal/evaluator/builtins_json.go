package evaluator

import (
	"encoding/json"
	"fmt"
	"github.com/funvibe/funxy/internal/typesystem"
	"strings"
)

// JSON encoding/decoding functions for lib/json

// jsonEncode converts any Object to a JSON string
func jsonEncode(obj Object) (string, error) {
	value, err := objectToGo(obj)
	if err != nil {
		return "", err
	}

	bytes, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("JSON encoding error: %v", err)
	}

	return string(bytes), nil
}

// objectToGo converts an Object to a Go value suitable for json.Marshal
func objectToGo(obj Object) (interface{}, error) {
	switch v := obj.(type) {
	case *Integer:
		return v.Value, nil
	case *Float:
		return v.Value, nil
	case *Boolean:
		return v.Value, nil
	case *Nil:
		return nil, nil
	case *Char:
		return string(rune(v.Value)), nil
	case *List:
		// Check if it's a string (List<Char>) - but only if non-empty
		// Empty list should be encoded as [] not ""
		if v.len() > 0 && isStringListJson(v) {
			return listToStringJson(v), nil
		}
		// Regular list -> JSON array
		arr := make([]interface{}, v.len())
		for i, el := range v.ToSlice() {
			val, err := objectToGo(el)
			if err != nil {
				return nil, err
			}
			arr[i] = val
		}
		return arr, nil
	case *Tuple:
		// Tuple -> JSON array
		arr := make([]interface{}, len(v.Elements))
		for i, el := range v.Elements {
			val, err := objectToGo(el)
			if err != nil {
				return nil, err
			}
			arr[i] = val
		}
		return arr, nil
	case *RecordInstance:
		// Record -> JSON object
		obj := make(map[string]interface{})
		for _, f := range v.Fields {
			goVal, err := objectToGo(f.Value)
			if err != nil {
				return nil, err
			}
			obj[f.Key] = goVal
		}
		return obj, nil
	case *DataInstance:
		// Option: Some(x) -> x, None -> null
		if v.Name == "Some" && len(v.Fields) == 1 {
			return objectToGo(v.Fields[0])
		}
		if v.Name == "None" {
			return nil, nil
		}
		// Result: Ok(x) -> x, Fail(e) -> error
		if v.Name == "Ok" && len(v.Fields) == 1 {
			return objectToGo(v.Fields[0])
		}
		if v.Name == "Fail" {
			return nil, fmt.Errorf("cannot encode Fail to JSON")
		}
		// Other ADTs - encode as object with constructor name
		result := map[string]interface{}{
			"_type": v.Name,
		}
		if len(v.Fields) > 0 {
			fields := make([]interface{}, len(v.Fields))
			for i, f := range v.Fields {
				val, err := objectToGo(f)
				if err != nil {
					return nil, err
				}
				fields[i] = val
			}
			result["_fields"] = fields
		}
		return result, nil
	case *BigInt:
		// BigInt -> string (JSON numbers are limited)
		return v.Value.String(), nil
	case *Rational:
		// Rational -> string "num/den"
		return v.Value.RatString(), nil
	default:
		return nil, fmt.Errorf("cannot encode %s to JSON", obj.Type())
	}
}

// inferFromJson infers Object type from JSON value
func inferFromJson(data interface{}, e *Evaluator) (Object, error) {
	switch v := data.(type) {
	case nil:
		return &Nil{}, nil
	case bool:
		return &Boolean{Value: v}, nil
	case float64:
		// Check if it's an integer
		if v == float64(int64(v)) {
			return &Integer{Value: int64(v)}, nil
		}
		return &Float{Value: v}, nil
	case string:
		return stringToListJson(v), nil
	case []interface{}:
		elements := make([]Object, len(v))
		for i, item := range v {
			obj, err := inferFromJson(item, e)
			if err != nil {
				return nil, err
			}
			elements[i] = obj
		}
		return newList(elements), nil
	case map[string]interface{}:
		fields := make(map[string]Object)
		for k, val := range v {
			obj, err := inferFromJson(val, e)
			if err != nil {
				return nil, err
			}
			fields[k] = obj
		}
		return NewRecord(fields), nil
	default:
		return nil, fmt.Errorf("unknown JSON value type: %T", data)
	}
}

// isStringListJson checks if a list is a string (List<Char>)
func isStringListJson(l *List) bool {
	if l.ElementType == "Char" {
		return true
	}
	for _, el := range l.ToSlice() {
		if _, ok := el.(*Char); !ok {
			return false
		}
	}
	return true
}

// listToStringJson converts List<Char> to Go string
func listToStringJson(l *List) string {
	var sb strings.Builder
	for _, el := range l.ToSlice() {
		if c, ok := el.(*Char); ok {
			sb.WriteRune(rune(c.Value))
		}
	}
	return sb.String()
}

// stringToListJson converts Go string to List<Char>
func stringToListJson(s string) *List {
	runes := []rune(s)
	elements := make([]Object, len(runes))
	for i, r := range runes {
		elements[i] = &Char{Value: int64(r)}
	}
	return newListWithType(elements, "Char")
}

// makeOkJson creates Result.Ok
func makeOkJson(value Object) *DataInstance {
	return &DataInstance{Name: "Ok", TypeName: "Result", Fields: []Object{value}}
}

// RegisterJsonBuiltins registers JSON types and functions into an environment
func RegisterJsonBuiltins(env *Environment) {
	// Types
	env.Set("Json", &TypeObject{TypeVal: typesystem.TCon{Name: "Json"}})

	// Constructors
	env.Set("JNull", &DataInstance{Name: "JNull", Fields: []Object{}, TypeName: "Json"})
	env.Set("JBool", &Constructor{Name: "JBool", TypeName: "Json", Arity: 1})
	env.Set("JNum", &Constructor{Name: "JNum", TypeName: "Json", Arity: 1})
	env.Set("JStr", &Constructor{Name: "JStr", TypeName: "Json", Arity: 1})
	env.Set("JArr", &Constructor{Name: "JArr", TypeName: "Json", Arity: 1})
	env.Set("JObj", &Constructor{Name: "JObj", TypeName: "Json", Arity: 1})

	// Functions
	builtins := JsonBuiltins()
	SetJsonBuiltinTypes(builtins)
	for name, fn := range builtins {
		env.Set(name, fn)
	}
}

// JsonBuiltins returns built-in functions for lib/json virtual package
func JsonBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		"jsonEncode":    {Name: "jsonEncode", Fn: builtinEncode},
		"jsonDecode":    {Name: "jsonDecode", Fn: builtinDecode},
		"jsonParse":     {Name: "jsonParse", Fn: builtinParseJson},
		"jsonFromValue": {Name: "jsonFromValue", Fn: builtinToJson},
		"jsonGet":       {Name: "jsonGet", Fn: builtinJsonGet},
		"jsonKeys":      {Name: "jsonKeys", Fn: builtinJsonKeys},
	}
}

// SetJsonBuiltinTypes sets type info for JSON builtins
func SetJsonBuiltinTypes(builtins map[string]*Builtin) {
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}
	jsonType := typesystem.TCon{Name: "Json"}
	resultType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{typesystem.TVar{Name: "T"}, stringType},
	}
	resultJsonType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Result"},
		Args:        []typesystem.Type{jsonType, stringType},
	}
	optionJsonType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "Option"},
		Args:        []typesystem.Type{jsonType},
	}

	types := map[string]typesystem.Type{
		"jsonEncode": typesystem.TFunc{
			Params:     []typesystem.Type{typesystem.TVar{Name: "A"}},
			ReturnType: stringType,
		},
		"jsonDecode": typesystem.TFunc{
			Params:     []typesystem.Type{stringType},
			ReturnType: resultType,
		},
		"jsonParse": typesystem.TFunc{
			Params:     []typesystem.Type{stringType},
			ReturnType: resultJsonType,
		},
		"jsonFromValue": typesystem.TFunc{
			Params:     []typesystem.Type{typesystem.TVar{Name: "A"}},
			ReturnType: jsonType,
		},
		"jsonGet": typesystem.TFunc{
			Params:     []typesystem.Type{jsonType, stringType},
			ReturnType: optionJsonType,
		},
		"jsonKeys": typesystem.TFunc{
			Params:     []typesystem.Type{jsonType},
			ReturnType: typesystem.TApp{Constructor: typesystem.TCon{Name: "List"}, Args: []typesystem.Type{stringType}},
		},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}

func builtinEncode(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("encode requires exactly 1 argument")
	}

	result, err := jsonEncode(args[0])
	if err != nil {
		return makeFailStr(err.Error())
	}
	return stringToListJson(result)
}

func builtinDecode(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("decode requires exactly 1 argument")
	}

	// Get JSON string
	jsonStr := ""
	if list, ok := args[0].(*List); ok && isStringListJson(list) {
		jsonStr = listToStringJson(list)
	} else {
		return makeFailStr("decode argument must be a String")
	}

	// Parse JSON first
	data, parseErr := parseJsonValueWithError(jsonStr)
	if parseErr != nil {
		return makeFailStr(parseErr.Error())
	}

	// Infer types from JSON
	result, err := inferFromJson(data, e)
	if err != nil {
		return makeFailStr(err.Error())
	}

	return makeOkJson(result)
}

// parseJsonValueWithError parses JSON string to interface{} with error
func parseJsonValueWithError(jsonStr string) (interface{}, error) {
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("invalid JSON: %v", err)
	}
	return data, nil
}

// builtinParseJson parses JSON string into Json ADT
func builtinParseJson(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("parseJson requires exactly 1 argument")
	}

	jsonStr := ""
	if list, ok := args[0].(*List); ok && isStringListJson(list) {
		jsonStr = listToStringJson(list)
	} else {
		return makeFailStr("parseJson argument must be a String")
	}

	data, parseErr := parseJsonValueWithError(jsonStr)
	if parseErr != nil {
		return makeFailStr(parseErr.Error())
	}

	result := goToJsonADT(data)
	return makeOkJson(result)
}

// builtinToJson converts any value to Json ADT
func builtinToJson(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("toJson requires exactly 1 argument")
	}
	return objectToJsonADT(args[0])
}

// builtinJsonGet gets a field from JObj
func builtinJsonGet(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("jsonGet requires exactly 2 arguments")
	}

	json := args[0]
	keyList, ok := args[1].(*List)
	if !ok || !isStringListJson(keyList) {
		return newError("jsonGet second argument must be a String")
	}
	key := listToStringJson(keyList)

	// Check if json is JObj
	if di, ok := json.(*DataInstance); ok && di.Name == "JObj" {
		if len(di.Fields) > 0 {
			if pairList, ok := di.Fields[0].(*List); ok {
				for _, elem := range pairList.ToSlice() {
					if tuple, ok := elem.(*Tuple); ok && len(tuple.Elements) >= 2 {
						if keyStr, ok := tuple.Elements[0].(*List); ok && isStringListJson(keyStr) {
							if listToStringJson(keyStr) == key {
								return makeSome(tuple.Elements[1])
							}
						}
					}
				}
			}
		}
	}
	return makeNone()
}

// builtinJsonKeys gets all keys from JObj
func builtinJsonKeys(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("jsonKeys requires exactly 1 argument")
	}

	json := args[0]
	keys := []Object{}

	if di, ok := json.(*DataInstance); ok && di.Name == "JObj" {
		if len(di.Fields) > 0 {
			if pairList, ok := di.Fields[0].(*List); ok {
				for _, elem := range pairList.ToSlice() {
					if tuple, ok := elem.(*Tuple); ok && len(tuple.Elements) >= 2 {
						keys = append(keys, tuple.Elements[0])
					}
				}
			}
		}
	}

	return newList(keys)
}

// goToJsonADT converts Go interface{} to Json ADT
func goToJsonADT(data interface{}) Object {
	switch v := data.(type) {
	case nil:
		return &DataInstance{Name: "JNull", Fields: []Object{}, TypeName: "Json"}
	case bool:
		return &DataInstance{Name: "JBool", Fields: []Object{&Boolean{Value: v}}, TypeName: "Json"}
	case float64:
		return &DataInstance{Name: "JNum", Fields: []Object{&Float{Value: v}}, TypeName: "Json"}
	case string:
		return &DataInstance{Name: "JStr", Fields: []Object{stringToListJson(v)}, TypeName: "Json"}
	case []interface{}:
		elements := make([]Object, len(v))
		for i, item := range v {
			elements[i] = goToJsonADT(item)
		}
		arr := newList(elements)
		return &DataInstance{Name: "JArr", Fields: []Object{arr}, TypeName: "Json"}
	case map[string]interface{}:
		pairs := make([]Object, 0, len(v))
		for key, val := range v {
			pair := &Tuple{Elements: []Object{stringToListJson(key), goToJsonADT(val)}}
			pairs = append(pairs, pair)
		}
		pairList := newList(pairs)
		return &DataInstance{Name: "JObj", Fields: []Object{pairList}, TypeName: "Json"}
	default:
		return &DataInstance{Name: "JNull", Fields: []Object{}, TypeName: "Json"}
	}
}

// objectToJsonADT converts any Object to Json ADT
func objectToJsonADT(obj Object) Object {
	switch v := obj.(type) {
	case *Nil:
		return &DataInstance{Name: "JNull", Fields: []Object{}, TypeName: "Json"}
	case *Boolean:
		return &DataInstance{Name: "JBool", Fields: []Object{v}, TypeName: "Json"}
	case *Integer:
		return &DataInstance{Name: "JNum", Fields: []Object{&Float{Value: float64(v.Value)}}, TypeName: "Json"}
	case *Float:
		return &DataInstance{Name: "JNum", Fields: []Object{v}, TypeName: "Json"}
	case *Char:
		return &DataInstance{Name: "JStr", Fields: []Object{newListWithType([]Object{v}, "Char")}, TypeName: "Json"}
	case *List:
		if isStringListJson(v) {
			return &DataInstance{Name: "JStr", Fields: []Object{v}, TypeName: "Json"}
		}
		elements := make([]Object, v.len())
		for i, item := range v.ToSlice() {
			elements[i] = objectToJsonADT(item)
		}
		arr := newList(elements)
		return &DataInstance{Name: "JArr", Fields: []Object{arr}, TypeName: "Json"}
	case *Tuple:
		elements := make([]Object, len(v.Elements))
		for i, item := range v.Elements {
			elements[i] = objectToJsonADT(item)
		}
		arr := newList(elements)
		return &DataInstance{Name: "JArr", Fields: []Object{arr}, TypeName: "Json"}
	case *RecordInstance:
		pairs := make([]Object, 0, len(v.Fields))
		for _, f := range v.Fields {
			pair := &Tuple{Elements: []Object{stringToListJson(f.Key), objectToJsonADT(f.Value)}}
			pairs = append(pairs, pair)
		}
		pairList := newList(pairs)
		return &DataInstance{Name: "JObj", Fields: []Object{pairList}, TypeName: "Json"}
	case *DataInstance:
		// Option: Some -> value, None -> null
		if v.Name == "Some" && len(v.Fields) == 1 {
			return objectToJsonADT(v.Fields[0])
		}
		if v.Name == "None" || v.Name == "JNull" {
			return &DataInstance{Name: "JNull", Fields: []Object{}, TypeName: "Json"}
		}
		// Json types pass through
		if v.TypeName == "Json" {
			return v
		}
		// Other ADTs
		return &DataInstance{Name: "JNull", Fields: []Object{}, TypeName: "Json"}
	default:
		return &DataInstance{Name: "JNull", Fields: []Object{}, TypeName: "Json"}
	}
}
