package evaluator

import (
	"fmt"
	"os"
	"github.com/funvibe/funxy/internal/typesystem"

	"gopkg.in/yaml.v3"
)

// YAML encoding/decoding functions for lib/yaml

// yamlDecode parses a YAML string into Funxy values.
// Maps become Records, sequences become Lists, scalars become
// Int/Float/Bool/String/Nil as appropriate.
func yamlDecode(content string, e *Evaluator) (Object, error) {
	var data interface{}
	if err := yaml.Unmarshal([]byte(content), &data); err != nil {
		return nil, fmt.Errorf("YAML parse error: %v", err)
	}

	result, err := inferFromYaml(data)
	if err != nil {
		return nil, err
	}

	return makeOk(result), nil
}

// inferFromYaml converts Go values (from yaml.Unmarshal) to Funxy Objects.
// Unlike inferFromJson, this handles int (yaml.v3 returns int for integers,
// not float64 like encoding/json).
func inferFromYaml(data interface{}) (Object, error) {
	switch v := data.(type) {
	case nil:
		return &Nil{}, nil
	case bool:
		return &Boolean{Value: v}, nil
	case int:
		return &Integer{Value: int64(v)}, nil
	case int64:
		return &Integer{Value: v}, nil
	case float64:
		if v == float64(int64(v)) {
			return &Integer{Value: int64(v)}, nil
		}
		return &Float{Value: v}, nil
	case string:
		return stringToListJson(v), nil
	case []interface{}:
		elements := make([]Object, len(v))
		for i, item := range v {
			obj, err := inferFromYaml(item)
			if err != nil {
				return nil, err
			}
			elements[i] = obj
		}
		return newList(elements), nil
	case map[string]interface{}:
		fields := make(map[string]Object)
		for k, val := range v {
			obj, err := inferFromYaml(val)
			if err != nil {
				return nil, err
			}
			fields[k] = obj
		}
		return NewRecord(fields), nil
	case map[interface{}]interface{}:
		fields := make(map[string]Object)
		for k, val := range v {
			obj, err := inferFromYaml(val)
			if err != nil {
				return nil, err
			}
			fields[fmt.Sprintf("%v", k)] = obj
		}
		return NewRecord(fields), nil
	default:
		return nil, fmt.Errorf("unsupported YAML value type: %T", data)
	}
}

// yamlEncode converts a Funxy value to a YAML string.
func yamlEncode(obj Object) (string, error) {
	value, err := objectToGo(obj)
	if err != nil {
		return "", err
	}

	bytes, err := yaml.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("YAML encoding error: %v", err)
	}

	return string(bytes), nil
}

// yamlRead reads and parses a YAML file.
func yamlRead(path string, e *Evaluator) (Object, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return makeFailStr(fmt.Sprintf("Cannot read file: %v", err)), nil
	}

	return yamlDecode(string(content), e)
}

// yamlWrite writes a Funxy value to a YAML file.
func yamlWrite(path string, obj Object) (Object, error) {
	content, err := yamlEncode(obj)
	if err != nil {
		return makeFailStr(err.Error()), nil
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return makeFailStr(fmt.Sprintf("Cannot write file: %v", err)), nil
	}

	return makeOk(&Nil{}), nil
}

// YamlBuiltins returns built-in functions for lib/yaml virtual package
func YamlBuiltins() map[string]*Builtin {
	return map[string]*Builtin{
		"yamlDecode": {Name: "yamlDecode", Fn: builtinYamlDecode},
		"yamlEncode": {Name: "yamlEncode", Fn: builtinYamlEncode},
		"yamlRead":   {Name: "yamlRead", Fn: builtinYamlRead},
		"yamlWrite":  {Name: "yamlWrite", Fn: builtinYamlWrite},
	}
}

// SetYamlBuiltinTypes sets type info for YAML builtins
func SetYamlBuiltinTypes(builtins map[string]*Builtin) {
	stringType := typesystem.TApp{
		Constructor: typesystem.TCon{Name: "List"},
		Args:        []typesystem.Type{typesystem.Char},
	}
	resultType := func(t typesystem.Type) typesystem.Type {
		return typesystem.TApp{
			Constructor: typesystem.TCon{Name: "Result"},
			Args:        []typesystem.Type{stringType, t},
		}
	}
	tVar := typesystem.TVar{Name: "T"}

	types := map[string]typesystem.Type{
		// yamlDecode(str: String) -> Result<String, T>
		"yamlDecode": typesystem.TFunc{
			Params:     []typesystem.Type{stringType},
			ReturnType: resultType(tVar),
		},
		// yamlEncode(value) -> String
		"yamlEncode": typesystem.TFunc{
			Params:     []typesystem.Type{typesystem.TVar{Name: "A"}},
			ReturnType: stringType,
		},
		// yamlRead(path: String) -> Result<String, T>
		"yamlRead": typesystem.TFunc{
			Params:     []typesystem.Type{stringType},
			ReturnType: resultType(tVar),
		},
		// yamlWrite(path: String, value) -> Result<String, Nil>
		"yamlWrite": typesystem.TFunc{
			Params:     []typesystem.Type{stringType, typesystem.TVar{Name: "A"}},
			ReturnType: resultType(typesystem.Nil),
		},
	}

	for name, typ := range types {
		if b, ok := builtins[name]; ok {
			b.TypeInfo = typ
		}
	}
}

// Builtin function implementations

func builtinYamlDecode(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("yamlDecode(content: String)")
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("yamlDecode: argument must be String")
	}
	result, err := yamlDecode(listToString(list), e)
	if err != nil {
		return makeFailStr(err.Error())
	}
	return result
}

func builtinYamlEncode(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("yamlEncode(value)")
	}
	result, err := yamlEncode(args[0])
	if err != nil {
		return newError("yamlEncode: %s", err.Error())
	}
	return stringToList(result)
}

func builtinYamlRead(e *Evaluator, args ...Object) Object {
	if len(args) != 1 {
		return newError("yamlRead(path: String)")
	}
	list, ok := args[0].(*List)
	if !ok {
		return newError("yamlRead: argument must be String")
	}
	result, err := yamlRead(listToString(list), e)
	if err != nil {
		return makeFailStr(err.Error())
	}
	return result
}

func builtinYamlWrite(e *Evaluator, args ...Object) Object {
	if len(args) != 2 {
		return newError("yamlWrite(path: String, value)")
	}
	pathList, ok := args[0].(*List)
	if !ok {
		return newError("yamlWrite: first argument must be String")
	}
	result, err := yamlWrite(listToString(pathList), args[1])
	if err != nil {
		return makeFailStr(err.Error())
	}
	return result
}
